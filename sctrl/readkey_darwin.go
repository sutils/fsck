package main

import (
	"os"

	"github.com/sutils/readkey"
)

var exitf = func(code int) {
	readkey.Close()
	os.Exit(code)
}

var readkeyRead = func(n string) (key []byte, err error) {
	key, err = readkey.Read()
	return
}

var readkeyClose = func(n string) {
	readkey.Close()
}

var readkeyOpen = func(n string) {
	readkey.Open()
}

var readkeyGetSize = func() (w, h int) {
	return readkey.GetSize()
}

var readkeySetSize = func(fd uintptr, w, h int) (err error) {
	return readkey.SetSize(fd, w, h)
}
