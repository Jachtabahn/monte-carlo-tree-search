package searcher

import (
    "testing"
    "os"
    // "fmt"
    "github.com/Habimm/monte-carlo-tree-search/config"
    "github.com/Habimm/monte-carlo-tree-search/predictor"
    "github.com/op/go-logging"
)

func TestSearcher(t *testing.T) {
    // prepare logging
    logFile, err := os.Create("searcher.log")
    if err != nil {
        panic("Could not create log file")
    }
    // other flags: %{shortfile} %{color} %{color:reset}
    logFormat := logging.MustStringFormatter(`%{time:15:04:05.000000} %{shortfunc}() ▶ %{message}`)
    formattedBackend := logging.NewBackendFormatter(logging.NewLogBackend(logFile, "", 0), logFormat)
    logging.SetBackend(formattedBackend)
    logging.SetLevel(logging.DEBUG, "searcher")

    searcher := NewSearcher()
    predictor.StartService(searcher.predictChan)
    searcher.Reset()
    nsims := config.Int["nsims"]
    searcher.Search(nsims)
    searcher.Exploit()
    searcher.Explore()
}
