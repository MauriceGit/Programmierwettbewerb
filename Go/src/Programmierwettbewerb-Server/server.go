package main

import (
    . "Programmierwettbewerb-Server/vector"
    . "Programmierwettbewerb-Server/shared"
    . "Programmierwettbewerb-Server/organisation"
    . "Programmierwettbewerb-Server/data"
    . "Programmierwettbewerb-Server/connections"

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
// MiddlewareRegistration
//
////////////////////////////////////////////////////////////////////////

type MiddlewareRegistration struct {
    botId                   BotId
    botInfo                 BotInfo
    statistics              Statistics
}

////////////////////////////////////////////////////////////////////////
//
// MiddlewareCommand
//
////////////////////////////////////////////////////////////////////////

type MiddlewareCommand struct {
    botId                   BotId
    botCommand              BotCommand
}

////////////////////////////////////////////////////////////////////////
//
// ServerSettings
//
////////////////////////////////////////////////////////////////////////

type ServerSettings struct {
    fieldSize                   Vec2

    MinNumberOfBots             int
    MaxNumberOfBots             int
    MaxNumberOfFoods            int
    MaxNumberOfToxins           int

    foodDistributionName        string
    toxinDistributionName       string
    botDistributionName         string

    foodDistribution            []Vec2
    toxinDistribution           []Vec2
    botDistribution             []Vec2
}

func NewSettings() ServerSettings {
    defaultDistributionName := "black.bmp"
    defaultFieldSize := Vec2{ 1000, 1000 }

    return ServerSettings{
        fieldSize:              defaultFieldSize,

        MinNumberOfBots:        8,
        MaxNumberOfBots:        30,
        MaxNumberOfFoods:       1000,
        MaxNumberOfToxins:      30,

        foodDistributionName:   defaultDistributionName,
        toxinDistributionName:  defaultDistributionName,
        botDistributionName:    defaultDistributionName,

        foodDistribution:       loadSpawnImage(defaultFieldSize, defaultDistributionName, 10),
        toxinDistribution:      loadSpawnImage(defaultFieldSize, defaultDistributionName, 10),
        botDistribution:        loadSpawnImage(defaultFieldSize, defaultDistributionName, 10),
    }
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

func NewGameState(serverSettings ServerSettings) GameState {
    var gameState GameState

    gameState.foods         = make(map[FoodId]Food)
    gameState.bots          = make(map[BotId]Bot)
    gameState.toxins        = make(map[ToxinId]Toxin)

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

    return gameState
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

func NewIds(settings ServerSettings) Ids {
    return Ids{
        nextGuiId:              0,
        nextBotId:              1,
        nextBlobId:             1,
        nextServerCommandId:    0,
        nextFoodId:             FoodId(settings.MaxNumberOfFoods) + 1,
        nextToxinId:            ToxinId(settings.MaxNumberOfToxins) + 1,
    }
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
    standbyMutex                sync.Mutex
    standby                     *sync.Cond
    standbyActive               bool

    runningStateMutex           sync.Mutex
    runningState                bool

    middlewareCommands          chan MiddlewareCommand
    middlewareRegistrations     chan MiddlewareRegistration
    middlewareTerminations      chan BotId

    serverCommandsMutex         sync.Mutex
    serverCommands              []string
    messagesToServerGui         chan interface{}
    
    serverGuiIsConnectedMutex   sync.Mutex
    serverGuiIsConnected        bool

    profiling                   bool

    guiConnections              GuiConnections
    middlewareConnections       MiddlewareConnections
    settings                    ServerSettings
    ids                         Ids
}

var app Application

func (app* Application) initialize() {
    app.standby                     = sync.NewCond(&app.standbyMutex)

    app.middlewareCommands          = make(chan MiddlewareCommand, 100)
    app.middlewareRegistrations     = make(chan MiddlewareRegistration, 100)
    app.middlewareTerminations      = make(chan BotId, 100)

    app.runningState                = true    
    
    app.messagesToServerGui         = make(chan interface{}, 10)
    app.serverGuiIsConnected        = false

    app.profiling                   = false

    app.guiConnections              = NewGuiConnections()
    app.middlewareConnections       = NewMiddlewareConnections()
    app.settings                    = NewSettings()
    app.ids                         = NewIds(app.settings)
}

func stopServer() {
    app.runningStateMutex.Lock()
    defer app.runningStateMutex.Unlock()
    
    app.runningState = false
}

func isServerRunning() bool {
    app.runningStateMutex.Lock()
    defer app.runningStateMutex.Unlock()
    
    return app.runningState
}

////////////////////////////////////////////////////////////////////////
//
// IdsContainer
//
////////////////////////////////////////////////////////////////////////

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

////////////////////////////////////////////////////////////////////////
//
// Connections
//
////////////////////////////////////////////////////////////////////////

func hasStandbyRelevantConnections() bool {
    numRealBots := 0
    app.middlewareConnections.Foreach(func(botId BotId, middlewareConnection MiddlewareConnection) {
        if middlewareConnection.IsStandbyChanging {
            numRealBots += 1
        }
    })
    return numRealBots > 0 || app.guiConnections.Count() > 0
}

type WaitNotifier func(active bool)
func waitOnStandbyChangingConnections(waitNotifier WaitNotifier) bool {
    app.standbyMutex.Lock()
    defer app.standbyMutex.Unlock()
    
    result := app.standbyActive
    if !hasStandbyRelevantConnections() {
        waitNotifier(app.standbyActive)
        app.standbyActive = true
        app.middlewareConnections.Foreach(func(botId BotId, middlewareConnection MiddlewareConnection) {
            middlewareConnection.StandbyNotification <- true
        })
        app.standby.Wait()
    }
    return result
}

func wakeUpFromStandby() {
    app.standbyMutex.Lock()
    defer app.standbyMutex.Unlock()
    
    if app.standbyActive {
        app.standbyActive = false
        app.middlewareConnections.Foreach(func(botId BotId, middlewareConnection MiddlewareConnection) {
            middlewareConnection.StandbyNotification <- false
        })
        LogfColored(LtDebug, LcBlue, "Exited Standby!\n")
    }
    app.standby.Broadcast()
}

////////////////////////////////////////////////////////////////////////
//
// Spawn Image Paths
//
////////////////////////////////////////////////////////////////////////

func makeLocalSpawnName(name string) string {
    return fmt.Sprintf("../Public/spawns/%v", name)
}

func makeURLSpawnName(name string) string {
    return fmt.Sprintf("/spawns/%v", name)
}

////////////////////////////////////////////////////////////////////////
//
// Finding Positions
//
////////////////////////////////////////////////////////////////////////

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

func newBotPos(gameState *GameState, settings *ServerSettings) (Vec2, bool) {
    length := len(settings.botDistribution)
    if length == 0 {
        return Vec2{}, false
    }
    // Check, that the player doesn't spawn inside another blob!
    for i := 1; i < 10; i++{
        pos := settings.botDistribution[rand.Intn(length)]
        if len(gameState.bots) == 0 {
            return pos, true
        }
        allGood := true
        for _,bot := range(gameState.bots) {
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

    pos := settings.botDistribution[rand.Intn(length)]
    return pos, true
}

////////////////////////////////////////////////////////////////////////
//
// Simulations
//
////////////////////////////////////////////////////////////////////////

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

func throwAllBlobsOfBot(gameState *GameState, bot *Bot, botId BotId) bool {
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
            gameState.foods[foodId] = food

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

func explodeBlob(gameState *GameState, botId BotId, blobId BlobId, newMap *map[BlobId]Blob, ids *Ids)  {
    blobCount := 12
    splitRadius := float32(3.0)

    // ToDo(Maurice): Make exploded Bubbles in random/different sizes with one of them
    // consisting of half the mass!
    blob := gameState.bots[botId].Blobs[blobId]
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

func makeServerMiddlewareBlobs(gameState *GameState, botId BotId) []ServerMiddlewareBlob {
    var blobArray []ServerMiddlewareBlob

    for blobId, blob := range gameState.bots[botId].Blobs {
        blobArray = append(blobArray, makeServerMiddlewareBlob(botId, blobId, blob))
    }

    return blobArray
}

func limitPosition(settings *ServerSettings, position *Vec2) {
    if (*position).X < 0 { (*position).X = 0 }
    if (*position).Y < 0 { (*position).Y = 0 }
    if (*position).X > settings.fieldSize.X { (*position).X = settings.fieldSize.X }
    if (*position).Y > settings.fieldSize.Y { (*position).Y = settings.fieldSize.Y }
}

////////////////////////////////////////////////////////////////////////
//
// NaN Problem
//
////////////////////////////////////////////////////////////////////////

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
func checkAllValuesOnNaN(gameState *GameState, prefix string) {
    for _,bot := range gameState.bots {
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
    for _,food := range gameState.foods {
        checkNaNF(food.Mass, prefix, "food.Mass")
        checkNaNV(food.Position, prefix, "food.Position")
        checkNaNV(food.Velocity, prefix, "food.Velocity")
    }
    for _,toxin := range gameState.toxins {
        checkNaNV(toxin.Position, prefix, "toxin.Position")
        checkNaNF(toxin.Mass, prefix, "toxin.Mass")
        checkNaNV(toxin.Velocity, prefix, "toxin.Velocity")
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

////////////////////////////////////////////////////////////////////////
//
// FoodBuffer for QuadTree
//
////////////////////////////////////////////////////////////////////////

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

////////////////////////////////////////////////////////////////////////
//
// BotKill
//
////////////////////////////////////////////////////////////////////////

type BotKill struct {
    botId               BotId
    name                string
    statisticsThisGame  Statistics
}

func NewBotKill(botId BotId, bot Bot) BotKill {
    return BotKill{
        botId:              botId,
        name:               bot.Info.Name,
        statisticsThisGame: bot.StatisticsThisGame,
    }
}

////////////////////////////////////////////////////////////////////////
//
// Update Function
//
////////////////////////////////////////////////////////////////////////

func update(gameState *GameState, settings *ServerSettings, ids *Ids, profile *Profile, dt float32) ([]BotKill, []FoodId, []ToxinId) {
    deadBots    := make([]BotKill, 0)
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
                        deadBots = append(deadBots, NewBotKill(botId, bot))
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

                limitPosition(settings, &blob.Position)

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
            var windowDiameter float32 = 50.0 * float32(math.Log(float64(completeMass))) - 20.0

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
        for len(gameState.toxins) < settings.MaxNumberOfToxins {
            if pos, ok := newToxinPos(); ok {
                newToxinId := ids.createToxinId()
                gameState.toxins[newToxinId] = Toxin{true, false, pos, false, 0, toxinMassMin, RandomVec2()}
            }
        }
        for len(gameState.foods) < settings.MaxNumberOfFoods {
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
                limitPosition(settings, &food.Position)
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
                limitPosition(settings, &toxin.Position)
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
    // SPLIT BOTS
    ////////////////////////////////////////////////////////////////
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
                throwAllBlobsOfBot(gameState, &bot, botId)
                gameState.bots[botId] = bot
            }
        }
        endProfileEvent(profile, &profileEventSplitBot)
    }

    ////////////////////////////////////////////////////////////////
    // SPLIT THE TOXINS
    ////////////////////////////////////////////////////////////////
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

    ////////////////////////////////////////////////////////////////
    // REUNION OF BLOBS
    ////////////////////////////////////////////////////////////////
    killedBlobs := NewIdsContainer()
    {
        profileEventSubblobReunion := startProfileEvent(profile, "Blob reunion")
        for botId, _ := range gameState.bots {
            var bot = gameState.bots[botId]
            var botRef = &bot
            // Reunion of Subblobs
            calcSubblobReunion(&killedBlobs, botId, botRef)
            gameState.bots[botId] = *botRef
        }
        endProfileEvent(profile, &profileEventSubblobReunion)
    }

    ////////////////////////////////////////////////////////////////
    // COLLISION WITH TOXINS
    ////////////////////////////////////////////////////////////////
    {
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

                        explodeBlob(gameState, botId, blobId, &subMap, ids)
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

    ////////////////////////////////////////////////////////////////
    // PUSH BLOBS APART
    ////////////////////////////////////////////////////////////////
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

    ////////////////////////////////////////////////////////////////
    // BUILD QUAD TREE FOR FOODS
    ////////////////////////////////////////////////////////////////
    quadTree := NewQuadTree(NewQuad(Vec2{0,0}, 1000))
    {
        profileEventQuadTreeBuilding := startProfileEvent(profile, "QuadTree Building for Foods")
        {
            for foodId, food := range gameState.foods {
                quadTree.Insert(food.Position, foodId)
            }
        }
        endProfileEvent(profile, &profileEventQuadTreeBuilding);
    }

    ////////////////////////////////////////////////////////////////
    // EATING FOODS
    ////////////////////////////////////////////////////////////////
    {
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
    }

    ////////////////////////////////////////////////////////////////
    // TOXINS EATING FOODS
    ////////////////////////////////////////////////////////////////
    {
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
    }

    ////////////////////////////////////////////////////////////////
    // BLOBS EATING BLOBS
    ////////////////////////////////////////////////////////////////
    {
        profileEventEatingBlobs := startProfileEvent(profile, "Eating Blobs")
        for botId1, bot1 := range gameState.bots {

            var bot1Mass float32
            for blobId1, blob1 := range gameState.bots[botId1].Blobs {
                bot1Mass += blob1.Mass

                for botId2, bot2 := range gameState.bots {
                    if botId1 != botId2 {
                        for blobId2, blob2 := range gameState.bots[botId2].Blobs {
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
                                    deadBots = append(deadBots, NewBotKill(botId2, bot2))

                                    bot1.StatisticsThisGame.BotKillCount += 1
                                    
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

func (app* Application) startUpdateLoop(gameState* GameState) {
    ticker := time.NewTicker(time.Millisecond * 30)
    var lastTime = time.Now()


    type MessageCounters struct {
        guiMessageCounter           int
        mWMessageCounter            int
        guiStatisticsMessageCounter int
    }

    var simulationStepCounter = 0

    var lastMiddlewareStart = float32(0.0)

    ////////////////////////////////////////////////////////////////
    // Functions
    ////////////////////////////////////////////////////////////////
    startBashScript := func(path string) {
        script := exec.Command("/bin/bash", path)
        script.Dir = "../"
        if err := script.Start(); err != nil {
            Logf(LtDebug, "error on starting bash script %v: %v\n",path, err)
        }
    }

    ////////////////////////////////////////////////////////////////
    // Main Loop
    ////////////////////////////////////////////////////////////////
    for t := range ticker.C {
        // When we want to shut down the server, we have to notify all the go-routines that server the connections.
        if !isServerRunning() {
            app.guiConnections.Foreach(func(guiId GuiId, guiConnection GuiConnection) {
                guiConnection.StopServerNotification <- true
            })
            app.middlewareConnections.Foreach(func(botId BotId, middlewareConnection MiddlewareConnection) {
                middlewareConnection.StopServerNotification <- true
            })
            return
        }
        
        // Go into Standby when there are no relevant connections.
        waitOnStandbyChangingConnections(func(standbyActive bool) {
            if standbyActive {
                LogfColored(LtDebug, LcBlue, "Standby was initiated by a connection timeout. The main loop is waiting now!\n")
            } else {
                LogfColored(LtDebug, LcBlue, "Entering Standby!\n")
            }
        })

        profile := NewProfile()

        var dt = float32(t.Sub(lastTime).Nanoseconds()) / 1e9
        lastTime = t

        if dt >= 0.03 { dt = 0.03 }

        ////////////////////////////////////////////////////////////////
        // Save statistics
        ////////////////////////////////////////////////////////////////
        if simulationStepCounter == 300 {
            for _,bot := range gameState.bots {
                go WriteStatisticToFile(bot.Info.Name, bot.StatisticsThisGame)
            }
            simulationStepCounter = 0
        }
        simulationStepCounter += 1

        ////////////////////////////////////////////////////////////////
        // HANDLE EVENTS
        ////////////////////////////////////////////////////////////////
        botsKilledByServerGui := make([]BotKill, 0)
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
                            stopServer()
                            Logf(LtDebug, "Server is shutting down.\n")
                            // We give the other go routines a few seconds to gracefully shut down!
                            time.Sleep(3000 * time.Millisecond)
                            Logf(LtDebug, "Sleep finished\n")
                            os.Exit(1)
                        case "ToggleProfiling":
                            Logf(LtDebug, "Toggle Profiling\n");
                            app.profiling = !app.profiling
                        case "KillAllBots":
                            for botId, bot := range gameState.bots {
                                delete(gameState.bots, botId)
                                botsKilledByServerGui = append(botsKilledByServerGui, NewBotKill(botId, bot))
                            }
                            Logf(LtDebug, "Killed all bots\n")
                        case "KillBotsWithoutConnection":
                            Logf(LtDebug, "The server does not support the command \"KillBotsWithoutConnection\" anylonger.\n")
                        case "KillBotsAboveMassThreshold":
                            for botId, bot := range gameState.bots {
                                var mass float32 = 0
                                for _, blob := range bot.Blobs {
                                    mass += blob.Mass
                                }
                                if mass > float32(command.Value) {
                                    delete(gameState.bots, botId)
                                    botsKilledByServerGui = append(botsKilledByServerGui, NewBotKill(botId, bot))
                                }
                            }
                            Logf(LtDebug, "Killed bots above mass threshold\n")
                        case "FoodSpawnImage":
                            app.settings.foodDistribution     = loadSpawnImage(app.settings.fieldSize, command.Image, 10)
                            app.settings.foodDistributionName = command.Image
                        case "ToxinSpawnImage":
                            app.settings.toxinDistribution     = loadSpawnImage(app.settings.fieldSize, command.Image, 10)
                            app.settings.toxinDistributionName = command.Image
                        case "BotSpawnImage":
                            app.settings.botDistribution     = loadSpawnImage(app.settings.fieldSize, command.Image, 10)
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
        terminatedBots := make([]BotKill, 0, 10)
        {
            profileEventReadFromMiddleware := startProfileEvent(&profile, "Read from Middleware")

            ProcessNewRegistrations:
            for {
                select {
                    case middlewareRegistration := <-app.middlewareRegistrations:
                        if len(gameState.bots) < app.settings.MaxNumberOfBots {
                            bot, ok := createStartingBot(gameState, middlewareRegistration.botInfo, middlewareRegistration.statistics)
                            if ok {
                                gameState.bots[middlewareRegistration.botId] = bot
                            } else {
                                Logf(LtDebug, "Due to a spawn image with a 0 spawn rate, there is no possible spawn position for this bot.\n")
                            }
                        }
                    default:
                        break ProcessNewRegistrations
                }
            }

            ProcessingNewCommands:
            for {
                select {
                case middlewareCommand := <-app.middlewareCommands:
                    if bot, ok := gameState.bots[middlewareCommand.botId]; ok {
                        bot.Command = middlewareCommand.botCommand
                        gameState.bots[middlewareCommand.botId] = bot
                    }
                default:
                    break ProcessingNewCommands
                }
            }

            ProcessingTerminations:
            for {
                select {
                case botId := <-app.middlewareTerminations:
                    if bot, ok := gameState.bots[botId]; ok {
                        delete(gameState.bots, botId)
                        terminatedBots = append(terminatedBots, NewBotKill(botId, bot))
                    }
                default:
                    break ProcessingTerminations
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
                if len(gameState.bots) < app.settings.MinNumberOfBots {
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
        deadBots, eatenFoods, eatenToxins := update(gameState, &app.settings, &app.ids, &profile, dt)
        deadBots = append(deadBots, botsKilledByServerGui...)
        deadBots = append(deadBots, terminatedBots...)

        ////////////////////////////////////////////////////////////////
        // CHECK ANYTHING ON NaN VALUES
        ////////////////////////////////////////////////////////////////
        checkAllValuesOnNaN(gameState, "end")

        ////////////////////////////////////////////////////////////////
        // WRITE STATISTICS FOR DEAD BOTS
        ////////////////////////////////////////////////////////////////
        for _, botKill := range deadBots {
            go WriteStatisticToFile(botKill.name, botKill.statisticsThisGame)
        }
        
        ////////////////////////////////////////////////////////////////
        // REMOVE THE CONNECTIONS OF THE DEAD BOTS
        ////////////////////////////////////////////////////////////////
        for _, botKill := range deadBots {
            app.middlewareConnections.Delete(botKill.botId)
        }

        ////////////////////////////////////////////////////////////////
        // PREPARE DATA TO BE SENT TO THE MIDDLEWARES
        ////////////////////////////////////////////////////////////////
        {
            profileEventPrepareDataForMiddleware := startProfileEvent(&profile, "Prepare data to be sent to the middlewares")
            if simulationStepCounter % mwMessageEvery == 0 {
                app.middlewareConnections.Foreach(func(botId BotId, middlewareConnection MiddlewareConnection) {
                    channel := middlewareConnection.MessageChannel

                    bot, ok := gameState.bots[botId]
                    if (ok) {
                        // Collecting other blobs
                        var otherBlobs []ServerMiddlewareBlob
                        for otherBotId, otherBot := range gameState.bots {
                            if botId != otherBotId {
                                for otherBlobId, otherBlob := range otherBot.Blobs {
                                    if IsInViewWindow(bot.ViewWindow, otherBlob.Position, otherBlob.Radius()) {
                                        otherBlobs = append(otherBlobs, makeServerMiddlewareBlob(otherBotId, otherBlobId, otherBlob))
                                    }
                                }
                            }
                        }

                        // Collecting foods
                        var foods []Food
                        for _, food := range gameState.foods {
                            if IsInViewWindow(bot.ViewWindow, food.Position, Radius(food.Mass)) {
                                foods = append(foods, food)
                            }
                        }

                        // Collecting toxins
                        var toxins []Toxin
                        for _, toxin := range gameState.toxins {
                            if IsInViewWindow(bot.ViewWindow, toxin.Position, Radius(toxin.Mass)) {
                                toxins = append(toxins, toxin)
                            }
                        }

                        var wrapper = ServerMiddlewareGameState{
                            MyBlob:         makeServerMiddlewareBlobs(gameState, botId),
                            OtherBlobs:     otherBlobs,
                            Food:           foods,
                            Toxin:          toxins,
                        }

                        channel <- wrapper
                    } else {
                        Logf(LtDebug, "While sending the data to all middlewares, we encountered a middleware connection, for which we did not find a bot.\n")
                    }
                })
            }
            endProfileEvent(&profile, &profileEventPrepareDataForMiddleware)
        }

        ////////////////////////////////////////////////////////////////
        // PREPARE DATA TO BE SENT TO THE GUIS
        ////////////////////////////////////////////////////////////////
        {
            profileEventPrepareDataForGui := startProfileEvent(&profile, "Prepare data to be sent to the middlewares")
            app.guiConnections.Foreach(func(guiId GuiId, guiConnection GuiConnection) {
                channel := guiConnection.MessageChannel
                message := NewServerGuiUpdateMessage()

                for botId, bot := range gameState.bots {
                    key := strconv.Itoa(int(botId))
                    if bot.GuiNeedsInfoUpdate || guiConnection.IsNewConnection {
                        message.CreatedOrUpdatedBotInfos[key] = bot.Info
                    }

                    if simulationStepCounter % guiMessageEvery == 0 {
                        message.CreatedOrUpdatedBots[key] = NewServerGuiBot(bot)
                    }

                    if simulationStepCounter % guiStatisticsMessageEvery == 0 {
                        message.StatisticsThisGame[key] = bot.StatisticsThisGame
                        // @Todo: The global one MUCH more rarely!
                        message.StatisticsGlobal[key] = bot.StatisticsOverall
                    }
                }

                deadBotIds := make([]BotId, 0, 10)
                for _, botKill := range deadBots {
                    deadBotIds = append(deadBotIds, botKill.botId)
                }

                message.DeletedBotInfos = deadBotIds
                message.DeletedBots = deadBotIds

                if simulationStepCounter % guiMessageEvery == 0 {
                    for foodId, food := range gameState.foods {
                        if food.IsMoving || food.IsNew || guiConnection.IsNewConnection {
                            key := strconv.Itoa(int(foodId))
                            message.CreatedOrUpdatedFoods[key] = NewServerGuiFood(food)
                        }
                    }
                }

                message.DeletedFoods = eatenFoods

                if simulationStepCounter % guiMessageEvery == 0 {
                    for toxinId, toxin := range gameState.toxins {
                        if toxin.IsNew || toxin.IsMoving || guiConnection.IsNewConnection {
                            key := strconv.Itoa(int(toxinId))
                            message.CreatedOrUpdatedToxins[key] = NewServerGuiToxin(toxin)
                        }
                    }
                }

                message.DeletedToxins = eatenToxins

                channel <- message
            })
            endProfileEvent(&profile, &profileEventPrepareDataForGui)
        }

        ////////////////////////////////////////////////////////////////
        // RESET UPDATE INDICATORS OF THE GAME OBJECTS
        ////////////////////////////////////////////////////////////////
        for toxinId, toxin := range gameState.toxins {
            if toxin.IsNew {
                toxin.IsNew = false
                gameState.toxins[toxinId] = toxin
            }
        }
        for foodId, food := range gameState.foods {
            if food.IsNew {
                food.IsNew = false
                gameState.foods[foodId] = food
            }
        }
        for botId, bot := range gameState.bots {
            bot.GuiNeedsInfoUpdate = false
            gameState.bots[botId] = bot
        }

        app.guiConnections.MakeAllOld()
        
        ////////////////////////////////////////////////////////////////
        // RESETTING BOT COMMANDS
        ////////////////////////////////////////////////////////////////
        for botId, bot := range gameState.bots {
            bot.Command = BotCommand{ BatNone, bot.Command.Target }
            gameState.bots[botId] = bot
        }

        ////////////////////////////////////////////////////////////////
        // PROFILING
        ////////////////////////////////////////////////////////////////
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

func handleGui(ws *websocket.Conn) {
    var guiId = app.ids.createGuiId()
    LogfColored(LtDebug, LcYellow, "===> Got connection for Gui %v\n", guiId)

    messageChannel         := make(chan ServerGuiUpdateMessage, 1000)
    stopServerNotification := make(chan bool, 1)

    sendingDone   := make(chan bool)
    receivingDone := make(chan bool)

    app.guiConnections.Add(guiId, GuiConnection{ ws, true, messageChannel, stopServerNotification })

    wakeUpFromStandby()

    defer func() {
        app.guiConnections.Delete(guiId)
        LogfColored(LtDebug, LcYellow, "<=== Gui connection (GuiId: %v): Connection was handled.\n", guiId)
    }()

    ////////////////////////////////////////////////////////////////
    // TERMINATE THE GO-ROUTINES FOR SENDING AND RECEIVING
    ////////////////////////////////////////////////////////////////
    terminate := func() {
        ws.Close()
        if _, isOpen := <-messageChannel; isOpen {
            close(messageChannel)
        }
    }

    ////////////////////////////////////////////////////////////////
    // SENDING
    ////////////////////////////////////////////////////////////////
    go func() {
        defer func() {
            sendingDone <- true
            terminate()
            LogfColored(LtDebug, LcYellow, "<=== Gui connection (BotId: %v): Go-routine for sending messages is shutting down.\n", guiId)
        }()

        timeoutDuration := 5*time.Second
        timeout := time.NewTimer(timeoutDuration)

        for {
            select {
                case message, isOpen := <-messageChannel:
                    if !isOpen {
                        LogfColored(LtDebug, LcYellow, "<=== Gui connection (GuiId: %v): Go-routine for sending is shutting down.\n")
                        return
                    }

                    // Consume all the messages from the channel.
                    otherMessages := make([]ServerGuiUpdateMessage, 0, 11)
                    Consuming:
                    for {
                        select {
                            case message := <-messageChannel:
                                otherMessages = append(otherMessages, message)
                            default:
                                break Consuming
                        }
                        if len(otherMessages) > 10 {
                            LogfColored(LtDebug, LcYellow, "<=== More than 10 messages are in the Queue for gui %v. So we just shut it down!\n", guiId)
                            return
                        }
                    }

                    // Send the messages.
                    var err error
                    if len(otherMessages) == 0 {
                        err = websocket.JSON.Send(ws, message)
                    } else {
                        allMessages := append([]ServerGuiUpdateMessage{message}, otherMessages...)
                        for _, m := range(allMessages) {
                            m.CreatedOrUpdatedBots = make(map[string]ServerGuiBot)
                            m.StatisticsThisGame   = make(map[string]Statistics)
                            m.StatisticsGlobal     = make(map[string]Statistics)
                            err = websocket.JSON.Send(ws, m)
                        }
                    }

                    if err != nil {
                        LogfColored(LtDebug, LcYellow, "<=== ServerGuiUpdateMessage could not be sent because of: %v\n", err)
                        return
                    }

                    timeout.Reset(timeoutDuration)
                case <-timeout.C:
                    LogfColored(LtDebug, LcYellow, "<=== Gui connection (GuiId: %v): Timeout for Gui messages.\n", guiId)
                    return
            }
        }
    }()

    ////////////////////////////////////////////////////////////////
    // RECEIVING
    ////////////////////////////////////////////////////////////////
    go func () {
        defer func() {
            receivingDone <- true
            terminate()
            LogfColored(LtDebug, LcYellow, "<=== Gui connection (GuiId: %v): Go-routine for receiving messages is shutting down.\n", guiId)
        }()

        // We never receive anything from the gui right now.
        for {
            var reply string
            if err := websocket.Message.Receive(ws, &reply); err != nil {
                Logf(LtDebug, "Can't receive (%v)\n", err)
                return
            }
        }
    }()

    ////////////////////////////////////////////////////////////////
    // WAITING FOR THE WORKERS
    ////////////////////////////////////////////////////////////////
    waiter := 2
    for waiter > 0 {
        select {
            case <-receivingDone: waiter -= 1
            case <-sendingDone: waiter -= 1
            case <-stopServerNotification: terminate()
        }
    }
}

func createStartingBot(gameState *GameState, botInfo BotInfo, statistics Statistics) (Bot, bool) {
    if pos, ok := newBotPos(gameState, &app.settings); ok {
        blob := Blob {
            Position:       pos,
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
        }, true
    }

    return Bot{}, false
}

func handleServerCommands(ws *websocket.Conn) {
    commandId := app.ids.createServerCommandId()
    
    // Only one server gui can be connected at the time.
    app.serverGuiIsConnectedMutex.Lock()
    if app.serverGuiIsConnected {
        app.serverGuiIsConnectedMutex.Unlock()
        return
    }
    app.serverGuiIsConnected = true
    app.serverGuiIsConnectedMutex.Unlock()
    
    defer func() {
        app.serverGuiIsConnectedMutex.Lock()
        app.serverGuiIsConnected = false
        app.serverGuiIsConnectedMutex.Unlock()
    }()
    
    LogfColored(LtDebug, LcCyan, "===> Starting ServerGui: %v\n", commandId)
    
    sendingDone   := make(chan bool, 1)
    receivingDone := make(chan bool, 1)
    
    stopSending   := make(chan bool, 1)

    ////////////////////////////////////////////////////////////////
    // TERMINATE
    ////////////////////////////////////////////////////////////////
    terminate := func() {
        ws.Close()
        select {
            case stopSending <- true:
            default:
        }
    }

    ////////////////////////////////////////////////////////////////
    // SENDING
    ////////////////////////////////////////////////////////////////
    go func() {
        defer func() {
            sendingDone <- true
            terminate()
            LogfColored(LtDebug, LcCyan, "<=== Server Gui (ServerCommandId: %v): Go-routine for sending messages is shutting down.\n", commandId)
        }()
        
        for {
            select {
                case message := <-app.messagesToServerGui:
                    if err := websocket.JSON.Send(ws, message); err != nil {
                        Logf(LtDebug, "ERROR when trying to send profiling information to %v: %s\n", commandId, err.Error())
                        app.serverGuiIsConnected = false
                        ws.Close()
                        return
                    }
                case <-stopSending:
                    return
            }
        }
    }()

    ////////////////////////////////////////////////////////////////
    // RECEIVING
    ////////////////////////////////////////////////////////////////
    go func() {
        defer func() {
            receivingDone <- true
            terminate()
            LogfColored(LtDebug, LcCyan, "<=== Server Gui (ServerCommandId: %v): Go-routine for receiving messages is shutting down.\n", commandId)
        }()
        
        for {
            var message string
            if err := websocket.Message.Receive(ws, &message); err != nil {
                LogfColored(LtDebug, LcCyan, "<=== Server Gui (ServerCommandId: %v): Error while Receiving: %v.\n", commandId, err.Error())
                return
            }
            
            app.serverCommandsMutex.Lock()
            app.serverCommands = append(app.serverCommands, message)
            app.serverCommandsMutex.Unlock()
        }
    }()

    ////////////////////////////////////////////////////////////////
    // WAITING FOR THE WORKERS
    ////////////////////////////////////////////////////////////////
    waiter := 2
    for waiter > 0 {
        select {
            case <-receivingDone: waiter -= 1
            case <-sendingDone: waiter -= 1
        }
    }
}

func handleMiddleware(ws *websocket.Conn) {
    var botId = app.ids.createBotId()

    defer func() {
        app.middlewareConnections.Delete(botId)
        app.middlewareTerminations <- botId
        LogfColored(LtDebug, LcYellow, "<=== Middleware connection (BotId: %v): Connection was handled.\n", botId)
    }()

    LogfColored(LtDebug, LcYellow, "===> Got connection from Middleware %v\n", botId)

    messageChannel         := make(chan ServerMiddlewareGameState, 100)
    stopServerNotification := make(chan bool ,1)
    standbyNotification    := make(chan bool, 1)

    isRegistered := false

    sendingDone   := make(chan bool)
    receivingDone := make(chan bool)

    ////////////////////////////////////////////////////////////////
    // TERMINATE THE GO-ROUTINES FOR SENDING AND RECEIVING
    ////////////////////////////////////////////////////////////////
    terminate := func() {
        ws.Close()
        // This has to be checked because go panics, when a closed channel is being closed.
        if _, isOpen := <-messageChannel; isOpen {
            close(messageChannel)
        }
    }

    ////////////////////////////////////////////////////////////////
    // SENDING
    ////////////////////////////////////////////////////////////////
    go func() {
        defer func() {
            sendingDone <- true
            terminate()
            LogfColored(LtDebug, LcYellow, "<=== Middleware connection (BotId: %v): Go-routine for sending messages is shutting down.\n", botId)
        }()

        // This also means, that the bots have "timeoutDuration" to register themselves.
        timeoutDuration := 5*time.Second
        timeout := time.NewTimer(timeoutDuration)

        for {
            select {
                case message, isOpen := <-messageChannel:
                    if !isOpen { return }
                    if isRegistered {
                        // Consuming all messages from the channel
                        otherMessages := make([]ServerMiddlewareGameState, 0, 11)
                        Consuming:
                        for {
                            select {
                                case message := <-messageChannel:
                                    otherMessages = append(otherMessages, message)
                                default:
                                    break Consuming
                            }
                            if len(otherMessages) > 10 {
                                LogfColored(LtDebug, LcYellow, "<=== More than 10 messages are in the Queue for middleware %v. So we just shut it down!\n", botId)
                                return
                            }
                        }

                        // Sending the messages
                        var err error
                        if len(otherMessages) == 0 {
                            err = websocket.JSON.Send(ws, message)
                        } else {
                            LogfColored(LtDebug, LcYellow, "<=== Middleware %v skips one message, as it is not fast enough receiving the ones before...\n", botId)
                        }

                        if err != nil {
                            LogfColored(LtDebug, LcYellow, "<=== JSON could not be sent because of: %v\n", err)
                            return
                        }
                        
                        timeout.Reset(timeoutDuration)
                    }
                case standbyActive := <-standbyNotification:
                    if standbyActive {
                        timeout.Stop()
                    } else {
                        timeout.Reset(timeoutDuration)
                    }
                case <-timeout.C:
                    LogfColored(LtDebug, LcYellow, "<=== Middleware connection (BotId: %v): Timeout for Middleware messages.\n", botId)
                    return
            }
        }
    }()

    ////////////////////////////////////////////////////////////////
    // RECEIVING
    ////////////////////////////////////////////////////////////////
    go func() {
        defer func() {
            receivingDone <- true
            terminate()
            LogfColored(LtDebug, LcYellow, "<=== Middleware connection (BotId: %v): Go-routine for receiving is shutting down.\n", botId)
        }()

        for {
            // Receive the message
            var message MessageMiddlewareServer
            if err := websocket.JSON.Receive(ws, &message); err != nil {
                LogfColored(LtDebug, LcYellow, "<=== Can't receive from bot %v. Error: %v\n", botId, err)
                return
            }

            // Evaluate the message
            switch (message.Type) {
                case MmstBotCommand:
                    if message.BotCommand != nil {
                        app.middlewareCommands <- MiddlewareCommand{
                                                      botId:   botId,
                                                      botCommand: *message.BotCommand,
                                                  }
                    } else {
                        LogfColored(LtDebug, LcRed, "Got a dirty message from bot %v. BotCommand is nil.\n", botId)
                    }
                case MmstBotInfo:
                    if message.BotInfo != nil {
                        // Check, if a player with this name is actually allowed to play
                        // So we take the time to sort out old statistics from files here and not
                        // in the main game loop (so adding, say, 100 bots, doesn't affect the other, normal computations!)
                        isAllowed, repository, statisticsOverall := CheckPotentialPlayer(message.BotInfo.Name)

                        sourceIP := strings.Split(ws.Request().RemoteAddr, ":")[0]
                        myIP := getIP()

                        if message.BotInfo.Name == "dummy" && sourceIP != myIP && sourceIP != "localhost" && sourceIP != "127.0.0.1" {
                            isAllowed = false
                            LogfColored(LtDebug, LcRed, "The player name 'dummy' is not allowed! Request from: %s\n", sourceIP)
                        }

                        if isAllowed {
                            app.middlewareRegistrations <- MiddlewareRegistration{
                                                               botId:       botId,
                                                               botInfo:     *message.BotInfo,
                                                               statistics:  statisticsOverall,
                                                       }

                            app.middlewareConnections.Add(botId, NewMiddlewareConnection(ws, messageChannel, standbyNotification, stopServerNotification, message.BotInfo.Name != "dummy"))
                            isRegistered = true

                            wakeUpFromStandby()

                            LogfColored(LtDebug, LcGreen, "New bot connection: The player \"%v\" can be associated with the svn-repository \"%v\".\n", message.BotInfo.Name, repository)
                        } else {
                            LogfColored(LtDebug, LcMagenta, "New bot connection: The player \"%v\" can not be associated with any svn-repositories!\n", message.BotInfo.Name)
                            return
                        }
                    } else {
                        LogfColored(LtDebug, LcRed,  "Got a dirty message from bot %v. BotInfo is nil.\n", botId)
                    }
            }
        }
    }()

    ////////////////////////////////////////////////////////////////
    // WAITING FOR THE WORKERS
    ////////////////////////////////////////////////////////////////
    waiter := 2
    for waiter > 0 {
        select {
            case <-receivingDone: waiter -= 1
            case <-sendingDone: waiter -= 1
            case <-stopServerNotification: terminate()
        }
    }
}

func loadSpawnImage(fieldSize Vec2, imageName string, shadesOfGray int) []Vec2 {
    var filename = makeLocalSpawnName(imageName)

    var distributionArray []Vec2
    fImg, err1 := os.Open(filename)
    image, err2 := bmp.Decode(fImg)
    if err1 != nil || err2 != nil {
        Logf(LtDebug, "Error while trying to load image %v. err1: %v and err2: %v\n", filename, err1, err2)
        return distributionArray
    }

    rgbToGrayscale := func(r, g, b uint32) uint8 {
        y := (299*r + 587*g + 114*b + 500) / 1000
        return uint8(y >> 8)
    }

    for x := image.Bounds().Min.X; x < image.Bounds().Max.X; x++ {
        for y := image.Bounds().Min.Y; y < image.Bounds().Max.Y; y++ {
            r, g, b, _ := image.At(x,y).RGBA()
            gray := (255 - float32(rgbToGrayscale(r,g,b))) / 255.0

            arrayCount := int(gray * float32(shadesOfGray))

            for i := 0; i < arrayCount; i++ {
                minX := (x-1) * (int(fieldSize.X) / image.Bounds().Max.X)
                maxX :=  x    * (int(fieldSize.X) / image.Bounds().Max.X)
                if x == 0 {
                    minX = maxX
                }
                minY := (y-1) * (int(fieldSize.Y) / image.Bounds().Max.Y)
                maxY :=  y    * (int(fieldSize.Y) / image.Bounds().Max.Y)
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
    runtime.GOMAXPROCS(32)

    SetLoggingDebug(true)
    SetLoggingVerbose(false)

    createConfigFile()

    app.initialize()

    InitOrganisation()
    UpdateAllSVN()

    gameState := NewGameState(app.settings)
    go app.startUpdateLoop(&gameState)

    // HTML sites
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
