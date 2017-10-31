package main

import (
	"fmt"
	"os"
	"testing"
)

func TestBash(t *testing.T) {
	cmd := NewCmd("n1", "bash")
	cback := make(chan []byte)
	cmd.EnableCallback([]byte("-sctrl: "), cback)
	cmd.SetOut(os.Stdout)
	err := cmd.Start()
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Fprintf(cmd, "echo kkkss && echo '-sctrl: %v'\n", "t00")
	msg := <-cback
	if string(msg) == "t00'" {
		msg = <-cback
	}
	if string(msg) != "t00" {
		t.Error(string(msg))
		return
	}
}
