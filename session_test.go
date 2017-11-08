package fsck

import (
	"encoding/binary"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/Centny/gwf/util"
)

func TestEchoSession(t *testing.T) {
	sp1 := NewSessionPool()
	sp2 := NewSessionPool()
	ew1 := &echowriter{W: sp1}
	ew2 := &echowriter{W: sp2}
	//
	sp1.Dail(100, "echo", ew2)
	ss2 := sp2.Start(100, ew1)
	ss2.MaxDelay = 100 * time.Millisecond
	ss2.Timeout = 500 * time.Millisecond
	//
	pings := NewEchoPing(ss2)
	for i := 0; i < 10; i++ {
		used, slaverCall, slaverBack, err := pings.Ping("abc")
		if err != nil {
			t.Error(err)
			return
		}
		if slaverCall != 100 || used < 1 || slaverBack < 1 {
			t.Error("data error")
			return
		}
		fmt.Println("-->", used, slaverCall, slaverBack)
	}
	{
		fmt.Println("testing error...")
		//
		ew1.Idx = 1
		_, _, _, err := pings.Ping("abc")
		if err == nil {
			t.Error(nil)
			return
		}
		fmt.Println(err)
		//
		ew1.Idx = 2
		_, _, _, err = pings.Ping("abc")
		if err == nil {
			t.Error(nil)
			return
		}
		fmt.Println(err)
		//
		ew1.Idx = 3
		_, _, _, err = pings.Ping("abc")
		if err == nil {
			t.Error(nil)
			return
		}
		fmt.Println(err)
		//
		ew1.Idx = 0
		ew2.Idx = 4
		_, _, _, err = pings.Ping("abc")
		if err == nil {
			t.Error(nil)
			return
		}
		fmt.Println(err)
		//
		echorw := NewEchoReadWriteCloser()
		echorw.Close()
		_, err = echorw.Read(make([]byte, 100))
		if err != io.EOF {
			t.Error(err)
		}
		_, err = echorw.Write(make([]byte, 100))
		if err != io.EOF {
			t.Error(err)
		}
		fmt.Println((&ErrOK{}).Error())
	}
	//
	sp1.Close()
	sp2.Close()
	time.Sleep(time.Second)

}

type echowriter struct {
	W   io.Writer
	Idx int
}

func (e *echowriter) Write(p []byte) (n int, err error) {
	switch e.Idx {
	case 1:
		err = nil
	case 2:
		err = &ErrOK{Data: "error"}
	case 3:
		err = fmt.Errorf("error")
	case 4:
		n, err = e.W.Write(p)
		if err != nil {
			return
		}
		err = &ErrOK{Data: util.S2Json(util.Map{
			"used": 100,
		})}
	default:
		time.Sleep(10 * time.Millisecond)
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(util.Now()))
		n, err = e.W.Write(append(p, buf...))
		if err != nil {
			return
		}
		err = &ErrOK{Data: util.S2Json(util.Map{
			"used": 100,
		})}
	}
	return
}
