package main

import (
    "bytes"
    "golang.org/x/crypto/ssh"
    "fmt"
    "log"
    "io/ioutil"
    "os"
    "time"
    "bufio"
    "strings"
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

func main() {
    cmd := "ls"
    hosts := [...]string{"192.168.2.187"}

    config := userPassAuth()
    reader := bufio.NewReader(os.Stdin)

    for {
        repeat:
        fmt.Print("Enter command shortcut: ")
        text, _ := reader.ReadString('\n')

        switch strings.ToLower(strings.Trim(text, " \t\n")) {
            case "h", "help", "":
                fmt.Println("You can basically only type help, HA!")
                goto repeat
            case "exit":
                return
            default:
                fmt.Println("Default")
                cmd = "ls -l"
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
