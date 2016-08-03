package shared

import (
    . "Programmierwettbewerb-Server/vector"

    "fmt"
    "runtime"
    "time"
    "github.com/fatih/color"
    "sync"
)

type GuiId uint32
type BotId uint32
type TeamId uint32
type BlobId uint32
type FoodId uint32
type ToxinId uint32
type CommandId uint32

type Color struct {
    R byte
    G byte
    B byte
}

type BotActionType uint8
const (
    BatNone     BotActionType = iota
    BatThrow
    BatSplit
)

type MessageMiddlewareServerType int
const (
    MmstBotInfo         MessageMiddlewareServerType = iota
    MmstBotCommand
)

type BotCommand struct {
    Action  BotActionType
    Target  Vec2
}

type Statistics struct {
    // Maximum size achieved
    MaxSize         float32     `json:"size"`                           // check.
    // Longest survival time achieved
    MaxSurvivalTime float32     `json:"survivalTime"`                   // check.
    // How many blobs it killed overall
    BlobKillCount   int         `json:"blobKillCount"`                  // check.
    // How many bots it killed overall (No surviving blob of that bot!)
    BotKillCount    int         `json:"botKillCount"`                   // check.
    // How often it duplicated a toxin
    ToxinThrow      int         `json:"toxinThrow"`                     // check.
    // How often the duplicated toxin actually exploded another blob!
    SuccessfulToxin int         `json:"successfulToxin"`                // check.
    // How often it has split
    SplitCount      int         `json:"splitCount"`                     // check.
    // How often the splitted blob ate at least one other blob! (Only immediately, not 10s later!)
    SuccessfulSplit int         `json:"successfulSplit"`                // check.
    // We have to talk about that one ;)
    // Probably like feeding a team mate, resulting in eating an enemy blob or similar
    SuccessfulTeam  int         `json:"successfulTeaming"`              //
    // For example eating a complete bot of the own team...
    BadTeaming      int         `json:"badTeaming"`                     //
}

type BotInfo struct {
    Name        string  `json:"name"`
    Color       Color   `json:"color"`
    ImagePath   string  `json:"image"` // This is not used right now.
}

type MessageMiddlewareServer struct {
    Type                MessageMiddlewareServerType
    BotCommand          *BotCommand
    BotInfo             *BotInfo
}

type Food struct {
    IsNew       bool    `json:"new"`
    IsMoving    bool    `json:"moving"`
    IsThrown    bool    `json:"thrown"`
    // We need the bot-ID here for statistic reasons
    IsThrownBy  BotId
    Mass        float32 `json:"mass"`
    Position    Vec2    `json:"pos"`
    Velocity    Vec2    `json:"vel"`
}

type Toxin struct {
    IsNew      bool     `json:"new"`
    IsMoving   bool     `json:"moving"`
    Position   Vec2     `json:"pos"`
    IsSplit    bool
    IsSplitBy  BotId
    Mass       float32  `json:"mass"`
    Velocity   Vec2     `json:"vel"`
}

type ServerMiddlewareBlob struct {
    BotId       uint32      `json:"botId"`
    TeamId      uint32      `json:"teamId"`
    Index       uint32      `json:"index"`
    Position    Vec2        `json:"pos"`
    Mass        uint32      `json:"mass"`
}

type ServerMiddlewareGameState struct {
    MyBlob      []ServerMiddlewareBlob  `json:"myBlobs"`
    OtherBlobs  []ServerMiddlewareBlob  `json:"otherBlobs"`
    Food        []Food                  `json:"food"`
    Toxin       []Toxin                 `json:"toxin"`
}

// -------------------------------------------------------------------------------------------------
// Logging
// -------------------------------------------------------------------------------------------------

var loggingMutex sync.Mutex

var (
    globalDebug             bool = false
    globalVerbose           bool = false
    globalMute              bool = false
    globalPrintLineNumber   bool = true
    globalPrefix            string = ""
)

type LogType int
const (
    LtAlways        LogType =      iota // You cannot test on this using a bitwise and operation, the result is always zero.
    LtVerbose       LogType = 1 << iota
    LtDebug         LogType = 1 << iota
)

type LogColor int
const (
    LcGreen         LogColor = LogColor(color.FgGreen)
    LcMagenta       LogColor = LogColor(color.FgMagenta)
    LcYellow        LogColor = LogColor(color.FgYellow)
    LcRed           LogColor = LogColor(color.FgRed)
    LcBlue          LogColor = LogColor(color.FgBlue)
    LcCyan          LogColor = LogColor(color.FgCyan)
)

func SetLoggingDebug(value bool) {
    globalDebug = value
}

func SetLoggingVerbose(value bool) {
    globalVerbose = value
}

func SetLoggingMute(value bool) {
    globalMute = value
}

func SetPringLineNumber(value bool) {
    globalPrintLineNumber = value
}

func SetLoggingPrefix(value string) {
    globalPrefix = value
}

type LogFun func(format string, a ...interface{}) (n int, err error)
func log(f LogFun, logType LogType, format string, a ...interface{}) (n int, err error) {
    if globalMute {
        return 0, nil
    }
    if logType != 0 {
        if logType & LtVerbose != 0 && !globalVerbose {
            return 0, nil
        }
        if logType & LtDebug != 0 && !globalDebug {
            return 0, nil
        }
    }
    if len(globalPrefix) > 0 {
        fmt.Printf("%s ", globalPrefix)
    }
    if globalPrintLineNumber {
        _, _, line, _ := runtime.Caller(2)
        lineString := fmt.Sprintf("%d", line)
        for len(lineString) < 5 {
            lineString = fmt.Sprintf(" %s", lineString)
        }
        str := fmt.Sprintf("%s Line:%s |  ", time.Now().Format("02 Jan 15:04:05"), lineString)
        fmt.Printf(str)
    }
    return f(format, a...)
}

func Logln(logType LogType, a ...interface{}) (n int, err error) {
    loggingMutex.Lock()
    defer loggingMutex.Unlock()

    f := func(format string, a ...interface{}) (n int, err error) {
        return fmt.Println(a...)
    }
    return log(f, logType, "", a...)
}

func Logf(logType LogType, format string, a ...interface{}) (n int, err error) {
    loggingMutex.Lock()
    defer loggingMutex.Unlock()

    f := func(format string, a ...interface{}) (n int, err error) {
        return fmt.Printf(format, a...)
    }
    return log(f, logType, format, a...)
}

func LogfColored(logType LogType, logColor LogColor, format string, a ...interface{}) (n int, err error) {
    loggingMutex.Lock()
    defer loggingMutex.Unlock()
    
    color.Set(color.Attribute(logColor))
    color.Set(color.Bold)
    defer color.Unset()
    
    f := func(format string, a ...interface{}) (n int, err error) {
        return fmt.Printf(format, a...)
    }
    return log(f, logType, format, a...)
}

func Max(a, b int) int {
    if a > b {
        return a
    }
    return b
}

func Min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
