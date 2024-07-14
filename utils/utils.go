package utils

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	config "github.com/arun-kushwaha04/DownloadHub/configs"
	pkg "github.com/arun-kushwaha04/DownloadHub/pkg"
)

func GetClient(method string, url *url.URL, body *io.Reader, header *map[string][]string) (*http.Client, *http.Request, error) {
	client := &http.Client{}

	requestUri := url.String()
	req, err := http.NewRequest(method, requestUri, *body)

	if err != nil {
		return nil, nil, HttpRequestError
	}

	// adding headers
	req.Header = *header
	req.Header.Add("Host", url.Hostname())
	req.Header.Add("User-Agent", config.DEFAULT_USER_AGENT)

	return client, req, nil
}

func GetMetaData(resourceString *string) (*pkg.ResourceInfo, error) {

	parsedUrl, err := url.Parse(*resourceString)
	if err != nil {
		return nil, URLParseError
	}

	fileName := path.Base(parsedUrl.Path)

	var headers map[string][]string
	client, req, err := GetClient("HEAD", parsedUrl, nil, &headers)
	if err != nil {
		return nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, HttpClientIntalizationError
	}

	defer res.Body.Close()

	fileSizeInString := res.Header.Get("Content-Length")
	if fileSizeInString == "" {
		return nil, InvalidResourceSize
	}

	fileSize, err := strconv.ParseInt(fileSizeInString, 10, 64)
	if err != nil {
		return nil, InvalidResourceSize
	}

	acceptRanges := res.Header.Get("Accept-Ranges")
	if acceptRanges == "" {
		// not resumable download

		return &pkg.ResourceInfo{FileSize: fileSize, FileName: fileName, Resumeable: false, Url: parsedUrl}, nil
	}

	return &pkg.ResourceInfo{FileSize: fileSize, FileName: fileName, Resumeable: true, Url: parsedUrl}, nil
}

func CreateFile(parentDir string, fileName string, fileSize int64) (string, error) {
	fileName += config.TEMP_EXT

	if err := os.MkdirAll(parentDir, os.ModePerm); err != nil {
		return "", DirCreatePermissionError
	}

	fullPath := filepath.Join(parentDir, fileName)

	file, err := os.Create(fullPath)
	if err != nil {
		return "", FileCreatePermissionError
	}

	defer file.Close()

	if fileSize > 0 {
		if err := file.Truncate(fileSize); err != nil {
			return "", NoEnoughSpace
		}
	}

	return fullPath, nil
}

func GetByteRangeHeader(start *int64, end *int64) *map[string][]string {

	values := []string{ fmt.Sprintf("bytes=%d-%d",start,end)}
	var header map[string][]string

	header["Range"] = values

	return &header
}

func RenameFile(src string, newFileName string) error {
	_, err := os.Stat(src)
	if os.IsNotExist(err) {
		return FileNotFound
	}

	if err == nil {
		dirPath := filepath.Dir(src)
		dest := filepath.Join(dirPath, newFileName)

		if err := os.Rename(src, dest); err != nil {
			return FileRenameError
		}

		return nil
	}

	return err
}

func GetStats(startTime *time.Time, writeDuration *time.Time, bytesRead *int64, bytesWrote *int64, fileSize *int64) *pkg.DownloadStats {

	elapsed := time.Since(*startTime)

	downloadSpeed := float64(*bytesRead) / elapsed.Seconds()

	diskWriteSpeed := float64(*bytesWrote) / float64((*writeDuration).Second())

	estimatedRemainingTime := time.Duration(float64(*fileSize-*bytesRead)/downloadSpeed) * time.Second

	progress := float32(float64(*bytesRead) * (100 / float64(*fileSize)))

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return &pkg.DownloadStats{DownloadSpeed: &downloadSpeed, DiskWriteSpeed: &diskWriteSpeed, EstimateRemainingTime: &estimatedRemainingTime, ElapsedTime: &elapsed, Progress: &progress, MemoryUsed: &m.Alloc}
}

func GetDownloadFolder(ext string) string {

	for _, v := range config.VideoExtensions {
		if v == ext {
			return filepath.Join(config.DOWNLOAD_DIRECTORY, config.VIDEO_SUB_FOLDER)
		}
	}
	for _, v := range config.MusicExtensions {
		if v == ext {
			return filepath.Join(config.DOWNLOAD_DIRECTORY, config.MUSIC_SUB_FOLDER)
		}
	}
	for _, v := range config.ProgramExtensions {
		if v == ext {
			return filepath.Join(config.DOWNLOAD_DIRECTORY, config.PROGRAMS_SUB_FOLDER)
		}
	}
	for _, v := range config.CompressedExtensions {
		if v == ext {
			return filepath.Join(config.DOWNLOAD_DIRECTORY, config.COMPRESSED_SUB_FOLDER)
		}
	}
	for _, v := range config.DocumentExtensions {
		if v == ext {
			return filepath.Join(config.DOWNLOAD_DIRECTORY, config.DOCUMENT_SUB_FOLDER)
		}
	}

	return filepath.Join(config.DOWNLOAD_DIRECTORY, config.GENERAL_SUB_FOLDER)

}
