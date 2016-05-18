type Position   = (Float, Float)
type Mass       = Int
type BotId      = Int -- Wenn sich ein Blob teilt, ist diese Id für alle gleich.
type TeamId     = Int -- Teams werden im Wettbewerb vergeben und sind für einige Bots gleich.
type Index      = Int -- Jeder Blob, jedes Food und jedes Toxin bekommt einen eindeutigen Index.
type Name       = String
type Image      = String
type Color      = (Int, Int, Int)
type ImageUrl   = String
type Bot        = (Name, ImageUrl, Color)

type BlobbyDude = (Position, Mass) -- TODO(henk): Should Food and Toxin be the same?
type Food       = BlobbyDude
type Toxin      = BlobbyDude
type Blob       = (BotId, TeamId, Index, Position, Mass)

data BlobAction = None | Throw | Split

-- ---------------------------------------------------------------------------------------------

{-

    Gui  <----------------------+
                                |
    ^                           |
    |                           |
    | (Websocket for json)      | (http for html)
    |                           |
    v                           v

    Server                      Webserver

    ^
    |
    | (Websocket)
    |
    +---------------+---------------+----- ...
    |               |               |
    |               |               |
    v               v               v

    Middleware      Middleware      Middleware

    ^               ^               ^
    |               |               |
    | (stdin/       | (stdin/       | (stdin/
    |  stdout)      |  stdout)      |  stdout)
    |               |               |
    v               v               v

    Bot             Bot             Bot

-}

-- ---------------------------------------------------------------------------------------------

-- TODO(henk): Versendet der Server die Daten zu den Bots und zur GUI immer mit seiner eigenen
--             update-Rate? Oder könnte es gut sein, den Server das Spielfeld auf einer höheren
--             Frequenz berechnen zu lassen?
--             Beispiel:
--                  - Update des Spielzustands  alle 20ms (so sind die Integrationen korrekter)
--                  - Update der Bots           alle 50ms
--                  - Update der GUIs           alle 100ms

-- ---------------------------------------------------------------------------------------------

--
-- MIDDLEWARE ---> SERVER
--
-- Wann: Wenn sich ein Bot beim Server anmeldet.
--
-- Nachricht für die Anmeldung eines neuen Bots beim Server.
--
-- 1. Der Name des Bots wird im Makefile festgelegt.
-- 2. Die Farbe wird im Makefile festgelegt.
-- 3. Das Bild wird im Makefile festgelegt und im JSON-Format
--    übertragen, ist jedoch optional.
--
-- Eine Farbe wird auch benötigt, wenn ein Bild festgelegt wird, da
-- der Rand des Blobs (Und abgegebene Materie) diese Farbe erhält.
--
-- Der Server gibt keine spezielle Antwort auf diese Nachricht; der
-- normale Spielbetrieb wird gestartet.
--
type RegisterMessage_WM_Server = (Name, Color, Maybe Image)

--
-- SERVER ---> MIDDLEWARE
--
-- Wann: Mit der bot-update-Rate des Servers.
--
-- Nachricht für das Übertragen des Spielstandes zur Middleware.
--
-- ([Eigene Blobs], [Alle anderen Blobs], [Alle Foods], [Alle Toxins])
--
-- Die Listen [Alle anderen Blobs], [Alle Foods] und [Alle Toxins]
-- enthalten nur die Blobs, Foods und Toxins, die im Sichtbereich des
-- Bots sind.
--
type UpdateMessage_Server_WM = ([Blob], [Blob], [Food], [Toxin])

--
-- MIDDLEWARE ---> SERVER
--
-- Wann: Immer, wenn der Bot eine Berechnung gemacht hat.
--
-- Nachricht vom Bot, die von der Middleware nur weitergeleitet wird.
--
type UpdateMessage_MW_Server = Message_perFrame_Bot_MW

-- ---------------------------------------------------------------------------------------------

--
-- MIDDLEWARE ---> BOT
--
-- Wann: Mit der bot-update-Rate des Servers.
--
-- Dies ist die Nachricht, die der Server and die Middleware gesendet hat.
-- Sie wird einfach an den Bot weitergeleitet. Dies geschieht jedoch nur,
-- wenn der Bot auch eine Antwort liefert.
--
-- TODO(henk): Was passiert, wenn der Bot gar nicht mehr antwortet?
--             Startet die Middleware den Bot neu?
--
type UpdateMessage_MW_Bot = UpdateMessage_Server_WM

--
-- BOT ---> MIDDLEWARE
--
-- Wann: Immer, wenn der Bot eine Berechnung gemacht hat.
--
-- Dies ist die Antwort des Bots auf den Spielzustand. Diese wird von der
-- Middleware an den Server weitergeleitet.
--
type UpdateMessage_Bot_MW  = (BlobAction, Position)

-- ---------------------------------------------------------------------------------------------

--
-- SERVER ---> GUI
--
-- Wann: Wenn sich eine GUI verbindet oder wenn sich ein neuer Bot mit dem
--       Server verbunden hat.
--
-- Wenn sich eine GUI mit dem Server verbindet, schickt dieser eine Nachricht
-- mit den Daten, die sich nicht pro frame ändern.
--
-- Wenn sich ein neuer Bot am Server anmeldet, wird diese Nachricht neu an die
-- Gui geschickt. Das sollte einem vollständigen Zurücksetzen der Gui entsprechen.
--
-- TODO(henk): Foods können wir erstmal pro frame schicken. Optimieren können
--             wir später!?
--
type StateMessage_Server_Gui = [Bot]

--
-- SERVER ---> GUI
--
-- Wann: Mit der gui-update Rate des Servers.
--
-- In jedem Berechnungsschritt des Servers werden diese Daten an alle GUIs gesendet.
--
type UpdateMessage_Server_Gui = ([Blob], [Food], [Toxin])
