package service

import (
	"io"
	"net/http"
	"os"
	"path"
	"sync"
	"time"

	"github.com/arun-kushwaha04/DownloadHub/configs"
	"github.com/arun-kushwaha04/DownloadHub/pkg"
	"github.com/arun-kushwaha04/DownloadHub/utils"
	"github.com/google/uuid"
)

type downloaderThread struct {
	threadId          uuid.UUID
	startByte         int64
	endByte           int64
	startTime         time.Time
	runTime           time.Duration
	fullPath          string
	resourceInfo      *pkg.ResourceInfo
	bytesDownloaded   *int64
	bytesWritten      *int64
	downloadWriteLock *sync.RWMutex
	diskWriteLock     *sync.RWMutex
	waitGroup         *sync.WaitGroup
	messageChan       chan string
	errorChan         chan error
}

type Downloader struct {
	downloaderId      uuid.UUID
	resourceInfo      *pkg.ResourceInfo
	downloadStats     *pkg.DownloadStats
	downloadPrt       pkg.DownloadSpeed
	threads           *[]*downloaderThread
	fullPath          string
	startTime         time.Time
	runTime           time.Duration
	bytesDownloaded   *int64
	bytesWritten      *int64
	downloadWriteLock *sync.RWMutex
	diskWriteLock     *sync.RWMutex
}

func (thread *downloaderThread) StartThread() error {
	defer thread.waitGroup.Done()
	thread.startTime = time.Now()

	file, err := os.OpenFile(thread.fullPath, os.O_RDWR, 0644)
	defer file.Close()
	if err != nil {
		return utils.FileReadPermissionError
	}

	client, req, err := utils.GetClient("GET", thread.resourceInfo.Url, nil, utils.GetByteRangeHeader(&thread.startByte, &thread.endByte))

	if err != nil {
		return err
	}

	res, err := client.Do(req)
	if err != nil {
		return utils.HttpClientIntalizationError
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusPartialContent && res.StatusCode != http.StatusOK {
		return utils.ServerError
	}

	fileBuffer := make([]byte, configs.BUFF_SIZE)
	fileBufferIdx := 0

	for {
		// read res body in buffer[idx:len(buff)]
		n, err := res.Body.Read(fileBuffer[fileBufferIdx:])

		if err == io.EOF {
			// end of response body writing remaining bytes to files
			if fileBufferIdx > 0 {
				// write to file from buff[0:idx-1]
				wt, err := file.Write(fileBuffer[:fileBufferIdx])
				if err != nil {
					return utils.FileWritePermissionError
				}
				fileBufferIdx = 0

				thread.diskWriteLock.Lock()
				*thread.bytesWritten += int64(wt)
				thread.diskWriteLock.Unlock()
			}
			break
		}

		if err != nil {
			return utils.UnexpectedServerResponse
		}

		if n > 0 {
			fileBufferIdx += n
			thread.downloadWriteLock.Lock()
			*thread.bytesDownloaded += int64(n)
			thread.downloadWriteLock.Unlock()
		}

		//deciding whether to write to file
		if fileBufferIdx == configs.FILE_BUFF_SIZE {
			// write to file from buff[0:idx-1]
			wt, err := file.Write(fileBuffer[:fileBufferIdx])
			if err != nil {
				return utils.FileWritePermissionError
			}
			fileBufferIdx = 0
			thread.diskWriteLock.Lock()
			*thread.bytesWritten += int64(wt)
			thread.diskWriteLock.Unlock()
		}

	}
	thread.runTime = time.Since(thread.startTime)
	return nil
}

func (downloader *Downloader) StartDownload() error {

	return nil
}

func CreateDownloader(resourceUrl string, downloadPrt pkg.DownloadSpeed) (*Downloader, error) {
	resourceInfo, err := utils.GetMetaData(&resourceUrl)
	if err != nil {
		return nil, err
	}

	parentDir := utils.GetDownloadFolder(path.Ext((*resourceInfo.Url).Path))

	fullPath, err := utils.CreateFile(parentDir, (*resourceInfo).FileName, (*resourceInfo).FileSize)

	if err != nil {
		return nil, err
	}

	threads := make([]*downloaderThread, 0, downloadPrt.GetMaxThreads())

	var downloadWriteLock, diskWriteLock sync.RWMutex
	var wg sync.WaitGroup

	// channels
	messageChan := make(chan string)
	errorChan := make(chan error)

	bytesPerThread := (*resourceInfo).FileSize / int64(downloadPrt.GetMaxThreads())
	var bytesDownloaded, bytesWritten *int64

	// creating threads
	var i uint8
	for i = 0; i < downloadPrt.GetMaxThreads(); i++ {
		threadId := uuid.New()
		fullPath, err := utils.CreateFile(configs.TEMP_DIRECTORY, threadId.String(), 0)
		if err != nil {
			return nil, err
		}
		thread := &downloaderThread{
			threadId:          threadId,
			startByte:         int64(i) * bytesPerThread,
			endByte:           int64(i+1)*bytesPerThread - 1,
			resourceInfo:      resourceInfo,
			startTime:         time.Now(),
			runTime:           time.Since(time.Now()),
			fullPath:          fullPath,
			bytesDownloaded:   bytesDownloaded,
			bytesWritten:      bytesWritten,
			downloadWriteLock: &downloadWriteLock,
			diskWriteLock:     &diskWriteLock,
			waitGroup:         &wg,
			messageChan:       messageChan,
			errorChan:         errorChan,
		}
		threads = append(threads, thread)
	}

	return &Downloader{
		downloaderId:      uuid.New(),
		resourceInfo:      resourceInfo,
		downloadStats:     new(pkg.DownloadStats),
		downloadPrt:       downloadPrt,
		threads:           &threads,
		startTime:         time.Now(),
		runTime:           time.Since(time.Now()),
		fullPath:          fullPath,
		bytesDownloaded:   bytesDownloaded,
		bytesWritten:      bytesWritten,
		downloadWriteLock: &downloadWriteLock,
		diskWriteLock:     &diskWriteLock,
	}, nil
}
