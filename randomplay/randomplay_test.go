package randomplay

import (
    "testing"
    "math/rand"
    "os"
    "github.com/op/go-logging"
    "gitlab.com/Habimm/tree-search-golang/config"
)

func TestSearcher(t *testing.T) {
    rand.Seed(int64(config.Int["random_seed"]))
    ExtendConfig()

    // prepare logging
    logFile, err := os.Create("randomplay.log")
    if err != nil {
        panic("Could not create log file")
    }
    // other flags: %{shortfile} %{color} %{color:reset}
    logFormat := logging.MustStringFormatter(`%{time:15:04:05.000000} %{shortfunc}() ▶ %{message}`)
    formattedBackend := logging.NewBackendFormatter(logging.NewLogBackend(logFile, "", 0), logFormat)
    logging.SetBackend(formattedBackend)
    logging.SetLevel(logging.DEBUG, "randomplay")
    logging.SetLevel(logging.DEBUG, "gogame")

    agent := New()
    log.Debugf("%+v", agent)
    log.Debugf("Name: %s", agent.Name())

    agent.Reset()
    for !agent.Finished() {
        agent.Search()
        actionIdx, _ := agent.Exploit()
        agent.Step(actionIdx)

        log.Debugf("Finished: %t", agent.Finished())
        log.Debugf("Color: %d", agent.Color())
        log.Debugf("Observation: %v", agent.Observation())
        log.Debugf("FavourableLegalActions: %v", agent.FavourableLegalActions())
    }
    log.Debugf("Outcome: %v", agent.Outcome())
}
