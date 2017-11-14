package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strings"
)

type WebHeader interface {
	Header() map[string]string
}

type NoBufferResponseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func NewNoBufferResponseWriter(w http.ResponseWriter) *NoBufferResponseWriter {
	writer := &NoBufferResponseWriter{
		w: w,
	}
	if flusher, ok := w.(http.Flusher); ok {
		writer.flusher = flusher
	}
	return writer
}

func (n *NoBufferResponseWriter) Write(p []byte) (writed int, err error) {
	writed, err = n.w.Write(p)
	if n.flusher != nil {
		n.flusher.Flush()
	}
	return
}

type OnWebCmd func(w *Web, cmds string) (data interface{}, err error)

type Web struct {
	H OnWebCmd
}

func NewWeb(h OnWebCmd) *Web {
	return &Web{
		H: h,
	}
}

func (w *Web) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		fmt.Fprintf(resp, `
		<form method="POST" action="." enctype="text/plain">
			<textarea name="cmds"></textarea>
			<input type="submit " value="exec ">
		</form>
			`)
	case "POST":
		cmds, err := ioutil.ReadAll(req.Body)
		if err != nil {
			resp.WriteHeader(400)
			fmt.Fprintf(resp, "read body fail with %v\n", err)
			return
		}
		realCmds := strings.TrimPrefix(strings.TrimSpace(string(cmds)), "cmds=")
		data, err := w.H(w, realCmds)
		if err != nil {
			resp.WriteHeader(500)
			fmt.Fprintf(resp, "%v\n", err)
			return
		}
		header, ok := data.(WebHeader)
		if ok && header != nil {
			fields := header.Header()
			for key, val := range fields {
				resp.Header().Set(key, val)
			}
		}
		switch data.(type) {
		case io.Reader:
			// resp.Header().Add("Content-Type", "application/octet-stream")
			writer := NewNoBufferResponseWriter(resp)
			io.Copy(writer, data.(io.Reader))
		case []byte:
			resp.Write(data.([]byte))
		default:
			fmt.Fprintf(resp, "%v", data)
		}
	default:
		resp.WriteHeader(400)
	}
}

var ctrlcSig chan os.Signal

func ExecWebCmd(url string, cmds string, keys map[string]string, out io.Writer) (code int, err error) {
	if keys == nil {
		keys = map[string]string{}
	}
	var sterm = func(tid string) {
		resp, err := http.Post(url, "text/plain", bytes.NewBufferString("cmds=sterm "+tid))
		if err != nil {
			fmt.Fprintf(out, "-send kill to task(%v) fail with %v", tid, err)
			return
		}
		_, err = io.Copy(out, resp.Body)
		if err != nil && err != io.EOF {
			fmt.Fprintf(out, "-kill task(%v) reply error %v", tid, err)
			return
		}
	}
	resp, err := http.Post(url, "text/plain", bytes.NewBufferString("cmds="+cmds))
	if err != nil {
		return
	}
	code = resp.StatusCode
	callback := make(chan []byte, 3)
	outw := NewOutWriter()
	outw.Out = out
	outw.EnableCallback([]byte(WebCmdPrefix), callback)
	tid := resp.Header.Get("tid")
	//
	var running = true
	if len(tid) > 0 {
		ctrlcSig = make(chan os.Signal)
		signal.Notify(ctrlcSig, os.Interrupt)
		defer signal.Stop(ctrlcSig)
		go func() {
			for running {
				<-ctrlcSig
				fmt.Println()
				sterm(tid)
			}
		}()
	}
	//
	_, err = io.Copy(outw, resp.Body)
	if err == io.EOF {
		err = nil
	}
	if err == nil {
		callback <- nil
		reply := <-callback
		if reply != nil && string(reply) != "ok" {
			err = fmt.Errorf("%v", string(reply))
		}
	}
	running = false
	return
}

func ExecWebLog(url string, ns string, pre string, out *WebLogPrinter) (code int, err error) {
	resp, err := http.Get(url + "?name=" + ns + "&pre=" + pre)
	if err != nil {
		code = -1
		return
	}
	defer resp.Body.Close()
	out.Reader = resp.Body
	_, err = io.Copy(out, out.Reader)
	if err != nil {
		code = resp.StatusCode
	}
	return
}
