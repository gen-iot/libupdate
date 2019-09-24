package main

import (
	"context"
	"fmt"
	"github.com/gen-iot/libupdate"
	"log"
	"time"
)

var repo1 = &libupdate.SimpleRepo{
	BaseUrl:    "http://192.168.20.49:12136/api/v1/versions",
	CurrentVer: "v1.0.1",
	RepoName:   "test",
}

func main() {
	config := &libupdate.Config{
		Frequency:                  5 * time.Second,
		MainEntryPoint:             engine,
		UpdateAvailable:            updateAvailable,
		UpdateReady:                nil,
		NoGracefulExit:             false,
		UseLink:                    false,
		WorkDir:                    "download",
		DisableAutoBoot:            true,
		DisableParentQuitWithChild: true,
	}
	updater := libupdate.NewUpdater(config, repo1)
	err := updater.Execute(context.Background())
	log.Println("execute updater :", err)
}

func engine() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		<-ticker.C
		fmt.Println(time.Now().Format("2006-01-02 15-04-05.000"))
	}
}

func updateAvailable() (download bool) {
	fmt.Println("updateAvailable: download!")
	return true
}

func updateReady() {
	fmt.Println("updateReady")
}
