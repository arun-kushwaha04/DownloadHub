package service

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/arun-kushwaha04/DownloadHub/configs"
	"github.com/arun-kushwaha04/DownloadHub/utils"
)

type Segment struct {
	segmentId    int64
	segmentStart int64
	segmentEnd   int64
	segmentPath  string

	waitGroup *sync.WaitGroup

	bytesTransfered       int64
	currentBandwidthTaken int64

	requested      [][2]int64
	requestedMutex *sync.Mutex

	completedChunks     [][2]int64 // need to restore download when pause due to less bandwidh
	completedChunkMutex *sync.Mutex

	maxThreads   uint8
	maxChunkSize int64

	file        *os.File
	threads     map[uint8]*thread
	threadMutex *sync.Mutex

	errorChan   chan error
	controlChan chan uint8

	downloader *downloader
}

func (segment *Segment) addThread(thread *thread) {
	segment.threadMutex.Lock()
	segment.threads[thread.threadId] = thread
	segment.threadMutex.Unlock()
}

func (segment *Segment) removeThread(threadId uint8, downloadedChunk [2]int64) {
	segment.threadMutex.Lock()
	segment.montiorChunkDownload(downloadedChunk)
	delete(segment.threads, threadId)
	segment.threadMutex.Unlock()
}

func (segment *Segment) requestChunk() [2]int64 {
	segment.requestedMutex.Lock()
	i := 1
	for ; i < len(segment.requested); i++ {
		if segment.requested[i-1][1] != segment.requested[i][0] {
			break
		}
	}
	if i == len(segment.requested) {
		// download completed
		segment.requestedMutex.Unlock()
		return [2]int64{-1, -1}
	}

	chunk := [2]int64{segment.requested[i-1][1], min(segment.requested[i-1][1]+segment.maxChunkSize, segment.requested[i][0])}

	segment.requested = append(segment.requested[:i+1], segment.requested[i:]...)
	segment.requested[i] = chunk
	segment.requestedMutex.Unlock()

	return chunk
}

func (segment *Segment) updateChunk(start int64, newEndChunk int64) {
	segment.requestedMutex.Lock()
	i := 1
	for ; i < len(segment.requested); i++ {
		if segment.requested[i][0] == start {
			break
		}
	}
	segment.requested[i][1] = newEndChunk
	segment.requestedMutex.Unlock()
}

func (segment *Segment) montiorChunkDownload(downloadedChunk [2]int64) {
	i := 0
	for ; i < len(segment.completedChunks); i++ {
		if segment.completedChunks[i][0] >= downloadedChunk[1] {
			break
		}
		if segment.completedChunks[i][1] == downloadedChunk[0] {
			i++
			break
		}
	}
	if i == 0 {
		// starting chunk
		segment.completedChunks = append([][2]int64{downloadedChunk}, segment.completedChunks[0:]...)
	} else if i == len(segment.completedChunks) {
		segment.completedChunks = append(segment.completedChunks, downloadedChunk)
	} else {
		segment.completedChunks = append(segment.completedChunks[:i], segment.completedChunks[i-1:]...)
		segment.completedChunks[i] = downloadedChunk
	}
}

func (segment *Segment) StartSegment() {
	// defer segment.downloader.waitGroup.Done()
	fmt.Println("Start of segment", segment.segmentId)
	file, err := os.OpenFile(segment.segmentPath, os.O_RDWR, 0644)
	if err != nil {
		fmt.Println("Segment", segment.segmentId, "file open error")
		segment.downloader.errorChan <- err
		return
	}
	segment.file = file
	defer file.Close()

	// go func() {
	// 	// code stuck here until channel closes
	// 	for i := range segment.controlChan {
	// 		if i == 1 {
	// 			for i := range segment.threads {
	// 				segment.threads[i].controlChan <- 1
	// 				close(segment.threads[i].controlChan)
	// 			}
	// 			segment.errorChan <- nil
	// 			file.Close()
	// 		}
	// 	}
	// }()

	go func() {
		for err := range segment.errorChan {
			if err != nil {
				segment.downloader.errorChan <- err
				return
			}
		}
	}()

	limiter := make(chan uint8, segment.maxThreads)
	var i uint8 = 0
	for {
		chunk := segment.requestChunk()
		if chunk[1] == -1 {
			break
		}

		segment.waitGroup.Add(1)
		limiter <- 1
		go func(i uint8) {
			var controlChan chan uint8
			thread := &thread{
				threadId:    i,
				startTime:   time.Now(),
				startByte:   chunk[0],
				endByte:     chunk[1],
				segment:     segment,
				controlChan: controlChan,
			}
			segment.addThread(thread)
			thread.StartThread()
			segment.removeThread(i, chunk)
			<-limiter
		}(i)
		i++
	}

	segment.waitGroup.Wait()
	close(segment.errorChan)
	utils.PrintToTerminal("Segment downloaded", segment.segmentId, 1, false)

	return
}

func CreateNewSegment(segmentId int64, segmentParentFolder string, downloader *downloader) *Segment {

	//logic for segment here
	fileName := strconv.FormatInt(segmentId, 10) + configs.SEG_EXT
	filePath := path.Join(segmentParentFolder, fileName)

	var control chan uint8
	segmentFileSize, err := utils.FileExits(segmentParentFolder, fileName, true)

	if err != nil {
		fmt.Println("Unable to get information for segment file", err)
		return nil
	}
	// to think again
	if segmentFileSize > 0 && segmentFileSize != configs.SEGMENT_SIZE {
		// delete the old file and create new one
		err := utils.DeleteAndCreateNewFile(segmentParentFolder, fileName)
		if err != nil {
			fmt.Println("Unable to delete file", err)
			return nil
		}
	}

	var requestMutex sync.Mutex
	var threadMutex sync.Mutex
	var completedChunkMutex sync.Mutex
	waitGroup := &sync.WaitGroup{}

	segmentStart := segmentId * configs.SEGMENT_SIZE
	segmentEnd := min(((segmentId + 1) * configs.SEGMENT_SIZE), downloader.resourceInfo.FileSize)

	var requested, completedChunks [][2]int64
	var s = [2]int64{segmentStart - 1, segmentStart}
	var e = [2]int64{segmentEnd, segmentEnd}
	requested = append(requested, s)
	requested = append(requested, e)

	thread := make(map[uint8]*thread)
	errorChan := make(chan error)

	return &Segment{
		segmentId:    segmentId,
		segmentStart: segmentStart,
		segmentEnd:   segmentEnd,
		segmentPath:  filePath,

		waitGroup: waitGroup,

		bytesTransfered:       0,
		currentBandwidthTaken: 0,

		requested:      requested,
		requestedMutex: &requestMutex,

		completedChunks:     completedChunks,
		completedChunkMutex: &completedChunkMutex,

		threads:     thread,
		threadMutex: &threadMutex,

		maxThreads:   2,
		maxChunkSize: 1024 * 1024,

		errorChan:   errorChan,
		controlChan: control,

		downloader: downloader,
	}
}
