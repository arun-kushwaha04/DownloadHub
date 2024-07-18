package service

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/arun-kushwaha04/DownloadHub/configs"
	"github.com/arun-kushwaha04/DownloadHub/pkg"
	"github.com/arun-kushwaha04/DownloadHub/utils"
	"github.com/google/uuid"
)

type downloader struct {
	downloaderId  uuid.UUID
	resourceInfo  *pkg.ResourceInfo
	downloadStats *pkg.DownloadStats
	downloadPrt   pkg.DownloadSpeed
	fullPath      string

	startTime        time.Time
	runTime          time.Duration
	writeTime        time.Duration
	writeTimeChannel chan time.Duration

	bytesDownloaded    int64
	bytesWrittenToDisk int64
	bytesUpdateChannel chan [2]int

	maxNumberOfSegments int
	totalSegments       int64

	activeSegments    map[int64]*Segment
	completedSegments int64
	segmentMutex      *sync.Mutex

	waitGroup *sync.WaitGroup
	errorChan chan error

	statsUpdateInterval time.Duration
	prevDownloadSpeed   float64
	maxBandwidth        float64

	intervalBytesDownload int
	intervalByteMutex     *sync.Mutex
	instantDownloadSpeed  float64

	speedLimited     bool
	maxDownloadSpeed float64

	lastSyncTime time.Time
	client       *http.Client
}

func (downloader *downloader) addSegement(segment *Segment) {
	downloader.segmentMutex.Lock()
	downloader.activeSegments[segment.segmentId] = segment
	downloader.completedSegments++
	downloader.segmentMutex.Unlock()
}

func (downloader *downloader) removeSegement(segmentId int64) {
	downloader.segmentMutex.Lock()
	delete(downloader.activeSegments, segmentId)
	downloader.segmentMutex.Unlock()
}

func (downloader *downloader) Intalize(
	uuid uuid.UUID,
	resourceInfo *pkg.ResourceInfo,
	downloadStats *pkg.DownloadStats,
	downloadPrt pkg.DownloadSpeed,
	fullPath string,

	writeTime time.Duration,
	writeTimeChannel chan time.Duration,

	bytesUpdateChannel chan [2]int,

	maxNumberOfSegments int,
	totalSegments int64,

	activeSegments map[int64]*Segment,
	segmentMutex *sync.Mutex,

	waitGroup *sync.WaitGroup,
	errorChan chan error,

	statsUpdateInterval time.Duration,
	maxBandwidth float64,

	intervalByteMutex *sync.Mutex,

	speedLimited bool,
	maxDownloadSpeed float64,

) {
	downloader.downloaderId = uuid
	downloader.resourceInfo = resourceInfo
	downloader.downloadStats = downloadStats
	downloader.downloadPrt = downloadPrt
	downloader.fullPath = fullPath

	downloader.startTime = time.Now()
	downloader.runTime = time.Since(time.Now())
	downloader.writeTime = time.Duration(0)
	downloader.writeTimeChannel = writeTimeChannel

	downloader.bytesDownloaded = 0
	downloader.bytesUpdateChannel = bytesUpdateChannel
	downloader.bytesWrittenToDisk = 0

	downloader.maxNumberOfSegments = maxNumberOfSegments
	downloader.totalSegments = totalSegments

	downloader.activeSegments = activeSegments
	downloader.completedSegments = 0
	downloader.segmentMutex = segmentMutex

	downloader.waitGroup = waitGroup
	downloader.errorChan = errorChan

	downloader.statsUpdateInterval = statsUpdateInterval
	downloader.prevDownloadSpeed = float64(0)
	downloader.maxBandwidth = maxBandwidth

	downloader.intervalBytesDownload = 0
	downloader.intervalByteMutex = intervalByteMutex
	downloader.instantDownloadSpeed = float64(0)

	downloader.speedLimited = speedLimited
	downloader.maxDownloadSpeed = maxDownloadSpeed

	downloader.lastSyncTime = time.Now()
	downloader.client = &http.Client{}
}

func (downloader downloader) GetDownloadUrl() *url.URL {
	return downloader.resourceInfo.Url
}

func (downloader *downloader) MonitorDownloadResource() {

	downloader.intervalByteMutex.Lock()
	bytes := downloader.intervalBytesDownload
	downloader.intervalBytesDownload = 0
	downloader.intervalByteMutex.Unlock()

	prevDownloadSpeed := downloader.instantDownloadSpeed

	elapsedTime := time.Since(downloader.startTime)
	bytesRead := downloader.bytesDownloaded
	bytesWritten := downloader.bytesWrittenToDisk
	writeTime := downloader.writeTime
	fileSize := downloader.resourceInfo.FileSize

	downloadSpeed := float64(bytesRead) / elapsedTime.Seconds()
	diskWriteSpeed := float64(bytesWritten) / float64(writeTime)

	estimatedRemainigTime := time.Duration(float64(fileSize-bytesRead)/(downloadSpeed)) * time.Second

	progress := float32(float64(bytesRead) * (100 / float64(fileSize)))

	consistenProgress := float32(float64(downloader.completedSegments*configs.SEGMENT_SIZE) * (100 / float64(fileSize)))

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	downloader.downloadStats.UpdateDownloadStats(downloadSpeed, diskWriteSpeed, m.Alloc, elapsedTime, estimatedRemainigTime, progress, consistenProgress)

	downloader.instantDownloadSpeed = float64(bytes) / downloader.statsUpdateInterval.Seconds()

	if downloader.instantDownloadSpeed >= prevDownloadSpeed {
		downloader.instantDownloadSpeed = downloader.instantDownloadSpeed * 1.1 //10% increase
	} else {
		downloader.instantDownloadSpeed = downloader.instantDownloadSpeed * 0.9 //10% decrease
	}

	if downloader.instantDownloadSpeed > float64(downloader.maxBandwidth) {
		downloader.instantDownloadSpeed = float64(downloader.maxBandwidth)
	}

	if downloader.speedLimited && downloader.instantDownloadSpeed > downloader.maxDownloadSpeed {
		downloader.instantDownloadSpeed = downloader.maxDownloadSpeed
	}

	if time.Since(downloader.lastSyncTime) >= 5*time.Second {
		downloader.lastSyncTime = time.Now()
		fmt.Printf("Time %s %.2f%% | Download Speed: %.2f B/s | Remaining Time: %s | Disk Write Speed: %.2f B/s | \nMemory Alloc: %d bytes | Instatneous Speed %.2f B/s | Consistent Progress %.2f%% | \nCompleted Segments: %d | Active Segments %d | Bandwidth %.2f |\n",
			elapsedTime.String(), progress, downloadSpeed, estimatedRemainigTime.Truncate(time.Second), diskWriteSpeed, m.Alloc, downloader.instantDownloadSpeed, consistenProgress, downloader.completedSegments, len(downloader.activeSegments), downloader.maxBandwidth)
	}
}

func (downloader *downloader) StartDownload() {
	// defer downloader.closeActiveSegments()
	defer close(downloader.errorChan)
	defer close(downloader.writeTimeChannel)
	defer close(downloader.bytesUpdateChannel)

	downloader.startTime = time.Now()

	segmentParentFolder := path.Join(configs.TEMP_DIRECTORY, downloader.downloaderId.String())

	segmentToDownload := make(chan int64)

	go func() {
		for i := range downloader.totalSegments {
			segmentToDownload <- i
		}
		close(segmentToDownload)
	}()

	go func() {
		for err := range downloader.errorChan {
			if err != nil {
				fmt.Println(err)
				return
			}
		}
	}()

	// monitoring downloader usage
	ticker := time.NewTicker(downloader.statsUpdateInterval)
	quit := make(chan struct{})
	go func() {
		for {
			select {

			case <-ticker.C:
				downloader.MonitorDownloadResource()

			case update := <-downloader.bytesUpdateChannel:
				if update[0] == 0 {
					downloader.bytesDownloaded += int64(update[1])
					downloader.intervalByteMutex.Lock()
					downloader.intervalBytesDownload += update[1]
					downloader.intervalByteMutex.Unlock()
				} else {
					downloader.bytesWrittenToDisk += int64(update[1])
				}

			case time := <-downloader.writeTimeChannel:
				downloader.writeTime += time

			case <-quit:
				ticker.Stop()
				return

			}

		}
	}()

	limiter := make(chan uint8, downloader.maxNumberOfSegments)
	for segmentId := range segmentToDownload {
		limiter <- 1
		downloader.waitGroup.Add(1)

		// downloader.activeSegments = append(downloader.activeSegments, segment)

		go func() {
			defer downloader.waitGroup.Done()
			segment := CreateNewSegment(segmentId, segmentParentFolder, downloader)
			downloader.addSegement(segment)
			segment.StartSegment()
			downloader.removeSegement(segmentId)
			fmt.Println("Exiting the segment", segment.segmentId)
			<-limiter
		}()
	}

	downloader.waitGroup.Wait()
	close(quit)
	close(limiter)

	fmt.Println("Download completed")

	// merge downloaded files
	if err := downloader.MergeDownload(); err != nil {
		fmt.Println(err, utils.FileRebiuldError)
	} else {
		if err := utils.RenameFile(downloader.fullPath, downloader.resourceInfo.FileName); err != nil {
			fmt.Println(err, utils.DownloadFailedRenameError)
		}
	}

	downloader.runTime = time.Since(downloader.startTime)
	fmt.Println("Time taken", downloader.runTime)
	// close(quit)

	return
}

func (downloader downloader) PrintStruct(place string) {
	fmt.Println(place, &downloader)
	v := reflect.ValueOf(downloader)
	typeOfS := v.Type()

	for i := 0; i < v.NumField(); i++ {
		fmt.Printf("%s %v \n", typeOfS.Field(i).Name, v.Field(i).Interface())
	}
	fmt.Printf("\n")
}

func (downloader downloader) MergeDownload() error {
	fmt.Println("Merging downloads")
	tempFolder := path.Join(configs.TEMP_DIRECTORY, downloader.downloaderId.String())
	for i := range downloader.totalSegments {
		filePath := path.Join(tempFolder, strconv.FormatInt(i, 10)+configs.SEG_EXT)

		if err := utils.MergeSegment(i*configs.SEGMENT_SIZE, filePath, downloader.fullPath); err != nil {
			fmt.Println(err)
			return err
		}
	}
	return nil
}

func (downloader *downloader) closeActiveSegments() {
	for _, segment := range downloader.activeSegments {
		segment.controlChan <- 1
	}
}

func CreateDownloader(resourceUrl string, downloadPrt pkg.DownloadSpeed) (*downloader, error) {

	downloader := downloader{}

	resourceInfo, err := utils.GetMetaData(resourceUrl)
	if err != nil {
		return nil, err
	}

	totalSegments := resourceInfo.FileSize / configs.SEGMENT_SIZE

	if totalSegments*configs.SEGMENT_SIZE != resourceInfo.FileSize {
		totalSegments++
	}

	fmt.Println("File size", resourceInfo.FileSize, "Total segments", totalSegments)

	parentDir := utils.GetDownloadFolder(path.Ext((*resourceInfo.Url).Path))

	fullPath, err := utils.CreateFile(parentDir, (*resourceInfo).FileName, (*resourceInfo).FileSize)

	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup

	// channels
	errorChan := make(chan error)
	writeTimeChannel := make(chan time.Duration)
	bytesUpdateCahnnel := make(chan [2]int)

	statsUpdateInterval := 1 * time.Second
	maxBandwidth := 1.0 * configs.BANDWIDTH

	var writeTime time.Duration = 0

	var intervalByteMutex sync.Mutex
	var segmentMutex sync.Mutex

	stat := pkg.NewDownloadStats()

	activeSegments := make(map[int64]*Segment)

	speedLimited := false
	maxDownloadSpeed := math.MaxFloat64

	maxNumberSegments := 20

	downloader.Intalize(
		uuid.New(),
		resourceInfo,
		stat,
		downloadPrt,
		fullPath,

		writeTime,
		writeTimeChannel,

		bytesUpdateCahnnel,

		maxNumberSegments,
		totalSegments,

		activeSegments,
		&segmentMutex,

		&wg,
		errorChan,

		statsUpdateInterval,
		maxBandwidth,

		&intervalByteMutex,

		speedLimited,
		maxDownloadSpeed,
	)
	return &downloader, nil
}
