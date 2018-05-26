package main

import (
	"io"
	"os"
	"time"

	"github.com/sutils/fsck"
)

func main() {
	cmd := fsck.NewCmd("bac", "", "bash")
	cmd.Start()
	time.Sleep(3 * time.Second)
	go io.Copy(cmd, os.Stdin)
	io.Copy(os.Stdout, cmd)
}
