package gorules

import (
	"testing"
	"os"
	"github.com/op/go-logging"
	"fmt"
)

func TestSetup(t *testing.T) {
	logFormat := logging.MustStringFormatter(`%{color}%{time:15:04:05.000000} %{shortfile} %{shortfunc}() ▶ %{color:reset}%{message}`)
	formattedBackend := logging.NewBackendFormatter(logging.NewLogBackend(os.Stderr, "", 0), logFormat)
	logging.SetBackend(formattedBackend)
	logging.SetLevel(logging.DEBUG, "gorules")
}

func Example_hello() {
    fmt.Println("hello")
    // Output: hello
}

func ExampleSalutations() {
	state := NewState()
    fmt.Printf("%+v", state)
    // Output: {board:map[] differences:[{add:map[] rem:-1}] currentColor:1 legalActions:[0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25] lastPass:false}
	// Current position:
	// - - - - -
	// - - - - -
	// - - - - -
	// - - - - -
	// - - - - -
	// Position 1:
	// - - - - -
	// - - - - -
	// - - - - -
	// - - - - -
	// - - - - -
	// Position 2:
	// - - - - -
	// - - - - -
	// - - - - -
	// - - - - -
	// - - - - -
	// Position 3:
	// - - - - -
	// - - - - -
	// - - - - -
	// - - - - -
	// - - - - -
}

func TestPlay(t *testing.T) {
	state := NewState()
	nMoves := 7
	for i := 0; i < nMoves; i++ {
		state.Step(0)
	}
    // t.Errorf("Done %d moves", nMoves)
}
