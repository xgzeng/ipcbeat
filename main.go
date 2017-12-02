package main;

import (
    "os"
    "github.com/xgzeng/ipcbeat/cmd"
    "github.com/xgzeng/ipcbeat/outputs/websocket"
)

func main() {
    websocket.Init()
    err := cmd.RootCmd.Execute()
    if err != nil {
        os.Exit(1)
    }
}
