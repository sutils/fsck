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
    <form action="addForward" method="POST">
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
                            <td class="noneborder">{{printf "%v" $f | html}}</td>
                            <td class="noneborder" style="width:60px;text-align:center;">
								<a style="margin-left:10px;" href="removeForward?local={{$f.Local}}">Remove</a>
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
                <a style="margin-left:10px;font-size:20px;text-decoration:none;font-weight:bold;" href="addForward?forwards={{$f}}">+</a>
			</td>
			<td class="noneborder">
                <a style="margin-left:10px;font-size:20px;text-decoration:none;font-weight:bold;" href="removeRecent?forwards={{$f}}">-</a>
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

type WebUI struct {
	WS           string
	WebSuffix    string
	Auth         string
	Forward      *Forward
	TEMP         *template.Template
	sequence     uint64
	AllForwardsF func() (ns []string, fs map[string]*ChannelInfo, err error)
}

func NewWebUI(forward *Forward, all func() (ns []string, fs map[string]*ChannelInfo, err error)) (webui *WebUI) {
	webui = &WebUI{
		Forward:      forward,
		AllForwardsF: all,
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

func (w *WebUI) Hand(mux *routing.SessionMux, pre string) {
	mux.HFunc("^"+pre+".*$", w.AllFilterH)
	mux.HFunc("^"+pre+"/removeForward(\\?.*)?$", w.RemoveForwardH)
	mux.HFunc("^"+pre+"/addForward(\\?.*)?$", w.AddForwardH)
	mux.HFunc("^"+pre+"/removeRecent(\\?.*)?$", w.AddForwardH)
	mux.HFunc("^"+pre+".*$", w.IndexH)
}

func (w *WebUI) AllFilterH(hs *routing.HTTPSession) routing.HResult {
	host := hs.R.Host
	if len(w.WebSuffix) > 0 && strings.HasSuffix(host, w.WebSuffix) {
		name := strings.Trim(strings.TrimSuffix(host, w.WebSuffix), ". ")
		if len(name) > 0 {
			return w.Forward.ProcWebForward(hs)
		}
	}
	username, password, ok := hs.R.BasicAuth()
	if len(w.Auth) > 0 && !(ok && w.Auth == fmt.Sprintf("%v:%s", username, password)) {
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
	w.Forward.RemoveForward(local)
	hs.Redirect("/")
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
		_, err := w.Forward.AddUriForward(name, f)
		if err != nil {
			return hs.Printf("%v", err)
		}
		oldRecent[f]++
	}
	w.WriteRecent(oldRecent)
	hs.Redirect("/")
	return routing.HRES_RETURN
}

func (w *WebUI) RemoveRecentH(hs *routing.HTTPSession) routing.HResult {
	oldRecent := w.ReadRecent()
	delete(oldRecent, hs.RVal("forwards"))
	w.WriteRecent(oldRecent)
	hs.Redirect("/")
	return routing.HRES_RETURN
}

func (w *WebUI) IndexH(hs *routing.HTTPSession) routing.HResult {
	ns, forwards, err := w.AllForwardsF()
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
		if channel, ok := forwards[oldForward.Name]; ok {
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
	sorter := util.NewSorter(RecentLess(""), recents)
	sorter.Desc = true
	sort.Sort(sorter)
	err = w.TEMP.Execute(hs.W, map[string]interface{}{
		"ns":        ns,
		"forwards":  forwards,
		"recents":   recents,
		"webSuffix": w.WebSuffix,
	})
	if err != nil {
		log.E("Parse html fail with %v", err)
	}
	return routing.HRES_RETURN
}
