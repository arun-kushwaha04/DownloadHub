package utils

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"

	config "github.com/arun-kushwaha04/DownloadHub/configs"
	pkg "github.com/arun-kushwaha04/DownloadHub/pkg"
)

func GetClient(method string, url *url.URL, body io.Reader, header *map[string]string) (*http.Client, *http.Request, error) {
	client := &http.Client{}

	requestUri := url.String()
	req, err := http.NewRequest(method, requestUri, body)

	if err != nil {
		return nil, nil, HttpRequestError
	}

	// adding headers
	for key, value := range *header {
		req.Header.Add(key, value)
	}
	req.Header.Add("Host", url.Hostname())
	req.Header.Add("User-Agent", config.DEFAULT_USER_AGENT)

	return client, req, nil
}

func GetMetaData(resourceString string) (*pkg.ResourceInfo, error) {

	parsedUrl, err := url.Parse(resourceString)
	if err != nil {
		return nil, URLParseError
	}

	fileName := path.Base(parsedUrl.Path)

	var headers map[string]string
	client, req, err := GetClient("HEAD", parsedUrl, nil, &headers)
	if err != nil {
		return nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
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

	fullPath := filepath.Join(parentDir, fileName)

	fs, err := FileExits(parentDir, fileName, false)
	if err != nil {
		return "", err
	}

	if fs > 1 {
		return fullPath, nil
	}

	fileName += config.TEMP_EXT

	if err := os.MkdirAll(parentDir, os.ModePerm); err != nil {
		return "", DirCreatePermissionError
	}

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

func GetByteRangeHeader(start *int64, end *int64) *map[string]string {

	values := fmt.Sprintf("bytes=%d-%d", *start, *end)
	header := make(map[string]string)

	header["Range"] = values

	return &header
}

func RenameFile(src string, newFileName string) error {
	fmt.Println("Renaming download")
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

func MergeSegment(offset int64, segmentPath string, filePath string) error {

	srcFile, err := os.Open(segmentPath)
	if err != nil {
		return MissingSegmentFile
	}

	dstFile, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return MissingMainFile
	}
	defer dstFile.Close()

	if _, err := dstFile.Seek(offset, io.SeekStart); err != nil {
		return FileReadPermissionError
	}

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return FileWritePermissionError
	}

	return nil
}

func GetSegmentName(fileName string, i int) string {
	return fileName + config.SEG_EXT + strconv.Itoa(i)
}

func FileExits(parentDir string, fileName string, ifNotCreate bool) (int64, error) {
	fileInfo, err := os.Stat(path.Join(parentDir, fileName))
	if err == nil {
		return fileInfo.Size(), nil
	}
	if os.IsNotExist(err) {
		if ifNotCreate {
			_, err := CreateFile(parentDir, fileName, 0)
			if err != nil {
				return 0, FileCreatePermissionError
			}
			return 0, nil
		} else {
			return 0, nil
		}
	}
	return 0, FileReadPermissionError
}

func DeleteAndCreateNewFile(parentDir string, fileName string) error {
	filePath := path.Join(parentDir, fileName)
	err := os.Remove(filePath)
	if err != nil {
		return err
	}
	_, err = CreateFile(parentDir, fileName, 0)
	if err != nil {
		return err
	}
	return nil
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

func PrintToTerminal(reason string, segmentId int64, threadId uint8, isError bool) {
	// if isError {
	// 	fmt.Printf("Error: Segment id %d thread id %d - %s\n", segmentId, threadId, reason)
	// } else {
	// 	fmt.Printf("Info: Segment id %d thread id %d - %s\n", segmentId, threadId, reason)
	// }
}
