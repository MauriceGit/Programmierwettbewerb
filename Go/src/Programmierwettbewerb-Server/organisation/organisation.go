package organisation

import (
    . "Programmierwettbewerb-Server/shared"
    "time"
    "encoding/json"
    "io/ioutil"
    "os/exec"
    "os"
    "bufio"
)


type PlayerData struct {
    Nicknames   []string  `json:"nicknames"`
    Score       int       `json:"playerScore"`
    // ...
}

type SvnPlayerData struct {
    SvnReposInformation map[string]PlayerData `json:"svnReposMap"`
}

var botNames = "bot.names"
var playerStatsFile = "playerStats.json"
var svnBasePath = "/home/maurice/Uni"
// Old date, so it updates the data the very first time.
var lastUpdate = time.Date(2016, time.January, 1, 1, 1, 1, 1, time.FixedZone("Europe", 1))
var playerData SvnPlayerData

func updateAllData()  {
    file, e := ioutil.ReadFile(playerStatsFile)
    if e != nil {
        Logf(LtDebug, "File error while updating player data: %v\n", e)
        return
    }
    json.Unmarshal(file, &playerData)
    //Logf(LtDebug, "Results: %v\n", playerData)
}

func printError(err error) {
    if err != nil {
        Logf(LtDebug, "==> Error: %s\n", err.Error())
    }
}

func printOutput(outs []byte) {
    if len(outs) > 0 {
        Logf(LtDebug, "==> Output: %s\n", string(outs))
    }
}

func pullSVN(pathToSvn string) {
    // This will be "svn update" in the end
    //cmd := exec.Command("git", "pull")
    // This is just for debugging purposes!
    cmd := exec.Command("ls")
    cmd.Dir = pathToSvn
    cmd.CombinedOutput() // returnes: output, err := ...
    //printError(err)
    //printOutput(output)
}

// readLines reads a whole file into memory
// and returns a slice of its lines.
func readLines(path string) ([]string, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    var lines []string
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        lines = append(lines, scanner.Text())
    }
    return lines, scanner.Err()
}

// reads all lines from the bot.names files and updates the json file.
func updateJsonFile(pathToSvn string, repos string) {

    // OK, for now we make it just quick and dirty!
    // Read json file, read the bot.names and if name not in map, append name.
    // Then just overwrite the original file with the new content!

    // Reading bot.names as list with Nicknames
    lines, err := readLines(pathToSvn + "/" + botNames)
    if err != nil {
        Logf(LtDebug, "Error reading the bot.names: %v\n", err)
        return
    }

    Logf(LtDebug, "This now runs for the repos: %v\n", repos)

    for i,l := range lines {
        Logf(LtDebug, "line: %v --> %v\n", i, l)
    }

    // Reading the JSON file with playerdata
    var tmpData SvnPlayerData
    file, e := ioutil.ReadFile(playerStatsFile)
    if e != nil {
        Logf(LtDebug, "File error while reading player data: %v\n", e)
        return
    }
    json.Unmarshal(file, &tmpData)

    reposData := tmpData.SvnReposInformation[repos]

    // Check, if we have a new nickname!
    for _, newNick := range lines {
        found := false
        for _,nick := range reposData.Nicknames {
            if nick == newNick {
                found = true
            }
        }
        if !found {
            // Add the new nickname!
            reposData.Nicknames = append(reposData.Nicknames, newNick)
        }
    }

    // Write changed data back
    tmpData.SvnReposInformation[repos] = reposData

    // Writing back to file
    b, err := json.Marshal(tmpData)

    f, err := os.Create(playerStatsFile)
    if err != nil {
        Logf(LtDebug, "Something went wront when creating the new file...: %v\n", err)
    }
    defer f.Close()

    f.Write(b)
    f.Sync()
}

func UpdateAllSVN() {

    files, _ := ioutil.ReadDir(svnBasePath)
    for _, f := range files {
        if f.IsDir() {
            pullSVN(svnBasePath + "/" + f.Name())
            updateJsonFile(svnBasePath + "/" + f.Name(), f.Name())
        }
    }

    updateAllData()
    lastUpdate = time.Now()
}


// If the last SVN-Update is older than 10 Minutes, update SVN-Repositories.
// And always when the server is first started!
//
// ATTENTION:
// Please take care, that the global path variables are correctly set (Especially: svnBasePath)!
// --> The svnBasePath contains all the SVN-directories, for example: pwb_14/
// --> The server is started in the same directory ideally but can vary.
// --> Every SVN directory must (or should normally, nothing breaks if it isn't there) contain a bot.names file!
// --> The playerStatsFile must be in the same directory as the server!
//
// This function checks, if the given nickname is a valid one, (from at least one bot.names).
// Right now, it updates itself automatically (svn update, file update, data update) every 1 minutes (maximum but only when this function is called).
func CheckPotentialPlayer(playerNickname string) bool {

    if time.Now().Sub(lastUpdate).Minutes() > 1 {
        UpdateAllSVN()
    }

    for i,svn := range playerData.SvnReposInformation {
        for _,nick := range svn.Nicknames {
            if nick == playerNickname {
                i=i
                //Logf(LtDebug, "The player %v can be associated with the svn-repos %v\n", playerNickname, i)
                return true
            }
        }
    }

    Logf(LtDebug, "The player %v can not be associated with any svn-repos!\n", playerNickname)
    return false
}


