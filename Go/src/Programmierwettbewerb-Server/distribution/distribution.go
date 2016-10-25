package distribution

import (
    "bufio"
    "bytes"
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "strings"
    "time"
    "strconv"
    "github.com/BurntSushi/toml"
    "golang.org/x/crypto/ssh"
    "net"
    //"reflect"
)

// Info from config file
type Config struct {
    LoginName       string
    LoginPass       string
    Hosts           []string
    SVNs            []string
    PrivateKey      string
}

var config Config

const (
        Nothing = iota
        Help
        Execute
        Kill
)

func executeCmd(cmd, hostname string, config *ssh.ClientConfig) string {
    if conn, err := ssh.Dial("tcp", hostname+":22", config); err == nil {
        if session, err2 := conn.NewSession(); err2 == nil {

            defer session.Close()

            var stdoutBuf bytes.Buffer
            session.Stdout = &stdoutBuf
            session.Run(cmd)

            return hostname + ": " + stdoutBuf.String()
        }
    }
    return "Network error for " + hostname + "\n"
}

func userPassAuth() *ssh.ClientConfig {
    return &ssh.ClientConfig{
        // User authentification with user/password
        User: config.LoginName,
        Auth: []ssh.AuthMethod{
            ssh.Password(config.LoginPass),
        },
    }
}

func privateKeyAuth() *ssh.ClientConfig {
    pkey, err := ioutil.ReadFile(config.PrivateKey)
    if err != nil {
        log.Fatalf("unable to read private key: %v", err)
    }

    // Create the Signer for this private key.
    signer, err := ssh.ParsePrivateKey(pkey)
    if err != nil {
        log.Fatalf("unable to parse private key: %v", err)
    }

    return &ssh.ClientConfig{
        // User authentication with ssh-key
        User: os.Getenv(config.LoginName),
        Auth: []ssh.AuthMethod{
            // Use the PublicKeys method for remote authentication.
            ssh.PublicKeys(signer),
        },
    }
}

func printHelp() {
    fmt.Println("The following commands are recognised:")
    fmt.Println("go PWB COUNT")
    fmt.Println("   PWB: 'all'")
    fmt.Println("        svn")
    fmt.Println("        [svn] // Without space between entries!")
    fmt.Println("   COUNT: Number of bots to start. For >1 bot, how often each one is started.")
}

func isValidSVN(v string, svns []string) bool {
    for _, svn := range svns {
        if v == svn {
            return true
        }
    }
    return false
}

// Reads info from config file
func readConfig(name string) (Config, error) {
    var configfile = name
    _, err := os.Stat(configfile)
    if err != nil {
        fmt.Printf("Config file is missing: %v\n", configfile)
        //os.Exit(1)
        return Config{}, err
    }

    var config Config
    if _, err := toml.DecodeFile(configfile, &config); err != nil {
        fmt.Printf("%v\n", err)
        return Config{}, err
        //os.Exit(1)
    }

    return config, nil
}

// Parses the command that was given on stdin (one line)
func parseRunCommand(commandSlice []string) []string {

    botCount := 1
    svns := config.SVNs
    var svnList []string

    if len(commandSlice) > 1 {
        if v, err := strconv.Atoi(commandSlice[1]); err == nil {
            botCount = v
        }
    }

    if len(commandSlice) > 0 {
        switch commandSlice[0] {
            case "all":
                svnList = svns
            default:
                if isValidSVN(commandSlice[0], svns) {
                    svnList = []string{commandSlice[0]}
                } else {
                    // I completely ignore, that something different then "[svn1, svn2, ...]" can get here! No error handling, careful!
                    f := func(c rune) bool {
                        return c == ',' || c == '[' || c == ']' || c == ' '
                    }
                    svnList = strings.FieldsFunc(strings.Trim(commandSlice[0], "[]"), f)
                }
        }

        var finalSvnList []string
        for _, svn := range svnList {
            cleanSvn := strings.Trim(svn, " ")
            if isValidSVN(cleanSvn, svns) {
                for i:=0; i < botCount; i++ {
                    finalSvnList = append(finalSvnList, cleanSvn)
                }
            }
        }
        return finalSvnList
    }

    return []string{}
}

func startBots(botsToStart, hosts []string, serverIP string) int {

    auth := userPassAuth()

    // One bot per host!
    if len(botsToStart) > len(hosts) {
        fmt.Printf("Not enough hosts (%v) to start all bots (%v)\n", len(hosts), len(botsToStart))
        return 0
    }

    executeOnHost := 0
    results := make(chan string, len(hosts))
    timeout := time.After(500 * time.Millisecond)

    // Execute Run-Middleware command parallel on all hosts
    for _, bot := range botsToStart {
        hostname := hosts[executeOnHost]
        go func(hostname, botName string) {
            // Issue command as nohup, to be sure, it continues executing after ssh disconnect.
            command := "nohup $(cd pwb_" + botName + "; ./Programmierwettbewerb-Middleware -connection ws://" + serverIP + "/middleware/) &"

            results <- executeCmd(command, hostname, auth)
        }(hostname, bot)
        executeOnHost += 1
    }

    // Don't really wait for anything to finish. If something comes back, it is
    // very likely a network error of sorts. The timeout is expected behaviour!
    for i := 0; i < len(hosts); i++ {
        select {
        case res := <-results:
            fmt.Print(res + "\n")
        case <-timeout:
            fmt.Println("started.")
        }
    }

    return len(botsToStart)
}

func killBots(hosts []string) {
    auth := userPassAuth()

    results := make(chan string, len(hosts))
    timeout := time.After(500 * time.Millisecond)

    // I don't care, if I started a middleware here or not.
    // If not, killing the non-existing process is not so bad.
    for _, host := range hosts {
        go func(hostname string) {
            // Just kill the corresponding process ASAP.
            command := "kill -KILL $(pidof Programmierwettbewerb-Middleware)"

            results <- executeCmd(command, hostname, auth)
        }(host)
    }

    // Don't really wait for anything to finish. If something comes back, it is
    // very likely a network error of sorts. The timeout is expected behaviour!
    for i := 0; i < len(hosts); i++ {
        select {
        case res := <-results:
            fmt.Print(res + "\n")
        case <-timeout:
            fmt.Println("killed.")
        }
    }
}

// Has to be executed before runs/kills!
// Returns Error. So has to be nil.
func InitRemoteDistribution () error {
    var err error = nil
    config, err = readConfig("../distribution.conf")

    if err != nil {
        fmt.Printf("Error on reading the config file. %v\n", err)
    }

    return err
}

// starts all given bots (if possible) on remote machines, specified in the
// config file.
// Returns Number of bots started.
func RemoteStartBots(botsToStart []string, serverIP string) int {

    var svns []string

    for _,svn := range botsToStart {
        if svn == "all" {
            svns = append(svns, config.SVNs...)
        } else {
            svns = append(svns, svn)
        }
    }
    fmt.Printf("trying to start the following svns: %v\n", svns)

    count := startBots(svns, config.Hosts, serverIP)
    fmt.Printf("%v bots started.\n", count)
    return count
}

// kill all possibly started bots on all machines!
func RemoteKillBots() {
    killBots(config.Hosts)
}

func getIP() string {
    addrs, err := net.InterfaceAddrs()
    if err != nil {
        os.Stderr.WriteString("Oops: " + err.Error() + "\n")
        return "localhost"
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

func interactiveRemoteInterface() {
    reader := bufio.NewReader(os.Stdin)

    for {

        status := Nothing
        var botsToStart []string
        fmt.Print("Enter command shortcut: ")
        text, _ := reader.ReadString('\n')

        result := strings.Split(strings.ToLower(strings.Trim(text, " \t\n")), " ")

        switch result[0] {
        case "kill":
            status = Kill
        case "go":
            botsToStart = parseRunCommand(result[1:])
            status = Execute
        case "exit":
            return
        case "h", "help", "":
            status = Help
        default:
            status = Help
        }

        switch status {
            case Execute:
                RemoteStartBots(botsToStart, getIP())
            case Kill:
                RemoteKillBots()
            case Help:
                printHelp()
            case Nothing:
        }

    }
}

func main() {

    if InitRemoteDistribution() != nil {
        return
    }

    interactiveRemoteInterface()

}
