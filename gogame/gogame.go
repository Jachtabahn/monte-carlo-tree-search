package gogame

import (
	"gitlab.com/Habimm/tree-search-golang/config"
	"github.com/op/go-logging"
	"fmt"
	"regexp"
	"os"
)

const (
	// it is important that EMPTY gets the zero value, because a missing position in the board map indicates an empty position
	EMPTY = iota
	BLACK
	WHITE
)

var (
	log = logging.MustGetLogger("gogame")
	PASS = config.Int["boardsize"] * config.Int["boardsize"]
	UNDEF = -1
)

type Game struct {
	board 		 map[int]int

	/**
		differences is a ring storing the stones to add or remove to get the previous history_size-1 positions
		len(differences)-1 is the next index we are going to write to
		the diff needed to get the previous position is at len(differences)-2
		the diff needed to get the least recent position recorded is at len(differences)-1
	*/
	differences  []boardDifference

	currentColor int

	/**
		an action index (actionIdx) is an index to the favourableLegalActions slice
		an action (action) is a possible value of that slice
	*/
	favourableLegalActions []int // ordered ascendingly
	lastPass	 bool
}

func New() *Game {
	board := make(map[int]int)

	differences := make([]boardDifference, config.Int["history_size"]-1)
	for i := range differences {
		differences[i].rem = UNDEF
	}
	differences = differences[:1]

	numActions := config.Int["boardsize"]*config.Int["boardsize"]+1
	favourableLegalActions := make([]int, 0, numActions)
	for a := 0; a < numActions; a++ {
		favourableLegalActions = append(favourableLegalActions, a)
	}

	game := &Game{board, differences, BLACK, favourableLegalActions, false}
	return game
}

func NewSimple() *Game {
	board := map[int]int{1:1, 3:2, 5:1, 6:1, 8:2, 9:2, 11:1, 13:2, 15:1, 16:1, 18:2, 19:2}

	differences := []boardDifference{
		{add:map[int]int{}, rem:19},
		{add:map[int]int{}, rem:5},
		{add:map[int]int{}, rem:9}}
	differences = differences[:1]

	numActions := config.Int["boardsize"]*config.Int["boardsize"]+1
	favourableLegalActions := make([]int, 0, numActions)
	favourableLegalActions = append(
		favourableLegalActions,
		[]int{2, 7, 12, 17, 20, 21, 22, 23, 24, 25}...)

	game := &Game{board, differences, BLACK, favourableLegalActions, false}
	return game
}

func (game *Game) Step(action int) {
	diff := boardDifference{add: make(map[int]int), rem: UNDEF}
	if action != PASS {
		if game.board[action] != EMPTY {
			log.Panicf("Called Step() with action %d which is not empty of color %d", action, game.board[action])
		}
		game.board[action] = game.currentColor

		// possibly remove stones of the other color
		otherColor := other(game.currentColor)
		neighbours := adjacentPositions(action)
		for _, neigh := range neighbours {
			if game.board[neigh] == otherColor {
				captured := game.capturedStones(neigh)

				// add the captured stones to the diff so we can recover the current position later
				// and remove the captured stones from the board
				for cap := range captured {
					diff.add[cap] = otherColor
					delete(game.board, cap)
				}
			}
		}

		// possibly remove my own stones
		captured := game.capturedStones(action)
		for cap := range captured {
			// don't add the new suicidal stone to the previous position because it wasn't there
			if cap != action {
				diff.add[cap] = game.currentColor
			}
			delete(game.board, cap)
		}

		// if my new stone is still on the board, remember to remove it to get to the previous position
		if game.board[action] == game.currentColor {
			diff.rem = action
		}
	}

	// store the new boardDifference object into our ring buffer and rotate the latter
	game.differences[len(game.differences)-1] = diff
	if len(game.differences) < cap(game.differences) {
	    game.differences = game.differences[:len(game.differences)+1]
	} else {
	    game.differences = game.differences[:1]
	}

	game.currentColor = other(game.currentColor)

	game.favourableLegalActions = game.favourableLegalActions[:0]
	if !(action == PASS && game.lastPass) {
		game.updateLegalActions()
	}

	if action == PASS {
		game.lastPass = true
	} else {
		game.lastPass = false
	}
}

func (game *Game) Score() float32 {
	// count black and white stones
	whiteScore, blackScore := float32(0.0), float32(0.0)
	for _, color := range game.board {
		switch color {
		case BLACK:
			blackScore++
		case WHITE:
			whiteScore++
		}
	}
	log.Debugf("Number of black stones: %.1f\n", blackScore)
	log.Debugf("Number of white stones: %.1f\n", whiteScore)
	log.Debugf("Total number of stones: %d\n", len(game.board))

	// initially, all empty fields are known, i.e. it is not determined which color they belong to, if any
	boardsize := config.Int["boardsize"]
	boardLength := boardsize * boardsize
	unknownTerritory := make(map[int]struct{}, boardLength - len(game.board))
	for a := 0; a < boardLength; a++ {
		_, present := game.board[a]
		if !present {
			unknownTerritory[a] = struct{}{}
		}
	}

	// go through each unknown position and build its induced connected graph consisting only of empty fields
	for unknownPos := range unknownTerritory {

		// this map contains the "outer shell" of the territory we are currently exploring
		newTerritory := make(map[int]struct{}, 1)
		newTerritory[unknownPos] = struct{}{}
		blackTerritory := false
		whiteTerritory := false
		count := 0
		for len(newTerritory) > 0 {
			for pos := range newTerritory {
				count++
				delete(newTerritory, pos)
				delete(unknownTerritory, pos)

				neighbours := adjacentPositions(pos)
				for _, neigh := range neighbours {
					switch game.board[neigh] {
					case EMPTY:
						_, present := unknownTerritory[neigh]
						if present {
							newTerritory[neigh] = struct{}{}
						}
					case BLACK:
						blackTerritory = true
					case WHITE:
						whiteTerritory = true
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

	komi := config.Float["komi"]
	whiteScore += komi
	log.Debugf("Total black score is %.1f", blackScore)
	log.Debugf("Total white score after komi is %.1f", whiteScore)

	switch game.currentColor {
	case BLACK:
		return blackScore - whiteScore
	case WHITE:
		return whiteScore - blackScore
	default:
		log.Panicf("Current color is invalid %d", game.currentColor)
		return float32(0.0) // required because the compiler treats log.Panicf() and panic() differently
	}
}

func (game *Game) Outcome() float32 {
	score := game.Score()
	if score > 0.0 {
		return float32(1.0)
	} else if score < 0.0 {
		return float32(-1.0)
	}
	log.Panicf("Outcome is draw")
	return 0.0 // necessary for some reason
}

func (game *Game) Observation() [][][]float32 {
	// create full maps for all memorized differences
	boards := make([]map[int]int, cap(game.differences)+1)
	boards[0] = game.board
	for t := 0; t < len(boards)-1; t++ {
		boards[t+1] = make(map[int]int, len(boards[t]))
		for pos, color := range boards[t] {
			boards[t+1][pos] = color
		}
		game.applyDiff(boards[t+1], t)
	}

	boardsize := config.Int["boardsize"]
	observation := make([][][]float32, boardsize)
	num_channels := len(boards) * 2 + 1
	action := 0
	for height := 0; height < boardsize; height++ {
		observation[height] = make([][]float32, boardsize)
		for width := 0; width < boardsize; width, action = width+1, action+1 {
			observation[height][width] = make([]float32, num_channels)
			channel := 0
			for brdIdx := 0; brdIdx < len(boards); brdIdx++ {
				for _, color := range [2]int{game.currentColor, other(game.currentColor)} {
					if boards[brdIdx][action] == color {
						observation[height][width][channel] = float32(1.0)
					}
					channel++
				}
			}
			if game.currentColor == WHITE {
				observation[height][width][channel] = float32(1.0)
			}
		}
	}
	return observation
}

func (game *Game) Copy() (gameCopy *Game) {
	gameCopy = new(Game)
	gameCopy.board = make(map[int]int, len(game.board))
	for pos, color := range game.board {
		gameCopy.board[pos] = color
	}

	// this copies the board differences
	// it's so complicated because len(differences) is misused as a pointer to the end of the ring buffer
	wholeLen := cap(game.differences)
	gameCopy.differences = make([]boardDifference, wholeLen)
	diffLen := len(game.differences)
	game.differences = game.differences[:wholeLen]
	copy(gameCopy.differences, game.differences)
	game.differences = game.differences[:diffLen]
	gameCopy.differences = gameCopy.differences[:diffLen]

	gameCopy.currentColor = game.currentColor

	gameCopy.favourableLegalActions = make([]int, len(game.favourableLegalActions))
	for a, action := range game.favourableLegalActions {
		gameCopy.favourableLegalActions[a] = action
	}

	gameCopy.lastPass = game.lastPass
	return
}

// the returned string always ends in a newline
func (game *Game) String() (nice string) {
	// this prepends an internal representation of the game to the output
	nice += fmt.Sprintf("%+v\n", *game) // dereference game to avoid recursion

	nice += fmt.Sprintf("Current position:\n")
	nice += boardString(game.board)

	// copy game.board to oldBoard
	oldBoard := make(map[int]int, len(game.board))
	for pos, color := range game.board {
		oldBoard[pos] = color
	}

	// display the past couple positions we have memorized
	// this code is so complicated because game.differences is a slice representing a ring buffer
	// whose beginning depends on the slice's length
	historySize := config.Int["history_size"]
	for i := 0; i < historySize-1; i++ {
		game.applyDiff(oldBoard, i)
		nice += fmt.Sprintf("Position %d:\n", i+1)
		nice += boardString(oldBoard)
	}
	return
}

func SgfActions(filename string) []int {
	PASS = config.Int["boardsize"] * config.Int["boardsize"]
	sgfMoveRegex := regexp.MustCompile(`;[B,W]\[[a-z]{0,2}\]`)
	sgfFile, err := os.Open(filename)
	if err != nil {
		panic("Could not open SGF file")
	}
	sgfBytes := make([]byte, 2048)
	count, err := sgfFile.Read(sgfBytes)
	if err != nil {
		panic("Could not read from the SGF file")
	}
	sgfString := string(sgfBytes[:count])
	sgfActionBars := sgfMoveRegex.FindAllString(sgfString, -1)

    actions := make([]int, 0, len(sgfActionBars))
    alternatingColor := BLACK
    for _, sgfActionBar := range sgfActionBars {
    	// sanity check
    	color := fromSgfColor(sgfActionBar[1])
    	if color != alternatingColor {
    		panic("The colors of the SGF moves do not alternate")
    	}
        alternatingColor = other(alternatingColor)

    	var action int
    	if len(sgfActionBar) <= 4 {
	        action = PASS
    	} else {
	        action = sgfToAction(sgfActionBar[3:5])
    	}

        actions = append(actions, action)
    }
    return actions
}

func FillSgfBytes(recordBytes []byte, color int, actions []int, outcome float32) []byte {
	recordBytes = append(recordBytes, "(;"...)
	recordBytes = append(recordBytes, "GM[1]"...)
	recordBytes = append(recordBytes, "FF[4]"...)
	recordBytes = append(recordBytes, "CA[UTF-8]"...)
	recordBytes = append(recordBytes, "AP[actor:0.0.0]"...)
	recordBytes = append(recordBytes, fmt.Sprintf("KM[%.1f]", config.Float["komi"])...)
	recordBytes = append(recordBytes, fmt.Sprintf("SZ[%d]", config.Int["boardsize"])...)
	recordBytes = append(recordBytes, fmt.Sprintf("DT[2019-07-25]")...)
	recordBytes = append(recordBytes, fmt.Sprintf("RE[%.0f]", outcome)...)
	for _, action := range actions {
		colorByte := toSgfColor(color)
		color = other(color)
		sgfAction := actionToSgf(action)
		recordBytes = append(recordBytes, fmt.Sprintf(";%c[%c%c]", colorByte, sgfAction[0], sgfAction[1])...)
	}
	recordBytes = append(recordBytes, ')')
	return recordBytes
}

func (game *Game) Color() int {
	return game.currentColor
}

func (game *Game) FavourableLegalActions() []int {
	return game.favourableLegalActions
}

func (game *Game) Finished() bool {
	return len(game.favourableLegalActions) == 0
}

// introduce new game-specific knowledge into the configuration
func ExtendConfig() {
	config.Int["num_actions"] = config.Int["boardsize"] * config.Int["boardsize"] + 1
}

func (game *Game) updateLegalActions() {
	otherColor := other(game.currentColor)
	boardLength := config.Int["boardsize"] * config.Int["boardsize"]
	boardLoop: for action := 0; action < boardLength; action++ {
		// ensure that this intersection is empty
		if game.board[action] != EMPTY {
			continue boardLoop
		}

		// FORBID SUICIDE MOVES
		/*
			An empty intersection is a suicide move if and only if after putting my new stone there,
			- every neighbour is non-empty,
			- every enemy neighbour chain has at least one liberty, and
			- my new stone's chain has no liberties.
		*/

		// ensure that all neighbours are non-empty
		neighbours := adjacentPositions(action)
		for _, neigh := range neighbours {
			if game.board[neigh] == EMPTY {
				game.favourableLegalActions = append(game.favourableLegalActions, action)
				continue boardLoop
			}
		}

		// ensure that, after this move, no enemy neighbour chain is captured
		game.board[action] = game.currentColor
		for _, neigh := range neighbours {
			if game.board[neigh] == otherColor {
				captured := game.capturedStones(neigh)
				if len(captured) > 0 {
					delete(game.board, action)
					game.favourableLegalActions = append(game.favourableLegalActions, action)
					continue boardLoop
				}
			}
		}

		// ensure that the new stone's chain survives
		captured := game.capturedStones(action)
		delete(game.board, action)
		if len(captured) > 0 {
			continue boardLoop
		}

		// END FORBID SUICIDE MOVES

		// FORBID EYE MOVES
		/*
			An empty intersection is an eye move if and only if before putting my new stone there,
			- every neighbour is mine, and
			- every neighbour is in the same chain.
		*/

		// ensure that the new stone's neighbours are of the same color as itself
		for _, neigh := range neighbours {
			if game.board[neigh] != game.currentColor {
				game.favourableLegalActions = append(game.favourableLegalActions, action)
				continue boardLoop
			}
		}

		// ensure that the new stone's neighbours are not altogether in one chain
		sameChain := game.reachable(neighbours[0], neighbours[1:])
		if sameChain {
			continue boardLoop
		}

		// END FORBID EYE MOVES

		game.favourableLegalActions = append(game.favourableLegalActions, action)
	}
	game.favourableLegalActions = append(game.favourableLegalActions, PASS)
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

func fromSgfColor(c byte) int {
	switch c {
	case 'W':
		return WHITE
	case 'B':
		return BLACK
	}
	panic(fmt.Sprintf("Unaccepted color describing byte %b", c))
}

func toSgfColor(c int) byte {
	switch c {
	case WHITE:
		return 'W'
	case BLACK:
		return 'B'
	}
	panic(fmt.Sprintf("Unaccepted color %c", c))
}

// an SGF action consists of two alphabet letters <width><height>, where "aa" indicates the top-left corner
func sgfToAction(sgfAction string) int {
    aRune := rune('a')
    width, height := rune(sgfAction[0]) - aRune, rune(sgfAction[1]) - aRune

    boardsize := config.Int["boardsize"]
    action := int(height) * boardsize + int(width)
    return action
}

func actionToSgf(action int) [2]byte {
    boardsize := config.Int["boardsize"]
	height := rune(action / boardsize)
	width := rune(action % boardsize)
    aRune := rune('a')

    sgfAction := [2]byte{
    	byte(aRune + width),
		byte(aRune + height)}
    return sgfAction
}

func (game *Game) capturedStones(startPosition int) (positions map[int]int) {
	const (
		NEW = iota
		OLD
	)
	positions = make(map[int]int)
	positions[startPosition] = NEW
	color := game.board[startPosition]
	addedNew := true
	for addedNew {
		addedNew = false
		for pos, flag := range positions {
			if flag == OLD { continue }
			neighbours := adjacentPositions(pos)
			for _, neigh := range neighbours {
				if game.board[neigh] == EMPTY {
					return map[int]int{} // we found a liberty of this chain, so no stone will be removed of this chain
				} else if game.board[neigh] == color {
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

func (game *Game) reachable(startPosition int, targetPositions []int) bool {
	color := game.board[startPosition]
	positions := []int{startPosition}
	reached := 0
	oldCount := 0
	for oldCount < len(positions) {
		for _, pos := range positions[oldCount:] {
			neighbours := adjacentPositions(pos)
			for _, neigh := range neighbours {
				if game.board[neigh] != color {
					continue
				} else if !contains(positions, neigh) {
					if contains(targetPositions, neigh) {
						reached++
						if reached == len(targetPositions) {
							return true
						}
					}
					positions = append(positions, neigh)
				}
			}
			oldCount++
		}
	}
	return false
}

func contains(a []int, elem int) bool {
	for _, old := range a {
		if old == elem {
			return true
		}
	}
	return false
}

func adjacentPositions(pos int) (positions []int) {
	boardsize := config.Int["boardsize"]
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

	if pos < boardsize*boardsize-boardsize {
		below := pos+boardsize
		positions = append(positions, below)
	}
	return
}

// the returned string always ends in a newline
func boardString(board map[int]int) (nice string) {
	boardsize := config.Int["boardsize"]
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

type boardDifference struct {
	add map[int]int
	rem int
}

func (game *Game) applyDiff(board map[int]int, diffIndex int) {
	allDifferences := game.differences[:cap(game.differences)]
	index := len(game.differences)-2-diffIndex % len(allDifferences)
	if index < 0 { index += len(allDifferences) }
	diff := allDifferences[index]

	for pos, color := range diff.add {
		board[pos] = color
	}
	if diff.rem != UNDEF {
		delete(board, diff.rem)
	}
}
