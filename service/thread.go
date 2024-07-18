package service

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/arun-kushwaha04/DownloadHub/configs"
	"github.com/arun-kushwaha04/DownloadHub/utils"
)

type thread struct {
	threadId    uint8
	startByte   int64
	endByte     int64
	startTime   time.Time
	segment     *Segment
	controlChan chan uint8
}

func (thread *thread) StartThread() {
	defer thread.segment.waitGroup.Done()

	thread.startTime = time.Now()
	// utils.PrintToTerminal("Starting goroutine", thread.segment.segmentId, thread.threadId, false)

	url := thread.segment.downloader.GetDownloadUrl()

	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		utils.PrintToTerminal("Unable to Create request", thread.segment.segmentId, thread.threadId, true)
		thread.segment.errorChan <- err
		return
	}

	req.Header.Add("Host", url.Hostname())
	req.Header.Add("User-Agent", configs.DEFAULT_USER_AGENT)
	req.Header.Add("Range", fmt.Sprintf("bytes=%d-%d", thread.startByte, thread.endByte))

	res, err := thread.segment.downloader.client.Do(req)
	if err != nil {
		utils.PrintToTerminal("Unable to make request", thread.segment.segmentId, thread.threadId, false)
		thread.segment.errorChan <- err
		return
	}
	defer res.Body.Close()

	// go func() {
	// 	// code stuck here until channel closes
	// 	for i := range thread.controlChan {
	// 		if i == 1 {
	// 			utils.PrintToTerminal("Exiting goroutine control signal", thread.segment.segmentId, thread.threadId, false)
	// 			if currentReq != nil && currentReq.Body != nil {
	// 				currentReq.Body.Close()
	// 			}
	// 			thread.segment.waitGroup.Done()
	// 		}
	// 	}
	// }()

	// if res.StatusCode == http.StatusRequestedRangeNotSatisfiable {

	// }

	if res.StatusCode != http.StatusPartialContent && res.StatusCode != http.StatusOK {
		utils.PrintToTerminal(fmt.Sprintf("Invalid response %d", res.StatusCode), thread.segment.segmentId, thread.threadId, false)
		thread.segment.errorChan <- err
	}

	fileBuffer := make([]byte, 1024*1024)
	fileBufferIdx := 0
	var offset int64 = thread.startByte - (int64(thread.segment.segmentId) * configs.SEGMENT_SIZE)

	// utils.PrintToTerminal(fmt.Sprintf("Requested byte range %d-%d, %d", thread.startByte, thread.endByte, offset), thread.segment.segmentId, thread.threadId, false)

	for {
		// read res body in buffer[idx:len(buff)]
		n, err := res.Body.Read(fileBuffer[fileBufferIdx:])

		if err == io.EOF {
			// end of response body writing remaining bytes to files
			if fileBufferIdx > 0 {
				// write to file from buff[0:idx-1]
				thread.writeToFile(&fileBuffer, &fileBufferIdx, &offset)
			}
			break
		}
		if err != nil {
			thread.segment.updateChunk(thread.startByte, thread.startByte+offset)
			utils.PrintToTerminal("Error while reading response body", thread.segment.segmentId, thread.threadId, true)
			thread.segment.errorChan <- err
			return
		}

		//deciding whether to write to file
		if fileBufferIdx == configs.FILE_BUFF_SIZE {
			// write to file from buff[0:idx-1]
			thread.writeToFile(&fileBuffer, &fileBufferIdx, &offset)

			// pausing download so that synchronized bandwidth is maintained
			if thread.segment.downloader.instantDownloadSpeed > 0 {
				sleepTime := time.Duration(n) * time.Second / time.Duration(thread.segment.downloader.instantDownloadSpeed)
				if sleepTime > 0 {
					utils.PrintToTerminal(fmt.Sprintf("Go routine sleeping for %s", sleepTime), thread.segment.segmentId, thread.threadId, false)
					time.Sleep(sleepTime)
				}
			}
		}

		if n > 0 {
			fileBufferIdx += n
			thread.segment.downloader.bytesUpdateChannel <- [2]int{0, n}
		}
	}

	// utils.PrintToTerminal("Exiting goroutine", thread.segment.segmentId, thread.threadId, false)
	return
}

func (thread *thread) writeToFile(fileBuffer *[]byte, fileBufferIdx *int, offset *int64) {

	startTime := time.Now()
	wt, err := thread.segment.file.WriteAt((*fileBuffer)[:*fileBufferIdx], *offset)

	if err != nil {
		thread.segment.updateChunk(thread.startByte, thread.startByte+*offset)
		utils.PrintToTerminal("Unable to write to segment file", thread.segment.segmentId, thread.threadId, true)
		thread.segment.errorChan <- err
		return
	}
	thread.segment.downloader.writeTimeChannel <- time.Since(startTime)
	thread.segment.downloader.bytesUpdateChannel <- [2]int{1, wt}
	*offset += int64(wt)
	*fileBufferIdx = 0

	return
}
