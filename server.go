package fsck

import (
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/Centny/gwf/util"
)

const (
	//ConNormal is the normal message.
	ConNormal = 0
	//ConHostError is the error message.
	ConHostError = 10
	//ConClosed is the connection closed status.
	ConClosed = 20
	//ConHandshake is the connection handshake
	ConHandshake = 30
	//ConChannelAuth is the channel aut command.
	ConChannelAuth = 40
)

//Server is the tcp listener
type Server struct {
	net.Listener
	// URI     string
	channel map[uint16]*Channel
	hosts   map[uint16]net.Conn
	clck    sync.RWMutex
	sidc    uint16
	ACL     map[string]int
}

//NewServer create the listener.
func NewServer(addr string, acl map[string]int) (server *Server, err error) {
	raw, err := net.Listen("tcp", addr)
	if err == nil {
		server = &Server{
			Listener: raw,
			channel:  map[uint16]*Channel{},
			hosts:    map[uint16]net.Conn{},
			clck:     sync.RWMutex{},
			ACL:      acl,
		}
	}
	return
}

//Run will loop accept the connected.
func (s *Server) Run() {
	logInfo("Server start listne on %v", s.Addr())
	for {
		con, err := s.Accept()
		if err != nil {
			logError("Listener accept fail with %v", err)
			break
		}
		logInfo("Server accept from %v", con.RemoteAddr())
		go s.readChannel(NewChannel(con))
	}
}

func (s *Server) readChannel(conn *Channel) {
	_, code, buf, err := conn.ReadFrame()
	if err != nil {
		conn.Close()
		logWarn("Server read channel fail with %v", err)
		return
	}
	if code != ConChannelAuth {
		conn.Close()
		logWarn("Server read first frame is not auth command(%v),%v", code, string(buf))
		return
	}
	if s.ACL[string(buf)] < 1 {
		conn.Close()
		logWarn("Server verify channel by token(%v) fail", string(buf))
		return
	}
	logInfo("Server verify channel by token(%v) success", string(buf))
	for {
		sid, code, buf, err := conn.ReadFrame()
		if err != nil {
			conn.Close()
			logWarn("Server read channel fail with %v", err)
			break
		}
		logDebug("Server read %v channel data by sid(%v),code(%v)", len(buf), sid, code)
		//
		var host net.Conn
		if code == ConHandshake {
			var args = util.Map{}
			err = json.Unmarshal(buf, &args)
			if err != nil {
				logError("Server recieve handshake frame fail with %v", err)
				continue
			}
			uri := args.StrVal("uri")
			host, err = net.Dial("tcp", uri)
			if err != nil { //server is connected error
				conn.WriteCodeMessage(sid, ConHostError, []byte(err.Error()))
				logWarn("Server dail to %v on sid(%v) fail with %v", uri, sid, err)
				continue
			}
			s.clck.Lock()
			s.sidc++
			sid = s.sidc
			s.hosts[sid] = host
			s.channel[sid] = conn
			go s.readHost(sid, host)
			s.clck.Unlock()
			conn.WriteCodeMessage(sid, ConHandshake, buf)
			logInfo("Server dail to %v on sid(%v) success", uri, sid)
			continue
		}
		s.clck.Lock()
		host = s.hosts[sid]
		if code != ConNormal { //client is closed
			if host != nil {
				host.Close()
			}
			s.clck.Unlock()
			logInfo("Server receive sid(%v) is closed", sid)
			continue
		}
		s.clck.Unlock()
		if host == nil {
			conn.WriteCodeMessage(sid, ConHostError, []byte("closed"))
			logWarn("Server write host on sid(%v) fail with host not found", sid)
			continue
		}
		_, err = host.Write(buf)
		if err != nil { //server is closed
			conn.WriteCodeMessage(sid, ConHostError, []byte(err.Error()))
			logWarn("Server write host(%v) on sid(%v) fail with %v", host.RemoteAddr(), sid, err)
			host.Close()
			continue
		}
		logDebug("Server proxy request %v data on sid(%v) success", len(buf), sid)
	}
}

func (s *Server) readHost(sid uint16, conn net.Conn) {
	buf := make([]byte, 40960)
	var channel *Channel
	for {
		readed, err := conn.Read(buf)
		if err != nil {
			break
		}
		logDebug("Server read %v data on sid(%v) from %v", readed, sid, conn.RemoteAddr())
		var waited int64
		for {
			s.clck.RLock()
			channel = s.channel[sid]
			s.clck.RUnlock()
			if channel != nil {
				//if write error will wait new channel.
				_, err = channel.WriteCodeMessage(sid, ConNormal, buf[:readed])
				if err == nil { //one frame completed
					logDebug("Server proxy response %v data on sid(%v) success", readed, sid)
					break
				}
				logWarn("Server write data to channel on sid(%v) fail with %v", sid, err)
			}
			time.Sleep(100 * time.Millisecond)
			waited += 100
			if waited > 60000 {
				logWarn("Server wait channel on sid(%v) fail with timeout", sid)
				break
			}
		}
		if waited > 60000 {
			break
		}
	}
	conn.Close()
	s.clck.Lock()
	channel = s.channel[sid]
	delete(s.hosts, sid)
	delete(s.channel, sid)
	s.clck.Unlock()
	if channel != nil { //server is closed
		channel.WriteCodeMessage(sid, ConHostError, []byte("read host done"))
		logInfo("Server notify sid(%v) is closed", sid)
	}
}
