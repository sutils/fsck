package main

import (
	"bytes"
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

func TestMultiWriter(t *testing.T) {
	mw := NewMultiWriter()
	buf := bytes.NewBuffer(nil)
	mw.Add(buf)
	fmt.Fprintf(mw, "abc")
	if !bytes.Equal(buf.Bytes(), []byte("abc")) {
		t.Error("error")
		return
	}
	mw.Remove(buf)
	fmt.Fprintf(mw, "abc")
	if !bytes.Equal(buf.Bytes(), []byte("abc")) {
		t.Error("error")
		return
	}
}

func TestJoinArgs(t *testing.T) {
	if JoinArgs("a") != "a" {
		t.Error("erro")
		return
	}
	if JoinArgs("a", "b") != "a b" {
		t.Error("erro")
		return
	}
	if JoinArgs("", "b") != "b" {
		t.Error("erro")
		return
	}
	if JoinArgs("", "b", "xx ss") != "b \"xx ss\"" {
		t.Error("erro")
		return
	}
}
