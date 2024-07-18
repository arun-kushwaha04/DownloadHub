package main

import (
	"fmt"
	"os"

	"github.com/arun-kushwaha04/DownloadHub/pkg"
	"github.com/arun-kushwaha04/DownloadHub/service"
)

func main() {
	const url = "https://in.mirror.coganng.com/ubuntu-cdimage/24.04/ubuntu-24.04-desktop-amd64.iso"

	highPtr := &pkg.DownloadType{MaxThreadCount: 10}

	downloader, err := service.CreateDownloader(url, highPtr)
	if err != nil {
		fmt.Println("Unable to create download struct", err)
		os.Exit(-1)
	}
	downloader.StartDownload()
	fmt.Println("Download completed")
}
