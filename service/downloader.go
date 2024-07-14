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
	threadId          uint8
	startByte         int64
	endByte           int64
	fullPath          string
	startTime         time.Time
	runTime           time.Duration
	writeTime         *time.Duration
	resourceInfo      *pkg.ResourceInfo
	bytesDownloaded   *int64
	bytesWritten      *int64
	downloadWriteLock *sync.RWMutex
	diskWriteLock     *sync.RWMutex
	waitGroup         *sync.WaitGroup
	messageChan       chan rune
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
	writeTime         time.Duration
	bytesDownloaded   *int64
	bytesWritten      *int64
	downloadWriteLock *sync.RWMutex
	diskWriteLock     *sync.RWMutex
	waitGroup         *sync.WaitGroup
	errorChan         chan error
}

func (thread *downloaderThread) StartThread() error {
	defer thread.waitGroup.Done()
	thread.startTime = time.Now()

	file, err := os.OpenFile(thread.fullPath, os.O_RDWR, 0644)
	defer file.Close()
	if err != nil {
		thread.errorChan <- utils.FileReadPermissionError
		return utils.FileReadPermissionError
	}

	client, req, err := utils.GetClient("GET", thread.resourceInfo.Url, nil, utils.GetByteRangeHeader(&thread.startByte, &thread.endByte))

	if err != nil {
		thread.errorChan <- err
		return err
	}

	res, err := client.Do(req)
	if err != nil {
		thread.errorChan <- utils.HttpClientIntalizationError
		return utils.HttpClientIntalizationError
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusPartialContent && res.StatusCode != http.StatusOK {
		thread.errorChan <- utils.ServerError
		return utils.ServerError
	}

	// running another goroutine to monitor closing of goroutine (stop download)
	go func() {
		// code stuck here until channel closes
		for range thread.messageChan {
			break
		}
		file.Close()
		res.Body.Close()
		thread.waitGroup.Done()
	}()
	defer close(thread.messageChan)

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
			thread.errorChan <- utils.UnexpectedServerResponse
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
				thread.errorChan <- utils.FileWritePermissionError
				return utils.FileWritePermissionError
			}
			fileBufferIdx = 0
			thread.diskWriteLock.Lock()
			*thread.bytesWritten += int64(wt)
			thread.diskWriteLock.Unlock()
		}

	}
	thread.runTime = time.Since(thread.startTime)
	thread.errorChan <- nil
	return nil
}

func (downloader *Downloader) StartDownload() error {
	defer downloader.clearThreads()

	downloader.startTime = time.Now()

	// starting the threads
	for _, thread := range *downloader.threads {
		downloader.waitGroup.Add(1)
		go thread.StartThread()
	}

	// monitoring downloader usage
	ticker := time.NewTicker(5 * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				utils.GetStats(&downloader.startTime, &downloader.writeTime, downloader.bytesDownloaded, downloader.bytesWritten, &downloader.resourceInfo.FileSize, downloader.downloadStats)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	// closing all channels
	go func() {
		downloader.waitGroup.Wait()
		close(downloader.errorChan)
		close(quit)
	}()

	for err := range downloader.errorChan {
		if err != nil {
			return utils.DownloadFailed
		}
	}

	downloader.waitGroup.Wait()

	// merge downloaded files
	if err := downloader.MergeDownload(); err != nil {
		return utils.FileRebiuldError
	}

	if err := utils.RenameFile(downloader.fullPath, downloader.resourceInfo.FileName); err != nil {
		return utils.DownloadFailedRenameError
	}

	downloader.runTime = time.Since(downloader.startTime)
	return nil
}

func (downloader *Downloader) MergeDownload() error {
	for _, thread := range *downloader.threads {
		if err := utils.MergeSegment(thread.startByte, thread.fullPath, downloader.fullPath); err != nil {
			return err
		}
	}
	return nil
}

func (downloader *Downloader) clearThreads() {
	for _, thread := range *downloader.threads {
		*thread = downloaderThread{}
	}
	*downloader = Downloader{}
}

func (downloader *Downloader) PauseDownload() {
	for i := range *downloader.threads {
		close((*downloader.threads)[i].messageChan)
	}
	downloader.clearThreads()
}

func CreateDownloader(resourceUrl string, downloadPrt pkg.DownloadSpeed, ranges *map[uint8][2]int64) (*Downloader, error) {

	resumedDownload := false
	if ranges != nil {
		resumedDownload = true
	}

	resourceInfo, err := utils.GetMetaData(&resourceUrl)
	if err != nil {
		return nil, err
	}

	parentDir := utils.GetDownloadFolder(path.Ext((*resourceInfo.Url).Path))

	fullPath, err := utils.CreateFile(parentDir, (*resourceInfo).FileName, (*resourceInfo).FileSize)

	if err != nil {
		return nil, err
	}

	var downloadWriteLock, diskWriteLock sync.RWMutex
	var wg sync.WaitGroup

	// channels
	errorChan := make(chan error)

	var bytesDownloaded, bytesWritten *int64
	var writeDuration time.Duration = 0

	threads := make([]*downloaderThread, 0, downloadPrt.GetMaxThreads())

	bytesPerThread := (resourceInfo).FileSize / int64(downloadPrt.GetMaxThreads())
	// creating threads
	var i uint8
	for i = 0; i < downloadPrt.GetMaxThreads(); i++ {
		fullPath, err := utils.CreateFile(configs.TEMP_DIRECTORY, utils.GetSegmentName((resourceInfo).FileName, int(i)), 0)
		if err != nil {
			return nil, err
		}

		startByte := int64(i) * bytesPerThread
		endByte := int64(i+1)*bytesPerThread - 1

		if resumedDownload {
			value, ok := (*ranges)[i]
			if ok {
				startByte = value[0]
				endByte = value[1]
			} else {
				continue
			}
		}

		//channel for stopping goroutine
		messageChan := make(chan rune)

		thread := &downloaderThread{
			threadId:          i,
			startByte:         startByte,
			endByte:           endByte,
			resourceInfo:      resourceInfo,
			fullPath:          fullPath,
			startTime:         time.Now(),
			runTime:           time.Since(time.Now()),
			writeTime:         &writeDuration,
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
		writeTime:         writeDuration,
		fullPath:          fullPath,
		bytesDownloaded:   bytesDownloaded,
		bytesWritten:      bytesWritten,
		downloadWriteLock: &downloadWriteLock,
		diskWriteLock:     &diskWriteLock,
		waitGroup:         &wg,
		errorChan:         errorChan,
	}, nil
}

func (downloader *Downloader) ResumeDownload(resourceUrl string, downloadPrt pkg.DownloadSpeed) (*Downloader, error) {
	resourceInfo, err := utils.GetMetaData(&resourceUrl)
	if err != nil {
		return nil, err
	}

	ranges := make(map[uint8][2]int64)

	bytesPerThread := (resourceInfo).FileSize / int64(downloadPrt.GetMaxThreads())

	var i uint8
	for i = 0; i < downloadPrt.GetMaxThreads(); i++ {
		downloaded, err := utils.FileExits(configs.TEMP_DIRECTORY, utils.GetSegmentName((resourceInfo).FileName, int(i)), true)
		if err != nil {
			return nil, err
		}

		startByte := int64(i)*bytesPerThread + downloaded
		endByte := int64(i+1) * bytesPerThread

		if startByte != endByte {
			ranges[i] = [2]int64{startByte, endByte}
		}

	}

	return CreateDownloader(resourceUrl, downloadPrt, &ranges)
}
