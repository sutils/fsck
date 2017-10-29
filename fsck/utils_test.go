package main

import (
	"fmt"
	"testing"
)

func TestOutWriter(t *testing.T) {
	out := NewOutWriter()
	cback := make(chan []byte)
	out.EnableCallback([]byte("-sctrl: "), cback)
	fmt.Println("-->", []byte("-sctrl: "), '\r', '\n')
	go func() {
		fmt.Fprintf(out, `
			-sctrl: done1
	
-sctrl: done2
-sctrl: done3
			`)
		fmt.Fprintf(out, "-sctrl: done4\n")
		fmt.Fprintf(out, "-")
		fmt.Fprintf(out, "sctrl: done5\n")
		fmt.Fprintf(out, "-sc")
		fmt.Fprintf(out, "trl: don")
		fmt.Fprintf(out, "e6\n")
	}()
	cmd := <-cback
	fmt.Println("done1--->")
	if string(cmd) != "done1" {
		t.Error(cmd)
		return
	}
	cmd = <-cback
	fmt.Println("done2--->")
	if string(cmd) != "done2" {
		t.Error(cmd)
		return
	}
	cmd = <-cback
	fmt.Println("done3--->")
	if string(cmd) != "done3" {
		t.Error(cmd)
		return
	}
	cmd = <-cback
	fmt.Println("done4--->")
	if string(cmd) != "done4" {
		t.Error(cmd)
		return
	}
	cmd = <-cback
	fmt.Println("done5--->")
	if string(cmd) != "done5" {
		t.Error(cmd)
		return
	}
	cmd = <-cback
	fmt.Println("done6--->")
	if string(cmd) != "done6" {
		t.Error(cmd)
		return
	}
}
