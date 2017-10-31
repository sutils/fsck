package main

import (
	"encoding/json"
	"io/ioutil"
)

type Host struct {
	Name    string `json:"name"`
	URI     string `json:"uri"`
	Startup int    `json:"startup"`
}

type WorkConf struct {
	Name     string  `json:"name"`
	SrvAddr  string  `json:"server"`
	Login    string  `json:"login"`
	Bash     string  `json:"bash"`
	PS1      string  `json:"ps1"`
	Instance string  `json:"instance"`
	Hosts    []*Host `json:"hosts"`
}

func ReadWorkConf(path string) (conf *WorkConf, err error) {
	bys, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}
	conf = &WorkConf{}
	err = json.Unmarshal(bys, conf)
	return
}
