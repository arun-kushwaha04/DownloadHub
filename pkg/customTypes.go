package pkg

import (
	"net/url"
	"time"
)

type DownloadType struct {
	MaxThreadCount uint8
}

func (t *DownloadType) GetMaxThreads() uint8 {
	return t.MaxThreadCount
}

type ResourceInfo struct {
	FileSize   int64
	FileName   string
	Url        *url.URL
	Resumeable bool
}

type DownloadStats struct {
	downloadSpeed         *float64 //bytes per second
	diskWriteSpeed        *float64 //bytes per second
	memoryUsed            *uint64  //bytes
	elapsedTime           *time.Duration
	estimateRemainingTime *time.Duration
	progress              *float32
	consistentProgress    *float32
}

func (downloadStat DownloadStats) GetDownloadSpeed() float64 {
	return *downloadStat.downloadSpeed
}

func NewDownloadStats() *DownloadStats {
	downloadStat := new(DownloadStats)
	var ds, dw float64 = 0, 0
	var m uint64 = 0
	var p, cp float32 = 0, 0
	et := 0 * time.Second
	est := 0 * time.Second

	downloadStat.downloadSpeed = &ds
	downloadStat.diskWriteSpeed = &dw
	downloadStat.memoryUsed = &m
	downloadStat.elapsedTime = &et
	downloadStat.estimateRemainingTime = &est
	downloadStat.progress = &p
	downloadStat.consistentProgress = &cp

	return downloadStat
}

func (downloadStat *DownloadStats) UpdateDownloadStats(
	ds float64,
	dWs float64,
	m uint64,
	t time.Duration,
	eRt time.Duration,
	p float32,
	cp float32,
) {
	*downloadStat.downloadSpeed = ds
	*downloadStat.diskWriteSpeed = dWs
	*downloadStat.memoryUsed = m
	*downloadStat.elapsedTime = t
	*downloadStat.estimateRemainingTime = eRt
	*downloadStat.progress = p
	*downloadStat.consistentProgress = cp
}
