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
	W       io.Writer
	wait    chan error
	running bool
}

func NewWaitWriter(w io.Writer) *WaitWriter {
	return &WaitWriter{
		W:       w,
		wait:    make(chan error),
		running: true,
	}
}

func (w *WaitWriter) Write(p []byte) (n int, err error) {
	if w.W == nil {
		n = len(p)
		return
	}
	n, err = w.W.Write(p)
	if err != nil && w.running {
		w.wait <- err
		w.W = nil
	}
	return
}

func (w *WaitWriter) Wait() error {
	return <-w.wait
}

func (w *WaitWriter) Close() error {
	w.running = false
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

func (w *WebLogWriter) Add(writer io.Writer, pre bool) {
	w.lck.Lock()
	w.MultiWriter.Add(writer)
	buf := w.Buf.Bytes()
	if len(buf) > 0 && pre {
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
	Buffered int
	allreq   map[string]*WaitWriter
	LsName   func() []string
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
func (w *WebLogger) ListLogH(resp http.ResponseWriter, req *http.Request) {
	ns := []string{}
	if w.LsName != nil {
		ns = w.LsName()
	}
	if len(ns) < 1 {
		w.wslck.RLock()
		for n := range w.allws {
			ns = append(ns, n)
		}
		w.wslck.RUnlock()
	} else {
		ns = append(ns, "debug", "sctrl", "all", "allhost")
	}
	WriteColumn(resp, ns...)
}

func (w *WebLogger) WebLogH(resp http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	ns := req.FormValue("name")
	pre := req.FormValue("pre")
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
	w.wslck.Lock()
	w.allreq[fmt.Sprintf("%p", wwriter)] = wwriter
	allws := []*WebLogWriter{}
	for _, name := range strings.Split(ns, ",") {
		ws := w.allws[name]
		if ws == nil {
			ws = NewWebLogWriter(w.Buffered)
		}
		allws = append(allws, ws)
		w.allws[name] = ws
	}
	w.wslck.Unlock()
	for _, ws := range allws {
		ws.Add(wwriter, pre == "1")
	}
	wwriter.Wait()
	for _, ws := range allws {
		ws.Remove(wwriter)
	}
	w.wslck.Lock()
	delete(w.allreq, fmt.Sprintf("%p", wwriter))
	w.wslck.Unlock()
	log.Printf("web log by name(%v) is done", ns)
}

func (w *WebLogger) Write(tag string, p []byte) (n int, err error) {
	n = len(p)
	allws := []*WebLogWriter{}
	ns := []string{tag, "all"}
	if tag != "debug" && tag != "sctrl" {
		ns = append(ns, "allhost")
	}
	w.wslck.Lock()
	for _, n := range ns {
		ws := w.allws[n]
		if ws == nil {
			ws = NewWebLogWriter(w.Buffered)
		}
		w.allws[n] = ws
		allws = append(allws, ws)
	}
	w.wslck.Unlock()
	for _, ws := range allws {
		ws.Write(p)
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

type WebLogPrinter struct {
	Out    io.Writer
	Reader io.ReadCloser
}

func NewWebLogPrinter(out io.Writer) *WebLogPrinter {
	return &WebLogPrinter{Out: out}
}

func (w *WebLogPrinter) Write(p []byte) (n int, err error) {
	n, err = w.Out.Write(p)
	return
}

func (w *WebLogPrinter) Close() (err error) {
	if w.Reader != nil {
		err = w.Reader.Close()
	}
	return
}
