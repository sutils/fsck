package fsck

import (
	"bytes"
	"fmt"
	"strings"
	"sync"

	"github.com/Centny/gwf/routing"
	"github.com/Centny/gwf/util"
)

type RealLog struct {
	Last int64
	Log  util.Map
}

type RealTime struct {
	ls  map[string]*RealLog
	lck sync.RWMutex
}

func NewRealTime() *RealTime {
	return &RealTime{
		ls:  map[string]*RealLog{},
		lck: sync.RWMutex{},
	}
}

func (r *RealTime) UpdateH(hs *routing.HTTPSession) routing.HResult {
	data := map[string]util.Map{}
	err := hs.UnmarshalJ(&data)
	if err != nil {
		return hs.MsgResErr2(1, "arg-err", err)
	}
	r.lck.Lock()
	defer r.lck.Unlock()
	for name, log := range data {
		if len(strings.TrimSpace(name)) < 1 {
			err = fmt.Errorf("the top name is empty")
			return hs.MsgResErr2(2, "arg-err", err)
		}
		rl := r.ls[name]
		if rl == nil {
			rl = &RealLog{}
			r.ls[name] = rl
		}
		rl.Last = util.Now()
		rl.Log = log
	}
	return hs.MsgRes("ok")
}

func (r *RealTime) ListH(hs *routing.HTTPSession) routing.HResult {
	r.lck.RLock()
	defer r.lck.RUnlock()
	return hs.JRes(r.ls)
}

func (r *RealTime) ShowH(hs *routing.HTTPSession) routing.HResult {
	hs.R.ParseForm()
	keys := map[string]string{}
	for key := range hs.R.Form {
		if key == "name" {
			continue
		}
		keys[key] = hs.R.FormValue(key)
	}
	ns := map[string]int64{}
	nsstr := hs.R.FormValue("name")
	if len(nsstr) > 0 {
		for _, n := range strings.Split(nsstr, ",") {
			ns[n] = 2000
		}
	}
	hosts, alllog := r.MergeLog(ns, keys)
	return hs.JRes(util.Map{
		"code":  0,
		"hosts": hosts,
		"logs":  alllog,
	})
}

func (r *RealTime) MergeLog(ns map[string]int64, keys map[string]string) (hosts, alllog util.Map) {
	r.lck.Lock()
	now := util.Now()
	hosts = util.Map{}
	alllog = util.Map{}
	hostc := 0
	for name, log := range r.ls {
		if timeout, ok := ns["*"]; ok {
			if timeout > 0 && now-log.Last > timeout {
				hosts[name] = "offline"
				continue
			}
		} else if len(ns) > 0 {
			timeout, ok := ns[name]
			if !ok {
				continue
			}
			if timeout > 0 && now-log.Last > timeout {
				hosts[name] = "offline"
				continue
			}
		}
		for key, val := range keys {
			switch val {
			case "min":
				if alllog.Exist(key) {
					old := alllog.FloatVal(key)
					new := log.Log.FloatVal(key)
					if new < old {
						alllog.SetVal(key, new)
					}
				} else {
					alllog.SetVal(key, log.Log.FloatVal(key))
				}
			case "max":
				if alllog.Exist(key) {
					old := alllog.FloatVal(key)
					new := log.Log.FloatVal(key)
					if new > old {
						alllog.SetVal(key, new)
					}
				} else {
					alllog.SetVal(key, log.Log.FloatVal(key))
				}
			default:
				alllog.SetVal(key, alllog.FloatVal(key)+log.Log.FloatVal(key))
			}
		}
		hosts[name] = "ok"
		hostc++
	}
	r.lck.Unlock()
	if hostc > 0 {
		for key, val := range keys {
			if val == "avg" {
				fmt.Println(key, alllog.FloatVal(key), float64(hostc))
				alllog.SetVal(key, float64(int64(alllog.FloatVal(key)/float64(hostc)*100))/100)
			}
		}
	}
	return
}

func (r *RealTime) Clear() {
	r.lck.Lock()
	r.ls = map[string]*RealLog{}
	r.lck.Unlock()
}

func NotifyReal(url string, data util.Map) (res util.Map, err error) {
	_, res, err = util.HPostN2(url, "application/json;charset=utf8", bytes.NewBufferString(util.S2Json(data)))
	return
}
