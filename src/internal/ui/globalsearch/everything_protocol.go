package globalsearch

import (
	"encoding/binary"
	"errors"
	"regexp"
	"unicode"
	"unicode/utf16"
)

// Wire layout from voidtools' Everything SDK ipc/everything_ipc.h:
// QUERY2 uses seven DWORDs (including a 32-bit HWND on x64), followed by UTF-16.
// LIST2 uses five DWORDs followed by 8-byte ITEM2 records and length-prefixed strings.
const (
	ipcQueryHeaderBytes  = 28
	ipcListHeaderBytes   = 20
	ipcItemBytes         = 8
	ipcRegexFlag         = 8
	ipcFolderFlag        = 1
	ipcMaxReplyBytes     = 16 * 1024 * 1024
	ipcRequestFullPath   = 4
	ipcStringLengthBytes = 4
)

func everythingQueryPacket(replyWindow, replyID uint32, query string, limit int) []byte {
	if limit <= 0 {
		limit = defaultResultLimit
	}
	limit = min(limit, localVisitLimit)
	flags := uint32(0)
	searchText := query
	for _, character := range query {
		if !unicode.IsLetter(character) && !unicode.IsNumber(character) &&
			character != '.' && character != '-' && character != '_' {
			// Keep syntax/wildcards literal without interpreting Everything
			// operators. Regex searches substrings; no expensive .* wrapper.
			flags = ipcRegexFlag
			searchText = regexp.QuoteMeta(query)
			break
		}
	}
	text := utf16.Encode([]rune(searchText))
	packet := make([]byte, ipcQueryHeaderBytes+(len(text)+1)*2)
	binary.LittleEndian.PutUint32(packet, replyWindow)
	binary.LittleEndian.PutUint32(packet[4:], replyID)
	binary.LittleEndian.PutUint32(packet[8:], flags)
	binary.LittleEndian.PutUint32(packet[16:], uint32(limit))
	binary.LittleEndian.PutUint32(packet[20:], ipcRequestFullPath)
	binary.LittleEndian.PutUint32(packet[24:], 1) // Name ascending: indexed fast sort.
	for index, unit := range text {
		binary.LittleEndian.PutUint16(packet[ipcQueryHeaderBytes+index*2:], unit)
	}
	return packet
}

func parseEverythingReply(packet []byte, limit int) ([]Result, error) {
	if len(packet) < ipcListHeaderBytes || len(packet) > ipcMaxReplyBytes {
		return nil, errors.New("invalid Everything response size")
	}
	if binary.LittleEndian.Uint32(packet[12:]) != ipcRequestFullPath {
		return nil, errors.New("unexpected Everything response fields")
	}
	count := int(binary.LittleEndian.Uint32(packet[4:]))
	if limit <= 0 {
		limit = defaultResultLimit
	}
	if count > min(limit, localVisitLimit) || count > (len(packet)-ipcListHeaderBytes)/ipcItemBytes {
		return nil, errors.New("invalid Everything result count")
	}
	stringsStart := ipcListHeaderBytes + count*ipcItemBytes
	results := make([]Result, 0, count)
	for index := range count {
		item := packet[ipcListHeaderBytes+index*ipcItemBytes:]
		offset := uint64(binary.LittleEndian.Uint32(item[4:]))
		if offset < uint64(stringsStart) || offset+4 > uint64(len(packet)) {
			return nil, errors.New("invalid Everything result data offset")
		}
		length := uint64(binary.LittleEndian.Uint32(packet[offset:]))
		if length == 0 || offset+4+(length+1)*2 > uint64(len(packet)) {
			return nil, errors.New("invalid Everything result path length")
		}
		path, err := ipcString(packet, offset+ipcStringLengthBytes, stringsStart)
		if err != nil {
			return nil, err
		}
		if uint64(len(utf16.Encode([]rune(path)))) != length {
			return nil, errors.New("everything result path length mismatch")
		}
		results = append(results, Result{
			Path:      path,
			Directory: binary.LittleEndian.Uint32(item)&ipcFolderFlag != 0,
		})
	}
	return results, nil
}

func ipcString(packet []byte, offset uint64, minimum int) (string, error) {
	if minimum < 0 || offset < uint64(minimum) || offset >= uint64(len(packet)) || offset%2 != 0 {
		return "", errors.New("invalid Everything string offset")
	}
	units := make([]uint16, 0)
	for index := offset; index+1 < uint64(len(packet)); index += 2 {
		unit := binary.LittleEndian.Uint16(packet[index:])
		if unit == 0 {
			return string(utf16.Decode(units)), nil
		}
		units = append(units, unit)
	}
	return "", errors.New("unterminated Everything string")
}
