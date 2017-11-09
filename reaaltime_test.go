package fsck

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/Centny/gwf/util"

	"github.com/Centny/gwf/routing/httptest"
)

func TestRealTime(t *testing.T) {
	rl := NewRealTime()
	ts := httptest.NewMuxServer()
	ts.Mux.HFunc("/update", rl.UpdateH)
	ts.Mux.HFunc("/show", rl.ShowH)
	for i := 0; i < 15; i++ {
		name := fmt.Sprintf("x%v", i)
		NotifyReal(ts.URL+"/update", util.Map{
			name: util.Map{
				"a": 1,
				"b": 1,
			},
		})
	}
	time.Sleep(2 * time.Second)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("x%v", i)
		NotifyReal(ts.URL+"/update", util.Map{
			name: util.Map{
				"a": 1,
				"b": 1,
			},
		})
	}
	res, err := ts.G2("/show?name=x0,x1,x2,x3,x4,x5,x6,x7,x8,x9&a=avg&b=sum")
	if err != nil {
		t.Error(err)
		return
	}
	hosts := res.MapVal("hosts")
	if len(hosts) != 15 {
		fmt.Println("-->", hosts)
		t.Error("error")
		return
	}
	logs := res.MapVal("logs")
	if logs.IntVal("a") != 1 || logs.IntVal("b") != 10 {
		fmt.Println("-->", res)
		t.Error("data error")
		return
	}
	//
	ts.PostN2("/update", "application/json", bytes.NewBufferString("xxxx"))
}
