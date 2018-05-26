package fsck

import (
	"bytes"
	"container/list"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
)

type OutWriter struct {
	Out      io.Writer
	buf      []byte
	lck      sync.RWMutex
	Buffered bool
	cmd      int
	message  []byte
	prefix   []byte
	prelen   int
	callback chan []byte
}

func NewOutWriter() *OutWriter {
	return &OutWriter{
		lck: sync.RWMutex{},
	}
}

func (b *OutWriter) Write(p []byte) (n int, err error) {
	b.lck.Lock()
	defer b.lck.Unlock()
	// log.Printf("-->%v", string(p))
	if b.prelen > 0 {
		for idx := range p {
			// log.Print(b.cmd, p[idx])
			if b.cmd < b.prelen {
				if b.prefix[b.cmd] == p[idx] {
					b.cmd++
				} else {
					b.cmd = 0
					b.message = nil
				}
				continue
			}
			if p[idx] == '\n' {
				mlen := len(b.message)
				if b.message[mlen-1] == '\r' {
					b.message = b.message[:mlen-1]
				}
				b.callback <- b.message
				b.message = nil
				b.cmd = 0
			} else {
				b.message = append(b.message, p[idx])
			}
		}
	}
	if b.Out != nil {
		n, err = b.Out.Write(p)
	} else {
		n = len(p)
	}
	return
}

func (b *OutWriter) EnableCallback(prefix []byte, back chan []byte) {
	b.prefix = prefix
	b.prelen = len(prefix)
	b.callback = back
}

func (b *OutWriter) DisableCallback() {
	b.prefix = nil
	b.prelen = 0
	b.callback = nil
}

func Having(all []string, val string) bool {
	for _, v := range all {
		if v == val {
			return true
		}
	}
	return false
}

type MultiWriter struct {
	allws   *list.List
	Disable bool
}

func NewMultiWriter() *MultiWriter {
	return &MultiWriter{
		allws: list.New(),
	}
}

func (m *MultiWriter) Write(p []byte) (n int, err error) {
	if m.Disable {
		n = len(p)
		return
	}
	for em := m.allws.Front(); em != nil; em = em.Next() {
		em.Value.(io.Writer).Write(p)
	}
	n = len(p)
	return
}

func (m *MultiWriter) Add(w io.Writer) {
	m.allws.PushBack(w)
}

//
func (m *MultiWriter) Remove(w io.Writer) {
	for em := m.allws.Front(); em != nil; em = em.Next() {
		if em.Value == w {
			m.allws.Remove(em)
		}
	}
}

func JoinArgs(cmd string, args ...string) string {
	nargs := []string{}
	realArgs := []string{}
	if len(cmd) > 0 {
		realArgs = append([]string{cmd}, args...)
	} else {
		realArgs = args
	}
	for _, arg := range realArgs {
		if strings.Contains(arg, " ") {
			nargs = append(nargs, "\""+arg+"\"")
		} else {
			nargs = append(nargs, arg)
		}
	}
	return strings.Join(nargs, " ")
}

func MarshalAll(v interface{}) string {
	bys, _ := json.Marshal(v)
	return string(bys)
}

func ColumnBytes(prefix string, max []int, vals ...string) (buf *bytes.Buffer) {
	col := len(max)
	for idx, n := range vals {
		idx = idx % col
		if max[idx] < len(n) {
			max[idx] = len(n)
		}
	}
	buf = bytes.NewBuffer(nil)
	fmt.Fprintf(buf, prefix)
	for idx, n := range vals {
		fmt.Fprintf(buf, fmt.Sprintf("%%-%vs  ", max[idx%col]), n)
		if idx > 0 && idx%col == col-1 {
			fmt.Fprintf(buf, "\n%v", prefix)
		}
	}
	return
}

func WriteColumn(w io.Writer, vals ...string) (n int64, err error) {
	buf := ColumnBytes(" ", make([]int, 5), vals...)
	n, err = buf.WriteTo(w)
	return
}
