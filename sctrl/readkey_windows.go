package main

import (
	"os"
)

func exitf(code int) {
	os.Exit(code)
}

func readkeyRead(n string) (key []byte, err error) {
	c := make(chan int)
	<-c
	return
}

func readkeyClose(n string) {

}

func readkeyOpen(n string) {
}
func readkeyGetSize() (w, h int) {
	return 80, 60
}
func readkeySetSize(fd uintptr, w, h int) (err error) {
	return nil
}
