package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/sutils/fsck"
)

func JoinArgs(cmd string, args ...string) string {
	nargs := []string{}
	for _, arg := range append([]string{cmd}, args...) {
		if strings.Contains(arg, " ") {
			if strings.Contains(arg, "'") {
				nargs = append(nargs, "'"+arg+"'")
			} else {
				nargs = append(nargs, "\""+arg+"\"")
			}
		} else {
			nargs = append(nargs, arg)
		}
	}
	return strings.Join(nargs, " ")
}

func copyBuffer(dst io.Writer, src io.Reader, buf []byte) (written int64, err error) {
	// If the reader has a WriteTo method, use it to do the copy.
	// Avoids an allocation and a copy.
	if wt, ok := src.(io.WriterTo); ok {
		return wt.WriteTo(dst)
	}
	// Similarly, if the writer has a ReadFrom method, use it to do the copy.
	if rt, ok := dst.(io.ReaderFrom); ok {
		return rt.ReadFrom(src)
	}
	if buf == nil {
		buf = make([]byte, 32*1024)
	}
	for {
		nr, er := src.Read(buf)
		fmt.Println("-->", nr)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "-s":
		fsck.ShowLog = 1
		tokens := map[string]int{}
		err := json.Unmarshal([]byte(os.Args[3]), &tokens)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		server, err := fsck.NewServer(os.Args[2], tokens)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		server.Run()
	case "-c":
		debug, err := os.OpenFile("/tmp/fsck.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModePerm)
		if err != nil {
			panic(err)
		}
		// log.SetOutput(ioutil.Discard)
		webcmd, _ := filepath.Abs(os.Args[0])
		client := fsck.NewClient("localhost:9234", "abc")
		terminal := NewTerminal(client, os.Args[2], "bash", webcmd)
		log.SetOutput(io.MultiWriter(debug, NewNamedWriter("debug", terminal.Log)))
		terminal.Proc()
	case "-lc":
		url, err := findWebURL()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		os.Exit(ExecWebLog(url+"/log", strings.Join(os.Args[2:], ","), os.Stdout))
	default:
		url, err := findWebURL()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		cmds := JoinArgs(strings.TrimPrefix(os.Args[1], "-"), os.Args[2:]...)
		os.Exit(ExecWebCmd(url+"/exec", cmds, os.Stdout))
	}
}

func findWebURL() (string, error) {
	url := os.Getenv(KeyWebCmdURL)
	if len(url) < 1 {
		conf := map[string]interface{}{}
		data, err := ioutil.ReadFile("/tmp/fsck_conf.json")
		if err != nil {
			err = fmt.Errorf("find the fsck web url fail")
			return url, err
		}
		err = json.Unmarshal(data, &conf)
		if err != nil {
			err = fmt.Errorf("read fsck config file(%v) fail with %v", "/tmp/fsck_conf.json", err)
			return url, err
		}
		rurl, ok := conf["web_url"].(string)
		if !ok {
			err = fmt.Errorf("read fsck config file(%v) fail with %v", "/tmp/fsck_conf.json", "web_url not configured")
			return url, err
		}
		url = rurl
	}
	return url, nil
}

func printUsage() {
	fmt.Printf(`Usage: %v <mode> <arguemtns>
	%v -s <listen address> <token by json format>
	%v -c 
`, os.Args[0], os.Args[0], os.Args[0])
}
