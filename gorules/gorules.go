package gorules

import (
	"github.com/Habimm/monte-carlo-tree-search/config"
	"github.com/op/go-logging"
	"fmt"
)

const (
	// it is important that EMPTY gets the zero value, because a missing position in the board map indicates an empty position
	EMPTY = iota
	BLACK
	WHITE
	EMPTY_KNOWN
)

var (
	log = logging.MustGetLogger("gorules")
	PASS = config.Int("boardsize") * config.Int("boardsize")
	UNDEF = -1
)

type State struct {
	board 		 map[int]int

	/**
		differences is a ring storing the stones to add or remove to get the previous history_size-1 positions
		len(differences)-1 is the next index we are going to write to
		the diff needed to get the previous position is at len(differences)-2
		the diff needed to get the least recent position recorded is at len(differences)-1
	*/
	differences  []difference

	currentColor int
	legalActions []int
	lastPass	 bool
}

type difference struct {
	add map[int]int
	rem int
}

func NewState() *State {
	// initially, every action is legal
	boardsize := config.Int("boardsize")
	numActions := boardsize * boardsize + 1
	legalActions := make([]int, numActions)
	for a := 0; a < numActions; a++ {
		legalActions[a] = a
	}

	historySize := config.Int("history_size")
	differences := make([]difference, historySize-1)
	for i := range differences {
		differences[i].rem = UNDEF
	}
	differences = differences[:1]

	board := make(map[int]int)

	state := &State{board, differences, BLACK, legalActions, false}
	log.Debugf("Created new state:\n%+v", state)
	return state
}

func boardString(board map[int]int) (nice string) {
	boardsize := config.Int("boardsize")
	boardLength := boardsize * boardsize
	for field, column := 0, 0; field < boardLength; field++ {
		if board[field] == BLACK { nice += "X" }
		if board[field] == WHITE { nice += "O" }
		if board[field] == EMPTY { nice += "-" }

		if column == boardsize-1 {
			column = 0
			nice += "\n"
		} else {
			column++
			nice += " "
		}
	}
	return
}


func (state *State) applyDiff(board map[int]int, diffIndex int) {
	allDifferences := state.differences[:cap(state.differences)]
	index := len(state.differences)-2-diffIndex % len(allDifferences)
	if index < 0 { index += len(allDifferences) }
	diff := allDifferences[index]

	for pos, color := range diff.add {
		board[pos] = color
	}
	if diff.rem != UNDEF {
		delete(board, diff.rem)
	}
}

func (state *State) String() (nice string) {
	nice += fmt.Sprintf("Current position:\n")
	nice += boardString(state.board)

	// copy state.board to oldBoard
	oldBoard := make(map[int]int, len(state.board))
	for pos, color := range state.board {
		oldBoard[pos] = color
	}

	// display the past couple positions we have memorized
	// this code is so complicated because state.differences is a slice representing a ring buffer
	// whose beginning depends on the slice's length
	historySize := config.Int("history_size")
	for i := 0; i < historySize-1; i++ {
		state.applyDiff(oldBoard, i)
		nice += fmt.Sprintf("Position %d:\n", i+1)
		nice += boardString(oldBoard)
	}

	return fmt.Sprintf("%+v\n%s", *state, nice) // dereference state to avoid recursion
}

func (state *State) Step(legalAction int) {
	if legalAction >= len(state.legalActions) {
		log.Panicf(`Called Step() with invalid index %d, having
			only %d legal actions`, legalAction, len(state.legalActions))
	}

	action := state.legalActions[legalAction]
	isFinal := false
	diff := difference{add: make(map[int]int), rem: UNDEF}
	if action == PASS {
		log.Debugf("Taking the pass move")
		if state.lastPass {
			isFinal = true
		} else {
			state.lastPass = true
		}
	} else {
		log.Debugf("Taking action %d with color %d", action, state.currentColor)
		state.board[action] = state.currentColor

		// possibly remove stones of the other color
		otherColor := other(state.currentColor)
		neighbours := adjacentPositions(action)
		for _, neigh := range neighbours {
			if state.board[neigh] == otherColor {
				captured := state.capturedStones(neigh)

				// add the captured stones to the diff so we can recover the current position later
				// and remove the captured stones from the board
				for cap := range captured {
					diff.add[cap] = otherColor
					delete(state.board, cap)
				}
			}
		}

		// possibly remove my own stones
		captured := state.capturedStones(action)
		for cap := range captured {
			diff.add[cap] = state.currentColor
			delete(state.board, cap)
		}

		if state.board[action] == state.currentColor {
			// this is not necessarily true because this could have been a suicide move
			diff.rem = action
		}
	}

	state.currentColor = other(state.currentColor)

	// store the new difference object into our ring buffer and rotate the latter
	state.differences[len(state.differences)-1] = diff
	if len(state.differences) < cap(state.differences) {
	    state.differences = state.differences[:len(state.differences)+1]
	} else {
	    state.differences = state.differences[:1]
	}

	// take every empty intersection as a legal action to simplify computation
	state.legalActions = []int{}
	if !isFinal {
		boardsize := config.Int("boardsize")
		boardLength := boardsize * boardsize
		for action := 0; action < boardLength; action++ {
			if state.board[action] == EMPTY {
				state.legalActions = append(state.legalActions, action)
			}
		}
		state.legalActions = append(state.legalActions, PASS)
	}

	log.Debugf("Changed state:\n%+v", state)
}

func (state *State) capturedStones(startPosition int) (positions map[int]int) {
	const (
		NEW = iota
		OLD
	)
	positions = make(map[int]int)
	positions[startPosition] = NEW
	color := state.board[startPosition]
	addedNew := true
	for addedNew {
		addedNew = false
		for pos, flag := range positions {
			if flag == OLD { continue }
			neighbours := adjacentPositions(pos)
			for _, neigh := range neighbours {
				if state.board[neigh] == EMPTY {
					return map[int]int{} // we found a liberty of this chain, so no stone will be removed of this chain
				} else if state.board[neigh] == color {
					_, present := positions[neigh]
					if !present {
						positions[neigh] = NEW
						addedNew = true
					}
				}
			}
			positions[pos] = OLD
		}
	}
	return // we found no liberty, so remove the whole chain
}

func (state State) Observation() [][][]float32 {
	boardsize := 5
	num_channels := 9

	obs := make([][][]float32, boardsize)
	for height := 0; height < boardsize; height++ {
		width_channels := make([][]float32, boardsize)
		for width := 0; width < boardsize; width++ {
			width_channels[width] = make([]float32, num_channels)
		}
		obs[height] = width_channels
	}
	return obs
}

func (state *State) Color() int {
	return state.currentColor
}

func (state *State) LegalActions() []int {
	return state.legalActions
}

func (state *State) Final() bool {
	return len(state.legalActions) == 0
}

func (state *State) Outcome() float32 {
	// count black and white stones
	whiteScore, blackScore := float32(0.0), float32(0.0)
	for _, color := range state.board {
		switch color {
		case BLACK:
			blackScore++
		case WHITE:
			whiteScore++
		}
	}
	log.Debugf("Number of black stones: %.1f\n", blackScore)
	log.Debugf("Number of white stones: %.1f\n", whiteScore)
	log.Debugf("Total number of stones: %d\n", len(state.board))

	// initially, all empty fields are known, i.e. it is not determined which color they belong to, if any
	boardsize := config.Int("boardsize")
	boardLength := boardsize * boardsize
	unknownTerritory := make(map[int]struct{}, boardLength - len(state.board))
	for a := 0; a < boardLength; a++ {
		_, present := state.board[a]
		if !present {
			unknownTerritory[a] = struct{}{}
		}
	}

	// go through each unknown position and build its induced connected graph consisting only of empty fields
	for unknownPos := range unknownTerritory {
		log.Debugf("Popping unknown field %d", unknownPos)

		// this map contains the "outer shell" of the territory we are currently exploring
		newTerritory := make(map[int]struct{}, 1)
		newTerritory[unknownPos] = struct{}{}
		blackTerritory := false
		whiteTerritory := false
		count := 0
		for len(newTerritory) > 0 {
			count += len(newTerritory)
			for pos := range newTerritory {
				log.Debugf("Popping new field %d", pos)
				delete(newTerritory, pos)
				delete(unknownTerritory, pos)

				neighbours := adjacentPositions(pos)
				for _, neigh := range neighbours {
					switch state.board[neigh] {
					case EMPTY:
						_, present := unknownTerritory[neigh]
						if present {
							newTerritory[neigh] = struct{}{}
						}
					case BLACK:
						blackTerritory = true
						log.Debugf("The position %d is adjacent to black %d", pos, neigh)
					case WHITE:
						whiteTerritory = true
						log.Debugf("The position %d is adjacent to white %d", pos, neigh)
					}
				}
			}
		}

		if (blackTerritory && !whiteTerritory) {
			blackScore += float32(count)
			log.Debugf("The territory induced by %d, sized %d, goes to Black; thus black score is %.1f",
				unknownPos, count, blackScore)
		} else if (!blackTerritory && whiteTerritory) {
			whiteScore += float32(count)
			log.Debugf("The territory induced by %d, sized %d, goes to White; thus white score is %.1f",
				unknownPos, count, whiteScore)
		} else {
			log.Debugf("The territory induced by %d, sized %d, goes to noone", unknownPos, count)
		}
	}

	log.Debugf("Total black score is %.1f", blackScore)
	log.Debugf("Total white score before komi is %.1f", whiteScore)
	komi := config.Float("komi")
	whiteScore += komi
	log.Debugf("Total white score after komi is %.1f", whiteScore)

	switch state.currentColor {
	case BLACK:
		return blackScore - whiteScore
	case WHITE:
		return whiteScore - blackScore
	default:
		log.Panicf("Current color is invalid %d", state.currentColor)
		return float32(0.0) // required because the compiler treats log.Panicf() and panic() differently
	}
}

func other(color int) int {
	switch color {
	case BLACK:
		return WHITE
	case WHITE:
		return BLACK
	default:
		log.Panicf("Input %d not accepted (only %d and %d)", color, BLACK, WHITE)
		return UNDEF
	}
}

func adjacentPositions(pos int) (positions []int) {
	boardsize := config.Int("boardsize")
	if pos % boardsize != 0 {
		left := pos-1
		positions = append(positions, left)
	}

	right := pos+1
	if right % boardsize != 0 {
		positions = append(positions, right)
	}

	if pos >= boardsize {
		above := pos-boardsize
		positions = append(positions, above)
	}

	if pos < boardsize*boardsize - boardsize {
		below := pos+boardsize
		positions = append(positions, below)
	}
	return
}
