package main

import (
	"fmt"
	"testing"
)

func TestParseSshHost(t *testing.T) {
	host, err := ParseSshHost("abc", "root@loc.m")
	if err != nil || host.Channel != "master" || host.Username != "root" || host.URI != "loc.m:22" {
		fmt.Println(err)
		t.Error(host)
		return
	}
	host, err = ParseSshHost("abc", "root:sco@loc.m")
	if err != nil || host.Channel != "master" || host.Username != "root" || host.URI != "loc.m:22" || host.Password != "sco" {
		fmt.Println(err)
		t.Error(host)
		return
	}
	host, err = ParseSshHost("abc", "mx://root:sco@loc.m")
	if err != nil || host.Channel != "mx" || host.Username != "root" || host.URI != "loc.m:22" || host.Password != "sco" {
		fmt.Println(err)
		t.Error(host)
		return
	}
	host, err = ParseSshHost("abc", "mx://root:sco@loc.m?pty=vt100")
	if err != nil || host.Channel != "mx" || host.Username != "root" ||
		host.URI != "loc.m:22" || host.Password != "sco" ||
		host.Pty != "vt100" {
		fmt.Println(err)
		t.Error(host)
		return
	}
}
