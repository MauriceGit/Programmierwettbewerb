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
    Websocket               *websocket.Conn
    MessageChannel          chan ServerMiddlewareGameState
    StandbyNotification     chan bool
    StopServerNotification  chan bool
    IsStandbyChanging       bool
}

func NewMiddlewareConnection(websocket *websocket.Conn, messageChannel chan ServerMiddlewareGameState, standbyNotification chan bool, stopServerNotification chan bool, isStandbyChanging bool) MiddlewareConnection {
    return MiddlewareConnection{
        Websocket:              websocket,
        MessageChannel:         messageChannel,
        StandbyNotification:    standbyNotification,
        StopServerNotification: stopServerNotification,
        IsStandbyChanging:      isStandbyChanging,
    }
}

////////////////////////////////////////////////////////////////////////
//
// MiddlewareConnections
//
////////////////////////////////////////////////////////////////////////

type MiddlewareConnections struct {
    mutex           sync.Mutex
    connections     map[BotId]MiddlewareConnection
}

func NewMiddlewareConnections() MiddlewareConnections {
    return MiddlewareConnections{        
        connections: make(map[BotId]MiddlewareConnection),
    }
}

func (middlewareConnections *MiddlewareConnections) Add(botId BotId, middlewareConnection MiddlewareConnection) {
    middlewareConnections.mutex.Lock()
    defer middlewareConnections.mutex.Unlock()
    
    middlewareConnections.connections[botId] = middlewareConnection
}

func (middlewareConnections *MiddlewareConnections) Delete(botId BotId) {
    middlewareConnections.mutex.Lock()
    defer middlewareConnections.mutex.Unlock()
    
    middlewareConnection, found := middlewareConnections.connections[botId]
    if found {
        middlewareConnection.Websocket.Close()
        close(middlewareConnection.MessageChannel)
        delete(middlewareConnections.connections, botId)
    }
}

func (middlewareConnections *MiddlewareConnections) Count() int {
    middlewareConnections.mutex.Lock()
    defer middlewareConnections.mutex.Unlock()
    
    return len(middlewareConnections.connections)
}

type MiddlewareConnectionHandler func(BotId, MiddlewareConnection)
func (middlewareConnections *MiddlewareConnections) Foreach(middlewareConnectionHandler MiddlewareConnectionHandler) {
    middlewareConnections.mutex.Lock()
    defer middlewareConnections.mutex.Unlock()
    
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
    TeamId              TeamId
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

var serverGuiDecimalPlaceFactor float32 = 10

type ServerGuiBot struct {
    Blobs       map[string]ServerGuiBlob    `json:"blobs"`
    TeamId      TeamId                      `json:"teamId"`
    ViewWindow  ViewWindow                  `json:"viewWindow"`
}

func NewServerGuiBot(bot Bot) ServerGuiBot {
    blobs := make(map[string]ServerGuiBlob)
    for blobId, blob := range bot.Blobs {
        key := strconv.Itoa(int(blobId))
        blobs[key] = NewServerGuiBlob(blob)
    }
    viewWindow := ViewWindow{ 
        Position: ToFixedVec2(bot.ViewWindow.Position, serverGuiDecimalPlaceFactor),
        Size: ToFixedVec2(bot.ViewWindow.Size, serverGuiDecimalPlaceFactor),
    }
    return ServerGuiBot{ blobs, bot.TeamId, viewWindow }
}

type ServerGuiBlob struct {
    Position    Vec2       `json:"pos"`
    Mass        int        `json:"mass"`
}

func NewServerGuiBlob(blob Blob) ServerGuiBlob {
    return ServerGuiBlob{ ToFixedVec2(blob.Position, serverGuiDecimalPlaceFactor), int(blob.Mass) }
}

type ServerGuiFood struct {
    Position    Vec2        `json:"pos"`
    Mass        int         `json:"mass"`
}

func NewServerGuiFood(food Food) ServerGuiFood {
    return ServerGuiFood{ ToFixedVec2(food.Position, serverGuiDecimalPlaceFactor), int(food.Mass) }
}

type ServerGuiToxin struct {
    Position    Vec2        `json:"pos"`
    Mass        int         `json:"mass"`
}

func NewServerGuiToxin(toxin Toxin) ServerGuiToxin {
    return ServerGuiToxin{ ToFixedVec2(toxin.Position, serverGuiDecimalPlaceFactor), int(toxin.Mass) }
}

type ServerGuiUpdateMessage struct {
    // "JSON objects only support strings as keys; to encode a Go map type it must be of the form map[string]T (where T is any Go type supported by the json package)."
    // Source: http://blog.golang.org/json-and-go
    CreatedOrUpdatedBotInfos    map[string]BotInfo              `json:"0"`
    DeletedBotInfos             []BotId                         `json:"1"`
    CreatedOrUpdatedBots        map[string]ServerGuiBot         `json:"2"`
    DeletedBots                 []BotId                         `json:"3"`
    CreatedOrUpdatedFoods       map[string]ServerGuiFood        `json:"4"`
    DeletedFoods                []FoodId                        `json:"5"`
    CreatedOrUpdatedToxins      map[string]ServerGuiToxin       `json:"6"`
    DeletedToxins               []ToxinId                       `json:"7"`
    StatisticsThisGame          map[string]Statistics           `json:"8"`
    StatisticsGlobal            map[string]Statistics           `json:"9"`
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
    Connection                  *websocket.Conn
    IsNewConnection             bool
    MessageChannel              chan ServerGuiUpdateMessage
    StopServerNotification      chan bool
}

////////////////////////////////////////////////////////////////////////
//
// Gui Connections
//
////////////////////////////////////////////////////////////////////////

type GuiConnections struct {
    mutex           sync.Mutex
    connections     map[GuiId]GuiConnection
}

func NewGuiConnections() GuiConnections {
    return GuiConnections{
        connections: make(map[GuiId]GuiConnection),
    }
}

func (guiConnections *GuiConnections) Count() int {
    guiConnections.mutex.Lock()
    defer guiConnections.mutex.Unlock()
    
    return len(guiConnections.connections)
}

func (guiConnections *GuiConnections) MakeAllOld() {
    guiConnections.mutex.Lock()
    for guiConnectionId, guiConnection := range guiConnections.connections {
        guiConnection.IsNewConnection = false
        guiConnections.connections[guiConnectionId] = guiConnection
    }
    guiConnections.mutex.Unlock()
}

func (guiConnections *GuiConnections) Add(guiId GuiId, guiConnection GuiConnection) {
    guiConnections.mutex.Lock()
    defer guiConnections.mutex.Unlock()
    
    guiConnections.connections[guiId] = guiConnection
}

func (guiConnections *GuiConnections) Delete(guiId GuiId) {
    guiConnections.mutex.Lock()
    defer guiConnections.mutex.Unlock()
    
    guiConnection, found := guiConnections.connections[guiId]
    if found {
        guiConnection.Connection.Close()        
        close(guiConnection.MessageChannel)
        delete(guiConnections.connections, guiId)
    }
}

type GuiConnectionHandler func(int, GuiId, GuiConnection)
func (guiConnections *GuiConnections) Foreach(connectionHandler GuiConnectionHandler) {
    guiConnections.mutex.Lock()
    defer guiConnections.mutex.Unlock()
    
    var index int = 0
    for guiId, guiConnection := range guiConnections.connections {
        connectionHandler(index, guiId, guiConnection)
        index += 1
    }
}
