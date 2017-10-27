package fsck

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/Centny/gwf/util"
)

type Forward struct {
	*Client
}

func NewForward(client *Client) (forward *Forward) {
	return &Forward{Client: client}
}

//Run will loop accept the connected.
func (f *Forward) Run(addr, host string) (err error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return
	}
	var con net.Conn
	logInfo("Client start listne on %v", addr)
	for {
		con, err = listener.Accept()
		if err != nil {
			logError("Client accept fail with %v", err)
			break
		}
		logInfo("Client accept from %v", con.RemoteAddr())
		go f.Client.Proc(host, &NetConn{Conn: con})
	}
	return
}

type NetConn struct {
	net.Conn
	Sid uint16
}

func (n *NetConn) String() string {
	return n.RemoteAddr().String()
}

func (n *NetConn) SetSid(sid uint16) {
	n.Sid = sid
}

func (n *NetConn) GetSid() uint16 {
	return n.Sid
}

type Conn interface {
	io.ReadWriteCloser
	String() string
	SetSid(sid uint16)
	GetSid() uint16
}

//Client is the socket client.
type Client struct {
	//net.Listener
	URI      string
	Token    string
	cclients map[uint16]Conn
	sclients map[uint16]Conn
	channel  *Channel
	clck     sync.RWMutex
	cidc     uint16
}

//NewClient create the client.
func NewClient(uri, token string) (client *Client) {
	client = &Client{
		URI:      uri,
		Token:    token,
		cclients: map[uint16]Conn{},
		sclients: map[uint16]Conn{},
		clck:     sync.RWMutex{},
	}
	return
}

func (c *Client) getChannel() (channel *Channel, err error) {
	c.clck.Lock()
	channel = c.channel
	if channel == nil {
		var raw net.Conn
		raw, err = net.Dial("tcp", c.URI)
		if err == nil {
			channel = NewChannel(raw)
			channel.WriteCodeMessage(0, ConChannelAuth, []byte(c.Token)) //to login
			logInfo("Client connect to %v success and login by token(%v)", c.URI, c.Token)
			go c.readChannel(channel)
			c.channel = channel
		} else {
			logWarn("Client connect to %v fail with %v", c.URI, err)
		}
	}
	c.clck.Unlock()
	return
}

func (c *Client) Proc(host string, conn Conn) {
	header := make([]byte, 5)
	//
	channel, err := c.getChannel()
	if err != nil {
		conn.Close()
		return
	}
	//to handshake host.
	c.clck.Lock()
	c.cidc++
	cid := c.cidc
	c.cclients[cid] = conn
	c.clck.Unlock()
	channel.WriteCodeMessage(0, ConHandshake, []byte(fmt.Sprintf(`{"uri":"%v","cid":%v}`, host, cid)))
	//wait sid.
	for conn.GetSid() < 1 {
		logDebug("Client waiting sid by cid(%v),host(%v)", cid, host)
		time.Sleep(100 * time.Millisecond)
	}
	//
	binary.BigEndian.PutUint16(header[2:], conn.GetSid())
	header[4] = ConNormal
	buf := make([]byte, 40960)
	var readed int
	for {
		readed, err = conn.Read(buf)
		if err != nil {
			break
		}
		logDebug("Client read %v data from %v", readed, conn)
		binary.BigEndian.PutUint16(header, uint16(readed+5))
		var waited int64
		for {
			channel, err = c.getChannel()
			if channel != nil { //channel is well.
				_, err = channel.WriteAll(header, buf[:readed])
				if err == nil { //one frame done.
					logDebug("Client request %v data on sid(%v) success", readed, conn.GetSid())
					break
				}
				logWarn("Client send data to %v fail with %v", c.URI, err)
				c.channel = nil
			}
			time.Sleep(100 * time.Millisecond)
			waited += 100
			if waited > 60000 {
				logWarn("Client wait channel %v fail with time out", c.URI)
				break
			}
		}
		if waited > 60000 {
			break
		}
	}
	c.clck.Lock()
	delete(c.cclients, cid)
	delete(c.sclients, conn.GetSid())
	c.clck.Unlock()
	conn.Close()
	if c.channel != nil { //client is closed
		c.channel.WriteCodeMessage(conn.GetSid(), ConClosed, []byte("closed"))
	}
}

func (c *Client) readChannel(conn *Channel) {
	for {
		sid, code, buf, err := conn.ReadFrame()
		if err != nil {
			conn.Close()
			break
		}
		logDebug("Client read %v channel data on sid(%v),cod(%v)", len(buf), sid, code)
		if code == ConHandshake {
			var args = util.Map{}
			err = json.Unmarshal(buf, &args)
			if err != nil {
				conn.WriteCodeMessage(sid, ConClosed, []byte("closed"))
				logError("Client recieve handshake frame fail with %v", err)
				continue
			}
			cid := uint16(args.IntValV("cid", 0))
			c.clck.Lock()
			client := c.cclients[cid]
			if client == nil {
				conn.WriteCodeMessage(sid, ConClosed, []byte("closed"))
				logError("Client recieve handshake frame fail with cid(%v) not found", cid)
			} else {
				c.sclients[sid] = client
				client.SetSid(sid)
			}
			c.clck.Unlock()
			continue
		}
		c.clck.Lock()
		client := c.sclients[sid]
		c.clck.Unlock()
		if client == nil { //client is closed
			conn.WriteCodeMessage(sid, ConClosed, []byte("closed"))
			logWarn("Client found sid(%v) is closed", sid)
			break
		}
		if code != ConNormal { //server is closed
			client.Close()
			logWarn("Client receive sid(%v) is closed", sid)
			break
		}
		_, err = client.Write(buf)
		if err != nil { // client is closed
			conn.WriteCodeMessage(sid, ConClosed, []byte(err.Error()))
			logWarn("Client write to %v on sid(%v) fail with err", client, sid, err)
			client.Close()
			break
		}
		logDebug("Client reponse %v data on sid(%v) success", len(buf), sid)
	}
}

func (c *Client) Close() (err error) {
	c.clck.Lock()
	for sid, client := range c.sclients {
		if c.channel != nil {
			c.channel.WriteCodeMessage(sid, ConClosed, []byte("closed"))
		}
		client.Close()
	}
	for _, client := range c.cclients {
		client.Close()
	}
	if c.channel != nil {
		c.channel.Close()
	}
	c.clck.Unlock()
	return
}
