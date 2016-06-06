package main

import (
    .   "Programmierwettbewerb-Server/shared"

    "golang.org/x/net/websocket"
)

// -------------------------------------------------------------------------------------------------
// Global
// -------------------------------------------------------------------------------------------------

const (
    // This is used, when no connection is given via command line argument.
    globalDefaultAddress string = "ws://127.0.0.1:1234/servercommand/"
)

func setupServerConnection(address string) {

    //
    // Connect to the websocket server
    //
    origin := "http://localhost/" // TODO(henk): What is the origin?
    ws, err := websocket.Dial(address, "", origin)
    if err != nil {
        return
    }

    //
    // Try and exit
    //
    message := MessageServerCommands {
        Stuff:  3,
    }
    websocket.JSON.Send(ws, message)
    ws.Close()
}

func main() {
    setupServerConnection(globalDefaultAddress)
    Logf(LtDebug, "Finished for good.\n")
}

