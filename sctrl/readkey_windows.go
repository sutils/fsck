package main

import (
	"os"
)

var exitf = func(code int) {
	os.Exit(code)
}

var readkeyRead = func(n string) (key []byte, err error) {
	c := make(chan int)
	<-c
	return
}

var readkeyClose = func(n string) {

}

var readkeyOpen = func(n string) {
}

var readkeyGetSize = func() (w, h int) {
	return 80, 60
}

var readkeySetSize = func(fd uintptr, w, h int) (err error) {
	return nil
}
