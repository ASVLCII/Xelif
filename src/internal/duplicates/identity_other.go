//go:build !unix

package duplicates

import "os"

func identityOf(_ os.FileInfo) (fileIdentity, bool) {
	return fileIdentity{}, false
}
