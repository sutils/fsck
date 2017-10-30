package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

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

func ExecWebCmd(url string, cmds string, out io.Writer) (code int) {
	resp, err := http.Post(url, "text/plain", bytes.NewBufferString("cmds="+cmds))
	if err != nil {
		fmt.Fprintf(out, "-error: %v\n", err)
		code = 100
		return
	}
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		code = 200
		fmt.Fprintf(out, "-error: %v\n", err)
		return
	}
	return
}

func ExecWebLog(url string, ns string, out io.Writer) (code int, err error) {
	resp, err := http.Get(url + "?name=" + ns)
	if err != nil {
		code = -1
		return
	}
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		code = resp.StatusCode
	}
	return
}
