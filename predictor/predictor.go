package predictor

import (
	"gitlab.com/Habimm/tree-search-golang/config"
	"github.com/op/go-logging"
	tf "github.com/tensorflow/tensorflow/tensorflow/go"
	"time"
)

var (
    RequestsChannel = make(chan Request)
    serviceStop = make(chan int)
    serviceDown = make(chan int)
    log = logging.MustGetLogger("predictor")
)

type Request struct {
    Observation     [][][]float32
    ResultChan      chan Response
}

type Response struct {
    Policy  []float32
    Value   float32
}

func StopService() {
    serviceStop<- 1
    <-serviceDown
}

func StartService(modelPath string) {
    model, err := tf.LoadSavedModel(config.String["model_path"], []string{"dimitri"}, nil)
    if err != nil {
        log.Panicf("Could not load model at %s", modelPath)
    }
    go handlePredictionRequests(model)
}

func handlePredictionRequests(model *tf.SavedModel) {
    predictBatchSize := config.Int["predict_batch_size"]
    requests := make([]Request, 1, predictBatchSize)
    for {
        timeout := time.After(1 * time.Millisecond)
        select {
        case request := <-RequestsChannel:
            requests[len(requests)-1] = request

            if len(requests) < cap(requests) {
                requests = requests[:len(requests)+1]
            } else {
                computePredictions(requests, model)
                requests = requests[:1]
            }
        case <-timeout:
            if len(requests) > 1 {
                computePredictions(requests[:len(requests)-1], model)
                requests = requests[:1]
            }
        case <-serviceStop:
            if len(requests) > 1 {
                computePredictions(requests[:len(requests)-1], model)
                requests = requests[:1]
            }
            log.Infof("Service shutting down")
            serviceDown<- 1
            return
        }
    }
}

func computePredictions(requests []Request, model *tf.SavedModel) {
    batchSize := len(requests)
    batch := make([][][][]float32, batchSize)
    for b := 0; b < batchSize; b++ {
        batch[b] = requests[b].Observation
    }

    input, err := tf.NewTensor(batch)
    if err != nil {
        log.Panicf("Could not create tensor from batch with error: %s", err.Error())
    }

    graph := model.Graph
    inputs := map[tf.Output]*tf.Tensor{tf.Output{graph.Operation("observation_Input"), 0}: input}
    outputs := []tf.Output{
        tf.Output{graph.Operation("policy_head/MatMul"), 0},
        tf.Output{graph.Operation("value_head/Tanh"), 0}}
    prediction_arrays, err := model.Session.Run(inputs, outputs, nil)
    if err != nil {
        log.Panicf("Could not run the model session with error: %s", err.Error())
    }

    policies, ok := prediction_arrays[0].Value().([][]float32)
    if !ok {
        log.Panicf("Policy has a wrong type")
    }
    values, ok := prediction_arrays[1].Value().([][]float32)
    if !ok {
        log.Panicf("Value has a wrong type")
    }

    for b := 0; b < batchSize; b++ {
        requests[b].ResultChan <- Response{policies[b], values[b][0]}
    }
}
