package main

import (
    "time"
    "math"
    "github.com/Habimm/monte-carlo-tree-search/gorules"
    tf "github.com/tensorflow/tensorflow/tensorflow/go"
    "github.com/op/go-logging"
    "os"
)

var log = logging.MustGetLogger("searcher")

func trace(s string) string {
    log.Debugf("Enter %s()\n", s)
    return s
}

func un(s string) {
    log.Debugf("Leave %s()\n", s)
}

type PredictionRequest struct {
    observation     [][][]float32
    resultChan      chan Prediction
}

type Node struct {
    state           gorules.State
    values          []float32
    counts          []int
    totalCount      int
    policy          []float32
    parent          *Node
    parentAction    int
    children        []*Node
}

func NewNode(predictChan chan PredictionRequest) (newNode *Node) {
    defer un(trace("NewNode"))
    newState := gorules.New()
    newNode, _ = constructNewNode(newState, nil, -1, predictChan)
    return
}

func (node *Node) NewNode(action int, predictChan chan PredictionRequest) (newNode *Node, value float32) {
    newState := node.state.Step(action)
    return constructNewNode(newState, node, action, predictChan)
}

func (node *Node) Outcome() float32 {
    return node.state.Outcome()
}

func constructNewNode(state gorules.State, parentNode *Node, parentAction int, predictChan chan PredictionRequest) (newNode *Node, value float32) {
    defer un(trace("constructNewNode"))
    var policy []float32
    if state.Final() {
        value = state.Outcome()
    } else {
        prediction := RequestPrediction(state.Observation(), predictChan)
        policy, value = prediction.policy, prediction.value
    }
    numLegalActions := len(state.LegalActions())
    newNode = &Node{
        state: state,
        values: make([]float32, numLegalActions),
        counts: make([]int, numLegalActions),
        totalCount: 1,
        policy: policy,
        parent: parentNode,
        parentAction: parentAction,
        children: make([]*Node, numLegalActions)}
    return
}

func (node *Node) Final() bool {
    return node.state.Final()
}

func (node *Node) Update(action int, value float32) {
    node.counts[action] ++
    node.totalCount     ++
    node.values[action] += (value - node.values[action]) / float32(node.counts[action])
}

func (node *Node) Select(cPuct float32) (maxAction int) {
    numLegalActions := len(node.state.LegalActions())
    maxAction = -1
    maxScore := float32(math.Inf(-1))
    for action := 0; action < numLegalActions; action++ {
        score := node.values[action] + cPuct * node.policy[action] * float32(math.Sqrt(float64(node.totalCount))) / float32(1 + node.counts[action])
        if score > maxScore {
            maxAction = action
            maxScore = score
        }
    }
    if maxAction == -1 {
        log.Panicf("There is no maximal action")
    }
    return
}

type Prediction struct {
    policy  []float32
    value   float32
}

func RequestPrediction(observation [][][]float32, predictChan chan PredictionRequest) (prediction Prediction) {
    request := PredictionRequest{observation, make(chan Prediction)}
    predictChan <- request
    prediction = <-request.resultChan
    return
}

type Searcher struct {
    root            *Node
    predictChan     chan PredictionRequest
    cPuct           float32
}

func (searcher *Searcher) Reset() {
    defer un(trace("Reset"))
    searcher.root = NewNode(searcher.predictChan)
}

func NewSearcher() (searcher Searcher) {
    searcher.predictChan = make(chan PredictionRequest)
    searcher.cPuct = 2.0
    return
}

var sem = make(chan int)
func (searcher *Searcher) Simulate() {
    for {
        curNode := searcher.root
        var lastNode *Node
        var lastAction int
        for curNode != nil && !curNode.Final() {
            lastAction = curNode.Select(searcher.cPuct)
            lastNode = curNode
            curNode = curNode.children[lastAction]
        }

        var value float32
        if curNode != nil {
            value = curNode.Outcome()
        } else {
            if lastNode == nil {
                log.Panicf("The root node is nil")
            }
            lastNode.children[lastAction], value = lastNode.NewNode(lastAction, searcher.predictChan)
            curNode = lastNode.children[lastAction]
        }

        for curNode.parent != nil {
            action := curNode.parentAction
            curNode = curNode.parent
            curNode.Update(action, value)
        }
        sem <- 1
    }
}

func (searcher *Searcher) startSearching() {
    for i := 0; i < 666; i++ {
        go searcher.Simulate()
    }
}



func (searcher *Searcher) startPredicting() {
    modelPath := "/home/tischler/Software/tischler/main/out/models/uibam-10"
    // modelPath := "/root/tischler/main/out/models/uibam-10"
    model, err := tf.LoadSavedModel(modelPath, []string{"dimitri"}, nil)
    if err != nil {
        log.Panicf("Could not load model at %s", modelPath)
    }
    predict_batch_size := 666
    for i := 0; i < 1; i++ {
        go func() {
            requests := make([]PredictionRequest, predict_batch_size)
            // requests := make([]PredictionRequest, 1, predict_batch_size)
            requestIndex := 0
            for {
                timeout := time.After(1 * time.Millisecond)
                select {
                case request := <-searcher.predictChan:
                    requests[requestIndex] = request
                    requestIndex++

                    if requestIndex >= predict_batch_size {
                        SendPredictions(&requests, requestIndex, model)
                        requestIndex = 0
                    }
                case <-timeout:
                    if requestIndex > 0 {
                        SendPredictions(&requests, requestIndex, model)
                        requestIndex = 0
                    }
                }
            }
        }()
    }
}

func SendPredictions(requests *[]PredictionRequest, batchSize int, model *tf.SavedModel) {
    batch := make([][][][]float32, batchSize)
    for b := 0; b < batchSize; b++ {
        batch[b] = (*requests)[b].observation
    }

    policies, values := Predict(batch, model)

    for b := 0; b < batchSize; b++ {
        (*requests)[b].resultChan <- Prediction{policies[b], values[b][0]}
    }
}

func Predict(batch [][][][]float32, model *tf.SavedModel) ([][]float32, [][]float32) {
    log.Infof("Processing batch: %+v\n", batch)

    input, err := tf.NewTensor(batch)
    if err != nil {
        panic(err)
    }

    graph := model.Graph
    inputs := map[tf.Output]*tf.Tensor{tf.Output{graph.Operation("observation_Input"), 0}: input}
    outputs := []tf.Output{
        tf.Output{graph.Operation("policy_head/MatMul"), 0},
        tf.Output{graph.Operation("value_head/Tanh"), 0}}
    log.Infof("Before Run\n")
    prediction_arrays, err := model.Session.Run(inputs, outputs, nil)
    log.Infof("After Run\n")
    if err != nil {
        panic(err)
    }

    policies, ok := prediction_arrays[0].Value().([][]float32)
    if !ok {
        panic("Policy has a wrong type")
    }
    values, ok := prediction_arrays[1].Value().([][]float32)
    if !ok {
        panic("Value has a wrong type")
    }

    log.Errorf("Output: %+v\n%+v\n", policies, values)
    return policies, values
}

func main() {
    // prepare logging
    logFile, err := os.Create("searcher.log")
    if err != nil {
        panic("Could not create the log file")
    }
    logFormat := logging.MustStringFormatter(`%{color}%{time:15:04:05.000000} %{callpath} ▶ %{color:reset}%{message}`)
    formattedBackend := logging.NewBackendFormatter(logging.NewLogBackend(logFile, "", 0), logFormat)
    logging.SetLevel(logging.ERROR, "searcher")
    logging.SetBackend(formattedBackend)

    searcher := NewSearcher()
    searcher.startPredicting()
    searcher.Reset()
    searcher.startSearching()

    // Wait for a number of simulations to finish
    sims := 6666
    for i := 0; i < sims; i++ {
        <-sem
    }
    log.Debugf("%+v\n", searcher.root)
}
