package main

import (
	"context"
	"fmt"
	"github.com/gen-iot/libupdate"
	"github.com/gen-iot/std"
	"os"
	"time"
)

var repo1 = &libupdate.SimpleRepo{
	BaseUrl:    "http://192.168.20.49:12136/api/v1/versions",
	CurrentVer: "v1.0.1",
	RepoName:   "test",
}

func main() {
	go runUpdater()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		<-ticker.C
		fmt.Println(time.Now().Format("2006-01-02 15-04-05.000"))
	}
}

func runUpdater() {
	config := &libupdate.Config{
		Frequency:       5 * time.Second,
		UpdateAvailable: updateAvailable,
		UpdateReady:     updateReady,
		DownloadDir:     "download",
	}
	updater := libupdate.NewUpdater(config, repo1)
	updater.Execute(context.Background())
}

func updateAvailable() (download bool) {
	fmt.Println("updateAvailable: do download!")
	return true
}

func updateReady(latestExeAddr string) {
	//fmt.Println("updateReady latestVerAddr:", latestExeAddr)
	//input, err := os.Open(latestExeAddr)
	//std.AssertError(err, "open latest exe failed")
	//defer std.CloseIgnoreErr(input)
	//dst, err := os.OpenFile("sample1.update", os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0755)
	//std.AssertError(err, "create sample1.update failed")
	//_, err = io.Copy(dst, input)
	//std.AssertError(err, "copy file err")
	err := os.Symlink(latestExeAddr, "sample1.update")
	std.AssertError(err, "cant create sample1.update symbol link")
	os.Exit(0)
}
