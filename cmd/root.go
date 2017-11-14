package cmd

import (
	// register default heartbeat monitors
	// _ "github.com/elastic/beats/heartbeat/monitors/defaults"

		"github.com/xgzeng/ipcbeat/beater"
		"github.com/elastic/beats/libbeat/cmd"
)

// RootCmd to handle beats cli
var RootCmd = cmd.GenRootCmd("ipcbeat", "", beater.New)