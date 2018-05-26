package main

import (
	"os"

	"github.com/sutils/readkey"
)

func exitf(code int) {
	readkey.Close()
	os.Exit(code)
}

func readkeyRead(n string) (key []byte, err error) {
	key, err = readkey.Read()
	return
}

func readkeyClose(n string) {
	readkey.Close()
}

func readkeyOpen(n string) {
	readkey.Open()
}
func readkeyGetSize() (w, h int) {
	return readkey.GetSize()
}
func readkeySetSize(fd uintptr, w, h int) (err error) {
	return readkey.SetSize(fd, w, h)
}
