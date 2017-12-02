package cmd

import (
	"github.com/elastic/beats/libbeat/cmd"
	"github.com/xgzeng/ipcbeat/beater"
)

// RootCmd to handle beats cli
var RootCmd = cmd.GenRootCmd("ipcbeat", "", beater.New)
