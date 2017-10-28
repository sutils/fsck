package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/pkg/term"
	"github.com/sutils/fsck"
)

func main() {
	_, err := term.Open("/dev/ttyxxx")
	if err != nil {
		panic(err)
	}
	if len(os.Args) < 4 {
		printUsage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "-s":
		fsck.ShowLog = 1
		tokens := map[string]int{}
		err := json.Unmarshal([]byte(os.Args[3]), &tokens)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		server, err := fsck.NewServer(os.Args[2], tokens)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		server.Run()
	case "-cli":
		client := fsck.NewClient("localhost:9234", "abc")
		terminal := NewTerminal(client, os.Args[2])
		fmt.Println(terminal.Proc())
	case "-cli2":
		// fsck.ShowLog = 1
		client := fsck.NewClient("localhost:9234", "abc")
		host, err := ParseSshHost("loc.m", "root:sco@loc.m:22")
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		session := NewSshSession(client, host)
		session.SetOut(os.Stdout)
		session.Start()
		io.Copy(session, os.Stdin)
	case "-cli3":
		// fsck.ShowLog = 1
		client := fsck.NewClient("localhost:9234", "abc")
		host, err := ParseSshHost("loc.m", "root:sco@loc.m:22")
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		session := NewSshSession(client, host)
		var srvCon fsck.Conn
		var cliCon net.Conn
		if false {
			wg := sync.WaitGroup{}
			wg.Add(2)
			go func() {
				l, _ := net.Listen("tcp", ":2442")
				raw, _ := l.Accept()
				srvCon = &fsck.NetConn{Conn: raw}
				wg.Done()
			}()
			time.Sleep(time.Second)
			go func() {
				cliCon, err = net.Dial("tcp", ":2442")
				wg.Done()
			}()
			wg.Wait()

		}
		if true {
			srvCon, cliCon = SshPipe(host.URI)
		}
		// fsckCon, netCon := SshPipe(host.URI)
		// go io.Copy(fsckCon, con)
		// go io.Copy(con, fsckCon)
		go client.Proc("loc.m:22", srvCon)
		session.StartSession(cliCon)
		// session.SetOut(os.Stdout)
		// session.Start()
		io.Copy(session, os.Stdin)
	default:
		// client, err := fsck.NewForward(fsck.NewClient(os.Args[2], os.Args[3]))
		// if err != nil {
		// 	fmt.Println(err)
		// 	os.Exit(1)
		// }
		// client.Run()
	}
}

func printUsage() {
	fmt.Printf(`Usage: %v <mode> <arguemtns>
	%v -s <listen address> <token by json format>
	%v -c 
`, os.Args[0], os.Args[0], os.Args[0])
}
