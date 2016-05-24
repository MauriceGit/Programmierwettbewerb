package main

import (
    . "Programmierwettbewerb-Server/vector"
    . "Programmierwettbewerb-Server/shared"
    . "Programmierwettbewerb-Server/organisation"

    "golang.org/x/net/websocket"
    //"fmt"
    "log"
    "net/http"
    "math"
    "math/rand"
    "time"
    "strconv"
)

// -------------------------------------------------------------------------------------------------
// Global
// -------------------------------------------------------------------------------------------------

const (
    foodMassMin = 8
    foodMassMax = 12
    thrownFoodMass = 10
    massToBeAllowedToThrow = 120
    foodCount = 300  // TODO(henk): How do we decide how much food there is?
    foodCountMax = 500
    toxinCount = 100
    toxinCountMax = 200
    botMinMass = 10
    botMaxMass = 4000.0
    blobReunionTime = 10.0
    blobSplitMass   = 100.0
    blobSplitVelocity = float32(1.5)
    toxinMassMin = 100
    toxinMassMax = 150
    windowMin = 100
    windowMax = 400

    velocityDecreaseFactor = 0.95

    mwMessageEvery = 1
    guiMessageEvery = 1

)

// -------------------------------------------------------------------------------------------------
// Application
// -------------------------------------------------------------------------------------------------

type Blob struct {
    Position     Vec2       `json:"pos"`
    Mass         float32    `json:"mass"`
    VelocityFac  float32
    ReunionTime  float32
    TargetPos    Vec2
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
    ws                      *websocket.Conn
}

type Application struct {
    fieldSize           Vec2
    // TODO(henk): Simply search the key of 'blobs' and assign an unused key to a newly connected client.
    nextGuiId           GuiId
    nextBotId           BotId
    nextBlobId          BlobId
    nextFoodId          FoodId
    nextToxinId         ToxinId
    guiConnections      map[GuiId]GuiConnection
    foods               map[FoodId]Food
    toxins              map[ToxinId]Toxin
    bots                map[BotId]Bot

    mwInfo              chan(MwInfo)
}

var app Application

func (app* Application) initialize() {
    app.fieldSize       = Vec2{ 1000, 1000 }  // TODO(henk): How do we decide the field size? Is it a constant?
    app.nextGuiId       = 0                       // TODO(henk): Don't do this stuff. Simply search for free id. But this can't be implemented until the bots can connect.
    app.nextBotId       = 0
    app.nextBlobId      = 0
    app.nextFoodId      = foodCount
    app.nextToxinId     = toxinCount
    app.guiConnections  = make(map[GuiId]GuiConnection)

    app.foods           = make(map[FoodId]Food)
    app.bots            = make(map[BotId]Bot)
    app.toxins          = make(map[ToxinId]Toxin)

    app.mwInfo          = make(chan MwInfo, 1000)

    for i := FoodId(0); i < foodCount; i++ {
        mass := foodMassMin + rand.Float32() * (foodMassMax - foodMassMin)
        app.foods[i] = Food{ true, false, false, mass, Mulv(RandomVec2(), app.fieldSize), RandomVec2() }
    }

    for i := 0; i < toxinCount; i++ {
        app.toxins[ToxinId(i)] = Toxin{true, false, Mulv(RandomVec2(), app.fieldSize), toxinMassMin, RandomVec2()}
    }
}

func (app* Application) createGuiId() GuiId {
    var id = app.nextGuiId
    app.nextGuiId = id + 1 // TODO(henk): Must be synchronized
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

func calcBlobVelocityFromMass(vel Vec2, mass float32) Vec2 {
    // This is the maximum mass for now.
    var factor = 1.0 - mass/botMaxMass
    if mass > 0.9*botMaxMass {
        // So blobs never stop moving completely.
        factor = 0.125
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
        return mass - (mass/botMaxMass)*dt*50.0
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
                    Logln(LtDebug, "Reunion!")
                    // Merge them together.
                    var tmp = (*bot).Blobs[k]
                    tmp.Mass += (*bot).Blobs[k2].Mass
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
            newBlobMap[newIndex] = Blob{ subBlob.Position, newMass, blobSplitVelocity, blobReunionTime, NullVec2()}
        }
    }

    // Just so we don't edit the map while iterating over it!
    for index,blob := range newBlobMap {
        (*bot).Blobs[index] = blob
    }
}

func throwAllBlobsOfBot(bot *Bot) bool {
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
            checkNaNV(blob.TargetPos, prefix, "blob.TargetPos")
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

func (app* Application) startUpdateLoop() {
    ticker := time.NewTicker(time.Millisecond * 30)
    var lastTime = time.Now()

    var fpsAdd float32
    fpsAdd = 0
    var fpsCnt = 0

    var guiMessageCounter = 0
    var mWMessageCounter = 0

    for t := range ticker.C {
        var dt = float32(t.Sub(lastTime).Nanoseconds()) / 1e9
        lastTime = t

        //CheckPotentialPlayer("nick2")

        if dt >= 0.03 {
            dt = 0.03
        }

        if fpsCnt == 30 {
            Logf(LtDebug, "fps: %v\n", fpsAdd / float32(fpsCnt+1))
            fpsAdd = 0
            fpsCnt = 0
        }
        fpsCnt += 1
        fpsAdd += 1.0 / dt

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
                    if mwInfo.createNewBot {
                        app.bots[mwInfo.botId] = createStartingBot(mwInfo.ws, mwInfo.botInfo)
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
                newPosition =  Add (newPosition, blob.TargetPos)

                //Logf(LtDebug, "old: %v; new: %v, velocity: %v, time: %v\n", oldPosition, newPosition, velocity, time)

                blob.Position = newPosition

                //singleBlob.Position = Add(singleBlob.Position, Muls(calcBlobVelocity(&singleBlob, blob.TargetPos), dt * 100))
                blob.Mass = calcBlobbMassLoss(blob.Mass, dt)
                blob.VelocityFac = blob.VelocityFac * velocityDecreaseFactor

                // So this is not added all the time but just for a short moment!
                blob.TargetPos = Muls(blob.TargetPos, 20.0*dt)

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
            var windowDiameter float32 = 0.1 * completeMass + 100

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
        if rand.Intn(100) <= 5 && len(app.toxins) < toxinCountMax {
            newToxinId := app.createToxinId()
            app.toxins[newToxinId] = Toxin{true, false, Mulv(RandomVec2(), app.fieldSize), toxinMassMin, RandomVec2()}
        }
        if rand.Intn(100) <= 5 && len(app.foods) < foodCountMax {
            newFoodId := app.createFoodId()
            mass := foodMassMin + rand.Float32() * (foodMassMax - foodMassMin)
            app.foods[newFoodId] = Food{ true, false, false, mass, Mulv(RandomVec2(), app.fieldSize), RandomVec2() }
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
        // BOT INTERACTION WITH EVERYTHING
        ////////////////////////////////////////////////////////////////

        killedBlobs := NewIdsContainer()

        // Split command received - Creating new Blob with same ID.

        for botId, bot := range app.bots {
            if bot.Command.Action == BatSplit {
                Logln(LtDebug, "Split!")

                var bot = app.bots[botId]
                var botRef = &bot
                splitAllBlobsOfBot(botRef)
                app.bots[botId] = *botRef

            } else if bot.Command.Action == BatThrow {
                bot := app.bots[botId]
                somebodyThrew := throwAllBlobsOfBot(&bot)
                if somebodyThrew {
                    Logln(LtDebug, "Throw!")
                }
                app.bots[botId] = bot
            }
        }



        //checkAllValuesOnNaN("second")

        // Splitting the Toxin!
        for toxinId, toxin := range app.toxins {

            if toxin.Mass > toxinMassMax {
                // Reset Mass
                toxin.Mass = toxinMassMin
                // Create new Toxin (moving!)
                newId := app.createToxinId()
                newToxin := Toxin {
                    IsNew: true,
                    IsMoving: true,
                    Position: toxin.Position,
                    Mass: toxinMassMin,
                    Velocity: toxin.Velocity,
                }
                app.toxins[newId] = newToxin
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



        //checkAllValuesOnNaN("third")

        // Blob Collision with Toxin
        eatenToxins := make([]ToxinId, 0)
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

                    if Dist(singleBlob.Position, toxin.Position) < singleBlob.Radius() && singleBlob.Mass >= 400 {
                        subMap := make(map[BlobId]Blob)

                        explodeBlob(botId, blobId, &subMap)
                        exploded = true

                        // Add all the new explosions:
                        for i,b := range subMap {
                            mapOfAllNewSingleBlobs[i] = b
                        }

                        blobsToDelete = append(blobsToDelete, blobId)
                        //eatenToxins = append(eatenToxins, tId)

                        toxin.Position = Mulv(RandomVec2(), app.fieldSize)
                        toxin.IsNew = true
                        toxin.Mass = toxinMassMin
                        //delete(app.toxins, tId)
                    }
                }

                if exploded {
                    Logln(LtDebug, "Exploded!")
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
        eatenFoods := make([]FoodId, 0)
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
                            food.Position = Mulv(RandomVec2(), app.fieldSize)
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
                if Length(Sub(food.Position, toxin.Position)) < Radius(toxin.Mass) {
                    toxin.Mass = toxin.Mass + food.Mass
                    // Always get the velocity of the last eaten food so the toxin (when split)
                    // gets the right velocity of the last input.
                    if Length(food.Velocity) <= 0.01 {
                        food.Velocity = RandomVec2()
                    }
                    toxin.Velocity = Muls(NormalizeOrZero(food.Velocity), 100)
                    if food.IsThrown {
                        delete(app.foods, foodId)
                        eatenFoods = append(eatenFoods, foodId)
                    } else {
                        food.Position = Mulv(RandomVec2(), app.fieldSize)
                        food.IsNew = true
                        app.foods[foodId] = food
                    }
                }
                app.toxins[tId] = toxin
            }
        }

        //checkAllValuesOnNaN("thorth")

        //
        // Eating blobs
        //
        // ToDo(Maurice|Henk): Die Schleifen müssen umgestellt werden, dass keine Blob-Daten
        // innerhalb der inneren Schleife/n geändert/gelöscht werden. Damit machen wir uns unter Umständen
        // den Iterator der obersten Schleife kaputt!!!
        //
        // Nein machen wir nicht: https://golang.org/doc/effective_go.html#for  :-D
        //
        deadBots := make([]BotId, 0)

        for botId1, _ := range app.bots {
            bot1 := app.bots[botId1]
            for blobId1, blob1 := range app.bots[botId1].Blobs {
                //blob1 := bot1.Blobs[blobId1]

                for botId2, bot2 := range app.bots {
                    if botId1 != botId2 {
                        //bot2 := app.bots[botId2]
                        for blobId2, blob2 := range app.bots[botId2].Blobs {
                            //blob2 := bot2.Blobs[blobId2]

                            inRadius := Dist(blob2.Position, blob1.Position) < blob1.Radius()
                            smaller := blob2.Mass < 0.9*blob1.Mass

                            if smaller && inRadius {
                                blob1.Mass = blob1.Mass + blob2.Mass

                                killedBlobs.insert(botId2, blobId2)

                                delete(app.bots[botId2].Blobs, blobId2)

                                // Completely delete this bot.
                                if len(app.bots[botId2].Blobs) <= 0 {
                                    deadBots = append(deadBots, botId2)
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

            }



            for _, botId := range deadBots {
                message.DeletedBots = append(message.DeletedBots, botId)
                message.DeletedBotInfos = append(message.DeletedBotInfos, botId)
            }

            if guiMessageCounter % guiMessageEvery == 0 {
                for foodId, food := range app.foods {
                    if food.IsMoving || food.IsNew || guiConnection.IsNewConnection {
                        key := strconv.Itoa(int(foodId))
                        message.CreatedOrUpdatedFoods[key] = makeServerGuiFood(food)
                        if food.IsNew {
                            food.IsNew = false
                        }
                    }
                    app.foods[foodId] = food
                }
            }

            for _, foodId := range eatenFoods {
                message.DeletedFoods = append(message.DeletedFoods, foodId)
            }

            if guiMessageCounter % guiMessageEvery == 0 {
                for toxinId, toxin := range app.toxins {
                    if toxin.IsNew || toxin.IsMoving || guiConnection.IsNewConnection {
                        key := strconv.Itoa(int(toxinId))
                        message.CreatedOrUpdatedToxins[key] = makeServerGuiToxin(toxin)
                        if toxin.IsNew {
                            toxin.IsNew = false
                        }
                    }
                    app.toxins[toxinId] = toxin
                }
            }

            for _, toxinId := range eatenToxins {
                message.DeletedToxins = append(message.DeletedToxins, toxinId)
            }

            err := websocket.JSON.Send(connection, message)
            if err != nil {
                Logf(LtDebug, "JSON could not be sent because of: %v\n", err)
                //return
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
    }
}

func handleGui(ws *websocket.Conn) {
    var guiId          = app.createGuiId()         // TODO(henk): When do we delete the blob?

    app.guiConnections[guiId] = GuiConnection{ ws, true }

    Logf(LtDebug, "Got connection for Gui %v\n", guiId)

    // Normal request/response loop
    var err error
    for {
        var reply string

        if err = websocket.Message.Receive(ws, &reply); err != nil {
            Logf(LtDebug, "Can't receive (%v)\n", err)
            break
        }

        //Logf(LtDebug, "Received back from client: %v\n", reply)

        //if reply != "OK!" {
        //    Logf(LtDebug, "Received something unexpected back from client: %v\n", reply)
        //}

        //msg := "Received:  " + reply
        //fmt.Println("Sending to client: " + msg)

        /*if err = websocket.Message.Send(ws, msg); err != nil {
            Logln(LtDebug, "Can't send")
            break
        }*/
    }
}

func createStartingBot(ws *websocket.Conn, botInfo BotInfo) Bot {
    blob := Blob {
        Position:       Mulv(RandomVec2(), app.fieldSize),  // TODO(henk): How do we decide this?
        Mass:           100.0,
        VelocityFac:    1.0,
        ReunionTime:    0.0,
        TargetPos:      NullVec2(),
    }
    return Bot{
        Info:                   botInfo,
        GuiNeedsInfoUpdate:     true,
        ViewWindow:             ViewWindow{ Position: Vec2{0,0}, Size:Vec2{100,100} },
        Blobs:                  map[BlobId]Blob{ 0: blob },
        Command:                BotCommand{ BatNone, RandomVec2(), },
        Connection:             ws,
        ConnectionAlive:        true,
    }
}

func handleMiddleware(ws *websocket.Conn) {
    var botId = app.createBotId()

    Logf(LtDebug, "Got connection from Middleware %v\n", botId)

    var err error
    for {
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
            app.mwInfo <- MwInfo {botId, cmd, false, false, bi, nil}

            return
        }

        //
        // Evaluate the message
        //
        if message.Type == MmstBotCommand {
            if message.BotCommand != nil {

                var bi  BotInfo
                app.mwInfo <- MwInfo{botId, *message.BotCommand, true, false, bi, nil}


            } else {
                Logf(LtDebug, "Got a dirty message from bot %v. BotCommand is nil.\n", botId)
            }
        } else if message.Type == MmstBotInfo {
            if message.BotInfo != nil {

                var cmd BotCommand
                app.mwInfo <- MwInfo{botId, cmd, true, true, *message.BotInfo, ws}

                Logf(LtDebug, "Bot %v registered: %v.\n", botId, *message.BotInfo)
            } else {
                Logf(LtDebug, "Got a dirty message from bot %v. BotInfo is nil.\n", botId)
            }
        }
    }
}

func main() {
    // TODO(henk): Maybe we wanna toggle this at runtime.
    SetLoggingDebug(true)
    SetLoggingVerbose(false)

    app.initialize()

    UpdateAllSVN()

    // Run the update-loop in parallel to serve the websocket on the main thread.
    go app.startUpdateLoop()

    // Assign the handler for Gui-Connections
    //http.Handle("/", websocket.Handler(handleGui))

    http.HandleFunc("/",
        func (w http.ResponseWriter, req *http.Request) {
            s := websocket.Server{Handler: websocket.Handler(handleGui)}
            s.ServeHTTP(w, req)
        });


    // Assign the handler for Middleware-Connections
    http.Handle("/middleware/", websocket.Handler(handleMiddleware))
    // Get the stuff running
    if err := http.ListenAndServe(":1234", nil); err != nil {
        log.Fatal("ListenAndServe:", err)
    }
}
