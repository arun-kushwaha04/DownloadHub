package types

type DownloadType struct {
	maxThreads uint8
}

func (T *DownloadType) getMaxThreads() uint8 {
	return T.maxThreads
}
