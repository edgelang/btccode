package main

import (
	"github.com/jinzhu/configor"
)

var config = struct {
	Port         int    `default:"8080"`
	RPCPort      int    `default:"8332"`
	RPCUser      string `default:"user"`
	RPCPassword  string `default:"pass"`
	Callback     string
	GuiJiaccount string
	Confirmation int `default:"3"`
	AllowIP      []string
}{}

func init() {
	configor.Load(&config, "config.toml")
}
