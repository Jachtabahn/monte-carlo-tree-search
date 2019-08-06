package main

import (
    "os"
    "time"
    "math/rand"
    "github.com/op/go-logging"
    "gitlab.com/Habimm/tree-search-golang/config"
    "gitlab.com/Habimm/tree-search-golang/record"
    "gitlab.com/Habimm/tree-search-golang/predictor"
    "gitlab.com/Habimm/tree-search-golang/treesearch"
    "gitlab.com/Habimm/tree-search-golang/randomplay"
)

var (
    log = logging.MustGetLogger("eval")
)

func Play(searcher *treesearch.Agent, searcherColor int, recordsChan chan *record.Info) {
    start := time.Now()
    searcher.Reset()
    record := &record.Info{InitialColor: searcher.Color(), Actions: make([]int, 0)}
    if searcherColor == config.BLACK {
        record.BlackName = searcher.Name()
        record.WhiteName = "Random player"
    } else {
        record.BlackName = "Random player"
        record.WhiteName = searcher.Name()
    }
    gameLength := 0
    for ; !searcher.Finished(); gameLength++ {
        var actionIdx int
        if searcher.Color() == searcherColor {
            searcher.Search()
            actionIdx, _ = searcher.Exploit()
        } else {
            actionIdx = randomplay.QuickStep(searcher.FavourableLegalActions())
        }
        action := searcher.FavourableLegalActions()[actionIdx]
        record.Actions = append(record.Actions, action)
        searcher.Step(actionIdx)
    }
    outcome := searcher.Outcome()
    record.Outcome = outcome
    recordsChan<- record

    elapsed := time.Now().Sub(start)
    log.Infof("Performed a play with outcome %.0f of length %d with the tree search agent in color %d against the random player in %v",
        outcome, gameLength, searcherColor, elapsed)
}

func main() {
    rand.Seed(int64(config.Int["random_seed"]))
    treesearch.ExtendConfig()

    // other flags: %{shortfile} %{color} %{color:reset}
    logFormat := logging.MustStringFormatter(`%{time:15:04:05.000000} %{shortfunc}() ▶ %{message}`)
    formattedBackend := logging.NewBackendFormatter(logging.NewLogBackend(os.Stderr, "", 0), logFormat)
    logging.SetBackend(formattedBackend)
    logging.SetLevel(logging.ERROR, "eval")
    logging.SetLevel(logging.ERROR, "gogame")
    logging.SetLevel(logging.ERROR, "predictor")
    logging.SetLevel(logging.ERROR, "randomplay")
    logging.SetLevel(logging.ERROR, "treesearch")
    logging.SetLevel(logging.ERROR, "record")

    predictor.StartService(config.String["model_path"])

    recordsChan := make(chan *record.Info, 1)
    go record.SaveRecords(recordsChan)

    searcher := treesearch.New(predictor.RequestsChannel)
    numEvalGames := config.Int["num_eval_games"]
    log.Debugf("%d", numEvalGames)
    for g := 0; ; g++ {
        Play(searcher, config.BLACK, recordsChan)
        log.Infof("Played game with searcher as Black %d", g)
        Play(searcher, config.WHITE, recordsChan)
        log.Infof("Played game with searcher as White %d", g)
    }

    close(recordsChan)
    time.Sleep(1 * time.Second)
}
