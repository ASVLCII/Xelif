//go:build !unix && !windows

package duplicates

import "os"

const retainIndexedInfos = false
const identityIndexThreshold = 0

func identityOf(_ string, _ os.FileInfo) (fileIdentity, bool) {
	return fileIdentity{}, false
}
