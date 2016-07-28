package organisation

import (
    . "Programmierwettbewerb-Server/shared"
    "time"
    "encoding/json"
    "io/ioutil"
    "os/exec"
    "os"
    "math"
    "bufio"
)

type PlayerData struct {
    Nicknames   []string    `json:"nicknames"`
    Statistics  Statistics  `json:"statistics"`
    // ...
}

type SvnPlayerData struct {
    SvnReposInformation map[string]PlayerData `json:"svnReposMap"`
}

var botNames = "bot.names"
var statisticsPath = "../Statistics/"
var playerStatsFile = "playerStats.json"
var svnBasePath = "../SVN/"
// Old date, so it updates the data the very first time.
var lastUpdate = time.Date(2016, time.January, 1, 1, 1, 1, 1, time.FixedZone("Europe", 1))
var playerData SvnPlayerData

// I can access the file, when I can read true from the channel.
// When I am finished, I write true into the empty channel.
// So whenever the file is in use, the channel is empty.
// When the file is not in use, the channel is full.
var fileNotInUse = make(chan bool, 1)

func updateAllData()  {
    file, e := ioutil.ReadFile(statisticsPath + playerStatsFile)
    if e != nil {
        Logf(LtDebug, "File error while updating player data: %v\n", e)
        return
    }
    json.Unmarshal(file, &playerData)
    //Logf(LtDebug, "Results: %v\n", playerData)
}

func printError(err error, path string) {
    if err != nil {
        Logf(LtDebug, "==> Error for %s: %s\n", path, err.Error())
    }
}

func printOutput(outs []byte, path string) {
    if len(outs) > 0 {
        Logf(LtDebug, "==> Output for %s: %s\n", path, string(outs))
    }
}

func pullSVN(pathToSvn string) {
    // This will be "svn update" in the end
    //cmd := exec.Command("git", "pull")
    // This is just for debugging purposes!
    cmd := exec.Command("svn", "update")
    cmd.Dir = pathToSvn
    output, err := cmd.CombinedOutput() // returnes: output, err := ...
    printError(err, pathToSvn)
    printOutput(output, pathToSvn)
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

    //Logf(LtDebug, "This now runs for the repos: %v\n", repos)

    //for i,l := range lines {
    //    Logf(LtDebug, "line: %v --> %v\n", i, l)
    //}

    // Reading the JSON file with playerdata
    var tmpData SvnPlayerData
    file, e := ioutil.ReadFile(statisticsPath + playerStatsFile)
    if e != nil {
        Logf(LtDebug, "File error while reading player data: %v\n", e)
        return
    }
    json.Unmarshal(file, &tmpData)

    reposData := tmpData.SvnReposInformation[repos]

    // So old names are discarted and cannot be used any more
    reposData.Nicknames = make([]string, 0)

    // Check, if we have a new nickname!
    for _, newNick := range lines {

        // validate, that no other repos contains this nickname!
        duplicateFound := false
        for svnName,svn := range tmpData.SvnReposInformation {
            for _,nick := range svn.Nicknames {
                if svnName != repos {
                    if newNick == nick {
                        duplicateFound = true
                    }
                }
            }
        }
        if duplicateFound {
            continue
        }

        found := false
        for _,nick := range reposData.Nicknames {
            if nick == newNick {
                found = true
                break
            }
        }
        if !found {
            // Add the new nickname!
            reposData.Nicknames = append(reposData.Nicknames, newNick)
        }
    }

    if tmpData.SvnReposInformation == nil {
        tmpData.SvnReposInformation = map[string]PlayerData{}
    }

    // Write changed data back
    tmpData.SvnReposInformation[repos] = reposData

    // Writing back to file
    b, err := json.Marshal(tmpData)

    f, err := os.Create(statisticsPath + playerStatsFile)
    if err != nil {
        Logf(LtDebug, "Something went wront when creating the new file...: %v\n", err)
    }
    defer f.Close()

    f.Write(b)
    f.Sync()
}

func UpdateAllSVN() {

    <- fileNotInUse
    files, _ := ioutil.ReadDir(svnBasePath)
    for _, f := range files {
        if f.IsDir() {
            pullSVN(svnBasePath + "/" + f.Name())
            updateJsonFile(svnBasePath + "/" + f.Name(), f.Name())

        }
    }

    updateAllData()
    fileNotInUse <- true
    lastUpdate = time.Now()
}

func WriteStatisticToFile(botName string, stats Statistics) {

    var repos string

    for svnName,svn := range playerData.SvnReposInformation {
        for _,nick := range svn.Nicknames {
            if botName == nick {
                repos = svnName
            }
        }
    }
    if repos == "" {
        Logf(LtDebug, "Something went wrong, the repos for %v is not found " +
            "while trying to save your statistics... Please do not change your bot.names while testing!\n", botName)
        return
    }

    <- fileNotInUse

    tmpData := playerData.SvnReposInformation[repos]
    tmpData.Statistics.MaxSize = float32(math.Max(float64(tmpData.Statistics.MaxSize), float64(stats.MaxSize)))
    tmpData.Statistics.MaxSurvivalTime = float32(math.Max(float64(tmpData.Statistics.MaxSurvivalTime), float64(stats.MaxSurvivalTime)))
    tmpData.Statistics.BlobKillCount = Max(tmpData.Statistics.BlobKillCount, stats.BlobKillCount)
    tmpData.Statistics.BotKillCount = Max(tmpData.Statistics.BotKillCount, stats.BotKillCount)
    tmpData.Statistics.ToxinThrow = Max(tmpData.Statistics.ToxinThrow, stats.ToxinThrow)
    tmpData.Statistics.SuccessfulToxin = Max(tmpData.Statistics.SuccessfulToxin, stats.SuccessfulToxin)
    tmpData.Statistics.SplitCount = Max(tmpData.Statistics.SplitCount, stats.SplitCount)
    tmpData.Statistics.SuccessfulSplit = Max(tmpData.Statistics.SuccessfulSplit, stats.SuccessfulSplit)
    tmpData.Statistics.SuccessfulTeam = Max(tmpData.Statistics.SuccessfulTeam, stats.SuccessfulTeam)
    tmpData.Statistics.BadTeaming = Max(tmpData.Statistics.BadTeaming, stats.BadTeaming)
    playerData.SvnReposInformation[repos] = tmpData

    // Writing back to file
    b, err := json.Marshal(playerData)

    f, err := os.Create(statisticsPath + playerStatsFile)
    if err != nil {
        Logf(LtDebug, "Something went wront when creating or appending the file...: %v\n", err)
    }

    f.Write(b)
    f.Sync()
    f.Close()

    fileNotInUse <- true
}

// If the last SVN-Update is older than 10 Minutes, update SVN-Repositories.
// And always when the server is first started!
//
// ATTENTION:
// Please take care, that the global path variables are correctly set (Especially: svnBasePath)!
// --> The svnBasePath contains all the SVN-directories, for example: pwb_14/
// --> The server is started in the GOBIN path!!!
// --> Every SVN directory must contain a bot.names file (or should normally, nothing breaks if it isn't there)!
//
// This function checks, if the given nickname is a valid one, (from at least one bot.names).
// Right now, it updates itself automatically (svn update, file update, data update) every 1 minutes (maximum but only when this function is called).
func CheckPotentialPlayer(playerNickname string) (bool, string, Statistics) {

    if time.Now().Sub(lastUpdate).Minutes() > 1 {
        UpdateAllSVN()
    }

    for i,svn := range playerData.SvnReposInformation {
        for _,nick := range svn.Nicknames {
            if nick == playerNickname {
                return true, i, svn.Statistics
            }
        }
    }
    return false, "", Statistics{}
}

// This must be called once before any other function of this module is called!
func InitOrganisation() {
    fileNotInUse <- true
}


