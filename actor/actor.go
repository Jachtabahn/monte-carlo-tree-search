package main

import (
	"encoding/json"
	"os"
	"time"
	"math/rand"
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/Habimm/monte-carlo-tree-search/config"
	"github.com/Habimm/monte-carlo-tree-search/searcher"
	"github.com/Habimm/monte-carlo-tree-search/predictor"
	"github.com/op/go-logging"
)

var (
    log = logging.MustGetLogger("actor")
)

type Example struct {
	Observation [][][]float32
	Policy		[]float32
	Value 		float32
}

func SaveExperience(experienceChan chan Example) {
	experienceBytes := make([]byte, 0)
	isOpen := true
	for isOpen {
		// collect a configured number of examples from SelfPlay through the experience channel
		for i := 0; i < config.Int["num_examples_per_file"]; i++ {
			example, open := <-experienceChan
			if !open {
				isOpen = false
				break
			}
			exampleBytes, err := json.Marshal(example)
			if err != nil {
				log.Panicf("Could not json-encode the example %+v", example)
			}
			experienceBytes = append(experienceBytes, exampleBytes...)
			experienceBytes = append(experienceBytes, '\n')
		}
		if len(experienceBytes) == 0 { continue }

		// write the collected experience bytes to a fresh file with a random filename
		exFile, err := os.Create(fmt.Sprintf("exp/%s.ex", uuid.Must(uuid.NewV4())))
		if err != nil {
			log.Panicf("Could not create a file to write the examples:\n%v", experienceBytes)
		}
		nWritten, err := exFile.Write(experienceBytes)
		if err != nil {
			log.Panicf("Could not write to experience file %v", exFile)
		}
		if nWritten != len(experienceBytes) {
			log.Panicf("Only wrote %d out of %d bytes to experience file %v",
				nWritten, len(experienceBytes), exFile)
		}
		exFile.Close()
		experienceBytes = experienceBytes[:0]
	}
}

func SelfPlay(searcher *searcher.Searcher, experienceChan chan Example) {
	maxGameLength := config.Int["max_game_length"]
	explorationLength := config.Int["exploration_length"]
	examples := make([]Example, 0, maxGameLength)
	gameLength := 0
	start := time.Now()
	searcher.Reset()
	for !searcher.Finished() && gameLength < maxGameLength {
		searcher.Search()
		var actionIdx int
		var policy []float32
		if gameLength < explorationLength {
			actionIdx, policy = searcher.Explore()
		} else {
			actionIdx, policy = searcher.Exploit()
		}
		gameLength++
		example := Example{Observation: searcher.Observation(), Policy: policy}
		examples = append(examples, example)
		searcher.Step(actionIdx)
	}
	t := time.Now()
	elapsed := t.Sub(start)
	log.Infof("Performed a self-play of length %d in %v", gameLength, elapsed)

	value := searcher.Outcome()
	log.Debugf("Outcome of game with length %d is %.4f from the perspective of player %d",
		gameLength, value, searcher.Color())
	for t := len(examples)-1; t >= 0; t-- {
		value *= -1.0
		examples[t].Value = value
		log.Debugf("Following example gets value %.4f\n%v", value, examples[t])
		experienceChan<- examples[t]
	}
}

func main() {
	// prepare logging
	logFile, err := os.Create("actor.log")
	if err != nil {
	    panic("Could not create log file")
	}
	// other flags: %{shortfile} %{color} %{color:reset}
	logFormat := logging.MustStringFormatter(`%{time:15:04:05.000000} %{shortfunc}() ▶ %{message}`)
	formattedBackend := logging.NewBackendFormatter(logging.NewLogBackend(logFile, "", 0), logFormat)
	logging.SetBackend(formattedBackend)
	logging.SetLevel(logging.DEBUG, "actor")

	rand.Seed(int64(config.Int["random_seed"]))
	maxGameLength := config.Int["max_game_length"]
	experienceChan := make(chan Example, maxGameLength)
	go SaveExperience(experienceChan)

	predictChan := make(chan predictor.Request)
	predictor.StartService(predictChan)
	searcher.ExtendConfig()
	searcher := searcher.NewSearcher(predictChan)
	SelfPlay(searcher, experienceChan)

	close(experienceChan)
	time.Sleep(1 * time.Second) // wait for SaveExperience() to save some more examples
}
