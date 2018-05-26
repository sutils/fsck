package fsck

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/Centny/gwf/log"
	"github.com/Centny/gwf/routing"
	"github.com/Centny/gwf/util"
	"github.com/alecthomas/template"
)

var WEBUI_HTML = `
<html>

<head>
    <meta charset="UTF-8">
    <meta name="renderer" content="webkit|ie-comp|ie-stand">
    <meta http-equiv="X-UA-Compatible" content="IE=Edge,chrome=1" />
    <meta http-equiv="pragma" content="no-cache">
    <meta http-equiv="cache-control" content="no-cache">
    <style type="text/css">
        .boder_1px_t {
            border-collapse: collapse;
            border: 1px solid black;
        }

        .boder_1px {
            border: 1px solid black;
            padding: 8px;
            text-align: center;
        }

        .noneborder {
            border: 0px;
            padding: 0px;
            margin: 0px;
            border-spacing: 0px;
        }

        form textarea {
            resize: none;
            width: 240px;
            height: 50px;
            max-width: 300px;
            max-height: 50px;
            margin: 0px;
            padding: 0px;
        }
    </style>
</head>

<body>
    <form action="/ui/addForward" method="POST">
        <table>
            <td>
                <textarea name="forwards"></textarea>
            </td>
            <td>
                <input id="submit" type="submit" value="Submit" />
            </td>
        </table>
    </form>
    <p class="list">
        <table class="boder_1px_t">
            <tr class="boder_1px">
                <th class="boder_1px">No</th>
                <th class="boder_1px">Name</th>
                <th class="boder_1px">Online</th>
                <th class="boder_1px">Remote</th>
                <th class="boder_1px">Forward</th>
			</tr>
			{{$webSuffix:=.webSuffix}}
            {{range $k, $v := .ns}} {{$channel := index $.forwards $v}}
            <tr class="boder_1px">
                <td class="boder_1px">{{$k}}</td>
                <td class="boder_1px">{{$channel.Name}}</td>
                <td class="boder_1px">{{$channel.Online}}</td>
                <td class="boder_1px">{{$channel.Remote}}</td>
                <td class="boder_1px">
                    <table class="noneborder" style="width:100%">
                        {{range $i, $f := $channel.MS}}
						<tr class="noneborder" style="height:20px">
                            <td class="noneborder">{{printf "%v@%v" $f.Name $f | html}}</td>
                            <td class="noneborder" style="width:60px;text-align:center;">
								<a style="margin-left:10px;" href="/ui/removeForward?local={{$f.Local}}">Remove</a>
							</td>
							<td class="noneborder" style="width:60px;text-align:center;">
								{{if eq $f.Local.Scheme "web" }}
								<a target="_blank" href="//{{$f.Local.Host}}{{$webSuffix}}">Open</a>
								{{else}}
								&nbsp;
								{{end}}
							</td>
                        </tr>
                        {{end}}
                    </table>
                </td>
            </tr>
            {{end}}
        </table>
    </p>
    <table class="boder_1px" style="position:absolute;right:30px;top:5px;">
        {{range $i, $r := $.recents}} {{$f := index $r "forward"}}
        <tr class="noneborder" style="height:20px;text-align:left;">
            <td class="noneborder">{{printf "%v" $f | html}}</td>
            <td class="noneborder">
                <a style="margin-left:10px;font-size:20px;text-decoration:none;font-weight:bold;" href="/ui/addForward?forwards={{$f}}">+</a>
			</td>
			<td class="noneborder">
                <a style="margin-left:10px;font-size:20px;text-decoration:none;font-weight:bold;" href="/ui/removeRecent?forwards={{$f}}">-</a>
            </td>
        </tr>
        {{end}}
    </table>
</body>

</html>
`

type RecentLess string

func (m RecentLess) Less(a, b interface{}, desc bool) bool {
	aval := util.MapVal(a).IntVal("used")
	bval := util.MapVal(b).IntVal("used")
	if aval == bval {
		if desc {
			return util.MapVal(a).StrVal("forward") > util.MapVal(b).StrVal("forward")
		} else {
			return util.MapVal(a).StrVal("forward") < util.MapVal(b).StrVal("forward")
		}
	} else {
		if desc {
			return aval > bval
		} else {
			return aval < bval
		}
	}
}

type ForwardCtrl interface {
	LoadForward() *Forward
	AllForwards() (ns []string, fs map[string]*ChannelInfo, err error)
}

type WebUI struct {
	WS       string
	TEMP     *template.Template
	sequence uint64
	Ctrl     ForwardCtrl
}

func NewWebUI(ctrl ForwardCtrl) (webui *WebUI) {
	webui = &WebUI{
		Ctrl: ctrl,
	}
	usr, err := user.Current()
	if err != nil {
		panic(fmt.Sprintf("get user info fail with %v", err))
	}
	webui.WS = usr.HomeDir
	webui.TEMP, _ = template.New("n").Parse(WEBUI_HTML)
	return
}

func (w *WebUI) ReadRecent() (recent map[string]int) {
	recent = map[string]int{}
	bys, err := ioutil.ReadFile(w.WS + "/recent.json")
	if err != nil && !os.IsNotExist(err) {
		log.W("read recent from %v fail with %v", w.WS+"/recent.json", err)
		return
	}
	err = json.Unmarshal(bys, &recent)
	if err != nil {
		log.W("nmarshal recent json data on %v fail with %v", w.WS+"/recent.json", err)
		return
	}
	return
}

func (w *WebUI) WriteRecent(recent map[string]int) {
	bys, err := json.Marshal(recent)
	if err != nil {
		log.W("marshal recent json fail with %v", err)
		return
	}
	err = ioutil.WriteFile(w.WS+"/recent.json", bys, os.ModePerm)
	if err != nil {
		log.W("save recent to %v fail with %v", w.WS+"/recent.json", err)
		return
	}
}

func (w *WebUI) Hand(mux *routing.SessionMux, redirect bool) {
	pre := "/ui"
	mux.HFunc("^"+pre+".*$", w.AuthFilterH)
	mux.HFunc("^"+pre+"/removeForward(\\?.*)?$", w.RemoveForwardH)
	mux.HFunc("^"+pre+"/addForward(\\?.*)?$", w.AddForwardH)
	mux.HFunc("^"+pre+"/removeRecent(\\?.*)?$", w.RemoveRecentH)
	mux.HFunc("^"+pre+".*$", w.IndexH)
	if redirect {
		mux.HFunc("^/(\\?.*)?$", func(hs *routing.HTTPSession) routing.HResult {
			hs.Redirect(pre)
			return routing.HRES_RETURN
		})
	}
}

func (w *WebUI) AuthFilterH(hs *routing.HTTPSession) routing.HResult {
	forward := w.Ctrl.LoadForward()
	auth := forward.WebAuth
	username, password, ok := hs.R.BasicAuth()
	if len(auth) > 0 && !(ok && auth == fmt.Sprintf("%v:%s", username, password)) {
		hs.W.Header().Set("WWW-Authenticate", "Basic realm=Reverse Server")
		hs.W.WriteHeader(401)
		hs.Printf("%v", "401 Unauthorized")
		return routing.HRES_RETURN
	}
	return routing.HRES_CONTINUE
}

func (w *WebUI) RemoveForwardH(hs *routing.HTTPSession) routing.HResult {
	var local string
	var err = hs.ValidF(`
		local,R|S,L:0;
		`, &local)
	if err != nil {
		return hs.Printf("%v", err)
	}
	w.Ctrl.LoadForward().RemoveForward(local)
	hs.Redirect("/ui")
	log.D("WebUI remove forward by %v", local)
	return routing.HRES_RETURN
}

func (w *WebUI) AddForwardH(hs *routing.HTTPSession) routing.HResult {
	name := fmt.Sprintf("n-%v", atomic.AddUint64(&w.sequence, 1))
	oldRecent := w.ReadRecent()
	for _, f := range strings.Split(hs.RVal("forwards"), "\n") {
		f = strings.TrimSpace(f)
		if len(f) < 1 {
			continue
		}
		_, err := w.Ctrl.LoadForward().AddUriForward(name, f)
		if err != nil {
			return hs.Printf("%v", err)
		}
		oldRecent[f]++
		log.D("WebUI add forward by %v,%v", name, f)
	}
	w.WriteRecent(oldRecent)
	hs.Redirect("/ui")
	return routing.HRES_RETURN
}

func (w *WebUI) RemoveRecentH(hs *routing.HTTPSession) routing.HResult {
	oldRecent := w.ReadRecent()
	forwards := hs.RVal("forwards")
	delete(oldRecent, forwards)
	w.WriteRecent(oldRecent)
	hs.Redirect("/ui")
	log.D("WebUI remove recent by %v", forwards)
	return routing.HRES_RETURN
}

func (w *WebUI) IndexH(hs *routing.HTTPSession) routing.HResult {
	ns, forwards, err := w.Ctrl.AllForwards()
	if err != nil {
		return hs.Printf("%v", err)
	}
	oldRecent := w.ReadRecent()
	recents := []util.Map{}
	for f, c := range oldRecent {
		oldForward, err := NewMapping("old", f)
		if err != nil {
			continue
		}
		using := false
		if channel, ok := forwards[oldForward.Channel]; ok {
			for _, runningForward := range channel.MS {
				if oldForward.String() == runningForward.String() {
					using = true
					break
				}
			}
		}
		if using {
			continue
		}
		recents = append(recents, util.Map{
			"forward": f,
			"used":    c,
		})
	}
	forward := w.Ctrl.LoadForward()
	sorter := util.NewSorter(RecentLess(""), recents)
	sorter.Desc = true
	sort.Sort(sorter)
	vals := map[string]interface{}{
		"ns":        ns,
		"forwards":  forwards,
		"recents":   recents,
		"webSuffix": forward.WebSuffix,
	}
	if hs.RVal("data") == "1" {
		hs.JRes(vals)
	} else {
		err = w.TEMP.Execute(hs.W, vals)
	}
	if err != nil {
		log.E("Parse html fail with %v", err)
	}
	return routing.HRES_RETURN
}
