package utils

import "errors"

var HttpClientIntalizationError = errors.New("Unable to create http client")
var HttpRequestError = errors.New("Unable to make http request")
var ServerError = errors.New("Server responsed with non 200 status")
var UnexpectedServerResponse = errors.New("Unexpected response body for resource")
var URLParseError = errors.New("Invalid download url")
var InvalidResourceSize = errors.New("Invalid resource size returned from http server")
var DirCreatePermissionError = errors.New("No permission to create dir")
var FileCreatePermissionError = errors.New("No permission to create file")
var FileReadPermissionError = errors.New("No permission to read file")
var FileWritePermissionError = errors.New("No permission to write to file")
var NoEnoughSpace = errors.New("Not enough space")
var FileNotFound = errors.New("File not found")
var FileRenameError = errors.New("No permission to rename file")
var TempDirCreatePermissionError = errors.New("No permission to create temporary directory")
var DownloadDirCreatePermissinError = errors.New("No permission to create download directory")
