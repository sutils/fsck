package fsck

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
)

var Logger = log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile)

func logDebug(format string, args ...interface{}) {
	Logger.Output(2, fmt.Sprintf("D "+format, args...))
}

func logInfo(format string, args ...interface{}) {
	Logger.Output(2, fmt.Sprintf("I "+format, args...))
}
func logWarn(format string, args ...interface{}) {
	Logger.Output(2, fmt.Sprintf("W "+format, args...))
}
func logError(format string, args ...interface{}) {
	Logger.Output(2, fmt.Sprintf("E "+format, args...))
}

type Channel struct {
	net.Conn
	lck sync.RWMutex
}

func NewChannel(conn net.Conn) *Channel {
	return &Channel{
		Conn: conn,
		lck:  sync.RWMutex{},
	}
}

func (c *Channel) WriteAll(bys ...[]byte) (n int, err error) {
	c.lck.Lock()
	defer c.lck.Unlock()
	var writed int
	for _, b := range bys {
		writed, err = c.Write(b)
		if err != nil {
			break
		}
		n += writed
	}
	return
}

func (c *Channel) ReadW(p []byte) error {
	buflen := len(p)
	all := 0
	buf := p
	for {
		readed, err := c.Read(buf)
		if err != nil {
			return err
		}
		all += readed
		if all < buflen {
			buf = p[all:]
			continue
		} else {
			break
		}
	}
	return nil
}

func (c *Channel) WriteCodeMessage(sid uint16, code byte, message []byte) (n int, err error) {
	header := make([]byte, 5)
	binary.BigEndian.PutUint16(header, uint16(len(message)+5))
	binary.BigEndian.PutUint16(header[2:], sid)
	header[4] = code
	return c.WriteAll(header, message)
}

func (c *Channel) ReadFrame() (sid uint16, code byte, data []byte, err error) {
	header := make([]byte, 5)
	err = c.ReadW(header)
	if err != nil {
		return
	}
	frameLengh := binary.BigEndian.Uint16(header)
	sid = binary.BigEndian.Uint16(header[2:])
	code = header[4]
	data = make([]byte, frameLengh-5)
	err = c.ReadW(data)
	return
}
