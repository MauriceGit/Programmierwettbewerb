package main

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

    "golang.org/x/crypto/ssh"
)

func executeCmd(cmd, hostname string, config *ssh.ClientConfig) string {
    conn, _ := ssh.Dial("tcp", hostname+":22", config)
    session, _ := conn.NewSession()
    defer session.Close()

    var stdoutBuf bytes.Buffer
    session.Stdout = &stdoutBuf
    session.Run(cmd)

    return hostname + ": " + stdoutBuf.String()
}

func userPassAuth() *ssh.ClientConfig {
    return &ssh.ClientConfig{
        // User authentification with user/password
        User: "yeti",
        Auth: []ssh.AuthMethod{
            ssh.Password("yeti"),
        },
    }
}

func privateKeyAuth() *ssh.ClientConfig {
    pkey, err := ioutil.ReadFile("/home/maurice/.ssh/id_rsa")
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
        User: os.Getenv("yeti"),
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
    fmt.Println("        [svn]")
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

func parseCommand(commandSlice, svns []string) []string {

    botCount := 1
    var svnList []string

    if len(commandSlice) > 1 {
        if v, err := strconv.Atoi(commandSlice[1]); err == nil {
            botCount = v
        }
    }

    fmt.Println(commandSlice)
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
                    fmt.Println(commandSlice[0])
                    svnList = strings.FieldsFunc(strings.Trim(commandSlice[0], "[]"), f)
                    fmt.Println(svnList)
                }
        }

        var finalSvnList []string
        for _, svn := range svnList {
            cleanSvn := strings.Trim(svn, " ")
            fmt.Println(cleanSvn)
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

func main() {
    cmd := "ls"
    hosts := []string{"192.168.2.187"}
    svns  := []string{"4", "36", "37"}

    config := userPassAuth()
    reader := bufio.NewReader(os.Stdin)

    for {
    repeat:
        var botsToStart []string
        fmt.Print("Enter command shortcut: ")
        text, _ := reader.ReadString('\n')

        result := strings.Split(strings.ToLower(strings.Trim(text, " \t\n")), " ")
        fmt.Println(result, len(result))
        switch result[0] {
        case "go":

            botsToStart = parseCommand(result[1:], svns)

            fmt.Println(botsToStart)

            //fmt.Println("go")
            goto repeat
        case "h", "help", "":
            printHelp()
            goto repeat
        case "exit":
            return
        default:
            printHelp()
            goto repeat
        }

        results := make(chan string, len(hosts))
        timeout := time.After(5 * time.Second)

        for _, hostname := range hosts {
            go func(hostname string) {
                results <- executeCmd(cmd, hostname, config)
            }(hostname)
        }

        for i := 0; i < len(hosts); i++ {
            select {
            case res := <-results:
                fmt.Print(res)
            case <-timeout:
                fmt.Println("Timed out!")
                return
            }
        }
    }
}
