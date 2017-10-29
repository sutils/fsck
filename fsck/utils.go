package main

import (
	"container/list"
	"io"
	"sync"
)

type OutWriter struct {
	out      io.Writer
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
	if b.out != nil {
		n, err = b.out.Write(p)
	} else {
		if b.Buffered {
			b.buf = append(b.buf, p...)
		}
		n = len(p)
	}
	return
}

func (b *OutWriter) SetOut(out io.Writer) {
	b.lck.Lock()
	defer b.lck.Unlock()
	b.out = out
	if len(b.buf) > 0 {
		b.out.Write(b.buf)
		b.buf = nil
	}
}

func (b *OutWriter) GetOut() (out io.Writer) {
	out = b.out
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
	allws *list.List
}

func NewMultiWriter() *MultiWriter {
	return &MultiWriter{
		allws: list.New(),
	}
}

func (m *MultiWriter) Write(p []byte) (n int, err error) {
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
