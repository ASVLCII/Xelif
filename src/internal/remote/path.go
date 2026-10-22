package remote

import (
	"errors"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

var errUnsafePath = errors.New("remote path is empty, malformed, or unsafe")

// RemotePath is a parsed rclone remote and slash-separated path. A zero Path
// identifies the root of the configured remote.
type RemotePath struct {
	Remote string
	Path   string
}

// ParsePath parses the literal remote:path form. It deliberately implements no
// shell quoting, expansion, variables, pipelines, or command separators.
func ParsePath(value string) (RemotePath, error) {
	colon := strings.IndexByte(value, ':')
	if colon <= 0 {
		return RemotePath{}, newError("parse", value, InvalidPath, errUnsafePath, "")
	}
	remote := value[:colon]
	objectPath := value[colon+1:]
	if err := validateRemoteName(remote); err != nil {
		return RemotePath{}, newError("parse", value, InvalidPath, err, "")
	}
	normalized, err := normalizeObjectPath(objectPath, true)
	if err != nil {
		return RemotePath{}, newError("parse", value, InvalidPath, err, "")
	}
	return RemotePath{Remote: remote, Path: normalized}, nil
}

func (p RemotePath) String() string { return p.Remote + ":" + p.Path }

func normalizeRemotePath(value RemotePath, allowRoot bool) (RemotePath, error) {
	if err := validateRemoteName(value.Remote); err != nil {
		return RemotePath{}, err
	}
	normalized, err := normalizeObjectPath(value.Path, allowRoot)
	if err != nil {
		return RemotePath{}, err
	}
	return RemotePath{Remote: value.Remote, Path: normalized}, nil
}

func validateRemoteName(name string) error {
	if strings.TrimSpace(name) == "" || name != strings.TrimSpace(name) || !utf8.ValidString(name) || hasControl(name) {
		return errUnsafePath
	}
	if strings.ContainsAny(name, ":/\\`'\"|&;<>(){}$") {
		return errUnsafePath
	}
	return nil
}

func normalizeObjectPath(value string, allowRoot bool) (string, error) {
	if !utf8.ValidString(value) || hasControl(value) ||
		strings.HasPrefix(value, "/") || strings.ContainsRune(value, '\\') {
		return "", errUnsafePath
	}
	if strings.ContainsAny(value, "`'\"|&;<>(){}$") {
		return "", errUnsafePath
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "." || segment == ".." {
			return "", errUnsafePath
		}
	}
	if value == "" {
		if allowRoot {
			return "", nil
		}
		return "", errUnsafePath
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", errUnsafePath
	}
	return cleaned, nil
}

func hasControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}
