package main;

import (
    "os"
    //"github.com/elastic/beats/libbeat/beat"
    //"github.com/xgzeng/ipcbeat/beater"
    "github.com/xgzeng/ipcbeat/cmd"
)

func main() {
    err := cmd.RootCmd.Execute()
    if err != nil {
        os.Exit(1)
    }

    //b, err := beater.New()
    //if err != nil {
    //    os.Exit(1)
    //}

    //b.Run(nil)
}
