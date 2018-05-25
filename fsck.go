package fsck

import (
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"time"
)

type DuplexPiped struct {
	UpReader   *os.File
	UpWriter   *os.File
	DownReader *os.File
	DownWriter *os.File
	closed     uint32
}

func (d *DuplexPiped) Close() error {
	if !atomic.CompareAndSwapUint32(&d.closed, 0, 1) {
		return fmt.Errorf("closed")
	}
	d.UpWriter.Close()
	d.DownWriter.Close()
	return nil
}

type PipedConn struct {
	piped *DuplexPiped
	up    bool
}

func CreatePipedConn() (a, b *PipedConn, err error) {
	piped := &DuplexPiped{}
	piped.UpReader, piped.DownWriter, err = os.Pipe()
	if err == nil {
		piped.DownReader, piped.UpWriter, err = os.Pipe()
	}
	if err == nil {
		a = &PipedConn{
			piped: piped,
			up:    true,
		}
		b = &PipedConn{
			piped: piped,
			up:    false,
		}
	}
	return
}

func (p *PipedConn) Read(b []byte) (n int, err error) {
	if p.up {
		n, err = p.piped.UpReader.Read(b)
	} else {
		n, err = p.piped.DownReader.Read(b)
	}
	return
}

func (p *PipedConn) Write(b []byte) (n int, err error) {
	if p.up {
		n, err = p.piped.UpWriter.Write(b)
	} else {
		n, err = p.piped.DownWriter.Write(b)
	}
	return
}

func (p *PipedConn) Close() error {
	return p.piped.Close()
}

func (p *PipedConn) LocalAddr() net.Addr {
	return p
}
func (p *PipedConn) RemoteAddr() net.Addr {
	return p
}
func (p *PipedConn) SetDeadline(t time.Time) error {
	return nil
}
func (p *PipedConn) SetReadDeadline(t time.Time) error {
	return nil
}
func (p *PipedConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (p *PipedConn) Network() string {
	return "piped"
}
func (p *PipedConn) String() string {
	return "piped"
}

type WriterF func(p []byte) (n int, err error)

func (w WriterF) Write(p []byte) (n int, err error) {
	n, err = w(p)
	return
}
