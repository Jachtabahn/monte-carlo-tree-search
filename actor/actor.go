package main

import (
	"encoding/json"
	"os"
	"time"
	"bufio"
	"math/rand"
	"fmt"
	"github.com/satori/go.uuid"
	"gitlab.com/Habimm/tree-search-golang/config"
	"gitlab.com/Habimm/tree-search-golang/treesearch"
	"gitlab.com/Habimm/tree-search-golang/predictor"
	"gitlab.com/Habimm/tree-search-golang/gogame"
	"github.com/op/go-logging"
)

var (
    log = logging.MustGetLogger("actor")
)

func handleCommands() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		commandBytes := scanner.Bytes()
		log.Debugf("Received command string: %s", commandBytes)

		var commandJson interface{}
		err := json.Unmarshal(commandBytes, &commandJson)
		if err != nil {
			log.Errorf("Ignoring following command that is not valid JSON: %s", commandBytes)
			continue
		}
		log.Debugf("Parsed command json: %s", commandJson)

		commandMap := commandJson.(map[string]interface{})
		commandName := commandMap["name"]
		switch commandName {
		case "LoadModel":
			modelPath := commandMap["model_path"]
			config.String["model_path"] = modelPath.(string)
			log.Debugf("Loading new model %s", modelPath)
			predictor.StopService()
			predictor.StartService()
		case "ChangeFolder":
			folder := commandMap["folder"]
			log.Debugf("Changing folder to %s", folder)
		default:
			log.Debugf("Unknown command: %s", commandName)
		}
	}
}

type Example struct {
	Observation [][][]float32
	Policy		[]float32
	Outcome 	float32
}

func SaveExperience(experienceChan chan Example) {
	experienceBytes := make([]byte, 0)
	isOpen := true
	expPrefix := config.String["exp_prefix"]
	for isOpen {
		experienceBytes = experienceBytes[:0]
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
		exFile, err := os.Create(fmt.Sprintf(
			"%s%c%s.ex", expPrefix,
			os.PathSeparator, uuid.Must(uuid.NewV4())))
		if err != nil {
			log.Panicf("Could not create a file to write the examples:\n%v", experienceBytes)
		}
		nWritten, err := exFile.Write(experienceBytes)
		if err != nil {
			log.Panicf("Could not write to experience file %v", exFile)
		}
		if nWritten != len(experienceBytes) {
			log.Panicf("Only wrote %d out of %d bytes to experience file %v (which may leave the experience file inconsistent)",
				nWritten, len(experienceBytes), exFile)
		}
		exFile.Close()
	}
}

func SelfPlay(
	searcher *treesearch.Searcher,
	experienceChan chan Example,
	recordsChan chan *record.Info) {
	maxGameLength := config.Int["max_game_length"]
	explorationLength := config.Int["exploration_length"]
	examples := make([]Example, 0, maxGameLength)
	gameLength := 0
	start := time.Now()
	searcher.Reset()
	record := &record.Info{
		InitialColor: searcher.Color(),
		Actions: make([]int, 0),
		BlackName: searcher.Name(),
		WhiteName: searcher.Name()}
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
		action := searcher.FavourableLegalActions()[actionIdx]
		record.Actions = append(record.Actions, action)
		log.Infof("Taking action %d", action)
		example := Example{Observation: searcher.Observation(), Policy: policy}
		examples = append(examples, example)
		searcher.Step(actionIdx)
	}
	t := time.Now()
	elapsed := t.Sub(start)
	log.Infof("Performed a self-play of length %d in %v", gameLength, elapsed)

	outcome := searcher.Outcome()
	record.Outcome = outcome
	recordsChan<- record

	for t := len(examples)-1; t >= 0; t-- {
		outcome *= -1.0
		examples[t].Outcome = outcome
		experienceChan<- examples[t]
	}
}

func main() {
	rand.Seed(int64(config.Int["random_seed"]))
	treesearch.ExtendConfig()

	// other flags: %{shortfile} %{color} %{color:reset}
	logFormat := logging.MustStringFormatter(`%{time:15:04:05.000000} %{shortfunc}() ▶ %{message}`)
	formattedBackend := logging.NewBackendFormatter(logging.NewLogBackend(os.Stderr, "", 0), logFormat)
	logging.SetBackend(formattedBackend)
	logging.SetLevel(logging.INFO, "actor")
	logging.SetLevel(logging.INFO, "predictor")
	logging.SetLevel(logging.INFO, "treesearch")
	logging.SetLevel(logging.INFO, "gogame")

	predictor.StartService()

	experienceChan := make(chan Example, config.Int["max_game_length"])
	go SaveExperience(experienceChan)

	recordsChan := make(chan *record.Info, 1)
	go record.Save(recordsChan)

	go handleCommands()
	searcher := treesearch.New(predictor.RequestsChannel)
	for i := 0; ; i++ {
		SelfPlay(searcher, experienceChan, recordsChan)
		log.Infof("Played game %d", i)
	}

	close(experienceChan)
	close(recordsChan)
	time.Sleep(1 * time.Second) // wait for SaveExperience() to save some more examples
}
