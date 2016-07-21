package connections

import (
    . "Programmierwettbewerb-Server/shared"
    . "Programmierwettbewerb-Server/vector"

    "golang.org/x/net/websocket"
    "sync"
    "math"
    "strconv"
)

////////////////////////////////////////////////////////////////////////
//
// MiddlewareConnection
//
////////////////////////////////////////////////////////////////////////

type MiddlewareConnection struct {
    Connection          *websocket.Conn
    MessageChannel      chan ServerMiddlewareGameState
    ConnectionAlive     bool                // TODO(henk): What shall we do when the connection is lost?
}

////////////////////////////////////////////////////////////////////////
//
// MiddlewareConnections
//
////////////////////////////////////////////////////////////////////////

type MiddlewareConnections struct {
    sync.Mutex
    connections         map[BotId]MiddlewareConnection
}

func NewMiddlewareConnections() MiddlewareConnections {
    return MiddlewareConnections{        
        connections: make(map[BotId]MiddlewareConnection),
    }
}

func (middlewareConnections *MiddlewareConnections) Add(botId BotId, middlewareConnection MiddlewareConnection) {
    middlewareConnections.Lock()
    defer middlewareConnections.Unlock()
    
    middlewareConnections.connections[botId] = middlewareConnection
}

func (middlewareConnections *MiddlewareConnections) Delete(botId BotId) {
    middlewareConnections.Lock()
    defer middlewareConnections.Unlock()
    
    middlewareConnection, found := middlewareConnections.connections[botId]
    if found {
        middlewareConnection.Connection.Close()    
        close(middlewareConnection.MessageChannel)

        delete(middlewareConnections.connections, botId)
    }
}

func (middlewareConnections *MiddlewareConnections) IsAlive(botId BotId) bool {
    middlewareConnections.Lock()
    defer middlewareConnections.Unlock()
    
    return middlewareConnections.connections[botId].ConnectionAlive
}

type MiddlewareConnectionHandler func(BotId, MiddlewareConnection)
func (middlewareConnections *MiddlewareConnections) Foreach(middlewareConnectionHandler MiddlewareConnectionHandler) {
    middlewareConnections.Lock()
    defer middlewareConnections.Unlock()
    
    for botId, middlewareConnection := range middlewareConnections.connections {
        middlewareConnectionHandler(botId, middlewareConnection)
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

func IsInViewWindow(viewWindow ViewWindow, position Vec2, radius float32) bool {
    return position.X > viewWindow.Position.X &&
           position.Y > viewWindow.Position.Y &&
           position.X < viewWindow.Position.X + viewWindow.Size.X &&
           position.Y < viewWindow.Position.Y + viewWindow.Size.Y;
}

////////////////////////////////////////////////////////////////////////
//
// ServerGuiUpdateMessage
//
////////////////////////////////////////////////////////////////////////

type ServerGuiBot struct {
    Blobs       map[string]ServerGuiBlob    `json:"blobs"`
    ViewWindow  ViewWindow                  `json:"viewWindow"`
}

func NewServerGuiBot(bot Bot) ServerGuiBot {
    blobs := make(map[string]ServerGuiBlob)
    for blobId, blob := range bot.Blobs {
        key := strconv.Itoa(int(blobId))
        blobs[key] = NewServerGuiBlob(blob)
    }
    return ServerGuiBot{ blobs, bot.ViewWindow }
}

type ServerGuiBlob struct {
    Position    Vec2        `json:"pos"`
    Mass        float32     `json:"mass"`
}

func NewServerGuiBlob(blob Blob) ServerGuiBlob {
    return ServerGuiBlob{ blob.Position, blob.Mass }
}

type ServerGuiFood struct {
    Position    Vec2        `json:"pos"`
    Mass        float32     `json:"mass"`
}

func NewServerGuiFood(food Food) ServerGuiFood {
    return ServerGuiFood{ food.Position, food.Mass }
}

type ServerGuiToxin struct {
    Position    Vec2        `json:"pos"`
    Mass        float32     `json:"mass"`
}

func NewServerGuiToxin(toxin Toxin) ServerGuiToxin {
    return ServerGuiToxin{ toxin.Position, toxin.Mass }
}

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

func NewServerGuiUpdateMessage() ServerGuiUpdateMessage {
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

////////////////////////////////////////////////////////////////////////
//
// GuiConnection
//
////////////////////////////////////////////////////////////////////////

type GuiConnection struct {
    Connection          *websocket.Conn
    IsNewConnection     bool
    MessageChannel      chan ServerGuiUpdateMessage
}

////////////////////////////////////////////////////////////////////////
//
// Gui Connections
//
////////////////////////////////////////////////////////////////////////

type GuiConnections struct {
    sync.Mutex
    connections     map[GuiId]GuiConnection
}

func NewGuiConnections() GuiConnections {
    return GuiConnections{
        connections: make(map[GuiId]GuiConnection),
    }
}

func (guiConnections *GuiConnections) Count() int {
    guiConnections.Lock()
    defer guiConnections.Unlock()
    
    return len(guiConnections.connections)
}

func (guiConnections *GuiConnections) MakeAllOld() {
    guiConnections.Lock()
    for guiConnectionId, guiConnection := range guiConnections.connections {
        guiConnection.IsNewConnection = false
        guiConnections.connections[guiConnectionId] = guiConnection
    }
    guiConnections.Unlock()
}

func (guiConnections *GuiConnections) Add(guiId GuiId, guiConnection GuiConnection) {
    guiConnections.Lock()
    defer guiConnections.Unlock()
    
    guiConnections.connections[guiId] = guiConnection
}

func (guiConnections *GuiConnections) Delete(guiId GuiId) {
    guiConnections.Lock()
    defer guiConnections.Unlock()
    
    guiConnection, found := guiConnections.connections[guiId]
    if found {
        guiConnection.Connection.Close()
        close(guiConnection.MessageChannel)
        delete(guiConnections.connections, guiId)
    }
}

type GuiConnectionHandler func(GuiId, GuiConnection)
func (guiConnections *GuiConnections) Foreach(connectionHandler GuiConnectionHandler) {
    guiConnections.Lock()
    defer guiConnections.Unlock()
    
    for guiId, guiConnection := range guiConnections.connections {
        connectionHandler(guiId, guiConnection)
    }
}


