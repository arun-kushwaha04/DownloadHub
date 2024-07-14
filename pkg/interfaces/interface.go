package interfaces

type DownloadPriority interface {
	getMaxThreads() uint8
}
