package searcher

import (
    "math"
    "time"
    "fmt"
    "math/rand"
    "gitlab.com/Habimm/tree-search-golang/gogame"
    "gitlab.com/Habimm/tree-search-golang/predictor"
    "gitlab.com/Habimm/tree-search-golang/config"
    "github.com/op/go-logging"
)

var (
    log = logging.MustGetLogger("searcher")
    virtualLossUnit = float32(1.0)
)

type Node struct {
    game           *gogame.Game
    values          []float32
    counts          []int
    virtualLosses   []float32
    legalPolicy     []float32
    children        []*Node
}

func NewNode(predictChan chan predictor.Request) (newNode *Node) {
    newGame := gogame.NewSimple()
    newNode, _ = constructNewNode(newGame, predictChan)
    log.Infof("Constructed new root node")
    log.Infof("%v", newNode)
    return
}

func (node *Node) AddChild(actionIdx int, predictChan chan predictor.Request) (newNode *Node, value float32) {
    newGame := node.game.Copy()
    legalActions := newGame.FavourableLegalActions()
    log.Debugf("Stepping with the %dth out of %d legal actions", actionIdx, len(legalActions))
    newGame.Step(legalActions[actionIdx])
    newNode, value = constructNewNode(newGame, predictChan)
    log.Infof("Added new child node for player %d with value %.4f", newGame.Color(), value)
    log.Infof("%v", newNode)
    return
}

func (node *Node) Update(actionIdx int, value float32) {
    node.virtualLosses[actionIdx] -= virtualLossUnit
    node.counts[actionIdx]++
    node.values[actionIdx] += (value - node.values[actionIdx]) / float32(node.counts[actionIdx])
}

func (node *Node) Score(actionIdx int, parentCount int) float32 {
    return node.values[actionIdx] - node.virtualLosses[actionIdx] +
        config.PolicyScoreFactor * node.legalPolicy[actionIdx] *
        float32(math.Sqrt(float64(parentCount))) / float32(1 + node.counts[actionIdx])
}

func (node *Node) Select(parentCount int) (maxActionIdx int) {
    maxScore := node.Score(0, parentCount)
    for actionIdx := 1; actionIdx < len(node.values); actionIdx++ {
        score := node.Score(actionIdx, parentCount)
        if score > maxScore {
            maxActionIdx = actionIdx
            maxScore = score
        }
    }
    node.virtualLosses[maxActionIdx] += virtualLossUnit
    return
}

// the returned string never ends in a newline
func (node *Node) String() (nice string) {
    nice += fmt.Sprintf("%+v\n", *node) // dereference to avoid recursion
    nice += node.game.String()
    legalActions := node.FavourableLegalActions()
    if len(legalActions) > 0 {
        nice += "Counts:\n"
        nice += statsString(node.counts, legalActions)+"\n"
        nice += "Values:\n"
        nice += statsString(node.values, legalActions)+"\n"
        nice += "Policy:\n"
        nice += statsString(node.legalPolicy, legalActions)
    }
    return
}

func (node *Node) Outcome() float32 {
    return node.game.Outcome()
}

func (node *Node) Color() int {
    return node.game.Color()
}

func (node *Node) Finished() bool {
    return node.game.Finished()
}

func (node *Node) FavourableLegalActions() []int {
    return node.game.FavourableLegalActions()
}

func (node *Node) Observation() [][][]float32 {
    return node.game.Observation()
}

// the returned string never ends in a newline
func statsString(stats interface{}, legalActions []int) (nice string) {
    // compute the maximum number of characters per item in stats to use as width to make everything look lean
    maxAction := -1
    maxVal := float32(math.Inf(-1))
    width := 0
    for actionIdx, action := range legalActions {
        switch casted := stats.(type) {
        case []int:
            str := fmt.Sprintf("%d", casted[actionIdx])
            if len(str) > width {
                width = len(str)
            }
            if float32(casted[actionIdx]) > maxVal {
                maxVal = float32(casted[actionIdx])
                maxAction = action
            }
        case []float32:
            str := fmt.Sprintf("%.4f", casted[actionIdx])
            if len(str) > width {
                width = len(str)
            }
            if casted[actionIdx] > maxVal {
                maxVal = casted[actionIdx]
                maxAction = action
            }
        }
    }
    width++

    // the length of legalActions is used as a pointer to the next legal action we expect to encounter when
    // we scan through all actions from left to right
    boardsize := config.Int["boardsize"]
    numActions := boardsize * boardsize + 1
    legalActions = legalActions[:1]
    for action, column := 0, 0; action < numActions; action++ {
        if action == legalActions[len(legalActions)-1] {
            switch casted := stats.(type) {
            case []int:
                widthFormat := fmt.Sprintf("%%%dd", width)
                nice += fmt.Sprintf(widthFormat, casted[len(legalActions)-1])
            case []float32:
                widthFormat := fmt.Sprintf("%%%d.4f", width)
                nice += fmt.Sprintf(widthFormat, casted[len(legalActions)-1])
            }
            if len(legalActions) < cap(legalActions) {
                legalActions = legalActions[:len(legalActions)+1]
            }
        } else {
            widthFormat := fmt.Sprintf("%%%ds", width)
            nice += fmt.Sprintf(widthFormat, "-")
        }

        if column == boardsize-1 {
            column = 0
            if action == maxAction {
                nice += "*"
            }
            nice += "\n"
        } else {
            column++
            if action < numActions-1 {
                if action == maxAction {
                    nice += "*"
                } else {
                    nice += " "
                }
            }
        }
    }
    return
}

func constructNewNode(game *gogame.Game, predictChan chan predictor.Request) (newNode *Node, value float32) {
    var legalPolicy []float32
    legalActions := game.FavourableLegalActions()
    if len(legalActions) == 0 {
        value = game.Outcome()
    } else {
        request := predictor.Request{game.Observation(), make(chan predictor.Response)}
        predictChan<- request
        prediction := <-request.ResultChan

        value = prediction.Value

        // normalize the legalPolicy logits of legal actions
        legalPolicy = make([]float32, len(legalActions))
        sum := float32(0.0)
        for actionIdx, action := range legalActions {
            legalPolicy[actionIdx] = float32(math.Exp(float64(prediction.Policy[action])))
            sum += legalPolicy[actionIdx]
        }
        for actionIdx := range legalPolicy {
            legalPolicy[actionIdx] /= sum
        }
    }
    newNode = &Node{
        game: game,
        values: make([]float32, len(legalActions)),
        counts: make([]int, len(legalActions)),
        virtualLosses: make([]float32, len(legalActions)),
        legalPolicy: legalPolicy,
        children: make([]*Node, len(legalActions))}
    return
}

type Searcher struct {
    root            *Node
    predictChan     chan predictor.Request
    rootCount       int
    done            chan int
}

func NewSearcher(predictChan chan predictor.Request) *Searcher {
    searcher := &Searcher{
        predictChan: predictChan,
        done: make(chan int)}
    return searcher
}

func (searcher *Searcher) Reset() {
    log.Debugf("Resetting the game tree")
    searcher.root = NewNode(searcher.predictChan)
    searcher.rootCount = 1
}

func (searcher *Searcher) Search() {
    predict_batch_size := config.Int["predict_batch_size"]
    start := time.Now()
    log.Infof("Starting simulations")
    for i := 0; i < predict_batch_size; i++ {
        go searcher.simulate(i)
    }
    for i := 0; i < predict_batch_size; i++ {
        <-searcher.done
    }
    t := time.Now()
    elapsed := t.Sub(start)
    log.Infof("Performed %d simulations in %v", predict_batch_size*config.Int["nsims_per_goroutine"], elapsed)
}

func (searcher *Searcher) Exploit() (actionIdx int, policy []float32) {
    actionIdx = -1
    maxCount := -1
    for a, count := range searcher.root.counts {
        if count > maxCount {
            maxCount = count
            actionIdx = a
        }
    }

    policy = make([]float32, config.Int["num_actions"])
    legalActions := searcher.root.FavourableLegalActions()
    policy[legalActions[actionIdx]] = float32(1.0)
    return
}

func (searcher *Searcher) Explore() (actionIdx int, policy []float32) {
    policy = make([]float32, config.Int["num_actions"])
    sum := searcher.rootCount-1
    if sum == 0 {
        log.Panicf("Called Explore() without prior doing any simulations")
    }

    // the following creates the policy from normalizing the visit counts
    // and at the same time samples an action index from that policy
    actionIdx = -1
    accumulated := float32(0.0)
    r := 1.0 - rand.Float32() // r is the minimum probability mass we want to gather
    legalActions := searcher.root.FavourableLegalActions()
    for a, action := range legalActions {
        policy[action] = float32(searcher.root.counts[a]) / float32(sum)

        if accumulated < r {
            actionIdx++
            accumulated += policy[action]
        }
    }
    log.Debugf("Chosen action index %d out of %d legal actions", actionIdx, len(legalActions))
    return
}

func (searcher *Searcher) Step(actionIdx int) {
    if logging.GetLevel("searcher") >= logging.DEBUG {
        legalActions := searcher.root.FavourableLegalActions()
        log.Debugf("Taking move %d", legalActions[actionIdx])
    }

    if searcher.root.children[actionIdx] == nil {
        searcher.root.children[actionIdx], _ = searcher.root.AddChild(actionIdx, searcher.predictChan)
    }
    searcher.root = searcher.root.children[actionIdx]
}

func (searcher *Searcher) Observation() [][][]float32 {
    return searcher.root.Observation()
}

func (searcher *Searcher) Outcome() float32 {
    return searcher.root.Outcome()
}

func (searcher *Searcher) Finished() bool {
    return searcher.root.Finished()
}

func (searcher *Searcher) Color() int {
    return searcher.root.Color()
}

func ExtendConfig() {
    gogame.ExtendConfig()
}

func (searcher *Searcher) simulate(grtIndex int) {
    for nsims := config.Int["nsims_per_goroutine"]; nsims > 0; nsims-- {
        curNode := searcher.root
        nodes := make([]*Node, 0)
        actionIdxs := make([]int, 0)

        actionIdx := curNode.Select(searcher.rootCount)
        actionIdxs = append(actionIdxs, actionIdx)
        nodes = append(nodes, curNode)
        parentCount := curNode.counts[actionIdx]
        curNode = curNode.children[actionIdx]
        for curNode != nil && !curNode.Finished() {
            actionIdx := curNode.Select(parentCount)
            actionIdxs = append(actionIdxs, actionIdx)
            nodes = append(nodes, curNode)
            parentCount = curNode.counts[actionIdx]
            curNode = curNode.children[actionIdx]
        }
        if len(nodes) == 0 {
            log.Panicf("Tried to simulate on a nil or finished root node")
        }

        log.Infof("Goroutine %d, at simulation %d, took actions %v", grtIndex, nsims, actionIdxs)

        var value float32
        if curNode != nil {
            value = curNode.Outcome()
        } else {
            node := nodes[len(nodes)-1]
            actionIdx := actionIdxs[len(actionIdxs)-1]
            node.children[actionIdx], value = node.AddChild(actionIdx, searcher.predictChan)
        }

        for i := len(nodes)-1; i >= 0; i-- {
            value *= -1.0 // in Go, the color always alternates between moves
            node := nodes[i]
            actionIdx := actionIdxs[i]
            node.Update(actionIdx, value)
            log.Infof("Updated player %d's node with %.4f", node.Color(), value)
            log.Debugf("%v", node)
        }
        searcher.rootCount++
    }
    searcher.done<- 1
}
