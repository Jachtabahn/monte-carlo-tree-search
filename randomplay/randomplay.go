package randomplay

import (
    "math/rand"
    "gitlab.com/Habimm/tree-search-golang/gogame"
    "gitlab.com/Habimm/tree-search-golang/config"
    "github.com/op/go-logging"
)

var (
    log = logging.MustGetLogger("randomplay")
)

type RandomAgent struct {
    game    *gogame.Game
}

func New() *RandomAgent {
    return new(RandomAgent)
}

func (agent *RandomAgent) Name() string {
    return "Random player"
}

func (agent *RandomAgent) Reset() {
    agent.game = gogame.New()
}

func (agent *RandomAgent) Search() { }

func (agent *RandomAgent) Exploit() (actionIdx int, policy []float32) {
    legalActions := agent.game.FavourableLegalActions()
    actionIdx = rand.Intn(len(legalActions))

    policy = make([]float32, config.Int["num_actions"])
    for _, action := range legalActions {
        policy[action] = float32(1.0) / float32(len(legalActions))
    }
    log.Debugf("Policy: %v", policy)
    log.Debugf("Sampled action index: %v", actionIdx)
    return
}

func (agent *RandomAgent) Explore() (actionIdx int, policy []float32) {
    return agent.Exploit()
}

func (agent *RandomAgent) Step(actionIdx int) {
    action := agent.game.FavourableLegalActions()[actionIdx]
    agent.game.Step(action)

    log.Debugf("Taking move %d", action)
}

func (agent *RandomAgent) Observation() [][][]float32 {
    return agent.game.Observation()
}

func (agent *RandomAgent) Outcome() float32 {
    return agent.game.Outcome()
}

func (agent *RandomAgent) Finished() bool {
    return agent.game.Finished()
}

func (agent *RandomAgent) Color() int {
    return agent.game.Color()
}

func (agent *RandomAgent) FavourableLegalActions() []int {
    return agent.game.FavourableLegalActions()
}

func ExtendConfig() {
    gogame.ExtendConfig()
}

func QuickStep(legalActions []int) int {
    return rand.Intn(len(legalActions))
}
