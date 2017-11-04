package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
)

type NameWriter interface {
	Write(name string, p []byte) (n int, err error)
}

type NamedWriter struct {
	Name string
	W    NameWriter
}

func NewNamedWriter(name string, w NameWriter) *NamedWriter {
	return &NamedWriter{
		Name: name,
		W:    w,
	}
}

func (n *NamedWriter) Write(p []byte) (writed int, err error) {
	writed, err = n.W.Write(n.Name, p)
	return
}

type WaitWriter struct {
	W    io.Writer
	wait chan error
}

func NewWaitWriter(w io.Writer) *WaitWriter {
	return &WaitWriter{
		W:    w,
		wait: make(chan error),
	}
}

func (w *WaitWriter) Write(p []byte) (n int, err error) {
	if w.W == nil {
		n = len(p)
		return
	}
	n, err = w.W.Write(p)
	if err != nil {
		w.wait <- err
		w.W = nil
	}
	return
}

func (w *WaitWriter) Wait() error {
	return <-w.wait
}

func (w *WaitWriter) Close() error {
	w.wait <- io.EOF
	close(w.wait)
	return nil
}

type WebLogWriter struct {
	*MultiWriter
	Buf *BufferedWriter
	lck sync.RWMutex
}

func NewWebLogWriter(buffered int) *WebLogWriter {
	return &WebLogWriter{
		MultiWriter: NewMultiWriter(),
		Buf:         NewBufferedWriterSize(ioutil.Discard, buffered),
		lck:         sync.RWMutex{},
	}
}

func (w *WebLogWriter) Add(writer io.Writer) {
	w.lck.Lock()
	w.MultiWriter.Add(writer)
	buf := w.Buf.Bytes()
	if len(buf) > 0 {
		writer.Write(buf)
	}
	w.lck.Unlock()
}

func (w *WebLogWriter) Write(p []byte) (n int, err error) {
	w.lck.RLock()
	n = len(p)
	w.MultiWriter.Write(p)
	w.Buf.Write(p)
	w.lck.RUnlock()
	return
}

type WebLogger struct {
	Name     string
	allws    map[string]*WebLogWriter
	wslck    sync.RWMutex
	all      io.WriteCloser
	Buffered int
	allreq   map[string]*WaitWriter
}

func NewWebLogger(name string, buffered int) *WebLogger {
	return &WebLogger{
		Name:     name,
		allws:    map[string]*WebLogWriter{},
		wslck:    sync.RWMutex{},
		Buffered: buffered,
		allreq:   map[string]*WaitWriter{},
	}
}

func (w *WebLogger) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	ns := req.FormValue("name")
	resp.Header().Add("Content-Type", "text/plain;charset=utf8")
	if len(ns) < 1 {
		fmt.Fprintf(resp, "name parameter is required\n")
		return
	}
	log.Printf("add web log by name(%v) from %v", ns, req.RemoteAddr)
	wwriter := NewWaitWriter(NewNoBufferResponseWriter(resp))
	buf := bytes.NewBuffer(nil)
	w.wslck.RLock()
	fmt.Fprintf(buf, "%v->web logger name:\n", w.Name)
	for name := range w.allws {
		fmt.Fprintf(buf, "  %v\n", name)
	}
	fmt.Fprintf(buf, "%v->web logger is started by %v\n", w.Name, ns)
	w.wslck.RUnlock()
	buf.WriteTo(wwriter)
	if ns == "all" {
		w.wslck.Lock()
		w.allreq[fmt.Sprintf("%p", wwriter)] = wwriter
		w.all = wwriter
		w.wslck.Unlock()
		wwriter.Wait()
		w.all = nil
	} else {
		w.wslck.Lock()
		w.allreq[fmt.Sprintf("%p", wwriter)] = wwriter
		used := []*WebLogWriter{}
		for _, name := range strings.Split(ns, ",") {
			ws := w.allws[name]
			if ws == nil {
				ws = NewWebLogWriter(w.Buffered)
			}
			ws.Add(wwriter)
			used = append(used, ws)
			w.allws[name] = ws
		}
		w.wslck.Unlock()
		wwriter.Wait()
		for _, mw := range used {
			mw.Remove(wwriter)
		}
	}
	w.wslck.Lock()
	delete(w.allreq, fmt.Sprintf("%p", wwriter))
	w.wslck.Unlock()
	log.Printf("web log by name(%v) is done", ns)
}

func (w *WebLogger) Write(name string, p []byte) (n int, err error) {
	n = len(p)
	w.wslck.Lock()
	ws := w.allws[name]
	if ws == nil {
		ws = NewWebLogWriter(w.Buffered)
	}
	w.allws[name] = ws
	w.wslck.Unlock()
	ws.Write(p)
	if allw := w.all; allw != nil {
		allw.Write(p)
	}
	return
}

func (w *WebLogger) Close() error {
	w.wslck.Lock()
	for _, w := range w.allreq {
		w.Close()
	}
	w.wslck.Unlock()
	return nil
}
