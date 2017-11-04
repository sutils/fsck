package main

import (
	"fmt"
	"net"
	"os"
	"testing"
)

func TestParseSshHost(t *testing.T) {
	host, err := ParseSshHost("abc", "root@loc.m", nil)
	if err != nil || host.Channel != "master" || host.Username != "root" || host.URI != "loc.m:22" {
		fmt.Println(err)
		t.Error(host)
		return
	}
	host, err = ParseSshHost("abc", "root:sco@loc.m", nil)
	if err != nil || host.Channel != "master" || host.Username != "root" || host.URI != "loc.m:22" || host.Password != "sco" {
		fmt.Println(err)
		t.Error(host)
		return
	}
	host, err = ParseSshHost("abc", "mx://root:sco@loc.m", nil)
	if err != nil || host.Channel != "mx" || host.Username != "root" || host.URI != "loc.m:22" || host.Password != "sco" {
		fmt.Println(err)
		t.Error(host)
		return
	}
	host, err = ParseSshHost("abc", "mx://root:sco@loc.m?pty=vt100", nil)
	if err != nil || host.Channel != "mx" || host.Username != "root" ||
		host.URI != "loc.m:22" || host.Password != "sco" ||
		host.Pty != "vt100" {
		fmt.Println(err)
		t.Error(host)
		return
	}
}

func TestScp(t *testing.T) {
	info, err := os.Stat("sctrl-ssh.sh")
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Printf("%o\n", info.Mode())
	host, err := ParseSshHost("abc", "mx://root:sco@loc.m?pty=vt100", nil)
	if err != nil {
		t.Error(err)
		return
	}
	sesson := NewSshSession(nil, host)
	sesson.Resize = false
	conn, err := net.Dial("tcp", "loc.m:22")
	if err != nil {
		t.Error(err)
		return
	}
	err = sesson.StartSession(conn)
	if err != nil {
		t.Error(err)
		return
	}
	err = sesson.UploadFile("ssh.go", "/tmp/", os.Stdout)
	if err != nil {
		t.Error(err)
		return
	}
}
