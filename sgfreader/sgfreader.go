package main

import (
    "github.com/Habimm/monte-carlo-tree-search/config"
    "github.com/Habimm/monte-carlo-tree-search/gorules"
    "fmt"
    "regexp"
    "os"
)

func getColor(c byte) int {
	switch c {
	case 'W':
		return gorules.WHITE
	case 'B':
		return gorules.BLACK
	}
	panic(fmt.Sprintf("Unaccepted color describing byte %b", c))
}

// an SGF action consists of two alphabet letters <width><height>, where "aa" indicates the top-left corner
func sgfToAction(sgfAction string) int {
    aAscii := rune('a')
    width, height := rune(sgfAction[0]) - aAscii, rune(sgfAction[1]) - aAscii

    boardsize := config.Int("boardsize")
    action := int(height) * boardsize + int(width)
    return action
}

func ReadSgf() {
	re := regexp.MustCompile(`;[B,W]\[[a-z]{0,2}\]`)
	// re := regexp.MustCompile(`[B,W]\[[a-z][a-z]\]`)
	sgfFile, err := os.Open("test2.sgf")
	if err != nil {
		panic("Could not open SGF file")
	}
	sgfBytes := make([]byte, 1024)
	count, err := sgfFile.Read(sgfBytes)
	if err != nil {
		panic("Could not read from the SGF file")
	}
	sgfString := string(sgfBytes[:count])
	sgfActionBars := re.FindAllString(sgfString, -1)

	state := gorules.NewState()
    for _, sgfActionBar := range sgfActionBars {
    	// sanity check
    	color := getColor(sgfActionBar[1])
    	if color != state.Color() {
    		panic("The color in the SGF does not match the color in the state")
    	}

    	var action int
    	if len(sgfActionBar) <= 4 {
	        action = gorules.PASS
    	} else {
	        action = sgfToAction(sgfActionBar[3:5])
    	}

        // SGF tells me the real action but Step() takes an index to that action in the internal array of legal actions, so we have to search
        actionIndex := gorules.UNDEF
        for a, lAction := range state.LegalActions() {
        	if action == lAction {
        		actionIndex = a
        		break
        	}
        }
        if actionIndex == gorules.UNDEF {
        	panic(fmt.Sprintf("Could not find the action %d from the SGF among the legal actions", action))
        }
        state.Step(actionIndex)
    }
    outcome := state.Outcome()
    fmt.Printf("The game outcome is %.1f\n", outcome)
}

func main() {
    ReadSgf()
}
