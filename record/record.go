package record

import (
    "os"
    "fmt"
    "time"
    "github.com/satori/go.uuid"
    "github.com/op/go-logging"
    "gitlab.com/Habimm/tree-search-golang/config"
)

var (
    log = logging.MustGetLogger("record")
)

type Info struct {
    InitialColor    int
    BlackName       string
    WhiteName       string
    Actions         []int
    Outcome         float32
}

func Save(recordsChan chan *Info) {
    recordBytes := make([]byte, 0)
    recordPrefix := config.String["record_prefix"]
    for record := range recordsChan {
        recordBytes = recordBytes[:0]
        recordBytes = FillSgfBytes(recordBytes, record)

        sgfFile, err := os.Create(fmt.Sprintf("%s/%s.sgf", recordPrefix, uuid.Must(uuid.NewV4())))
        if err != nil {
            log.Panicf("Could not create a file to write the game record:\n%v", recordBytes)
        }
        nWritten, err := sgfFile.Write(recordBytes)
        if err != nil {
            log.Panicf("Could not write to game record file %v", sgfFile)
        }
        if nWritten != len(recordBytes) {
            log.Panicf("Only wrote %d out of %d bytes to game record file %v",
                nWritten, len(recordBytes), sgfFile)
        }
        sgfFile.Close()
    }
}

func FillSgfBytes(recordBytes []byte, record *Info) []byte {
    recordBytes = append(recordBytes, "(;"...)
    recordBytes = append(recordBytes, "GM[1]"...)
    recordBytes = append(recordBytes, "FF[4]"...)
    recordBytes = append(recordBytes, "CA[UTF-8]"...)
    recordBytes = append(recordBytes, "AP[dimitri:0.0.0]"...)
    recordBytes = append(recordBytes, fmt.Sprintf("KM[%.1f]", config.Float["komi"])...)
    recordBytes = append(recordBytes, fmt.Sprintf("SZ[%d]", config.Int["boardsize"])...)
    recordBytes = append(recordBytes, fmt.Sprintf("DT[%s]", time.Now().Format(time.RubyDate))...)
    recordBytes = append(recordBytes, fmt.Sprintf("PB[%s]", record.BlackName)...)
    recordBytes = append(recordBytes, fmt.Sprintf("PW[%s]", record.WhiteName)...)

    /*
        In the RE field, put 'W' if White won and 'B' if Black won
        (this is so complicated because record.Outcome is from the perspective of the player
        whose turn it would be if the game were to continue)
    */
    var winnerByte byte
    parity := len(record.Actions) % 2
    if (parity == 0 && record.Outcome == float32(1.0)) ||
        (parity == 1 && record.Outcome == float32(-1.0)) {
        winnerByte = toSgfColor(record.InitialColor)
    } else {
        winnerByte = toSgfColor(other(record.InitialColor))
    }
    recordBytes = append(recordBytes, fmt.Sprintf("RE[%c]", winnerByte)...)

    color := record.InitialColor
    for _, action := range record.Actions {
        colorByte := toSgfColor(color)
        color = other(color)
        actionBytes := toSgfAction(action)
        recordBytes = append(recordBytes, fmt.Sprintf(";%c[%c%c]", colorByte, actionBytes[0], actionBytes[1])...)
    }
    recordBytes = append(recordBytes, ')')
    return recordBytes
}

func toSgfColor(color int) byte {
    switch color {
    case config.WHITE:
        return 'W'
    case config.BLACK:
        return 'B'
    default:
        log.Panicf("Unaccepted color %d", color)
        panic(0)
    }
}

func toSgfAction(action int) [2]byte {
    boardsize := config.Int["boardsize"]
    height := rune(action / boardsize)
    width := rune(action % boardsize)
    aRune := rune('a')

    actionBytes := [2]byte{byte(aRune + width), byte(aRune + height)}
    return actionBytes
}

func other(color int) int {
    switch color {
    case config.BLACK:
        return config.WHITE
    case config.WHITE:
        return config.BLACK
    default:
        log.Panicf("Input %d not accepted (only %d and %d)", color, config.BLACK, config.WHITE)
        panic(0)
    }
}
