package duplicates

import (
	"os"

	"golang.org/x/sys/windows"
)

const retainIndexedInfos = true

// Native Windows measurements put the handle-index crossover around 2,000
// files. Delay promotion to avoid handle allocation for the common small scan.
const identityIndexThreshold = 1024

func identityOf(path string, info os.FileInfo) (fileIdentity, bool) {
	// os.FileInfo.Sys exposes attributes but no file ID on Windows. Query a
	// real handle instead of depending on Go's private FileInfo representation.
	// os.Open also preserves Go's handling of long paths and directory handles.
	// If the read handle is denied, retain the original os.SameFile fallback.
	handle, err := os.Open(path)
	if err != nil {
		return fileIdentity{}, false
	}
	current, err := handle.Stat()
	if err != nil {
		handle.Close()
		return fileIdentity{}, false
	}
	var data windows.ByHandleFileInformation
	err = windows.GetFileInformationByHandle(windows.Handle(handle.Fd()), &data)
	handle.Close()
	// Close first: os.SameFile may lazily open the snapshot's path with
	// exclusive sharing. Keeping a read handle open would make that fail.
	if err != nil || !os.SameFile(info, current) {
		// The path may have been replaced since the snapshot was taken. Never
		// associate that snapshot with the replacement's identity.
		return fileIdentity{}, false
	}
	return fileIdentity{
		device: uint64(data.VolumeSerialNumber),
		inode:  uint64(data.FileIndexHigh)<<32 | uint64(data.FileIndexLow),
	}, true
}
