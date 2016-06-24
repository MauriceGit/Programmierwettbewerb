package main

import (
    . "Programmierwettbewerb-Server/vector"
    . "Programmierwettbewerb-Server/shared"
    . "Programmierwettbewerb-Server/organisation"

    "golang.org/x/net/websocket"
    "fmt"
    "log"
    "net/http"
    "math"
    "math/rand"
    "time"
    "strconv"
    "path/filepath"
    "os"
    "encoding/json"
    "io/ioutil"
    "golang.org/x/image/bmp"
    //"image/color"
    //"image"
    //"reflect"
    //"strings"
    "html/template"
    "os/exec"
)

// -------------------------------------------------------------------------------------------------
// Global
// -------------------------------------------------------------------------------------------------

const (
    foodMassMin = 8
    foodMassMax = 12
    thrownFoodMass = 10
    massToBeAllowedToThrow = 120
    botMinMass = 10
    botMaxMass = 4000.0
    blobReunionTime = 10.0
    blobSplitMass   = 100.0
    blobSplitVelocity = 1.5
    blobMinSpeedFactor = 0.225
    toxinMassMin = 100
    toxinMassMax = 119
    windowMin = 100
    windowMax = 400
    massLossFactor = 10.0
    minBlobMassToExplode = 400.0
    maxBlobCountToExplode = 10

    velocityDecreaseFactor = 0.95

    mwMessageEvery = 1
    guiMessageEvery = 1
    guiStatisticsMessageEvery = 1
)

// -------------------------------------------------------------------------------------------------
// Application
// -------------------------------------------------------------------------------------------------

type Blob struct {
    Position     Vec2       `json:"pos"`
    Mass         float32    `json:"mass"`
    VelocityFac  float32
    IsSplit      bool
    ReunionTime  float32
    IndividualTargetVec    Vec2
}

func Radius(mass float32) float32 {
    return float32(math.Sqrt(float64(mass / math.Pi)))
}

func (blob Blob) Radius() float32 {
    return Radius(blob.Mass)
}

type ViewWindow struct {
    Position    Vec2        `json:"pos"`
    Size        Vec2        `json:"size"`
}

func isInViewWindow(viewWindow ViewWindow, position Vec2, radius float32) bool {
    return position.X > viewWindow.Position.X &&
           position.Y > viewWindow.Position.Y &&
           position.X < viewWindow.Position.X + viewWindow.Size.X &&
           position.Y < viewWindow.Position.Y + viewWindow.Size.Y;
}

type Bot struct {
    Info                BotInfo
    GuiNeedsInfoUpdate  bool
    ViewWindow          ViewWindow
    Blobs               map[BlobId]Blob
    // This is updated regulary during the game
    StatisticsThisGame  Statistics
    // This is updated once the bot dies and will be up to date for the next game
    StatisticsOverall   Statistics
    Command             BotCommand
    Connection          *websocket.Conn
    ConnectionAlive     bool                // TODO(henk): What shall we do when the connection is lost?
}

type GuiConnection struct {
    Connection          *websocket.Conn
    IsNewConnection     bool
}

type MwInfo struct {
    botId                   BotId

    command                 BotCommand

    connectionAlive         bool

    createNewBot            bool
    botInfo                 BotInfo

    statistics              Statistics

    ws                      *websocket.Conn
}


type Application struct {
    settings                    ServerSettings

    fieldSize                   Vec2
    // TODO(henk): Simply search the key of 'blobs' and assign an unused key to a newly connected client.
    nextGuiId                   GuiId
    nextBotId                   BotId
    nextBlobId                  BlobId
    nextFoodId                  FoodId
    nextToxinId                 ToxinId
    nextServerCommandId         CommandId
    guiConnections              map[GuiId]GuiConnection
    foods                       map[FoodId]Food
    toxins                      map[ToxinId]Toxin
    bots                        map[BotId]Bot

    runningState                chan(bool)
    mwInfo                      chan(MwInfo)
    serverCommands              []string

    foodDistributionName        string
    toxinDistributionName       string
    botDistributionName         string

    foodDistribution            []Vec2
    toxinDistribution           []Vec2
    botDistribution             []Vec2
}

var app Application

func (app* Application) initialize() {
    app.settings.MinNumberOfBots    = 10
    app.settings.MaxNumberOfBots    = 100
    app.settings.MaxNumberOfFoods   = 400
    app.settings.MaxNumberOfToxins  = 200

    app.fieldSize                   = Vec2{ 1000, 1000 }
    app.nextGuiId                   = 0
    app.nextBotId                   = 1
    app.nextBlobId                  = 1
    app.nextServerCommandId         = 0
    app.nextFoodId                  = FoodId(app.settings.MaxNumberOfFoods) + 1
    app.nextToxinId                 = ToxinId(app.settings.MaxNumberOfToxins) + 1
    app.guiConnections              = make(map[GuiId]GuiConnection)

    app.foods                       = make(map[FoodId]Food)
    app.bots                        = make(map[BotId]Bot)
    app.toxins                      = make(map[ToxinId]Toxin)

    app.mwInfo                      = make(chan MwInfo, 1000)
    app.runningState                = make(chan bool, 1)

    app.foodDistributionName        = "fmctechnologies.bmp"
    app.toxinDistributionName       = "fmctechnologies.bmp"
    app.botDistributionName         = "bot_spawn.bmp"

    app.foodDistribution            = loadSpawnImage(app.foodDistributionName, 10)
    app.toxinDistribution           = loadSpawnImage(app.toxinDistributionName, 10)
    app.botDistribution             = loadSpawnImage(app.botDistributionName, 10)

    for i := FoodId(0); i < FoodId(app.settings.MaxNumberOfFoods); i++ {
        mass := foodMassMin + rand.Float32() * (foodMassMax - foodMassMin)
        app.foods[i] = Food{ true, false, false, BotId(0), mass, newFoodPos(), RandomVec2() }
    }

    for i := 0; i < app.settings.MaxNumberOfToxins; i++ {
        app.toxins[ToxinId(i)] = Toxin{true, false, newToxinPos(), false, BotId(0), toxinMassMin, RandomVec2()}
        // Mulv(RandomVec2(), app.fieldSize)
    }
}

func (app* Application) createGuiId() GuiId {
    var id = app.nextGuiId
    app.nextGuiId = id + 1 // TODO(henk): Must be synchronized
    return id
}

func (app* Application) createServerCommandId() CommandId {
    var id = app.nextServerCommandId
    app.nextServerCommandId = id + 1
    return id
}

func (app* Application) createBotId() BotId {
    var id = app.nextBotId
    app.nextBotId = id + 1 // TODO(henk): Must be synchronized
    return id
}

func (app* Application) createBlobId() BlobId {
    var id = app.nextBlobId
    app.nextBlobId = id + 1
    return id
}

func (app *Application) createFoodId() FoodId {
    var id = app.nextFoodId
    app.nextFoodId = id + 1
    return id
}

func (app* Application) createToxinId() ToxinId {
    var id = app.nextToxinId
    app.nextToxinId = id + 1
    return id
}


// -------------------------------------------------------------------------------------------------
// Communication with GUI
// -------------------------------------------------------------------------------------------------

type ServerGuiUpdateMessage struct {
    // "JSON objects only support strings as keys; to encode a Go map type it must be of the form map[string]T (where T is any Go type supported by the json package)."
    // Source: http://blog.golang.org/json-and-go
    CreatedOrUpdatedBotInfos    map[string]BotInfo              `json:"createdOrUpdatedBotInfos"`
    DeletedBotInfos             []BotId                         `json:"deletedBotInfos"`
    CreatedOrUpdatedBots        map[string]ServerGuiBot         `json:"createdOrUpdatedBots"`
    DeletedBots                 []BotId                         `json:"deletedBots"`
    CreatedOrUpdatedFoods       map[string]ServerGuiFood        `json:"createdOrUpdatedFoods"`
    DeletedFoods                []FoodId                        `json:"deletedFoods"`
    CreatedOrUpdatedToxins      map[string]ServerGuiToxin       `json:"createdOrUpdatedToxins"`
    DeletedToxins               []ToxinId                       `json:"deletedToxins"`
    StatisticsThisGame          map[string]Statistics           `json:"statisticsLocal"`
    StatisticsGlobal            map[string]Statistics           `json:"statisticsGlobal"`
}

func newServerGuiUpdateMessage() ServerGuiUpdateMessage {
    return ServerGuiUpdateMessage{
        CreatedOrUpdatedBotInfos:   make(map[string]BotInfo),
        DeletedBotInfos:            make([]BotId, 0),
        CreatedOrUpdatedBots:       make(map[string]ServerGuiBot),
        DeletedBots:                make([]BotId, 0),
        CreatedOrUpdatedFoods:      make(map[string]ServerGuiFood),
        DeletedFoods:               make([]FoodId, 0),
        CreatedOrUpdatedToxins:     make(map[string]ServerGuiToxin),
        DeletedToxins:              make([]ToxinId, 0),
        StatisticsThisGame:         make(map[string]Statistics),
        StatisticsGlobal:           make(map[string]Statistics),
    }
}

type ServerGuiBot struct {
    Blobs       map[string]ServerGuiBlob    `json:"blobs"`
    ViewWindow  ViewWindow                  `json:"viewWindow"`
}

func makeServerGuiBot(bot Bot) ServerGuiBot {
    blobs := make(map[string]ServerGuiBlob)
    for blobId, blob := range bot.Blobs {
        key := strconv.Itoa(int(blobId))
        blobs[key] = makeServerGuiBlob(blob)
    }
    return ServerGuiBot{ blobs, bot.ViewWindow }
}

type ServerGuiBlob struct {
    Position    Vec2        `json:"pos"`
    Mass        float32     `json:"mass"`
}

func makeServerGuiBlob(blob Blob) ServerGuiBlob {
    return ServerGuiBlob{ blob.Position, blob.Mass }
}

type ServerGuiFood struct {
    Position    Vec2        `json:"pos"`
    Mass        float32     `json:"mass"`
}

func makeServerGuiFood(food Food) ServerGuiFood {
    return ServerGuiFood{ food.Position, food.Mass }
}

type ServerGuiToxin struct {
    Position    Vec2        `json:"pos"`
    Mass        float32     `json:"mass"`
}

func makeServerGuiToxin(toxin Toxin) ServerGuiToxin {
    return ServerGuiToxin{ toxin.Position, toxin.Mass }
}

// -------------------------------------------------------------------------------------------------

type IdPair struct {
    BotId   BotId
    BlobId  BlobId
}
type IdsContainer []IdPair

func NewIdsContainer() IdsContainer {
    return make([]IdPair, 0)
}

func (blobContainer *IdsContainer) insert(botId BotId, blobId BlobId) {
    (*blobContainer) = append((*blobContainer), IdPair{ botId, blobId })
}

// -------------------------------------------------------------------------------------------------

func makeLocalSpawnName(name string) string {
    return fmt.Sprintf("../Public/spawns/%v", name)
}

func makeURLSpawnName(name string) string {
    return fmt.Sprintf("/spawns/%v", name)
}

// -------------------------------------------------------------------------------------------------

func newFoodPos() Vec2 {
    return app.foodDistribution[rand.Intn(len(app.foodDistribution))]
}
func newToxinPos() Vec2 {
    return app.toxinDistribution[rand.Intn(len(app.toxinDistribution))]
}
func newBotPos() Vec2 {
    return app.botDistribution[rand.Intn(len(app.botDistribution))]
}

func calcBlobVelocityFromMass(vel Vec2, mass float32) Vec2 {
    // This is the maximum mass for now.
    var factor = 1.0 - mass/botMaxMass
    if mass > 0.9*botMaxMass {
        // So blobs never stop moving completely.
        factor = blobMinSpeedFactor
    }
    if Length(vel) <= 0.01 {
        vel = RandomVec2()
    }
    return Muls(NormalizeOrZero(vel), factor)
}

func calcBlobVelocity(blob *Blob, targetPos Vec2) Vec2 {
    var diff = Sub(targetPos, blob.Position)

    if blob.VelocityFac < 0.2 && Length(diff) <= 0.5 {
        diff = RandomVec2()
        return NullVec2()
    }

    var velocity = calcBlobVelocityFromMass(diff, blob.Mass)

    //Logf(LtDebug, "velocity: %v, diff: %v, mass: %v, targetPos: %v, pos: %v\n", velocity, diff, blob.Mass, targetPos, blob.Position)
    var vel = Add(velocity, Muls(velocity, blob.VelocityFac))

    if math.IsNaN(float64(vel.X)) || math.IsNaN(float64(vel.Y)) {
        vel = Vec2{0,0}
    }

    return vel
}

func calcBlobbMassLoss(mass float32, dt float32) float32 {
    if mass > botMinMass {
        return mass - (mass/botMaxMass)*dt*massLossFactor
    }
    if mass > botMaxMass {
        return botMaxMass
    }
    return mass
}

func pushBlobsApart(blobs* map[BlobId]Blob) {
    for index, subBlob := range *blobs {
        for index2, subBlob2 := range *blobs {
            // Just move them out of each other, if it is not newly split!
            if index != index2 && subBlob.VelocityFac < 1.1 && subBlob2.VelocityFac < 1.1 {
                var dist = Dist(subBlob.Position, subBlob2.Position)
                var minDist  = subBlob.Radius() + subBlob2.Radius()

                // ToDo(Maurice): Make reunion time dynamic!
                if subBlob.ReunionTime <= 1.0 {
                    minDist *= subBlob.ReunionTime
                }

                var distDiff = minDist - dist
                // Push them out from each other
                if distDiff > 0 {
                    //fmt.Println("Pushing!")
                    var sub = Sub(subBlob.Position, subBlob2.Position)
                    if Length(sub) <= 0.01 {
                        sub = RandomVec2()
                    }
                    var dir = Muls(NormalizeOrZero(sub), distDiff/2)
                    var tmp = (*blobs)[index]
                    tmp.Position  = Add((*blobs)[index].Position, dir)
                    (*blobs)[index] = tmp
                    var tmp2 = (*blobs)[index2]
                    tmp2.Position = Sub((*blobs)[index2].Position, dir)
                    (*blobs)[index2] = tmp2

                }
            }
        }
    }
}

func calcSubblobReunion(killedBlobs *IdsContainer, botId BotId, bot *Bot) {
    for k,subBlob := range (*bot).Blobs {
        for k2,subBlob2 := range (*bot).Blobs {
            if k != k2 {
                var dist = Dist(subBlob.Position, subBlob2.Position)
                var shouldBe = subBlob.Radius() + subBlob2.Radius()
                if subBlob2.ReunionTime < 0.1 && dist < shouldBe && shouldBe - dist > subBlob2.Radius() {
                    // Merge them together.
                    var tmp = (*bot).Blobs[k]
                    tmp.Mass += (*bot).Blobs[k2].Mass
                    tmp.IsSplit = false
                    (*bot).Blobs[k] = tmp

                    // Delete blob
                    killedBlobs.insert(botId, k2)
                    delete((*bot).Blobs, k2)

                }
            }
        }
    }
}

func splitAllBlobsOfBot(bot *Bot) {
    var newBlobMap = make(map[BlobId]Blob)
    for subBlobToSplit, subBlob := range (*bot).Blobs {
        // Just split if bigger than 100
        if (*bot).Blobs[subBlobToSplit].Mass >= blobSplitMass {
            var newMass = subBlob.Mass / 2.0

            // Override the old mass and time to reunion, so it is not eaten right away.
            var tmp = (*bot).Blobs[subBlobToSplit]
            tmp.Mass = newMass
            tmp.ReunionTime = subBlob.ReunionTime + 1.0
            (*bot).Blobs[subBlobToSplit] = tmp

            var newIndex = app.createBlobId()
            newBlobMap[newIndex] = Blob{ subBlob.Position, newMass, blobSplitVelocity, true, blobReunionTime, NullVec2()}
        }
    }

    // Just so we don't edit the map while iterating over it!
    for index,blob := range newBlobMap {
        (*bot).Blobs[index] = blob
    }
}

func throwAllBlobsOfBot(bot *Bot, botId BotId) bool {
    somebodyThrew := false
    for blobId, blob := range (*bot).Blobs {
        if blob.Mass > massToBeAllowedToThrow {
            foodId := app.createFoodId()
            sub := Sub(bot.Command.Target, blob.Position)
            if Length(sub) <= 0.01 {
                sub = RandomVec2()
            }
            targetDirection := NormalizeOrZero(sub)
            food := Food{
                IsNew:    true,
                IsMoving: true,
                IsThrown: true,
                IsThrownBy: botId,
                Mass:     thrownFoodMass,
                Position: Add(blob.Position, Muls(targetDirection, 1.5*(blob.Radius() + Radius(thrownFoodMass)))),
                Velocity: Muls(targetDirection, 150),
            }
            app.foods[foodId] = food

            blob.Mass = blob.Mass - thrownFoodMass

            somebodyThrew = true
        }
        (*bot).Blobs[blobId] = blob
    }
    return somebodyThrew
}

// Calculates a vectors which are equally divided on a circle with given radius!
func randomVecOnCircle(radius float32) Vec2 {
    var angle = rand.Float64() * math.Pi * 2.0;
    var x = float32(math.Cos(angle)) * radius;
    var y = float32(math.Sin(angle)) * radius;
    return Vec2{x, y}
}

func explodeBlob(botId BotId, blobId BlobId, newMap *map[BlobId]Blob)  {

    blobCount := 12
    splitRadius := float32(3.0)

    // ToDo(Maurice): Make exploded Bubbles in random/different sizes with one of them
    // consisting of half the mass!
    blob := app.bots[botId].Blobs[blobId]
    for i := 0; i < blobCount; i++ {
        newIndex  := app.createBlobId()
        (*newMap)[newIndex] = Blob{
            Add(RandomVec2(), blob.Position),
            blob.Mass/float32(blobCount),
            // We need the 0.2 (or something small!) here, so that we don't
            // try to push them apart in the first step. Otherwise we have the exact same
            // position and the diff-Vector is undefined/null. Can't push them apart!
            blob.VelocityFac+0.1,
            false,
            10.0,
            // Random Vector with about same length. Should be uniformly divided!
            randomVecOnCircle(splitRadius),
        }
    }
}

func makeServerMiddlewareBlob(botId BotId, blobId BlobId, blob Blob) ServerMiddlewareBlob {
    return ServerMiddlewareBlob{
        BotId:  uint32(botId),
        TeamId: uint32(botId),
        Index:  uint32(blobId),
        Position: blob.Position,
        Mass:   uint32(blob.Mass),
    }
}

func makeServerMiddlewareBlobs(botId BotId) []ServerMiddlewareBlob {
    var blobArray []ServerMiddlewareBlob

    for blobId, blob := range app.bots[botId].Blobs {
        blobArray = append(blobArray, makeServerMiddlewareBlob(botId, blobId, blob))
    }

    return blobArray
}

func limitPosition(position *Vec2) {
    if (*position).X < 0 { (*position).X = 0 }
    if (*position).Y < 0 { (*position).Y = 0 }
    if (*position).X > app.fieldSize.X { (*position).X = app.fieldSize.X }
    if (*position).Y > app.fieldSize.Y { (*position).Y = app.fieldSize.Y }
}

func checkNaNV (v Vec2, prefix string, s string) {
    if math.IsNaN(float64(v.X)) || math.IsNaN(float64(v.Y)) {
        Logf(LtDebug, "NaN ( Vec ) is found for: __%v__ %v\n", prefix, s)
    }
}
func checkNaNF (f float32, prefix string, s string) {
    if math.IsNaN(float64(f)) {
        Logf(LtDebug, "NaN (Float) is found for: __%v__ %v\n", prefix, s)
    }
}
func checkAllValuesOnNaN(prefix string) {
    for _,bot := range app.bots {
        checkNaNV(bot.ViewWindow.Position, prefix, "bot.ViewWindow.Position")
        checkNaNV(bot.ViewWindow.Size, prefix, "bot.ViewWindow.Size")
        for _,blob := range bot.Blobs {
            checkNaNV(blob.Position, prefix, "blob.Position")
            checkNaNV(blob.IndividualTargetVec, prefix, "blob.IndividualTargetVec")
            checkNaNF(blob.Mass, prefix, "blob.Mass")
            checkNaNF(blob.ReunionTime, prefix, "blob.ReunionTime")
            checkNaNF(blob.VelocityFac, prefix, "blob.VelocityFac")
        }
    }
    for _,food := range app.foods {
        checkNaNF(food.Mass, prefix, "food.Mass")
        checkNaNV(food.Position, prefix, "food.Position")
        checkNaNV(food.Velocity, prefix, "food.Velocity")
    }
    for _,toxin := range app.toxins {
        checkNaNV(toxin.Position, prefix, "toxin.Position")
        checkNaNF(toxin.Mass, prefix, "toxin.Mass")
        checkNaNV(toxin.Velocity, prefix, "toxin.Velocity")
    }
}

func startMiddleware() {
    process := exec.Command("./Programmierwettbewerb-Middleware", "-bot=../BotPython/script.py", "-numBots=1", "-name=svn_03")
    process.Dir = "./"

    if err := process.Start(); err != nil {
        Logf(LtDebug, "error trying to start a middleware: %v\n", err)
    }
}

func updateServerFromGit() {
    fetch := exec.Command("git", "fetch", "--all")
    fetch.Dir = "../"
    if err := fetch.Start(); err != nil {
        Logf(LtDebug, "error on git fetch: %v\n", err)
    }

    reset := exec.Command("git", "reset", "--hard origin/master")
    reset.Dir = "../"
    if err := reset.Start(); err != nil {
        Logf(LtDebug, "error on git fetch: %v\n", err)
    }

    make := exec.Command("../make.sh")
    if err := make.Start(); err != nil {
        Logf(LtDebug, "error on git fetch: %v\n", err)
    }

}

func restartServer() {
    //restart := exec.Command("sleep 5; ./Programmierwettbewerb-Server &")
    restart := exec.Command("/bin/bash", "test.sh", "&")
    //restart.Dir = "./"
    restart.SysProcAttr.Setpgid = true
    if err := restart.Start(); err != nil {
        Logf(LtDebug, "error on restarting the server: %v\n", err)
    }
}

func (app* Application) startUpdateLoop() {
    ticker := time.NewTicker(time.Millisecond * 30)
    var lastTime = time.Now()

    var fpsAdd float32
    fpsAdd = 0
    var fpsCnt = 0

    var guiMessageCounter = 0
    var mWMessageCounter = 0
    var guiStatisticsMessageCounter = 0

    var lastMiddlewareStart = float32(0.0)

    for t := range ticker.C {
        var dt = float32(t.Sub(lastTime).Nanoseconds()) / 1e9
        lastTime = t

        if dt >= 0.03 {
            dt = 0.03
        }

        // Once every 10 seconds
        if fpsCnt == 300 {
            //Logf(LtDebug, "fps: %v\n", fpsAdd / float32(fpsCnt+1))

            for _,bot := range app.bots {
                go WriteStatisticToFile(bot.Info.Name, bot.StatisticsThisGame)
            }

            fpsAdd = 0
            fpsCnt = 0
        }
        fpsCnt += 1
        fpsAdd += 1.0 / dt

        // Eating blobs
        deadBots := make([]BotId, 0)

        ////////////////////////////////////////////////////////////////
        // HANDLE EVENTS
        ////////////////////////////////////////////////////////////////
        if len(app.serverCommands) > 0 {
            for _, commandString := range app.serverCommands {
                type Command struct {
                    Type    string  `json:"type"`
                    Value   int     `json:"value,string,omitempty"`
                    Image   string  `json:"image"`
                }

                var command Command
                err := json.Unmarshal([]byte(commandString), &command)
                if err == nil {
                    switch command.Type {
                    case "MinNumberOfBots":
                        app.settings.MinNumberOfBots = command.Value
                    case "MaxNumberOfBots":
                        app.settings.MaxNumberOfBots = command.Value
                    case "MaxNumberOfFoods":
                        app.settings.MaxNumberOfFoods = command.Value
                    case "MaxNumberOfToxins":
                        app.settings.MaxNumberOfToxins = command.Value
                    case "UpdateServer":
                        go updateServerFromGit()
                    case "RestartServer":
                        go restartServer()

                        terminateNonBlocking(app.runningState)
                        Logf(LtDebug, "Server is shutting down.\n")
                        // We give the other go routines 1 second to gracefully shut down!
                        time.Sleep(3000 * time.Millisecond)
                        Logf(LtDebug, "Sleep finished\n")
                        os.Exit(1)

                    case "KillAllBots":
                        for botId, _ := range app.bots {
                            delete(app.bots, botId)
                            deadBots = append(deadBots, botId)
                        }
                        Logf(LtDebug, "Killed all bots\n")
                    case "KillBotsWithoutConnection":
                        for botId, bot := range app.bots {
                            if !bot.ConnectionAlive {
                                delete(app.bots, botId)
                                deadBots = append(deadBots, botId)
                            }
                        }
                        Logf(LtDebug, "Killed bots without connection\n")
                    case "KillBotsAboveMassThreshold":
                        for botId, bot := range app.bots {
                            var mass float32 = 0
                            for _, blob := range bot.Blobs {
                                mass += blob.Mass
                            }
                            if mass > float32(command.Value) {
                                delete(app.bots, botId)
                                deadBots = append(deadBots, botId)
                            }
                        }
                        Logf(LtDebug, "Killed bots above mass threshold\n")
                    case "FoodSpawnImage":
                        app.foodDistribution  = loadSpawnImage(command.Image, 10)
                    case "ToxinSpawnImage":
                        app.toxinDistribution = loadSpawnImage(command.Image, 10)
                    case "BotSpawnImage":
                        app.botDistribution   = loadSpawnImage(command.Image, 10)
                    }
                } else {
                    Logf(LtDebug, "Err: %v\n", err.Error())
                }
            }

            app.serverCommands = make([]string, 0)
        }

        ////////////////////////////////////////////////////////////////
        // READ FROM MIDDLEWARE
        ////////////////////////////////////////////////////////////////
        for {
            finished := false
            select {
            case mwInfo, ok := <-app.mwInfo:
                if ok {
                    if _, ok := app.bots[mwInfo.botId]; ok {
                        bot := app.bots[mwInfo.botId]

                        bot.ConnectionAlive = mwInfo.connectionAlive

                        // There is actually a command
                        if (BotCommand{}) != mwInfo.command {
                            bot.Command = mwInfo.command
                        }

                        app.bots[mwInfo.botId] = bot
                    }
                    if mwInfo.createNewBot && len(app.bots) < app.settings.MaxNumberOfBots {
                        app.bots[mwInfo.botId] = createStartingBot(mwInfo.ws, mwInfo.botInfo, mwInfo.statistics)
                    }

                } else {
                    // Channel closed. Something is SERIOUSLY wrong.
                    Logf(LtDebug, "Something is SERIOUSLY wrong\n")
                }
            default:
                // No value to read
                finished = true
            }
            if finished {
                break
            }
        }




        ////////////////////////////////////////////////////////////////
        // ADD SOME MIDDLEWARES/BOTS IF NEEDED
        ////////////////////////////////////////////////////////////////
        if lastMiddlewareStart > 2 {
            if app.settings.MinNumberOfBots - len(app.bots) > 0 {
                //botsToStart := app.settings.MinNumberOfBots - len(app.bots)
                botsToStart := 1
                for i:=0; i<botsToStart; i++ {
                    go startMiddleware()
                }
                lastMiddlewareStart = 0
            }
        }
        lastMiddlewareStart += dt


        ////////////////////////////////////////////////////////////////
        // UPDATE BOT POSITION
        ////////////////////////////////////////////////////////////////

        //
        // Update all Bots
        //

        for botId, bot := range app.bots {
            for blobId, blob := range bot.Blobs {
                oldPosition := blob.Position
                velocity    := calcBlobVelocity(&blob, bot.Command.Target)
                time        := dt * 50
                newVelocity := Muls(velocity, time)
                newPosition := Add (oldPosition, newVelocity)
                newPosition =  Add (newPosition, blob.IndividualTargetVec)

                //Logf(LtDebug, "old: %v; new: %v, velocity: %v, time: %v\n", oldPosition, newPosition, velocity, time)

                blob.Position = newPosition

                //singleBlob.Position = Add(singleBlob.Position, Muls(calcBlobVelocity(&singleBlob, blob.IndividualTargetVec), dt * 100))
                blob.Mass = calcBlobbMassLoss(blob.Mass, dt)
                blob.VelocityFac = blob.VelocityFac * velocityDecreaseFactor

                // So this is not added all the time but just for a short moment!
                blob.IndividualTargetVec = Muls(blob.IndividualTargetVec, velocityDecreaseFactor)

                if blob.ReunionTime > 0.0 {
                    blob.ReunionTime -= dt
                }

                limitPosition(&blob.Position)

                app.bots[botId].Blobs[blobId] = blob
            }
            app.bots[botId] = bot
        }



        ////////////////////////////////////////////////////////////////
        // UPDATE VIEW WINDOWS
        ////////////////////////////////////////////////////////////////


        for botId, bot := range app.bots {
            //var diameter float32
            var center Vec2
            var completeMass float32 = 0
            for _, blob1 := range bot.Blobs {
                center = Add(center, blob1.Position)
                completeMass += blob1.Mass

                /*for _, blob2 := range bot.Blobs {
                    distance := Dist(blob1.Position, blob2.Position) + blob1.Radius() + blob2.Radius()
                    if distance > diameter {
                        diameter = distance
                    }
                }*/
            }
            center = Muls(center, 1.0 / float32(len(bot.Blobs)))


            // TODO(henk): Adjust this to the original.
            //var maxEnlargement float64 = 15.0
            //var decreaseFactor float32 = 0.01
            //windowDiameter := float32(maxEnlargement * math.Pow(math.E, float64(-decreaseFactor * diameter))) * diameter
            var windowDiameter float32 = 30.0 * float32(math.Log(float64(completeMass))) - 20.0



            //Logf(LtDebug, "Complete mass of bot %v = %v, window diameter = %v\n", botId, completeMass, windowDiameter)

            //Logf(LtDebug, "ViewWindow-Data: center: %v, windowDiameter: %v, maxEnlargement: %v, diameter: %v\n", center, windowDiameter, maxEnlargement, diameter)

            bot.ViewWindow = ViewWindow{
                Position:   Sub(center, Vec2{ windowDiameter / 2.0, windowDiameter / 2.0 }),
                Size: Vec2{ windowDiameter, windowDiameter },
            }
            app.bots[botId] = bot
        }



        //checkAllValuesOnNaN("first")

        ////////////////////////////////////////////////////////////////
        // POSSIBLY ADD A FOOD OR TOXIN
        ////////////////////////////////////////////////////////////////
        if rand.Intn(100) <= 5 && len(app.toxins) < app.settings.MaxNumberOfToxins {
            newToxinId := app.createToxinId()
            app.toxins[newToxinId] = Toxin{true, false, newToxinPos(), false, 0, toxinMassMin, RandomVec2()}
        }
        if rand.Intn(100) <= 5 && len(app.foods) < app.settings.MaxNumberOfFoods {
            newFoodId := app.createFoodId()
            mass := foodMassMin + rand.Float32() * (foodMassMax - foodMassMin)
            app.foods[newFoodId] = Food{ true, false, false, 0, mass, newFoodPos(), RandomVec2() }
        }

        ////////////////////////////////////////////////////////////////
        // UPDATE FOOD POSITION
        ////////////////////////////////////////////////////////////////
        for foodId, food := range app.foods {
            if food.IsMoving {
                food.Position = Add(food.Position, Muls(food.Velocity, dt))
                limitPosition(&food.Position)
                food.Velocity = Muls(food.Velocity, velocityDecreaseFactor)
                app.foods[foodId] = food
                if Length(food.Velocity) <= 0.001 {
                    food.IsMoving = false
                }
            }
        }

        ////////////////////////////////////////////////////////////////
        // UPDATE TOXIN POSITION
        ////////////////////////////////////////////////////////////////
        for toxinId, toxin := range app.toxins {
            if toxin.IsMoving {
                toxin.Position = Add(toxin.Position, Muls(toxin.Velocity, dt))
                limitPosition(&toxin.Position)
                toxin.Velocity = Muls(toxin.Velocity, velocityDecreaseFactor)
                app.toxins[toxinId] = toxin
                if Length(toxin.Velocity) <= 0.001 {
                    toxin.IsMoving = false
                }
            }
            app.toxins[toxinId] = toxin
        }

        ////////////////////////////////////////////////////////////////
        // DELETE RANDOM TOXIN IF THERE ARE TOO MANY
        ////////////////////////////////////////////////////////////////
        eatenToxins := make([]ToxinId, 0)
        for toxinId,_ := range app.toxins {
            if len(app.toxins) <= app.settings.MaxNumberOfToxins {
                break;
            }
            eatenToxins = append(eatenToxins, toxinId)
            delete(app.toxins, toxinId)
        }

        ////////////////////////////////////////////////////////////////
        // DELETE RANDOM FOOD IF THERE ARE TOO MANY
        ////////////////////////////////////////////////////////////////
        eatenFoods := make([]FoodId, 0)
        for foodId,_ := range app.foods {
            if len(app.foods) <= app.settings.MaxNumberOfFoods {
                break;
            }
            eatenFoods = append(eatenFoods, foodId)
            delete(app.foods, foodId)
        }

        ////////////////////////////////////////////////////////////////
        // BOT INTERACTION WITH EVERYTHING
        ////////////////////////////////////////////////////////////////

        killedBlobs := NewIdsContainer()

        // Split command received - Creating new Blob with same ID.

        for botId, bot := range app.bots {
            if bot.Command.Action == BatSplit && len(bot.Blobs) <= 10 {

                var bot = app.bots[botId]
                bot.StatisticsThisGame.SplitCount += 1
                var botRef = &bot
                splitAllBlobsOfBot(botRef)
                app.bots[botId] = *botRef

            } else if bot.Command.Action == BatThrow {
                bot := app.bots[botId]
                throwAllBlobsOfBot(&bot, botId)
                app.bots[botId] = bot
            }
        }

        // Splitting the Toxin!
        for toxinId, toxin := range app.toxins {

            if toxin.Mass > toxinMassMax {

                // Create new Toxin (moving!)
                newId := app.createToxinId()
                newToxin := Toxin {
                    IsNew: true,
                    IsMoving: true,
                    Position: toxin.Position,
                    IsSplit: true,
                    IsSplitBy: toxin.IsSplitBy,
                    Mass: toxinMassMin,
                    Velocity: toxin.Velocity,
                }
                app.toxins[newId] = newToxin

                possibleBot, foundIt := app.bots[toxin.IsSplitBy]
                if foundIt {
                    possibleBot.StatisticsThisGame.ToxinThrow += 1
                    app.bots[toxin.IsSplitBy] = possibleBot
                }

                // Reset Mass
                toxin.Mass = toxinMassMin
                toxin.IsSplit = false
                toxin.IsNew = false
                toxin.IsSplitBy = BotId(0)
                toxin.Velocity = RandomVec2()

            }
            app.toxins[toxinId] = toxin
        }

        // Reunion of Subblobs
        for botId, _ := range app.bots {
            var bot = app.bots[botId]
            var botRef = &bot
            // Reunion of Subblobs
            calcSubblobReunion(&killedBlobs, botId, botRef)
            app.bots[botId] = *botRef
        }

        // Blob Collision with Toxin
        //eatenToxins := make([]ToxinId, 0)
        for tId,_ := range app.toxins {
            var toxin = app.toxins[tId]

            for botId, _ := range app.bots {
                bot := app.bots[botId]

                mapOfAllNewSingleBlobs := make(map[BlobId]Blob)
                var blobsToDelete []BlobId
                var exploded = false

                // This loop should not alter ANY real data at all right now!
                // Just writing to tmp maps without alterning real data.
                for blobId,_ := range bot.Blobs {
                    var singleBlob = bot.Blobs[blobId]

                    if Dist(singleBlob.Position, toxin.Position) < singleBlob.Radius() && singleBlob.Mass >= minBlobMassToExplode {

                        // If a bot already has > 10 blobs (i.e.), don't explode, ignore!
                        if len(bot.Blobs) > maxBlobCountToExplode {
                            if toxin.IsSplit || len(app.toxins) >= app.settings.MaxNumberOfToxins {
                                eatenToxins = append(eatenToxins, tId)
                                delete(app.toxins, tId)
                                //Logf(LtDebug, "Test 0: len before: %v, eatenToxins: %v\n", len(eatenToxins), eatenToxins)
                                //Logf(LtDebug, "Test 0\n")
                            } else {

                                toxin.Position = newToxinPos()
                                toxin.IsSplitBy = BotId(0)
                                toxin.IsSplit = false
                                toxin.IsNew = true
                                toxin.Mass = toxinMassMin
                                //Logf(LtDebug, "Test 1\n")

                            }
                            continue
                        }

                        subMap := make(map[BlobId]Blob)

                        if toxin.IsSplit {
                            possibleBot, foundIt := app.bots[toxin.IsSplitBy]
                            if foundIt {
                                possibleBot.StatisticsThisGame.SuccessfulToxin += 1
                                app.bots[toxin.IsSplitBy] = possibleBot
                            }
                        }

                        explodeBlob(botId, blobId, &subMap)
                        exploded = true

                        // Add all the new explosions:
                        for i,b := range subMap {
                            mapOfAllNewSingleBlobs[i] = b
                        }

                        blobsToDelete = append(blobsToDelete, blobId)
                        //eatenToxins = append(eatenToxins, tId)

                        toxin.Position = newToxinPos()
                        toxin.IsSplitBy = BotId(0)
                        toxin.IsSplit = false
                        toxin.IsNew = true
                        toxin.Mass = toxinMassMin
                        //delete(app.toxins, tId)
                    }
                }

                if exploded {
                    // Delete the origin of the exploded Blobs.
                    for _,blobKey := range blobsToDelete {
                        killedBlobs.insert(botId, blobKey)
                        delete(bot.Blobs, blobKey)
                    }

                    // Add all new exploded Blobs.
                    for i,b := range mapOfAllNewSingleBlobs {
                        bot.Blobs[i] = b
                    }
                }

                app.bots[botId] = bot
            }

            app.toxins[tId] = toxin

        }

        //checkAllValuesOnNaN("third 2")

        //
        // Push Blobs apart
        //

        for botId, _ := range app.bots {

            var blob = app.bots[botId]

            var tmpA = app.bots[botId].Blobs
            pushBlobsApart(&tmpA)
            blob.Blobs = tmpA

            app.bots[botId] = blob
        }



        //checkAllValuesOnNaN("third 3")

        //
        // Eating food (by bots and toxins!)
        //
        //eatenFoods := make([]FoodId, 0)
        for foodId, food := range app.foods {
            // Bots eating food

            for botId, bot := range app.bots {
                for blobId, blob := range bot.Blobs {

                    if Length(Sub(food.Position, blob.Position)) < blob.Radius() {
                        blob.Mass = blob.Mass + food.Mass
                        if food.IsThrown {
                            delete(app.foods, foodId)
                            eatenFoods = append(eatenFoods, foodId)
                        } else {
                            food.Position = newFoodPos()
                            food.IsNew = true
                            app.foods[foodId] = food
                        }
                    }
                    bot.Blobs[blobId] = blob
                }
                app.bots[botId] = bot
            }



            // Toxins eating food (even if accidently!)
            for tId, toxin := range app.toxins {
                if food.IsThrown {
                    if Length(Sub(food.Position, toxin.Position)) < Radius(toxin.Mass) {
                        toxin.Mass = toxin.Mass + food.Mass
                        // Always get the velocity of the last eaten food so the toxin (when split)
                        // gets the right velocity of the last input.
                        if Length(food.Velocity) <= 0.01 {
                            food.Velocity = RandomVec2()
                        }
                        toxin.IsSplitBy = food.IsThrownBy
                        //Logf(LtDebug, "Food is thrown by %v\n", toxin.IsSplitBy)
                        toxin.Velocity = Muls(NormalizeOrZero(food.Velocity), 100)

                        delete(app.foods, foodId)
                        eatenFoods = append(eatenFoods, foodId)

                    }
                }
                app.toxins[tId] = toxin
            }
        }

        for botId1, bot1 := range app.bots {

            var bot1Mass float32
            for blobId1, blob1 := range app.bots[botId1].Blobs {
                //blob1 := bot1.Blobs[blobId1]
                bot1Mass += blob1.Mass

                for botId2, bot2 := range app.bots {
                    if botId1 != botId2 {
                        //bot2 := app.bots[botId2]
                        for blobId2, blob2 := range app.bots[botId2].Blobs {
                            //blob2 := bot2.Blobs[blobId2]

                            inRadius := Dist(blob2.Position, blob1.Position) < blob1.Radius()
                            smaller := blob2.Mass < 0.9*blob1.Mass

                            if smaller && inRadius {
                                blob1.Mass = blob1.Mass + blob2.Mass

                                bot1.StatisticsThisGame.BlobKillCount += 1

                                if blob1.IsSplit {
                                    bot1.StatisticsThisGame.SuccessfulSplit += 1
                                }

                                killedBlobs.insert(botId2, blobId2)

                                delete(app.bots[botId2].Blobs, blobId2)

                                // Completely delete this bot.
                                if len(app.bots[botId2].Blobs) <= 0 {
                                    deadBots = append(deadBots, botId2)

                                    bot1.StatisticsThisGame.BotKillCount += 1

                                    go WriteStatisticToFile(bot2.Info.Name, bot2.StatisticsThisGame)

                                    bot2.Connection.Close()
                                    delete(app.bots, botId2)
                                    break
                                }

                            }
                        }
                    }
                }

                app.bots[botId1].Blobs[blobId1] = blob1
            }

            stats := bot1.StatisticsThisGame
            stats.MaxSize = float32(math.Max(float64(stats.MaxSize), float64(bot1Mass)))
            stats.MaxSurvivalTime += dt

            bot1.StatisticsThisGame = stats
            app.bots[botId1] = bot1
        }

        ////////////////////////////////////////////////////////////////
        // CHECK ANYTHING ON NaN VALUES
        ////////////////////////////////////////////////////////////////
        checkAllValuesOnNaN("end")

        ////////////////////////////////////////////////////////////////
        // SEND UPDATED DATA TO MIDDLEWARE AND GUI
        ////////////////////////////////////////////////////////////////

        // TODO(Maurice/henk):
        // MAURICE: Could be VERY MUCH simplified, if we just convert All blobs to the right format once and then sort out later who gets what data!
        // HENK: First we would have to determine which blobs, toxins and foods are visible to the bots. We would have to profile that to check whats faster.
        if mWMessageCounter % mwMessageEvery == 0 {

            for botId, bot := range app.bots {
                var connection = app.bots[botId].Connection

                // Collecting other blobs
                var otherBlobs []ServerMiddlewareBlob
                for otherBotId, otherBot := range app.bots {
                    if botId != otherBotId {
                        for otherBlobId, otherBlob := range otherBot.Blobs {
                            if isInViewWindow(bot.ViewWindow, otherBlob.Position, otherBlob.Radius()) {
                                otherBlobs = append(otherBlobs, makeServerMiddlewareBlob(otherBotId, otherBlobId, otherBlob))
                            }
                        }
                    }
                }

                // Collecting foods
                var foods []Food
                for _, food := range app.foods {
                    if isInViewWindow(bot.ViewWindow, food.Position, Radius(food.Mass)) {
                        foods = append(foods, food)
                    }
                }

                // Collecting toxins
                var toxins []Toxin
                for _, toxin := range app.toxins {
                    if isInViewWindow(bot.ViewWindow, toxin.Position, Radius(toxin.Mass)) {
                        toxins = append(toxins, toxin)
                    }
                }

                var wrapper = ServerMiddlewareGameState{
                    MyBlob:         makeServerMiddlewareBlobs(botId),
                    OtherBlobs:     otherBlobs,
                    Food:           foods,
                    Toxin:          toxins,
                }
                websocket.JSON.Send(connection, wrapper)
            }


        }

        // Pre-calculate the list for eaten Toxins
        //reallyDeadToxins := make([]ToxinId, 0)
        //copied := copy(reallyDeadToxins, eatenToxins)
        //Logf(LtDebug, "copied %v elements\n", copied)

        //
        // Send the data to the clients
        //
        for guiId, guiConnection := range app.guiConnections {
            var connection = app.guiConnections[guiId].Connection

            message := newServerGuiUpdateMessage()

            //Logf(LtDebug, "Botcount: %v\n", app.bots)

            for botId, bot := range app.bots {

                key := strconv.Itoa(int(botId))
                if bot.GuiNeedsInfoUpdate || guiConnection.IsNewConnection {
                    message.CreatedOrUpdatedBotInfos[key] = bot.Info
                }

                if guiMessageCounter % guiMessageEvery == 0 {
                    message.CreatedOrUpdatedBots[key] = makeServerGuiBot(bot)
                }

                if guiStatisticsMessageCounter % guiStatisticsMessageEvery == 0 {
                    message.StatisticsThisGame[key] = bot.StatisticsThisGame
                    // @Todo: The global one MUCH more rarely!
                    message.StatisticsGlobal[key] = bot.StatisticsOverall
                }

            }

            message.DeletedBotInfos = deadBots
            message.DeletedBots = deadBots
            //for _, botId := range deadBots {
            //    message.DeletedBots = append(message.DeletedBots, botId)
            //    message.DeletedBotInfos = append(message.DeletedBotInfos, botId)
            //}

            if guiMessageCounter % guiMessageEvery == 0 {
                for foodId, food := range app.foods {
                    if food.IsMoving || food.IsNew || guiConnection.IsNewConnection {
                        key := strconv.Itoa(int(foodId))
                        message.CreatedOrUpdatedFoods[key] = makeServerGuiFood(food)
                    }
                }
            }

            message.DeletedFoods = eatenFoods
            //for _, foodId := range eatenFoods {
            //    message.DeletedFoods = append(message.DeletedFoods, foodId)
            //}

            if guiMessageCounter % guiMessageEvery == 0 {
                for toxinId, toxin := range app.toxins {
                    if toxin.IsNew || toxin.IsMoving || guiConnection.IsNewConnection {
                        key := strconv.Itoa(int(toxinId))
                        message.CreatedOrUpdatedToxins[key] = makeServerGuiToxin(toxin)
                    }
                }
            }

            message.DeletedToxins = eatenToxins

            err := websocket.JSON.Send(connection, message)
            if err != nil {
                Logf(LtDebug, "JSON could not be sent because of: %v\n", err)
                //return
            }

        }

        for toxinId, toxin := range app.toxins {
            if toxin.IsNew {
                toxin.IsNew = false
                app.toxins[toxinId] = toxin
            }
        }
        for foodId, food := range app.foods {
            if food.IsNew {
                food.IsNew = false
                app.foods[foodId] = food
            }
        }


        for botId, bot := range app.bots {
            bot.GuiNeedsInfoUpdate = false
            app.bots[botId] = bot
        }



        for guiConnectionId, guiConnection := range app.guiConnections {
            guiConnection.IsNewConnection = false
            app.guiConnections[guiConnectionId] = guiConnection

            err := websocket.Message.Send(guiConnection.Connection, "alive_test")
            if err != nil {
                Logf(LtDebug, "Gui %v is deleted because of network failure. Alive test failed.\n", guiConnectionId)
                delete(app.guiConnections, guiConnectionId)
            }
        }
        guiMessageCounter += 1
        mWMessageCounter += 1
        guiStatisticsMessageCounter += 1
    }
}

func handleGui(ws *websocket.Conn) {
    var guiId          = app.createGuiId()         // TODO(henk): When do we delete the blob?

    app.guiConnections[guiId] = GuiConnection{ ws, true }

    Logf(LtDebug, "Got connection for Gui %v\n", guiId)

    var err error
    for {

        if connectionIsTerminated(app.runningState) {
            Logf(LtDebug, "HandleGui is shutting down.\n")
            ws.Close()
            return
        }

        var reply string

        if err = websocket.Message.Receive(ws, &reply); err != nil {
            Logf(LtDebug, "Can't receive (%v)\n", err)
            break
        }
    }
}

func createStartingBot(ws *websocket.Conn, botInfo BotInfo, statistics Statistics) Bot {
    blob := Blob {
        Position:       newBotPos(),  // TODO(henk): How do we decide this?
        Mass:           100.0,
        VelocityFac:    1.0,
        IsSplit:        false,
        ReunionTime:    0.0,
        IndividualTargetVec:      NullVec2(),
    }
    statisticNew := Statistics{
        MaxSize:            100.0,
        MaxSurvivalTime:    0.0,
        BlobKillCount:      0,
        BotKillCount:       0,
        ToxinThrow:         0,
        SuccessfulToxin:    0,
        SplitCount:         0,
        SuccessfulSplit:    0,
        SuccessfulTeam:     0,
        BadTeaming:         0,
}

    return Bot{
        Info:                   botInfo,
        GuiNeedsInfoUpdate:     true,
        ViewWindow:             ViewWindow{ Position: Vec2{0,0}, Size:Vec2{100,100} },
        Blobs:                  map[BlobId]Blob{ 0: blob },
        StatisticsThisGame:     statisticNew,
        StatisticsOverall:      statistics,
        Command:                BotCommand{ BatNone, RandomVec2(), },
        Connection:             ws,
        ConnectionAlive:        true,
    }
}

func handleServerCommands(ws *websocket.Conn) {
    commandId := app.createServerCommandId()
    for {

        if connectionIsTerminated(app.runningState) {
            Logf(LtDebug, "HandleServerCommands is shutting down.\n")
            ws.Close()
            return
        }

        var message string
        if err := websocket.Message.Receive(ws, &message); err != nil {
            Logf(LtDebug, "The command line with id: %v is closed because of: %v\n", commandId, err)
            ws.Close()
            return
        }
        app.serverCommands = append(app.serverCommands, message)
    }
}

func handleMiddleware(ws *websocket.Conn) {
    var botId = app.createBotId()

    Logf(LtDebug, "Got connection from Middleware %v\n", botId)

    var err error
    for {

        if connectionIsTerminated(app.runningState) {
            Logf(LtDebug, "handleMiddleware is shutting down.\n")
            ws.Close()
            return
        }

        //
        // Receive the message
        //
        var message MessageMiddlewareServer
        if err = websocket.JSON.Receive(ws, &message); err != nil {
            Logf(LtDebug, "Can't receive from bot %v. Error: %v\n", botId, err)

            ws.Close()

            // In case the bot did not send a BotInfo for registration before the connection was lost, it's not in the map.
            var cmd BotCommand
            var bi  BotInfo

            app.mwInfo <- MwInfo {
                            botId:                  botId,
                            command:                cmd,
                            connectionAlive:        false,
                            createNewBot:           false,
                            botInfo:                bi,
                            statistics:             Statistics{},
                            ws:                     nil,
                          }

            return
        }

        //
        // Evaluate the message
        //
        if message.Type == MmstBotCommand {
            if message.BotCommand != nil {

                var bi  BotInfo
                app.mwInfo <- MwInfo{
                                botId:              botId,
                                command:            *message.BotCommand,
                                connectionAlive:    true,
                                createNewBot:       false,
                                botInfo:            bi,
                                statistics:         Statistics{},
                                ws:                 nil,
                              }


            } else {
                Logf(LtDebug, "Got a dirty message from bot %v. BotCommand is nil.\n", botId)
            }
        } else if message.Type == MmstBotInfo {
            if message.BotInfo != nil {
                // Check, if a player with this name is actually allowed to play
                // So we take the time to sort out old statistics from files here and not
                // in the main game loop (so adding, say, 100 bots, doesn't affect the other, normal computations!)
                isAllowed, statisticsOverall := CheckPotentialPlayer(message.BotInfo.Name)

                if !isAllowed {
                    Logf(LtDebug, "The player %v is not allowed to play. Please add %v to your bot.names.\n", message.BotInfo.Name, message.BotInfo.Name)
                    ws.Close()
                    return
                }

                var cmd BotCommand
                app.mwInfo <- MwInfo{
                                botId:              botId,
                                command:            cmd,
                                connectionAlive:    true,
                                createNewBot:       true,
                                botInfo:            *message.BotInfo,
                                statistics:         statisticsOverall,
                                ws:                 ws,
                              }

                Logf(LtDebug, "Bot %v registered: %v.\n", botId, *message.BotInfo)
            } else {
                Logf(LtDebug, "Got a dirty message from bot %v. BotInfo is nil.\n", botId)
            }
        }
    }
}

func rgbToGrayscale(r, g, b uint32) uint8 {
    y := (299*r + 587*g + 114*b + 500) / 1000
    return uint8(y >> 8)
}

func loadSpawnImage(imageName string, shadesOfGray int) []Vec2 {
    var filename = makeLocalSpawnName(imageName)

    var distributionArray []Vec2
    fImg, err1 := os.Open(filename)
    image, err2 := bmp.Decode(fImg)
    if err1 != nil || err2 != nil {
        Logf(LtDebug, "Error while trying to load image %v. err1: %v and err2: %v\n", filename, err1, err2)
        return distributionArray
    }

    for x := image.Bounds().Min.X; x < image.Bounds().Max.X; x++ {
        for y := image.Bounds().Min.Y; y < image.Bounds().Max.Y; y++ {
            r, g, b, _ := image.At(x,y).RGBA()
            gray := (255 - float32(rgbToGrayscale(r,g,b))) / 255.0

            arrayCount := int(gray * float32(shadesOfGray))

            for i := 0; i < arrayCount; i++ {
                minX := (x-1) * (int(app.fieldSize.X) / image.Bounds().Max.X)
                maxX := x   * (int(app.fieldSize.X) / image.Bounds().Max.X)
                if x == 0 {
                    minX = maxX
                }
                minY := (y-1) * (int(app.fieldSize.Y) / image.Bounds().Max.Y)
                maxY := y   * (int(app.fieldSize.Y) / image.Bounds().Max.Y)
                if y == 0 {
                    minY = maxY
                }

                // We make this calculation so the position is random but bounded by our 100x100 picture.
                // So we have actually 10x10 radius (fieldsize == 1000) to set the food or bot...
                pos := Vec2{float32(rand.Intn(maxX-minX+1)+minX), float32(rand.Intn(maxY-minY+1)+minY)}
                distributionArray = append(distributionArray, pos)
            }

        }
    }

    return distributionArray
}

func handleServerControl(w http.ResponseWriter, r *http.Request) {
    var imageNames []string

    entries, _ := ioutil.ReadDir("../Public/spawns")
    for _, entry := range entries {
        if filepath.Ext(makeLocalSpawnName(entry.Name())) == ".bmp" {
            imageNames = append(imageNames, entry.Name())
        }
    }

    data := struct {
        ImageNames []string
        FoodSpawnImage string
        ToxinSpawnImage string
        BotSpawnImage string
    }{
        ImageNames:         imageNames,
        FoodSpawnImage:     makeURLSpawnName(app.foodDistributionName),
        ToxinSpawnImage:    makeURLSpawnName(app.toxinDistributionName),
        BotSpawnImage:      makeURLSpawnName(app.botDistributionName),
    }

    t := template.New("Server Control")
    page, _ := ioutil.ReadFile("../ServerGui/index.html")
    t, _ = t.Parse(string(page))
    t.Execute(w, data)
}

func terminateNonBlocking(runningState chan(bool)) {
    // Try to send a non-blocking close to the channel...
    select {
    case runningState <- false:
    default:
        // Aaaaand it didn't work.
        Logf(LtDebug, "Not sending a close\n")
    }
}

func connectionIsTerminated(runningState chan(bool)) bool {
    select {
    case state, ok := <-runningState:
        if ok {
            if !state {
                terminateNonBlocking(runningState)
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

func main() {
    // TODO(henk): Maybe we wanna toggle this at runtime.
    SetLoggingDebug(true)
    SetLoggingVerbose(false)



    app.initialize()

    InitOrganisation()
    UpdateAllSVN()

    // Run the update-loop in parallel to serve the websocket on the main thread.
    go app.startUpdateLoop()

    // HTML sides
    http.Handle("/", http.FileServer(http.Dir("../Public/")))
    http.HandleFunc("/server/", handleServerControl)

    // Websocket connections

    // LEAVE THIS HERE. The handleGui call must stay like this!
    // Otherwise the websocket tries to check the 'origin' flag and
    // some browsers or the C++-Gui can not connect to the websocket any more!
    http.HandleFunc("/gui/",
        func (w http.ResponseWriter, req *http.Request) {
            s := websocket.Server{Handler: websocket.Handler(handleGui)}
            s.ServeHTTP(w, req)
        });

    http.Handle("/middleware/", websocket.Handler(handleMiddleware))
    http.Handle("/servercommand/", websocket.Handler(handleServerCommands))

    // Get the stuff running
    if err := http.ListenAndServe(":8080", nil); err != nil {
        log.Fatal("ListenAndServe:", err)
    }


}
