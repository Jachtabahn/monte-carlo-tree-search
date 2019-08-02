package predictor

import (
	"gitlab.com/Habimm/tree-search-golang/config"
	"github.com/op/go-logging"
	tf "github.com/tensorflow/tensorflow/tensorflow/go"
	"time"
)

var log = logging.MustGetLogger("predictor")

type Request struct {
    Observation     [][][]float32
    ResultChan      chan Response
}

type Response struct {
    Policy  []float32
    Value   float32
}

func StartService(predictChan chan Request) {
    modelPath := "/go/src/gitlab.com/Habimm/tree-search-golang/uibam-10"
    // modelPath := "/root/tischler/main/out/models/uibam-10"
    model, err := tf.LoadSavedModel(modelPath, []string{"dimitri"}, nil)
    if err != nil {
        log.Panicf("Could not load model at %s", modelPath)
    }
    predictBatchSize := config.Int["predict_batch_size"]
    numTimeouts := 0
    numRequests := 0
    for i := 0; i < 1; i++ {
        go func() {
            requests := make([]Request, 1, predictBatchSize)
            for {
                timeout := time.After(1 * time.Millisecond)
                select {
                case request := <-predictChan:
                    requests[len(requests)-1] = request

                    if len(requests) < cap(requests) {
                        requests = requests[:len(requests)+1]
                    } else {
                        sendPredictions(requests, model)
                        requests = requests[:1]
                    }
                    numRequests++
                    log.Debugf("There has been %d prediction requests!", numRequests)
                case <-timeout:
                    if len(requests) > 1 {
                        requests = requests[:len(requests)-1]
                        sendPredictions(requests, model)
                        requests = requests[:1]
                    }
                    numTimeouts++
                    log.Infof("There has been %d timeouts at %d prediction requests",
                        numTimeouts, numRequests)
                }
            }
        }()
    }
}

func sendPredictions(requests []Request, model *tf.SavedModel) {
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
