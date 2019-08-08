package main

import (
	"encoding/json"
	"os"
	"time"
	"bufio"
	"math/rand"
	"github.com/op/go-logging"
	"gitlab.com/Habimm/tree-search-golang/config"
	"gitlab.com/Habimm/tree-search-golang/treesearch"
	"gitlab.com/Habimm/tree-search-golang/predictor"
	"gitlab.com/Habimm/tree-search-golang/record"
)

var (
    log = logging.MustGetLogger("actor")
)

func handleCommands() {
	scanner := bufio.NewScanner(os.Stdin)
	log.Debugf("Scanner begins scanning stdin")
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
		commandName := commandMap["command"]
		switch commandName {
		case "LoadModel":
			modelPath := commandMap["model_path"].(string)
			log.Debugf("Received command to load new model %s", modelPath)
			predictor.StopService()
			predictor.StartService(modelPath)
		default:
			log.Debugf("Received unknown command: %s", commandName)
		}
	}
	log.Debugf("Scanner reached end of file")
}

type Example struct {
	Observation [][][]float32
	Policy		[]float32
	Outcome 	float32
}

func SendExperience(experienceChan chan Example) {
	expPrefix := config.String["exp_prefix"]
	experiencePipe, err := os.Create(expPrefix)
	if err != nil {
		log.Panicf("Could not open the pipe to write the examples")
	}
	fileInfo, err := experiencePipe.Stat()
	if err != nil {
		log.Panicf("Cannot read exp file information")
	}
	if fileInfo.Mode().IsRegular() {
		log.Panicf("Opened exp file is regular, that is, not a named pipe")
	}

	experienceBytes := make([]byte, 0)
	isOpen := true
	for isOpen {
		// num_examples_per_file > 2*max_game_length
		oldModelPath := config.String["model_path"]
		experienceBytes = experienceBytes[:0]
		// collect examples from SelfPlay through the experience channel
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

		// write the collected experience bytes to the pipe for the trainer to receive
		nWritten, err := experiencePipe.Write(experienceBytes)
		if err != nil {
			log.Panicf("Could not write to experience pipe %v", experiencePipe)
		}
		if nWritten != len(experienceBytes) {
			log.Panicf("Only wrote %d out of %d bytes to experience pipe %v",
				nWritten, len(experienceBytes), experiencePipe)
		}

		newModelPath := config.String["model_path"]
		if oldModelPath == newModelPath {
			log.Warningf("Wrote experience to pipe %s using model %s",
				experiencePipe.Name(), oldModelPath)
		} else {
			log.Warningf("Wrote experience to pipe %s using models between %s and %s",
				experiencePipe.Name(), oldModelPath, newModelPath)
		}
	}
	experiencePipe.Close()
}

func SelfPlay(
	searcher *treesearch.Agent,
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
		var (
			actionIdx int
			policy []float32
		)
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
	outcome := searcher.Outcome()

	// queue game record for writing
	record.Outcome = outcome
	recordsChan<- record

	// queue experience for writing
	for t := len(examples)-1; t >= 0; t-- {
		outcome *= -1.0
		examples[t].Outcome = outcome
		experienceChan<- examples[t]
	}

	elapsed := time.Now().Sub(start)
	log.Infof("Performed a self-play of length %d in %v", gameLength, elapsed)
}

func main() {
	rand.Seed(int64(config.Int["random_seed"]))
	treesearch.ExtendConfig()

	// other flags: %{shortfile} %{color} %{color:reset}
	logFormat := logging.MustStringFormatter(`%{time:15:04:05.000000} %{shortfunc}() ▶ %{message}`)
	formattedBackend := logging.NewBackendFormatter(logging.NewLogBackend(os.Stderr, "", 0), logFormat)
	logging.SetBackend(formattedBackend)
	logging.SetLevel(logging.DEBUG, "actor")
	logging.SetLevel(logging.DEBUG, "predictor")
	logging.SetLevel(logging.INFO, "treesearch")
	logging.SetLevel(logging.ERROR, "record")
	logging.SetLevel(logging.ERROR, "gogame")

	predictor.StartService(config.String["model_path"])

	experienceChan := make(chan Example, config.Int["max_game_length"])
	go SendExperience(experienceChan)

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
	time.Sleep(1 * time.Second) // wait for SendExperience() to save some more examples
}
