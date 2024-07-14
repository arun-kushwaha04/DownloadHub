package pkg

import (
	"net/url"
	"time"
)

type DownloadType struct {
	maxThreadCount uint8
}

func (t *DownloadType) GetMaxThreads() uint8 {
	return t.maxThreadCount
}

type DownloadInfo struct {
	FileSize   int64
	FileName   string
	Url        *url.URL
	Resumeable bool
}

type DownloadStats struct {
	DownloadSpeed         *float64 //bytes per second
	DiskWriteSpeed        *float64 //bytes per second
	MemoryUsed            *uint64  //bytes
	ElapsedTime           *time.Duration
	EstimateRemainingTime *time.Duration
	Progress              *float32
}
