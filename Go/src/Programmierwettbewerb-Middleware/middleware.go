package main

import (
    vec "Programmierwettbewerb-Server/vector"
    .   "Programmierwettbewerb-Server/shared"

    "golang.org/x/net/websocket"
    "github.com/BurntSushi/toml"
    "fmt"
    "os"
    "time"
    "flag"
    "sync"
    "os/exec"
    "bufio"
    "io"
    "strconv"
    "errors"
    "regexp"
    "strings"
    "math"
    "math/rand"
    //"bytes"
    //"reflect"
    //"io/ioutil"
    //"os/exec"
    //"bufio"
)

// -------------------------------------------------------------------------------------------------
// Global
// -------------------------------------------------------------------------------------------------

const (
    // This is used, when no connection is given via command line argument.
    //globalDefaultAddress string = "ws://127.0.0.1:8080/middleware/"
    globalDefaultAddress string = "ws://cagine.fh-wedel.de:8080/middleware/"
)


// Info from config file
type Config struct {
    Name       string
    Bot        string
    Connection string
}


// -------------------------------------------------------------------------------------------------

func usage() {
    fmt.Fprintf(os.Stderr, "NAME\n")
    fmt.Fprintf(os.Stderr, "    Programmierwettbewerb-Middleware [-bot=BOT] [-name=NAME] [-numBots=NUM]\n")
    fmt.Fprintf(os.Stderr, "\n")
    fmt.Fprintf(os.Stderr, "CONFIG\n")
    fmt.Fprintf(os.Stderr, "    There should be a config file middleware.conf to define the default parameters:\n")
    fmt.Fprintf(os.Stderr, "\n")
    fmt.Fprintf(os.Stderr, "        name=\"myname\"\n")
    fmt.Fprintf(os.Stderr, "        bot=\"java mybot arg1 arg2 ...\"\n")
    fmt.Fprintf(os.Stderr, "\n")
    fmt.Fprintf(os.Stderr, "    When arguments are provided to the program directly they will override the config file entries.\n")
    fmt.Fprintf(os.Stderr, "\n")
    fmt.Fprintf(os.Stderr, "ARGUMENTS\n")
    fmt.Fprintf(os.Stderr, "    BOT\n")
    fmt.Fprintf(os.Stderr, "        executable\n")
    fmt.Fprintf(os.Stderr, "\n")
    fmt.Fprintf(os.Stderr, "    NAME\n")
    fmt.Fprintf(os.Stderr, "        name from your bot.names\n")
    fmt.Fprintf(os.Stderr, "\n")
    fmt.Fprintf(os.Stderr, "    NUM\n")
    fmt.Fprintf(os.Stderr, "        number of bots to spawn\n")
}

// -------------------------------------------------------------------------------------------------
// Strings
// -------------------------------------------------------------------------------------------------

func firstN(str string, n int) string {
    return str[0:int(math.Min(float64(n), float64(len(str)-1)))]
}

func abbreviate(str string, n int) string {
    return firstN(str, n) + "..."
}

// -------------------------------------------------------------------------------------------------

// Exit-codes
const (
    ecEverythingOk      = iota
    ecParameterProblem
    ecBotProblem
    ecConnectionProblem
)

func fatalExit(message string, code int) {
    Logln(LtAlways, message)
    usage()
    os.Exit(code)
}

func fatalError(err error, code int) {
    fatalExit(err.Error(), code)
}

// -------------------------------------------------------------------------------------------------

type ParseResult struct {
    debug       bool
    verbose     bool
    mute        bool
    botPath     string
    botName     string
    numBots     int
    serverURL   string
}

func parseArguments() (parseResult ParseResult, e error) {
    flag.Usage = usage

    var debugFlag       = flag.Bool("debug", false, "Running in debug mode.")
    var verboseFlag     = flag.Bool("verbose", false, "Printing more stuff than unsual.")
    var muteFlag        = flag.Bool("mute", false, "Mute the output.")
    var botPath         = flag.String("bot", "", "Path to the bot")
    var botName         = flag.String("name", "", "Test name")
    var numBotsFlag     = flag.Int("numBots", 1, "Number of bots to start.")
    var serverURLFlag   = flag.String("connection", "", "URL to connect to the server")

    flag.Parse()

    result := ParseResult{
        debug:      *debugFlag,
        verbose:    *verboseFlag,
        mute:       *muteFlag,
        botPath:    *botPath,
        botName:    *botName,
        numBots:    *numBotsFlag,
        serverURL:  *serverURLFlag,
    }

    return result, nil
}

// -------------------------------------------------------------------------------------------------

type ServerConnection struct {
    GameStateFromServer     chan(ServerMiddlewareGameState)
    BotCommandToServer      chan(BotCommand)
}

// -------------------------------------------------------------------------------------------------

type Bot struct {
    process     *exec.Cmd
    stdin       io.WriteCloser
    stdout      io.ReadCloser
    stderr      io.ReadCloser
}

func startBot(parseResults ParseResult) (bot Bot, error error) {


    commandString := parseResults.botPath
    stringList := strings.Fields(commandString)

    bot.process = exec.Command(stringList[0], stringList[1:]...)

    stdin, err := bot.process.StdinPipe()
    if err != nil {
        return bot, err
    }
    bot.stdin = stdin

    stdout, err := bot.process.StdoutPipe()
    if err != nil {
        return bot, err
    }
    bot.stdout = stdout

    stderr, err := bot.process.StderrPipe()
    if err != nil {
        return bot, err
    }
    bot.stderr = stderr

    // Pass everything that the bot sends on stderr to stdout
    if !parseResults.mute {
        bot.process.Stderr = os.Stdout
    }

    if err = bot.process.Start(); err != nil {
        return bot, err
    }

    return bot, nil
}

func stopBot(bot Bot) {
    bot.stdin.Close()
    bot.stdout.Close()
    bot.stderr.Close()

    if err := bot.process.Process.Kill(); err != nil {
        fatalExit(fmt.Sprintf("Could not kill the bot. Error: %v\n", err.Error()), ecBotProblem)
    }
}

func terminateNonBlocking(runningState chan(bool), test string) {
    // Try to send a non-blocking close to the channel...
    select {
    case runningState <- false:
    default:
        // Aaaaand it didn't work.
        Logf(LtDebug, "Not sending a close: %v\n", test)
    }
}

func connectionIsTerminated(runningState chan(bool), test string) bool {
    select {
    case state, ok := <-runningState:
        if ok {
            if !state {
                terminateNonBlocking(runningState, test)
                return true
            }
        } else {
            return true
        }
    default:
        return false
    }
    return false
}


func setupServerConnection(address string, botInfo BotInfo, runningState chan(bool), wg sync.WaitGroup) (serverConnection ServerConnection, error error) {
    connection := ServerConnection{
        GameStateFromServer:    make(chan ServerMiddlewareGameState, 1),
        BotCommandToServer:     make(chan BotCommand),
    }

    //
    // Connect to the websocket server
    //
    origin := "http://localhost/" // TODO(henk): What is the origin?
    ws, err := websocket.Dial(address, "", origin)
    if err != nil {
        return connection, errors.New(fmt.Sprintf("Could not connect to server:  %v\n", err.Error()))
    }

    //
    // Registration
    //
    message := MessageMiddlewareServer{
        Type:               MmstBotInfo,
        BotCommand:         nil,
        BotInfo:            &botInfo,
    }
    websocket.JSON.Send(ws, message)


    //
    // Receive the messages from the server
    //
    wg.Add(1)
    go func() {
        defer wg.Done()
        for {

            if connectionIsTerminated(runningState, "from Server") {
                Logf(LtDebug, "Receiving from Server has stopped.\n")
                break
            }

            var message ServerMiddlewareGameState
            err := websocket.JSON.Receive(ws, &message)

            if err != nil {
                Logf(LtDebug, "Receive failed: %v\n", err.Error())
                terminateNonBlocking(runningState, "from Server 2")
                break
            }

            messageString := abbreviate(fmt.Sprintf("%v", message), 32)
            Logf(LtDebug | LtVerbose, "Received message from the server: %v.\n", messageString)


            select {
            case connection.GameStateFromServer <- message: // Put message in the channel unless it is full
                Logf(LtDebug | LtVerbose, "Added to channel 'fromServer': \"%v\"\n", messageString)
            default:
                //Logln(LtDebug, "Channel for messages from the server is full.")

                // Read message from the channel to free it and try again!
                //var message ServerMiddlewareGameState
                select {
                case msg := <-serverConnection.GameStateFromServer:
                    //message = msg
                    msg = msg
                    select {
                    case connection.GameStateFromServer <- message:
                    default:
                        Logf(LtDebug, "SHIIIIIIIT!!!\n")
                    }

                default:
                    // Right now there is nothing to read from the channel!
                    // Maybe next time :)
                    //continue
                }
            }
        }
        Logf(LtDebug, "Finished for good. 1.\n")
    }()

    //
    // Send messages to the server
    //
    wg.Add(1)
    go func() {
        defer wg.Done()
        for {

            if connectionIsTerminated(runningState, "to Server") {
                Logf(LtDebug, "Sending to Server has stopped.\n")
                break
            }

            var command BotCommand

            command = <-connection.BotCommandToServer

            message := MessageMiddlewareServer{
                Type:               MmstBotCommand,
                BotCommand:         &command,
                BotInfo:            nil,
            }

            err := websocket.JSON.Send(ws, message)
            if err != nil {
                terminateNonBlocking(runningState, "to Server 2")
                Logf(LtDebug, "Send failed: %v\n", err.Error())
                break
            }
        }
        Logf(LtDebug, "Finished for good. 2.\n")
    }()

    return connection, nil
}

func writeToBot(stdin io.WriteCloser, input string) {
    io.WriteString(stdin, input)
    //io.Copy(stdin, bytes.NewBufferString(input))
}

func readFromBot(stdout io.ReadCloser) string {
    // If this costs performance - take it out and pass the reader!
    var reader = bufio.NewReader(stdout)
    b, _ := reader.ReadString('\n')

    return string(b)
}

// to convert a float number to a string
func fToS(input_num float32) string {
    return strconv.FormatFloat(float64(input_num), 'f', 6, 32)
}

// Blob = (BotId, TeamId, Index, Position, Mass)
func blobsToString (blobs []ServerMiddlewareBlob) string {
    blobsString := "["
    first := true
    for _,blob := range blobs {
        if !first {
            blobsString += ","
        }
        positionString := "(" + fToS(blob.Position.X) + "," + fToS(blob.Position.Y) + ")"
        blobsString += "(" + fmt.Sprint(blob.BotId) + "," + fmt.Sprint(blob.TeamId) + "," + fmt.Sprint(blob.Index) + "," + positionString + "," + fmt.Sprint(blob.Mass) + ")"
        first = false
    }
    blobsString += "]"
    return blobsString
}

func foodToString (food []Food) string {
    foodString := "["
    first := true
    for _,f := range food {
        if !first {
            foodString += ","
        }
        positionString := "(" + fToS(f.Position.X) + "," + fToS(f.Position.Y) + ")"
        foodString += "(" + positionString + "," +  "10)"
        first = false
    }
    foodString += "]"

    return foodString
}

func toxinToString(toxins []Toxin) string {
    toxinString := "["
    first := true
    for _,t := range toxins {
        if !first {
            toxinString += ","
        }
        toxinString += "((" + fToS(t.Position.X) + "," + fToS(t.Position.Y) + ")," + fmt.Sprint(t.Mass) + ")"
        first = false
    }

    toxinString += "]"
    return toxinString
}

// To literally this format: ([Blob], [Blob], [Food], [Toxin])
func jsonToString(msg ServerMiddlewareGameState) string {

    myBlobString    := blobsToString(msg.MyBlob)
    otherBlobString := blobsToString(msg.OtherBlobs)
    foodString      := foodToString(msg.Food)
    toxinString     := toxinToString(msg.Toxin)
    botString       := "(" + myBlobString + "," + otherBlobString + "," + foodString + "," + toxinString + ")"

    return botString
}

func matchCommand(cmd string) (BotActionType, bool) {
    match, _ := regexp.MatchString("none|split|throw", cmd)
    action := BatNone
    switch cmd {
    case "split":
        action = BatSplit
    case "throw":
        action = BatThrow
    }
    return action, match
}

func matchTarget(slice []string) (vec.Vec2, bool) {
    x,e1 := strconv.ParseFloat(slice[0], 32)
    y,e2 := strconv.ParseFloat(slice[1], 32)

    //Logf(LtDebug, "The command %v, %v\n", e1, e2)

    if e1 != nil || e2 != nil {
        return vec.NullVec2(), false
    }

    return vec.Vec2{float32(x), float32(y)}, true
}

// Examples:
// none,739.825806,654.041382
// none,162,925
// 739.825806,654.041382
// 162,925
func matchSlice(slice []string) (BotActionType, vec.Vec2, bool) {
    var action = BatNone
    var target vec.Vec2

    switch len(slice) {
        case 2: // So it doesn't end up in default. 2 is perfectly all right. Defaults to action=None.
        case 3:
            tmpAction, matchCmd := matchCommand(slice[0])
            if !matchCmd {
                Logf(LtDebug, "The command '" + slice[0] + "' is not recognized! We use None for now... Please repair your bot!\n")
            } else {
                action = tmpAction
            }
            slice = slice[1:]
        default:
            Logf(LtDebug, "There is not enough information. It should be (Action,(Target.X, Target.Y))!\n")
            return action, target, false
    }

    tmpTarget, matchTarget := matchTarget(slice)
    if !matchTarget {
        Logf(LtDebug, "The target '" + strings.Join(slice,",") + "' is not recognized! Please repair your bot!\n")
        return action, target, false
    } else {
        target = tmpTarget
    }

    return action, target, true
}

func parseBotResponse(response string) (BotCommand, bool) {

    str := strings.ToLower(response)
    str = strings.Replace(str, " ",  "", -1)
    str = strings.Replace(str, "\n", "", -1)
    str = strings.Replace(str, "\r", "", -1)
    str = strings.Replace(str, "\t", "", -1)
    str = strings.Replace(str, "(",  "", -1)
    str = strings.Replace(str, ")",  "", -1)
    str = strings.Replace(str, "[",  "", -1)
    str = strings.Replace(str, "]",  "", -1)
    str = strings.Replace(str, "{",  "", -1)
    str = strings.Replace(str, "}",  "", -1)



    s := strings.Split(str, ",")

    action, target, ok := matchSlice(s)

    wrapper := BotCommand{
        Action:     action,
        Target:      target,
    }

    return wrapper, ok
}




func work(bot Bot, serverConnection ServerConnection, runningState chan(bool)) {
    ticker := time.NewTicker(time.Millisecond * 20)
    var lastTime = time.Now()
    var fpsAdd float32
    fpsAdd = 0
    var fpsCnt = 0
    for t := range ticker.C {
        var dt = float32(t.Sub(lastTime).Nanoseconds()) / 1e9
        lastTime = t

        if fpsCnt == 9 {
            //Logf(LtDebug, "fps: %v\n", fpsAdd / float32(fpsCnt+1))
            fpsAdd = 0
            fpsCnt = 0
        }
        fpsCnt += 1
        fpsAdd += 1.0 / dt

        if connectionIsTerminated(runningState, "work") {
            Logf(LtDebug, "Work has stopped.\n")
            break
        }

        //message := <-serverConnection.GameStateFromServer
        var message ServerMiddlewareGameState
        select {
        case msg := <-serverConnection.GameStateFromServer:
            message = msg
        default:
            // Right now there is nothing to read from the channel!
            // Maybe next time :)
            continue
        }

        messageString := jsonToString(message)

        writeToBot(bot.stdin, messageString + "\n")
        response := readFromBot(bot.stdout)

        if response == "" {
            //runningState <- false
            terminateNonBlocking(runningState, "work 2")
            Logf(LtDebug, "Work has stopped.\n")
            //close(serverConnection.BotCommandToServer)
            //close(serverConnection.GameStateFromServer)
            break
        }

        command, ok := parseBotResponse(response)

        if !ok {
            Logf(LtAlways, "Something is wrong with your bot. We could not read your response.\n")
            Logf(LtAlways, "You sent us: \"%v\"\n", response)
            Logf(LtAlways, "We sent you: \"%v\"\n", messageString)
            usage()
            os.Exit(0)
        }


        select {
            case serverConnection.BotCommandToServer <- command:
                Logf(LtDebug | LtVerbose, "Added to channel 'toServer': \"%v\"\n", command)
            default:
                //Logln(LtDebug, "Could not add message to channel 'toServer'. Channel is full.")

                // Read message from the channel to free it and try again!
                //var message ServerMiddlewareGameState
                select {
                case msg := <-serverConnection.BotCommandToServer:
                    //message = msg
                    msg = msg
                    select {
                    case serverConnection.BotCommandToServer <- command:
                    default:
                        Logf(LtDebug, "SHIIIIIIIT!!!\n")
                    }

                default:
                    // Right now there is nothing to read from the channel!
                    // Maybe next time :)
                    //continue
                }


        }
    }
}

// Reads info from config file
func readConfig(name string) Config {
    var configfile = name
    _, err := os.Stat(configfile)
    if err != nil {
        Logf(LtDebug, "Config file is missing: %v, %v\n", configfile)
        usage()
        os.Exit(1)
    }

    var config Config
    if _, err := toml.DecodeFile(configfile, &config); err != nil {
        Logf(LtDebug, "%v\n", err)
        usage()
        os.Exit(1)
    }

    return config
}


func main() {
    var wg sync.WaitGroup

    //
    // Parsing the arguments
    //
    parseResult, err := parseArguments()
    if err != nil {
        fatalExit(fmt.Sprintf("Could not parse the arguments. Error: %v\n", err.Error()), ecParameterProblem)
    }
    SetLoggingDebug(parseResult.debug)
    SetLoggingVerbose(parseResult.verbose)
    SetLoggingMute(parseResult.mute)
    SetLoggingPrefix("MIDDLEWARE")

    var config = readConfig("middleware.conf")
    if parseResult.botPath == "" {
        parseResult.botPath = config.Bot
    }
    if parseResult.botName == "" {
        parseResult.botName = config.Name
    }
    if parseResult.serverURL == "" {
        parseResult.serverURL = config.Connection
    }

    //
    // Start the bots, initialize the connections and start the work
    //
    for i := 0; i < parseResult.numBots; i++ {

        Logln(LtDebug, "soo.... number of bots: %v\n", parseResult.numBots)
        wg.Add(1)
        go func() {
            defer wg.Done()
            //
            // Start the bot
            //
            bot, err := startBot(parseResult)


            if err != nil {
                fatalExit(fmt.Sprintf("Could not start the bot. Error: %v\n", err.Error()), ecBotProblem)
            }
            defer stopBot(bot)

            //
            // Initiliaze the Websocket-Client
            //
            var address string
            if len(parseResult.serverURL) > 0 {
                address = parseResult.serverURL
            } else {
                address = globalDefaultAddress
            }
            //rand.Seed(time.Now().UTC().UnixNano())
            rand.Seed( time.Now().UnixNano())
            botInfo := BotInfo{ // TODO(henk): These are all dummy values.
                Name:       parseResult.botName,
                Color:      Color{ byte(rand.Float32() * 255), byte(rand.Float32() * 255), byte(rand.Float32() * 255) },
                ImagePath:  "",
            }
            runningState := make(chan bool, 1)
            serverConnection, err := setupServerConnection(address, botInfo, runningState, wg)
            if err != nil {
                fatalExit(fmt.Sprintf("Could not connect to Server. Error: %v\n", err.Error()), ecConnectionProblem)
            }

            //
            // Handle the stuff
            //
            work(bot, serverConnection, runningState)
            Logf(LtDebug, "Finished for good. 3.\n")
        }()

    }
    wg.Wait()
}

