package main

import (
    . "Programmierwettbewerb-Server/vector"
    . "Programmierwettbewerb-Server/shared"
    . "Programmierwettbewerb-Server/organisation"
    . "Programmierwettbewerb-Server/data"

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
    "reflect"
    "html/template"
    "os/exec"
    "sync"
    "strings"
    "net"
    "runtime"
)

////////////////////////////////////////////////////////////////////////
//
// Constants
//
////////////////////////////////////////////////////////////////////////

const (
    foodMassMin = 1
    foodMassMax = 3
    thrownFoodMass = 10
    massToBeAllowedToThrow = 100
    botMinMass = 10
    botMaxMass = 10000.0
    blobReunionTime = 10.0
    blobSplitMass   = 100.0
    blobSplitVelocity = 1.5
    blobMinSpeedFactor = 0.225
    toxinMassMin = 50
    toxinMassMax = 69
    windowMin = 100
    windowMax = 400
    massLossFactor = 10.0
    minBlobMassToExplode = 400.0
    maxBlobCountToExplode = 10
    minBlobMass = 10

    velocityDecreaseFactor = 0.95

    mwMessageEvery = 1
    guiMessageEvery = 1
    guiStatisticsMessageEvery = 1
    serverGuiPasswordFile = "../server_gui_password"
)

////////////////////////////////////////////////////////////////////////
//
// Profiling
//
////////////////////////////////////////////////////////////////////////

type ProfileEvent struct {
    Name            string
    Parent          int // Index into the Events-array of the profile struct.
    Start           time.Time
    Duration        time.Duration
}

type Profile struct {
    Stack       []int
    Events      []ProfileEvent
}

func NewProfile() Profile {
    return Profile{ Events: make([]ProfileEvent, 0, 100), Stack: make([]int, 0, 10) }
}

func startProfileEvent(profile *Profile, name string) ProfileEvent {
    var profileEvent ProfileEvent
    profileEvent.Name = name
    profileEvent.Start = time.Now()

    if len(profile.Stack) > 0 {
        profileEvent.Parent = profile.Stack[len(profile.Stack) - 1]
    } else {
        profileEvent.Parent = -1
    }

    var index = len(profile.Events)
    profile.Events = append(profile.Events, profileEvent)

    profile.Stack = append(profile.Stack, index)

    return profileEvent
}

func endProfileEvent(profile *Profile, profileEvent *ProfileEvent) {
    var lastProfileEventIndex = profile.Stack[len(profile.Stack) - 1]
    profile.Stack = profile.Stack[:len(profile.Stack) - 1]

    profile.Events[lastProfileEventIndex].Duration = time.Since(profileEvent.Start)
}

func printProfile(profile Profile) {
    fmt.Printf("Profile with %v events:\n", len(profile.Events))
    var overallNanoseconds int64 = 0
    for _, element := range profile.Events {
        overallNanoseconds += element.Duration.Nanoseconds()
    }
    for _, element := range profile.Events {
        relativeDuration := float32(element.Duration.Nanoseconds()) / float32(overallNanoseconds)
        if element.Parent != -1 {
            fmt.Printf("\t")
        }
        fmt.Printf("%s: %v (%.2f)\n", element.Name, element.Duration, relativeDuration)
        for i := 0; i < int(relativeDuration*100) + 1; i++ {
            fmt.Printf("#")
        }
        fmt.Printf("\n")
    }
}

////////////////////////////////////////////////////////////////////////
//
// Blob
//
////////////////////////////////////////////////////////////////////////

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

////////////////////////////////////////////////////////////////////////
//
// ViewWindow
//
////////////////////////////////////////////////////////////////////////

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

////////////////////////////////////////////////////////////////////////
//
// Bot
//
////////////////////////////////////////////////////////////////////////

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
    MessageChannel      chan ServerMiddlewareGameState
    Alive               chan bool
    ConnectionAlive     bool                // TODO(henk): What shall we do when the connection is lost?
}

////////////////////////////////////////////////////////////////////////
//
// GuiConnection
//
////////////////////////////////////////////////////////////////////////

type GuiConnection struct {
    Connection          *websocket.Conn
    IsNewConnection     bool
    MessageChannel      chan ServerGuiUpdateMessage
    Alive               chan bool
}

////////////////////////////////////////////////////////////////////////
//
// WmInfo
//
////////////////////////////////////////////////////////////////////////

type MwInfo struct {
    botId                   BotId

    command                 BotCommand

    connectionAlive         bool

    createNewBot            bool
    botInfo                 BotInfo

    messageChannel          chan ServerMiddlewareGameState
    alive                   chan bool

    statistics              Statistics

    ws                      *websocket.Conn
}

////////////////////////////////////////////////////////////////////////
//
// ServerSettings
//
////////////////////////////////////////////////////////////////////////

type ServerSettings struct {
    MinNumberOfBots     int
    MaxNumberOfBots     int
    MaxNumberOfFoods    int
    MaxNumberOfToxins   int

    foodDistributionName        string
    toxinDistributionName       string
    botDistributionName         string

    foodDistribution            []Vec2
    toxinDistribution           []Vec2
    botDistribution             []Vec2
}

func (settings *ServerSettings) initialize() {
    settings.MinNumberOfBots    = 20
    settings.MaxNumberOfBots    = 100
    settings.MaxNumberOfFoods   = 1000
    settings.MaxNumberOfToxins  = 50
    
    settings.foodDistributionName        = "black.bmp"
    settings.toxinDistributionName       = "black.bmp"
    settings.botDistributionName         = "black.bmp"

    settings.foodDistribution            = loadSpawnImage(settings.foodDistributionName, 10)
    settings.toxinDistribution           = loadSpawnImage(settings.toxinDistributionName, 10)
    settings.botDistribution             = loadSpawnImage(settings.botDistributionName, 10)
}

////////////////////////////////////////////////////////////////////////
//
// GameState
//
////////////////////////////////////////////////////////////////////////

type GameState struct {
    foods                   map[FoodId]Food
    toxins                  map[ToxinId]Toxin
    bots                    map[BotId]Bot
}

func (gameState* GameState) initialize(serverSettings ServerSettings) {
    gameState.foods                       = make(map[FoodId]Food)
    gameState.bots                        = make(map[BotId]Bot)
    gameState.toxins                      = make(map[ToxinId]Toxin)
    
    for i := FoodId(0); i < FoodId(app.settings.MaxNumberOfFoods); i++ {
        mass := foodMassMin + rand.Float32() * (foodMassMax - foodMassMin)
        if pos, ok := newFoodPos(); ok {
            gameState.foods[i] = Food{ true, false, false, BotId(0), mass, pos, RandomVec2() }
        }
    }

    for i := 0; i < app.settings.MaxNumberOfToxins; i++ {
        if pos, ok := newToxinPos(); ok {
            gameState.toxins[ToxinId(i)] = Toxin{true, false, pos, false, BotId(0), toxinMassMin, RandomVec2()}
        }
    }
}

////////////////////////////////////////////////////////////////////////
//
// Ids
//
////////////////////////////////////////////////////////////////////////

type Ids struct {
    mutex                       sync.Mutex
    nextGuiId                   GuiId
    nextBotId                   BotId
    nextBlobId                  BlobId
    nextFoodId                  FoodId
    nextToxinId                 ToxinId
    nextServerCommandId         CommandId
}

func (ids *Ids) initialize(settings ServerSettings) {
    ids.nextGuiId                   = 0
    ids.nextBotId                   = 1
    ids.nextBlobId                  = 1
    ids.nextServerCommandId         = 0
    ids.nextFoodId                  = FoodId(settings.MaxNumberOfFoods) + 1
    ids.nextToxinId                 = ToxinId(settings.MaxNumberOfToxins) + 1
}

func (ids* Ids) createGuiId() GuiId {
    ids.mutex.Lock()
    defer ids.mutex.Unlock()

    var id = ids.nextGuiId
    ids.nextGuiId = id + 1
    return id
}

func (ids* Ids) createServerCommandId() CommandId {
    ids.mutex.Lock()
    defer ids.mutex.Unlock()
    
    var id = ids.nextServerCommandId
    ids.nextServerCommandId = id + 1
    return id
}

func (ids* Ids) createBotId() BotId {
    ids.mutex.Lock()
    defer ids.mutex.Unlock()

    var id = ids.nextBotId
    ids.nextBotId = id + 1
    return id
}

func (ids* Ids) createBlobId() BlobId {
    ids.mutex.Lock()
    defer ids.mutex.Unlock()

    var id = ids.nextBlobId
    ids.nextBlobId = id + 1
    return id
}

func (ids* Ids) createFoodId() FoodId {
    ids.mutex.Lock()
    defer ids.mutex.Unlock()

    var id = ids.nextFoodId
    ids.nextFoodId = id + 1
    return id
}

func (ids* Ids) createToxinId() ToxinId {
    ids.mutex.Lock()
    defer ids.mutex.Unlock()

    var id = ids.nextToxinId
    ids.nextToxinId = id + 1
    return id
}

////////////////////////////////////////////////////////////////////////
//
// Application
//
////////////////////////////////////////////////////////////////////////

type Application struct {
    fieldSize                   Vec2    
    guiConnections              map[GuiId]GuiConnection

    standbyMode                 chan bool
    runningState                chan bool
    mwInfo                      chan MwInfo
    serverCommands              []string
    messagesToServerGui         chan interface{}
    serverGuiIsConnected        bool

    profiling                   bool
    
    settings                    ServerSettings
    ids                         Ids
    gameState                   GameState
}

var app Application

func (app* Application) initialize() {
    app.fieldSize                   = Vec2{ 1000, 1000 }
    app.guiConnections              = make(map[GuiId]GuiConnection)
    
    app.mwInfo                      = make(chan MwInfo, 1000)
    app.standbyMode                 = make(chan bool)
    app.runningState                = make(chan bool, 1)
    app.messagesToServerGui         = make(chan interface{}, 10)
    app.serverGuiIsConnected        = false

    app.profiling                   = false

    app.settings.initialize()
    app.ids.initialize(app.settings)
    app.gameState.initialize(app.settings)
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

func newFoodPos() (Vec2, bool) {
    length := len(app.settings.foodDistribution)
    if length == 0 {
        return Vec2{}, false
    }
    return app.settings.foodDistribution[rand.Intn(length)], true
}

func newToxinPos() (Vec2, bool) {
    length := len(app.settings.toxinDistribution)
    if length == 0 {
        return Vec2{}, false
    }
    return app.settings.toxinDistribution[rand.Intn(length)], true
}

// TODO(henk): Inline
func newBotPos() (Vec2, bool) {
    length := len(app.settings.botDistribution)
    if length == 0 {
        return Vec2{}, false
    }
    // Check, that the player doesn't spawn inside another blob!
    for i := 1; i < 10; i++{
        pos := app.settings.botDistribution[rand.Intn(length)]
        if len(app.gameState.bots) == 0 {
            return pos, true
        }
        allGood := true
        for _,bot := range(app.gameState.bots) {
            for _,blob := range(bot.Blobs) {
                var dist = Dist(pos, blob.Position)
                var minDist  = blob.Radius() + 30

                if minDist > dist {
                    allGood = false
                    break
                }
            }
            if !allGood {
                break
            }
        }
        if allGood {
            return pos, true
        }
    }

    Logf(LtDebug, "Bot position could NOT be determined. Bot is started at a random position!\n")

    pos := app.settings.botDistribution[rand.Intn(length)]
    return pos, true
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

func splitAllBlobsOfBot(bot *Bot, ids *Ids) {
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

            var newIndex = ids.createBlobId()
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
            foodId := app.ids.createFoodId()
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
            app.gameState.foods[foodId] = food

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

func explodeBlob(botId BotId, blobId BlobId, newMap *map[BlobId]Blob, ids *Ids)  {
    blobCount := 12
    splitRadius := float32(3.0)

    // ToDo(Maurice): Make exploded Bubbles in random/different sizes with one of them
    // consisting of half the mass!
    blob := app.gameState.bots[botId].Blobs[blobId]
    for i := 0; i < blobCount; i++ {
        newIndex  := ids.createBlobId()
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

    for blobId, blob := range app.gameState.bots[botId].Blobs {
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
    for _,bot := range app.gameState.bots {
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
    for _,food := range app.gameState.foods {
        checkNaNF(food.Mass, prefix, "food.Mass")
        checkNaNV(food.Position, prefix, "food.Position")
        checkNaNV(food.Velocity, prefix, "food.Velocity")
    }
    for _,toxin := range app.gameState.toxins {
        checkNaNV(toxin.Position, prefix, "toxin.Position")
        checkNaNF(toxin.Mass, prefix, "toxin.Mass")
        checkNaNV(toxin.Velocity, prefix, "toxin.Velocity")
    }
}

func startBashScript(path string) {
    script := exec.Command("/bin/bash", path)
    script.Dir = "../"
    if err := script.Start(); err != nil {
        Logf(LtDebug, "error on starting bash script %v: %v\n",path, err)
    }
}

// This locks the mutex so this is just evaluated once at a time!
func nobodyIsWatching() bool {
    someoneIsThere := false

    if len(app.gameState.bots) > app.settings.MinNumberOfBots {
        someoneIsThere = true
    } else {

        for _,bot := range app.gameState.bots {
            if bot.Info.Name != "dummy" {
                someoneIsThere = true
                break
            }
        }
        someoneIsThere = someoneIsThere || len(app.guiConnections) > 0
    }

    return !someoneIsThere
}

func sendDataToMiddleware(mWMessageCounter int) {

    // TODO(Maurice/henk):
    // MAURICE: Could be VERY MUCH simplified, if we just convert All blobs to the right format once and then sort out later who gets what data!
    // HENK: First we would have to determine which blobs, toxins and foods are visible to the bots. We would have to profile that to check whats faster.
    if mWMessageCounter % mwMessageEvery == 0 {
        for botId, bot := range app.gameState.bots {
            //var connection = app.bots[botId].Connection
            channel := app.gameState.bots[botId].MessageChannel

            // Collecting other blobs
            var otherBlobs []ServerMiddlewareBlob
            for otherBotId, otherBot := range app.gameState.bots {
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
            for _, food := range app.gameState.foods {
                if isInViewWindow(bot.ViewWindow, food.Position, Radius(food.Mass)) {
                    foods = append(foods, food)
                }
            }

            // Collecting toxins
            var toxins []Toxin
            for _, toxin := range app.gameState.toxins {
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

            //websocket.JSON.Send(connection, wrapper)
            channel <- wrapper

        }
    }
}

func sendDataToGui( guiMessageCounter int, guiStatisticsMessageCounter int, deadBots []BotId, eatenFoods []FoodId, eatenToxins []ToxinId,
                    guiConnections map[GuiId]GuiConnection, bots map[BotId]Bot, toxins map[ToxinId]Toxin, foods map[FoodId]Food) {

    // Pre-calculate the list for eaten Toxins
    //reallyDeadToxins := make([]ToxinId, 0)
    //copied := copy(reallyDeadToxins, eatenToxins)
    //Logf(LtDebug, "copied %v elements\n", copied)

    //
    // Send the data to the clients
    //
    for guiId, guiConnection := range app.guiConnections {
        //var connection = app.guiConnections[guiId].Connection
        channel := app.guiConnections[guiId].MessageChannel
        message := newServerGuiUpdateMessage()

        //Logf(LtDebug, "Botcount: %v\n", app.bots)

        for botId, bot := range app.gameState.bots {

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
            for foodId, food := range app.gameState.foods {
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
            for toxinId, toxin := range app.gameState.toxins {
                if toxin.IsNew || toxin.IsMoving || guiConnection.IsNewConnection {
                    key := strconv.Itoa(int(toxinId))
                    message.CreatedOrUpdatedToxins[key] = makeServerGuiToxin(toxin)
                }
            }
        }

        message.DeletedToxins = eatenToxins

        channel <- message
        //websocket.JSON.Send(connection, message)
        //if err != nil {
        //    Logf(LtDebug, "JSON could not be sent because of: %v\n", err)
        //    //return
        //}
    }

}

func checkPasswordAgainstFile(password string) bool {
    pw, err := ioutil.ReadFile(serverGuiPasswordFile)
    if err != nil {
        Logf(LtDebug, "Error while trying to load the password file %v. err: %v\n", serverGuiPasswordFile, err)
        return false
    }
    pwString := strings.Trim(string(pw), "\n \t")

    return pwString == password
}

const foodBufferSize int = 100
type FoodBuffer struct {
    values      [foodBufferSize]interface{}
    count       int
}

func (buffer *FoodBuffer) Append(value interface{}) {
    if buffer.count + 1 < foodBufferSize {
        buffer.values[buffer.count] = value
        buffer.count += 1
    }
}

type MessageCounters struct {
    guiMessageCounter           int
    mWMessageCounter            int
    guiStatisticsMessageCounter int
}

func update(gameState *GameState, settings *ServerSettings, ids *Ids, profile *Profile, dt float32) ([]BotId, []FoodId, []ToxinId) {
    deadBots    := make([]BotId,   0)
    eatenFoods  := make([]FoodId,  0)
    eatenToxins := make([]ToxinId, 0)
    
    ////////////////////////////////////////////////////////////////
    // UPDATE BOT POSITION
    ////////////////////////////////////////////////////////////////
    {
        profileEventUpdateBotPosition := startProfileEvent(profile, "Update Bot Position")
        for botId, bot := range gameState.bots {
            botDied := false
            for blobId, blob := range bot.Blobs {
                if blob.Mass < minBlobMass {
                    delete(bot.Blobs, blobId)
                    if len(bot.Blobs) == 0 {
                        botDied = true
                        deadBots = append(deadBots, botId)
                    }
                    break
                }

                oldPosition := blob.Position
                velocity    := calcBlobVelocity(&blob, bot.Command.Target)
                time        := dt * 50
                newVelocity := Muls(velocity, time)
                newPosition := Add (oldPosition, newVelocity)
                newPosition =  Add (newPosition, blob.IndividualTargetVec)

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

                gameState.bots[botId].Blobs[blobId] = blob
            }
            if !botDied {
                gameState.bots[botId] = bot
            }
        }
        endProfileEvent(profile, &profileEventUpdateBotPosition)
    }

    ////////////////////////////////////////////////////////////////
    // UPDATE VIEW WINDOWS AND MAX MASS
    ////////////////////////////////////////////////////////////////
    {
        profileEventViewWindowsAndMaxMass := startProfileEvent(profile, "View Windows and max Mass")
        for botId, bot := range gameState.bots {
            //var diameter float32
            var center Vec2
            var completeMass float32 = 0
            for _, blob1 := range bot.Blobs {
                center = Add(center, blob1.Position)
                completeMass += blob1.Mass
            }

            if completeMass > botMaxMass {

                // Percentage to cut off
                var dividor float32 = completeMass / botMaxMass

                for blobId, blob := range bot.Blobs {
                    blob.Mass /= dividor
                    bot.Blobs[blobId] = blob
                }

            }

            center = Muls(center, 1.0 / float32(len(bot.Blobs)))
            var windowDiameter float32 = 30.0 * float32(math.Log(float64(completeMass))) - 20.0

            bot.ViewWindow = ViewWindow{
                Position:   Sub(center, Vec2{ windowDiameter / 2.0, windowDiameter / 2.0 }),
                Size: Vec2{ windowDiameter, windowDiameter },
            }
            gameState.bots[botId] = bot
        }
        endProfileEvent(profile, &profileEventViewWindowsAndMaxMass)
    }

    ////////////////////////////////////////////////////////////////
    // POSSIBLY ADD A FOOD OR TOXIN
    ////////////////////////////////////////////////////////////////
    {
        profileEventAddFoodOrToxin := startProfileEvent(profile, "Add Food Or Toxin")
        if rand.Intn(100) <= 5 && len(gameState.toxins) < settings.MaxNumberOfToxins {
            if pos, ok := newToxinPos(); ok {
                newToxinId := ids.createToxinId()
                gameState.toxins[newToxinId] = Toxin{true, false, pos, false, 0, toxinMassMin, RandomVec2()}
            }
        }
        if rand.Intn(100) <= 5 && len(gameState.foods) < settings.MaxNumberOfFoods {
            mass := foodMassMin + rand.Float32() * (foodMassMax - foodMassMin)
            if pos, ok := newFoodPos(); ok {
                newFoodId := ids.createFoodId()
                gameState.foods[newFoodId] = Food{ true, false, false, 0, mass, pos, RandomVec2() }
            }
        }
        endProfileEvent(profile, &profileEventAddFoodOrToxin)
    }

    ////////////////////////////////////////////////////////////////
    // UPDATE FOOD POSITION
    ////////////////////////////////////////////////////////////////
    {
        profileEventFoodPosition := startProfileEvent(profile, "Food Position")
        for foodId, food := range gameState.foods {
            if food.IsMoving {
                food.Position = Add(food.Position, Muls(food.Velocity, dt))
                limitPosition(&food.Position)
                food.Velocity = Muls(food.Velocity, velocityDecreaseFactor)
                gameState.foods[foodId] = food
                if Length(food.Velocity) <= 0.001 {
                    food.IsMoving = false
                }
            }
        }
        endProfileEvent(profile, &profileEventFoodPosition)
    }

    ////////////////////////////////////////////////////////////////
    // UPDATE TOXIN POSITION
    ////////////////////////////////////////////////////////////////
    {
        profileEventToxinPosition := startProfileEvent(profile, "Toxin Position")
        for toxinId, toxin := range gameState.toxins {
            if toxin.IsMoving {
                toxin.Position = Add(toxin.Position, Muls(toxin.Velocity, dt))
                limitPosition(&toxin.Position)
                toxin.Velocity = Muls(toxin.Velocity, velocityDecreaseFactor)
                gameState.toxins[toxinId] = toxin
                if Length(toxin.Velocity) <= 0.001 {
                    toxin.IsMoving = false
                }
            }
            gameState.toxins[toxinId] = toxin
        }
        endProfileEvent(profile, &profileEventToxinPosition)
    }
    
    ////////////////////////////////////////////////////////////////
    // DELETE RANDOM TOXIN IF THERE ARE TOO MANY
    ////////////////////////////////////////////////////////////////
    {
        profileEventDeleteToxins := startProfileEvent(profile, "Delete Toxins")
        for toxinId,_ := range gameState.toxins {
            if len(gameState.toxins) <= settings.MaxNumberOfToxins {
                break;
            }
            eatenToxins = append(eatenToxins, toxinId)
            delete(gameState.toxins, toxinId)
        }
        endProfileEvent(profile, &profileEventDeleteToxins)
    }

    ////////////////////////////////////////////////////////////////
    // DELETE RANDOM FOOD IF THERE ARE TOO MANY
    ////////////////////////////////////////////////////////////////
    {
        profileEventDeleteFood := startProfileEvent(profile, "Delete Foods")
        for foodId,_ := range gameState.foods {
            if len(gameState.foods) <= settings.MaxNumberOfFoods {
                break;
            }
            eatenFoods = append(eatenFoods, foodId)
            delete(gameState.foods, foodId)
        }
        endProfileEvent(profile, &profileEventDeleteFood)
    }

    ////////////////////////////////////////////////////////////////
    // BOT INTERACTION WITH EVERYTHING
    ////////////////////////////////////////////////////////////////

    killedBlobs := NewIdsContainer()

    // Split command received - Creating new Blob with same ID.
    {
        profileEventSplitBot := startProfileEvent(profile, "Split Bot")
        for botId, bot := range gameState.bots {
            if bot.Command.Action == BatSplit && len(bot.Blobs) <= 10 {

                var bot = gameState.bots[botId]
                bot.StatisticsThisGame.SplitCount += 1
                var botRef = &bot
                splitAllBlobsOfBot(botRef, ids)
                gameState.bots[botId] = *botRef

            } else if bot.Command.Action == BatThrow {
                bot := gameState.bots[botId]
                throwAllBlobsOfBot(&bot, botId)
                gameState.bots[botId] = bot
            }
        }
        endProfileEvent(profile, &profileEventSplitBot)
    }

    // Splitting the Toxin!
    {
        profileEventSplitToxin := startProfileEvent(profile, "Split Toxin")
        for toxinId, toxin := range gameState.toxins {

            if toxin.Mass > toxinMassMax {

                // Create new Toxin (moving!)
                newId := ids.createToxinId()
                newToxin := Toxin {
                    IsNew: true,
                    IsMoving: true,
                    Position: toxin.Position,
                    IsSplit: true,
                    IsSplitBy: toxin.IsSplitBy,
                    Mass: toxinMassMin,
                    Velocity: toxin.Velocity,
                }
                gameState.toxins[newId] = newToxin

                possibleBot, foundIt := gameState.bots[toxin.IsSplitBy]
                if foundIt {
                    possibleBot.StatisticsThisGame.ToxinThrow += 1
                    gameState.bots[toxin.IsSplitBy] = possibleBot
                }

                // Reset Mass
                toxin.Mass = toxinMassMin
                toxin.IsSplit = false
                toxin.IsNew = false
                toxin.IsSplitBy = BotId(0)
                toxin.Velocity = RandomVec2()

            }
            gameState.toxins[toxinId] = toxin
        }
        endProfileEvent(profile, &profileEventSplitToxin)
    }

    // Reunion of Subblobs
    {
        profileEventSubblobReunion := startProfileEvent(profile, "Subblob reunion")
        for botId, _ := range gameState.bots {
            var bot = gameState.bots[botId]
            var botRef = &bot
            // Reunion of Subblobs
            calcSubblobReunion(&killedBlobs, botId, botRef)
            gameState.bots[botId] = *botRef
        }
        endProfileEvent(profile, &profileEventSubblobReunion)
    }

    // Blob Collision with Toxin
    {
        // QuadTree Building
        /*
        profileEventQuadTreeBuilding := startProfileEvent(&profile, "QuadTree Building (Toxins)")
        quadTree := NewQuadTree(NewQuad(Vec2{0, 0}, 1000))
        {
            for toxinId, toxin := range app.toxins {
                quadTree.Insert(toxin.Position, toxinId)
            }
        }
        endProfileEvent(&profile, &profileEventQuadTreeBuilding)
        */

        profileEventCollisionWithToxin := startProfileEvent(profile, "Collision with Toxin")
        for tId,toxin := range gameState.toxins {
            var toxinIsEaten = false
            var toxinIsRepositioned = false

            for botId, _ := range gameState.bots {
                bot := gameState.bots[botId]

                mapOfAllNewSingleBlobs := make(map[BlobId]Blob)
                var blobsToDelete []BlobId
                var exploded = false

                // This loop should not alter ANY real data at all right now!
                // Just writing to tmp maps without alterning real data.
                for blobId,_ := range bot.Blobs {
                    var singleBlob = bot.Blobs[blobId]

                    if Dist(singleBlob.Position, toxin.Position) < singleBlob.Radius() && singleBlob.Mass >= minBlobMassToExplode {

                        // If a bot already has > 10 blobs (i.e.), don't explode, eat it!!
                        if len(bot.Blobs) > maxBlobCountToExplode && !toxin.IsSplit {
                            if toxin.IsSplit || len(gameState.toxins) >= settings.MaxNumberOfToxins {
                                eatenToxins = append(eatenToxins, tId)
                                delete(gameState.toxins, tId)
                                toxinIsEaten = true
                            } else {
                                if pos, ok := newToxinPos(); ok {
                                    toxin.Position = pos
                                    toxin.IsSplitBy = BotId(0)
                                    toxin.IsSplit = false
                                    toxin.IsNew = true
                                    toxin.Mass = toxinMassMin
                                    toxinIsRepositioned = true
                                } else {
                                    eatenToxins = append(eatenToxins, tId)
                                    delete(gameState.toxins, tId)
                                    toxinIsEaten = true
                                }
                            }
                            break
                        }

                        subMap := make(map[BlobId]Blob)

                        if toxin.IsSplit {
                            possibleBot, foundIt := gameState.bots[toxin.IsSplitBy]
                            if foundIt {
                                possibleBot.StatisticsThisGame.SuccessfulToxin += 1
                                gameState.bots[toxin.IsSplitBy] = possibleBot
                            }
                        }

                        explodeBlob(botId, blobId, &subMap, ids)
                        exploded = true

                        // Add all the new explosions:
                        for i,b := range subMap {
                            mapOfAllNewSingleBlobs[i] = b
                        }

                        blobsToDelete = append(blobsToDelete, blobId)

                        if pos, ok := newToxinPos(); ok {
                            toxin.Position = pos
                            toxin.IsSplitBy = BotId(0)
                            toxin.IsSplit = false
                            toxin.IsNew = true
                            toxin.Mass = toxinMassMin
                        } else {
                            eatenToxins = append(eatenToxins, tId)
                            delete(gameState.toxins, tId)
                            toxinIsEaten = true
                        }
                    }
                }

                if toxinIsEaten || toxinIsRepositioned {
                    break
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

                gameState.bots[botId] = bot
            }

            if !toxinIsEaten {
                gameState.toxins[tId] = toxin
            }
        }
        endProfileEvent(profile, &profileEventCollisionWithToxin)
    }

    //
    // Push Blobs apart
    //
    {
        profileEventPushBlobsApart := startProfileEvent(profile, "Push Blobs Apart")
        for botId, _ := range gameState.bots {

            var blob = gameState.bots[botId]

            var tmpA = gameState.bots[botId].Blobs
            pushBlobsApart(&tmpA)
            blob.Blobs = tmpA

            gameState.bots[botId] = blob
        }
        endProfileEvent(profile, &profileEventPushBlobsApart)
    }

    //
    // Eating Foods
    //
    {
        quadTree := NewQuadTree(NewQuad(Vec2{0,0}, 1000))
        profileEventQuadTreeBuilding := startProfileEvent(profile, "QuadTree Building (Foods)")
        {
            for foodId, food := range gameState.foods {
                quadTree.Insert(food.Position, foodId)
            }
        }
        endProfileEvent(profile, &profileEventQuadTreeBuilding);


        //
        // Blobs eating Foods
        //
        profileEventQuadTreeSearching := startProfileEvent(profile, "QuadTree Seaching (Blobs eating Foods)")
        {
            var buffer FoodBuffer

            for botId, bot := range gameState.bots {
                for blobId, blob := range bot.Blobs {
                    radius := Radius(blob.Mass)
                    blobQuad := NewQuad(Vec2{ blob.Position.X - radius, blob.Position.Y - radius }, 2*radius)

                    quadTree.FindValuesInQuad(blobQuad, &buffer)

                    // Plow through the result of the query from the tree.
                    for i := 0; i < buffer.count; i = i + 1 {
                        id, _ := buffer.values[i].(FoodId)

                        foodId := id
                        food := gameState.foods[foodId]

                        if Length(Sub(food.Position, blob.Position)) < blob.Radius() {
                            blob.Mass = blob.Mass + food.Mass
                            if food.IsThrown {
                                delete(gameState.foods, foodId)
                                eatenFoods = append(eatenFoods, foodId)
                            } else {
                                if pos, ok := newFoodPos(); ok {
                                    food.Position = pos
                                    food.IsNew = true
                                    gameState.foods[foodId] = food
                                } else {
                                    delete(gameState.foods, foodId)
                                    eatenFoods = append(eatenFoods, foodId)
                                }
                            }
                        }
                    }
                    bot.Blobs[blobId] = blob

                    buffer.count = 0
                }
                gameState.bots[botId] = bot
            }
        }
        endProfileEvent(profile, &profileEventQuadTreeSearching)

        //
        // Toxins eating Foods
        //
        profileEventEatingFood := startProfileEvent(profile, "QuadTree Searching (Toxins eating Foods)")
        {
            var buffer FoodBuffer

            for tId, toxin := range gameState.toxins {
                radius := Radius(toxin.Mass)
                toxinQuad := NewQuad(Vec2{ toxin.Position.X - radius, toxin.Position.Y - radius }, 2*radius)

                quadTree.FindValuesInQuad(toxinQuad, &buffer)

                for i := 0; i < buffer.count; i = i + 1 {
                    foodId, _ := buffer.values[i].(FoodId)
                    food := gameState.foods[foodId]

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

                            delete(gameState.foods, foodId)
                            eatenFoods = append(eatenFoods, foodId)

                        }
                    }
                }
                gameState.toxins[tId] = toxin

                buffer.count = 0
            }
        }
        endProfileEvent(profile, &profileEventEatingFood)

        profileEventEatingBlobs := startProfileEvent(profile, "Eating Blobs")
        for botId1, bot1 := range gameState.bots {

            var bot1Mass float32
            for blobId1, blob1 := range gameState.bots[botId1].Blobs {
                //blob1 := bot1.Blobs[blobId1]
                bot1Mass += blob1.Mass

                for botId2, bot2 := range gameState.bots {
                    if botId1 != botId2 {
                        //bot2 := app.bots[botId2]
                        for blobId2, blob2 := range gameState.bots[botId2].Blobs {
                            //blob2 := bot2.Blobs[blobId2]

                            inRadius := DistFast(blob2.Position, blob1.Position) < blob1.Radius()*blob1.Radius()
                            smaller := blob2.Mass < 0.9*blob1.Mass

                            if smaller && inRadius {
                                blob1.Mass = blob1.Mass + blob2.Mass

                                bot1.StatisticsThisGame.BlobKillCount += 1

                                if blob1.IsSplit {
                                    bot1.StatisticsThisGame.SuccessfulSplit += 1
                                }

                                killedBlobs.insert(botId2, blobId2)

                                delete(gameState.bots[botId2].Blobs, blobId2)

                                // Completely delete this bot.
                                if len(gameState.bots[botId2].Blobs) <= 0 {
                                    deadBots = append(deadBots, botId2)

                                    bot1.StatisticsThisGame.BotKillCount += 1

                                    go WriteStatisticToFile(bot2.Info.Name, bot2.StatisticsThisGame)

                                    bot2.Connection.Close()
                                    delete(gameState.bots, botId2)
                                    break
                                }

                            }
                        }
                    }
                }

                gameState.bots[botId1].Blobs[blobId1] = blob1
            }

            stats := bot1.StatisticsThisGame
            stats.MaxSize = float32(math.Max(float64(stats.MaxSize), float64(bot1Mass)))
            stats.MaxSurvivalTime += dt

            bot1.StatisticsThisGame = stats
            gameState.bots[botId1] = bot1
        }
        endProfileEvent(profile, &profileEventEatingBlobs)
    }

    return deadBots, eatenFoods, eatenToxins
}

func (app* Application) startUpdateLoop() {
    ticker := time.NewTicker(time.Millisecond * 30)
    var lastTime = time.Now()

    messageCounters := MessageCounters{ 0, 0, 0 }
    
    var fpsCnt = 0
    
    var lastMiddlewareStart = float32(0.0)

    for t := range ticker.C {
        profile := NewProfile()

        var dt = float32(t.Sub(lastTime).Nanoseconds()) / 1e9
        lastTime = t

        if dt >= 0.03 { dt = 0.03 }

        // If there are only dummy-Bots on the field - Stop and wait for
        // something relevant to happen :)
        // Schroedinger or something ;)
        /*
        if nobodyIsWatching() {
            Logf(LtDebug, "________________ Thread is going to sleep ...\n")
            // This should block until a new connection (any) is established!
            <- app.standbyMode
            Logf(LtDebug, "________________ Thread is alive again - yay :)\n")
        }

        // Just empty the channel unblocking, so that new guis and stuff can connect...
        select {
            case _, ok := <-app.standbyMode:
                if ok {
                    Logf(LtDebug, "Emptied the channel. Worked.\n")
                }
            default:
        }
        */

        // Once every 10 seconds
        if fpsCnt == 300 {
            for _,bot := range app.gameState.bots {
                go WriteStatisticToFile(bot.Info.Name, bot.StatisticsThisGame)
            }
            fpsCnt = 0
        }
        fpsCnt += 1
        
        ////////////////////////////////////////////////////////////////
        // HANDLE EVENTS
        ////////////////////////////////////////////////////////////////
        botsKilledByServerGui := make([]BotId, 0)
        {
            profileEventHandleEvents := startProfileEvent(&profile, "Handle Events")
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
                            Logf(LtDebug, "Updating the server\n")
                            go startBashScript("./updateServer.sh")
                        case "RestartServer":
                            Logf(LtDebug, "Updating the server\n")
                            go startBashScript("./updateServer.sh")
                            time.Sleep(2000 * time.Millisecond)
                            Logf(LtDebug, "Restarting the server\n")
                            go startBashScript("./restartServer.sh")

                            terminateNonBlocking(app.runningState)
                            Logf(LtDebug, "Server is shutting down.\n")
                            // We give the other go routines a few seconds to gracefully shut down!
                            time.Sleep(3000 * time.Millisecond)
                            Logf(LtDebug, "Sleep finished\n")
                            os.Exit(1)
                        case "ToggleProfiling":
                            Logf(LtDebug, "Toggle Profiling\n");
                            app.profiling = !app.profiling
                        case "KillAllBots":
                            for botId, _ := range app.gameState.bots {
                                delete(app.gameState.bots, botId)
                                botsKilledByServerGui = append(botsKilledByServerGui, botId)
                            }
                            Logf(LtDebug, "Killed all bots\n")
                        case "KillBotsWithoutConnection":
                            for botId, bot := range app.gameState.bots {
                                if !bot.ConnectionAlive {
                                    delete(app.gameState.bots, botId)
                                    botsKilledByServerGui = append(botsKilledByServerGui, botId)
                                }
                            }
                            Logf(LtDebug, "Killed bots without connection\n")
                        case "KillBotsAboveMassThreshold":
                            for botId, bot := range app.gameState.bots {
                                var mass float32 = 0
                                for _, blob := range bot.Blobs {
                                    mass += blob.Mass
                                }
                                if mass > float32(command.Value) {
                                    delete(app.gameState.bots, botId)
                                    botsKilledByServerGui = append(botsKilledByServerGui, botId)
                                }
                            }
                            Logf(LtDebug, "Killed bots above mass threshold\n")
                        case "FoodSpawnImage":
                            app.settings.foodDistribution  = loadSpawnImage(command.Image, 10)
                            app.settings.foodDistributionName = command.Image
                        case "ToxinSpawnImage":
                            app.settings.toxinDistribution = loadSpawnImage(command.Image, 10)
                            app.settings.toxinDistributionName = command.Image
                        case "BotSpawnImage":
                            app.settings.botDistribution   = loadSpawnImage(command.Image, 10)
                            app.settings.botDistributionName = command.Image
                        }
                    } else {
                        if err != nil {
                            Logf(LtDebug, "Err: %v\n", err.Error())
                        }
                    }
                }

                app.serverCommands = make([]string, 0)
            }
            endProfileEvent(&profile, &profileEventHandleEvents)
        }

        ////////////////////////////////////////////////////////////////
        // READ FROM MIDDLEWARE
        ////////////////////////////////////////////////////////////////
        {
            profileEventReadFromMiddleware := startProfileEvent(&profile, "Read from Middleware")
            for {
                finished := false
                select {
                case mwInfo, ok := <-app.mwInfo:
                    if ok {
                        if _, ok := app.gameState.bots[mwInfo.botId]; ok {
                            bot := app.gameState.bots[mwInfo.botId]
                            bot.ConnectionAlive = mwInfo.connectionAlive
                            // There is actually a command
                            if (BotCommand{}) != mwInfo.command {
                                bot.Command = mwInfo.command
                            }

                            app.gameState.bots[mwInfo.botId] = bot
                        }
                        if mwInfo.createNewBot && len(app.gameState.bots) < app.settings.MaxNumberOfBots {
                            bot := createStartingBot(mwInfo.ws, mwInfo.botInfo, mwInfo.statistics, mwInfo.messageChannel, mwInfo.alive)
                            if !reflect.DeepEqual(bot,Bot{}) {
                                app.gameState.bots[mwInfo.botId] = bot
                            } else {
                                Logf(LtDebug, "Due to a spawn image with a 0 spawn rate, there is no possible spawn position for this bot.\n")
                            }
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
            endProfileEvent(&profile, &profileEventReadFromMiddleware)
        }

        ////////////////////////////////////////////////////////////////
        // ADD SOME MIDDLEWARES/BOTS IF NEEDED
        ////////////////////////////////////////////////////////////////
        {
            profileEventAddDummyBots := startProfileEvent(&profile, "Add Dummy Bots")
            if lastMiddlewareStart > 2 {
                if len(app.gameState.bots) < app.settings.MinNumberOfBots {
                    go startBashScript("./startMiddleware.sh")
                    lastMiddlewareStart = 0
                }
            }
            lastMiddlewareStart += dt
            endProfileEvent(&profile, &profileEventAddDummyBots)
        }
        
        ////////////////////////////////////////////////////////////////
        // UPDATE THE GAME STATE
        ////////////////////////////////////////////////////////////////
        deadBots, eatenFoods, eatenToxins := update(&app.gameState, &app.settings, &app.ids, &profile, dt)        
        deadBots = append(deadBots, botsKilledByServerGui...)

        ////////////////////////////////////////////////////////////////
        // CHECK ANYTHING ON NaN VALUES
        ////////////////////////////////////////////////////////////////
        checkAllValuesOnNaN("end")

        ////////////////////////////////////////////////////////////////
        // DELETE BOTS WITHOUT ACTIVE CONNECTION
        ////////////////////////////////////////////////////////////////

        for botId, bot := range app.gameState.bots {
            if !bot.ConnectionAlive {
                go WriteStatisticToFile(bot.Info.Name, bot.StatisticsThisGame)
                delete(app.gameState.bots, botId)
                deadBots = append(deadBots, botId)
            }
        }

        {
            profileEventSendDataToMiddlewareAndGui := startProfileEvent(&profile, "Send Data to Middleware|Gui")

            for guiConnectionId, guiConnection := range app.guiConnections {
                err := websocket.Message.Send(guiConnection.Connection, "alive_test")
                if err != nil {
                    Logf(LtDebug, "Gui %v is deleted because of network failure. Alive test failed.\n", guiConnectionId)
                    delete(app.guiConnections, guiConnectionId)
                }
            }

            ////////////////////////////////////////////////////////////////
            // SEND UPDATED DATA TO MIDDLEWARE AND GUI
            ////////////////////////////////////////////////////////////////

            sendDataToMiddleware(messageCounters.mWMessageCounter)
            sendDataToGui(messageCounters.guiMessageCounter, 
                          messageCounters.guiStatisticsMessageCounter, 
                          deadBots, 
                          eatenFoods, 
                          eatenToxins, 
                          app.guiConnections, 
                          app.gameState.bots, 
                          app.gameState.toxins, 
                          app.gameState.foods)

            for toxinId, toxin := range app.gameState.toxins {
                if toxin.IsNew {
                    toxin.IsNew = false
                    app.gameState.toxins[toxinId] = toxin
                }
            }
            for foodId, food := range app.gameState.foods {
                if food.IsNew {
                    food.IsNew = false
                    app.gameState.foods[foodId] = food
                }
            }
            for botId, bot := range app.gameState.bots {
                bot.GuiNeedsInfoUpdate = false
                app.gameState.bots[botId] = bot
            }

            for guiConnectionId, guiConnection := range app.guiConnections {
                guiConnection.IsNewConnection = false
                app.guiConnections[guiConnectionId] = guiConnection
            }
            messageCounters.guiMessageCounter += 1
            messageCounters.mWMessageCounter += 1
            messageCounters.guiStatisticsMessageCounter += 1

            endProfileEvent(&profile, &profileEventSendDataToMiddlewareAndGui)
        }


        if false && app.profiling && app.serverGuiIsConnected {
            type NanosecondProfileEvent struct {
                Name            string
                Parent          int
                Nanoseconds     int64
            }
            events := make([]NanosecondProfileEvent, 0, 100)
            for _, element := range profile.Events {
                events = append(events, NanosecondProfileEvent{
                    Name: element.Name,
                    Parent: element.Parent,
                    Nanoseconds: element.Duration.Nanoseconds(),
                })
            }

            app.messagesToServerGui <- events

        }
    }
}

func getOtherMessagesFromMWChannel(channel chan ServerMiddlewareGameState) []ServerMiddlewareGameState {
    var messages = make([]ServerMiddlewareGameState, 0)

    select {
        case message, ok := <-channel:
            if ok {
                messages = append(messages, message)
            }
        default:
            return messages
    }
    return messages
}

func sendMiddlewareMessages(botId BotId, ws *websocket.Conn, channel chan ServerMiddlewareGameState, alive chan bool) {

    Logf(LtDebug, "===> SendMiddlewareMessage go routine started.\n")

    for {

        // Alive Test!
        select {
            case state, ok := <-alive:
                if ok  && !state {
                    Logf(LtDebug, "===> Alive failed - MW go routine shutting down!\n")
                    return
                }
            default:
        }

        // After 5 seconds with no new Gui-message, it shuts down!
        timeout := make(chan bool, 1)
        go func() {
            time.Sleep(5 * time.Second)
            timeout <- true
        }()

        select {
            case message, ok := <-channel:
                if ok {
                    var err error
                    otherMessages := getOtherMessagesFromMWChannel(channel)

                    if len(otherMessages) == 0 {
                        // Just now, we send the whole message!
                        err = websocket.JSON.Send(ws, message)
                    } else {

                        // Do nothing! The Middleware will NOT get any messages, until it is fast enough to get them!
                        // It will get the next one though. But skipps all from this round.
                        Logf(LtDebug, "Middleware %v skips one message, as it is not fast enough receiving the ones before...\n", botId)
                    }

                    if err != nil {
                        Logf(LtDebug, "JSON could not be sent because of: %v\n", err)
                    }

                }
            case <-timeout:
                if !nobodyIsWatching() {
                    Logf(LtDebug, "===> Timeout for MW messages (botId: %v) - go routine shutting down!\n", botId)
                    return
                }
        }

    }
}

func getOtherMessagesFromGuiChannel(channel chan ServerGuiUpdateMessage) ([]ServerGuiUpdateMessage, int) {
    var messages = make([]ServerGuiUpdateMessage, 0)
    var count = 0
    select {
        case message, ok := <-channel:
            if ok {
                messages = append(messages, message)
                count++
            }
        default:
            return messages, count
    }
    return messages, count
}

func sendGuiMessages(guiId GuiId, ws *websocket.Conn, channel chan ServerGuiUpdateMessage, alive chan bool) {

    Logf(LtDebug, "===> SendGuiMessage go routine started.\n")

    for {

        // Alive Test!
        select {
            case state, ok := <-alive:
                if ok  && !state {
                    Logf(LtDebug, "===> Alive failed - go routine shutting down!\n")
                    return
                }
            default:
        }

        // After 5 seconds with no new Gui-message, it shuts down!
        timeout := make(chan bool, 1)
        go func() {
            time.Sleep(5 * time.Second)
            timeout <- true
        }()

        select {
            case message, ok := <-channel:
                if ok {
                    var err error

                    otherMessages, count := getOtherMessagesFromGuiChannel(channel)

                    if count > 10 {
                        Logf(LtDebug, "More than 10 messages are in the Queue for gui %v. So we just shut it down!\n", guiId)
                        delete(app.guiConnections, guiId)
                        ws.Close()
                        return
                    }

                    if len(otherMessages) == 0 {
                        // Just now, we send the whole message!
                        err = websocket.JSON.Send(ws, message)
                    } else {

                        allMessages := append([]ServerGuiUpdateMessage{message}, otherMessages...)

                        for _,m := range(allMessages) {
                            // Delete all information we do not want to send!
                            //if i != len(allMessages)-1 {
                                m.CreatedOrUpdatedBots = make(map[string]ServerGuiBot)
                                m.StatisticsThisGame   = make(map[string]Statistics)
                                m.StatisticsGlobal     = make(map[string]Statistics)
                            //}
                            // Send just essential stuff!
                            err = websocket.JSON.Send(ws, m)

                            Logf(LtDebug, "sent Message to Gui!\n")
                        }
                    }

                    if err != nil {
                        Logf(LtDebug, "JSON could not be sent because of: %v\n", err)
                    }

                }
            case <-timeout:
                Logf(LtDebug, "===> Timeout for Gui messages (GuiId: %v) - go routine shutting down!\n", guiId)
                //delete(app.guiConnections, guiId)
                //ws.Close()
                return

        }
    }
}

func handleGui(ws *websocket.Conn) {
    var guiId          = app.ids.createGuiId()         // TODO(henk): When do we delete the blob?

    //if nobodyIsWatching() {
    //    // This unblocks the main thread!
    //    app.standbyMode <- true
    //}

    var messageChannel = make(chan ServerGuiUpdateMessage, 10000)
    var isAlive        = make(chan bool, 1000)

    app.guiConnections[guiId] = GuiConnection{ ws, true, messageChannel, isAlive }

    go sendGuiMessages(guiId, ws, messageChannel, isAlive)

    Logf(LtDebug, "Got connection for Gui %v\n", guiId)

    var err error
    for {

        if connectionIsTerminated(app.runningState) {
            Logf(LtDebug, "HandleGui is shutting down.\n")
            delete(app.guiConnections, guiId)
            isAlive <- false
            ws.Close()
            return
        }

        var reply string

        if err = websocket.Message.Receive(ws, &reply); err != nil {
            Logf(LtDebug, "Can't receive (%v)\n", err)
            delete(app.guiConnections, guiId)
            isAlive <- false
            break
        }
    }
}


func createStartingBot(ws *websocket.Conn, botInfo BotInfo, statistics Statistics, messageChannel chan ServerMiddlewareGameState, alive chan bool) Bot {
    if pos, ok := newBotPos(); ok {

        blob := Blob {
            Position:       pos,  // TODO(henk): How do we decide this?
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
            MessageChannel:         messageChannel,
            Alive:                  alive,
            Connection:             ws,
            ConnectionAlive:        true,
        }
    }

    return Bot{}
}

func handleServerCommands(ws *websocket.Conn) {
    commandId := app.ids.createServerCommandId()

    app.serverGuiIsConnected = true

    go func() {
        Logf(LtDebug, "Starting ServerGui: %v\n", commandId)
        for {
            message := <- app.messagesToServerGui



            if err := websocket.JSON.Send(ws, message); err != nil {
                Logf(LtDebug, "ERROR when trying to send profiling information to %v: %s\n", commandId, err.Error())
                app.serverGuiIsConnected = false
                ws.Close()
                return
            }

        }
    }()

    for {

        if connectionIsTerminated(app.runningState) {
            Logf(LtDebug, "HandleServerCommands is shutting down.\n")
            app.serverGuiIsConnected = false
            ws.Close()
            return
        }

        var message string
        if err := websocket.Message.Receive(ws, &message); err != nil {
            Logf(LtDebug, "\n\nThe command line with id: %v is closed because of: %v\n\n", commandId, err)
            ws.Close()

            return
        }

        app.serverCommands = append(app.serverCommands, message)
    }



}

func handleMiddleware(ws *websocket.Conn) {
    var botId = app.ids.createBotId()

    Logf(LtDebug, "Got connection from Middleware %v\n", botId)

    if nobodyIsWatching() {
        // This unblocks the main thread!
        app.standbyMode <- true
    }

    var messageChannel = make(chan ServerMiddlewareGameState, 10000)
    var isAlive        = make(chan bool, 10000)

    var err error
    for {

        if connectionIsTerminated(app.runningState) {
            Logf(LtDebug, "handleMiddleware is shutting down.\n")
            isAlive <- false
            ws.Close()
            return
        }

        //
        // Receive the message
        //
        var message MessageMiddlewareServer
        if err = websocket.JSON.Receive(ws, &message); err != nil {
            Logf(LtDebug, "Can't receive from bot %v. Error: %v\n", botId, err)

            isAlive <- false

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
                            messageChannel:         messageChannel,
                            alive:                  isAlive,
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
                                messageChannel:     messageChannel,
                                alive:              isAlive,
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

                sourceIP := strings.Split(ws.Request().RemoteAddr, ":")[0]
                myIP := getIP()
                
                // TODO(henk): Remove this.
                Logf(LtDebug, "SourceIP: %v\n", sourceIP)
                Logf(LtDebug, "myIP: %v\n", myIP)

                // TODO(henk): Use this again. The adresses where not equal.
                //if message.BotInfo.Name == "dummy" && sourceIP != myIP && sourceIP != "localhost" && sourceIP != "127.0.0.1" {
                //    isAllowed = false
                //    Logf(LtDebug, "The player name 'dummy' is not allowed! Request from: %s, at: %s\n", sourceIP, time.Now().Format(time.RFC850))
                //}
                if message.BotInfo.Name == "dummy" {
                    isAllowed = true
                }

                if !isAllowed {
                    Logf(LtDebug, "The player %v is not allowed to play. Please add %v to your bot.names. Request from: %s, at: %s\n", message.BotInfo.Name, message.BotInfo.Name, sourceIP, time.Now().Format(time.RFC850))
                    ws.Close()
                    return
                }

                go sendMiddlewareMessages(botId, ws, messageChannel, isAlive)

                var cmd BotCommand
                app.mwInfo <- MwInfo{
                                botId:              botId,
                                command:            cmd,
                                connectionAlive:    true,
                                createNewBot:       true,
                                botInfo:            *message.BotInfo,
                                statistics:         statisticsOverall,
                                messageChannel:     messageChannel,
                                alive:              isAlive,
                                ws:                 ws,
                              }

                Logf(LtDebug, "Bot %v registered: %v. From: %s, at: %s\n", botId, *message.BotInfo, sourceIP, time.Now().Format(time.RFC850))
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

func getServerAddress() string {
    content, err := ioutil.ReadFile("../pwb.conf")
    if err != nil {
        panic("Could not read the address!")
    }
    return string(content)
}

func handleServerControl(w http.ResponseWriter, r *http.Request) {


    Logf(LtDebug, "request: %v, %v\n", r.Form, r.PostForm)

    //data := struct {}

    t := template.New("Server Control")
    page, _ := ioutil.ReadFile("../ServerGui/index.html")
    t, _ = t.Parse(string(page))
    t.Execute(w, nil)
}

func handleServerControlFinal(w http.ResponseWriter, r *http.Request) {
    var imageNames []string

    Logf(LtDebug, "Request for Password: %v\n", r.PostFormValue("Password"))

    if checkPasswordAgainstFile(r.PostFormValue("Password")) {

        entries, _ := ioutil.ReadDir("../Public/spawns")
        for _, entry := range entries {
            if filepath.Ext(makeLocalSpawnName(entry.Name())) == ".bmp" {
                imageNames = append(imageNames, entry.Name())
            }
        }

        data := struct {
            Address string
            ImageNames []string
            FoodSpawnImage string
            ToxinSpawnImage string
            BotSpawnImage string
        }{
            Address:            "ws://" + getServerAddress() + "/servercommand/",
            ImageNames:         imageNames,
            FoodSpawnImage:     makeURLSpawnName(app.settings.foodDistributionName),
            ToxinSpawnImage:    makeURLSpawnName(app.settings.toxinDistributionName),
            BotSpawnImage:      makeURLSpawnName(app.settings.botDistributionName),
        }

        t := template.New("Server Control")
        page, _ := ioutil.ReadFile("../ServerGui/index_final.html")
        t, _ = t.Parse(string(page))
        t.Execute(w, data)
    }
}

func handleGameHTML(w http.ResponseWriter, r *http.Request) {
    data := struct {
        Address string
    }{
        Address: strings.Replace("ws://" + getServerAddress() + "/gui/", "\n", "", -1),
    }
    t := template.New("Index")
    page, _ := ioutil.ReadFile("../Public/game.html")
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

func getIP() string {
    addrs, err := net.InterfaceAddrs()
    if err != nil {
        os.Stderr.WriteString("Oops: " + err.Error() + "\n")
        os.Exit(1)
    }

    for _, a := range addrs {
        if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
            if ipnet.IP.To4() != nil {
                return ipnet.IP.String()
            }
        }
    }
    return "localhost"
}

func createConfigFile() {
    f, err := os.Create("../pwb.conf")
    if err != nil {
        Logf(LtDebug, "Something went wront when creating the new file...: %v\n", err)
    }
    defer f.Close()

    f.Write([]byte(getIP() + ":8080"))
    f.Sync()
}

func main() {

    runtime.GOMAXPROCS(4)

    // TODO(henk): Maybe we wanna toggle this at runtime.
    SetLoggingDebug(true)
    SetLoggingVerbose(false)

    createConfigFile()

    app.initialize()

    InitOrganisation()
    UpdateAllSVN()

    // Run the update-loop in parallel to serve the websocket on the main thread.
    go app.startUpdateLoop()

    // HTML sides
    http.Handle("/", http.FileServer(http.Dir("../Public/")))
    http.HandleFunc("/game.html", handleGameHTML)
    http.HandleFunc("/server/", handleServerControl)
    http.HandleFunc("/server2/", handleServerControlFinal)

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
