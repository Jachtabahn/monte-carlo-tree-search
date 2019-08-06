package main

import (
    "os"
    "fmt"
    "github.com/op/go-logging"
    "regexp"
)

var (
	log = logging.MustGetLogger("sgf")
    sgfRegex = regexp.MustCompile(`PB\[(.+)\]PW\[(.+)\]RE\[(.{1,3})\]`)
	blackTreeWins = 0
	blackTreeLosses = 0
	whiteTreeWins = 0
	whiteTreeLosses = 0
)

const (
	treeName = "Tree search agent"
	randomName = "Random player"
	BLACK = 1
	WHITE = 2
	RESULT = 3
)


func EvalSgf(sgfBytes []byte, filename string) {
    sgfFile, err := os.Open(filename)
    if err != nil {
        log.Panicf("Could not open SGF file %s", filename)
    }
    count, err := sgfFile.Read(sgfBytes)
    if err != nil {
        log.Panicf("Could not read from the SGF file")
    }
    sgfBytes = sgfBytes[:count]

    sgfInfo := sgfRegex.FindSubmatch(sgfBytes)
    if len(sgfInfo[RESULT]) == 3 {
	    if sgfInfo[RESULT][0] == 'B' && sgfInfo[RESULT][1] == '+' ||
		    sgfInfo[RESULT][0] == 'W' && sgfInfo[RESULT][1] == '-' {
		    // Black has won
		    switch string(sgfInfo[BLACK]) {
		    case treeName:
		    	blackTreeWins++
		    case randomName:
		    	whiteTreeLosses++
		    }
	    } else {
		    // White has won
		    switch string(sgfInfo[WHITE]) {
		    case treeName:
		    	whiteTreeWins++
		    case randomName:
		    	blackTreeLosses++
		    }
	    }
    } else {
    	log.Panicf("Not implemented reading this type of result")
    }
}

func main() {
	sgfDir, err := os.Open("../eval/sgf")
	if err != nil {
		log.Panicf("Could not open the directory with the SGF files")
	}

	sgfNames, err := sgfDir.Readdirnames(0)
	if err != nil {
		log.Panicf("Could not read the directory with the SGF files")
	}

	MAX_SGF_SIZE := 2048
	sgfBytes := make([]byte, MAX_SGF_SIZE)
	for _, filename := range sgfNames {
		sgfBytes = sgfBytes[:MAX_SGF_SIZE]
		EvalSgf(sgfBytes, fmt.Sprintf("../eval/sgf/%s", filename))
	}

	log.Infof("blackTreeWins: %d", blackTreeWins)
	log.Infof("blackTreeLosses: %d", blackTreeLosses)
	log.Infof("whiteTreeWins: %d", whiteTreeWins)
	log.Infof("whiteTreeLosses: %d", whiteTreeLosses)
}
