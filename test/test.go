package main

import (
	"fmt"
	"log"

	"golang.org/x/net/websocket"
)

func main() {
	// cmd := fsck.NewCmd("bac", "", "bash")
	// cmd.Start()
	// time.Sleep(3 * time.Second)
	// go io.Copy(cmd, os.Stdin)
	// io.Copy(os.Stdout, cmd)
	origin := "http://localhost:2322/"
	url := "ws://localhost:2322/ws?xx=1"
	ws, err := websocket.Dial(url, "", origin)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(ws, "abc")
}
