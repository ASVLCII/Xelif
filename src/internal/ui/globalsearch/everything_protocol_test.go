package globalsearch

import (
	"context"
	"encoding/binary"
	"errors"
	"path/filepath"
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEverythingQueryPacketUsesLiteralUnicodeAndBoundedResults(t *testing.T) {
	packet := everythingQueryPacket(123, 77, `報告 (final).*`, 25)
	assert.Equal(t, uint32(123), binary.LittleEndian.Uint32(packet))
	assert.Equal(t, uint32(77), binary.LittleEndian.Uint32(packet[4:]))
	assert.Equal(t, uint32(ipcRegexFlag), binary.LittleEndian.Uint32(packet[8:]))
	assert.Equal(t, uint32(0), binary.LittleEndian.Uint32(packet[12:]))
	assert.Equal(t, uint32(25), binary.LittleEndian.Uint32(packet[16:]))
	text, err := ipcString(packet, ipcQueryHeaderBytes, ipcQueryHeaderBytes)
	require.NoError(t, err)
	assert.Equal(t, `報告 \(final\)\.\*`, text)
	packet = everythingQueryPacket(123, 78, "test", -1)
	assert.Equal(t, uint32(defaultResultLimit), binary.LittleEndian.Uint32(packet[16:]))
	assert.Zero(t, binary.LittleEndian.Uint32(packet[8:]), "ordinary names use indexed text matching")
}

func testEverythingReply() []byte {
	packet := make([]byte, ipcListHeaderBytes+ipcItemBytes)
	binary.LittleEndian.PutUint32(packet[4:], 1)
	binary.LittleEndian.PutUint32(packet[12:], ipcRequestFullPath)
	binary.LittleEndian.PutUint32(packet[ipcListHeaderBytes:], ipcFolderFlag)
	binary.LittleEndian.PutUint32(packet[ipcListHeaderBytes+4:], uint32(len(packet)))
	units := utf16.Encode([]rune(filepath.Join("X:", "work", "報告")))
	packet = binary.LittleEndian.AppendUint32(packet, uint32(len(units)))
	for _, unit := range append(units, 0) {
		packet = binary.LittleEndian.AppendUint16(packet, unit)
	}
	return packet
}

func TestEverythingReplyParsesUnicodeDirectoryAndRejectsMalformedPackets(t *testing.T) {
	results, err := parseEverythingReply(testEverythingReply(), 25)
	require.NoError(t, err)
	assert.Equal(t, []Result{{Path: filepath.Join("X:", "work", "報告"), Directory: true}}, results)

	for _, mutate := range []func([]byte) []byte{
		func(packet []byte) []byte { return packet[:10] },
		func(packet []byte) []byte { binary.LittleEndian.PutUint32(packet[4:], 100); return packet },
		func(packet []byte) []byte { binary.LittleEndian.PutUint32(packet[24:], 1); return packet },
		func(packet []byte) []byte { binary.LittleEndian.PutUint32(packet[24:], 99999); return packet },
		func(packet []byte) []byte { return packet[:len(packet)-2] },
	} {
		_, parseErr := parseEverythingReply(mutate(testEverythingReply()), 25)
		require.Error(t, parseErr)
	}
}

func TestDirectEverythingDoesNotLaunchFallbackOnSuccessOrCancellation(t *testing.T) {
	for _, directErr := range []error{nil, context.Canceled} {
		fallbackCalled := false
		search := withDirectEverything(func(context.Context, string, int) ([]Result, error) {
			return []Result{{Path: "found.txt"}}, directErr
		}, func(context.Context, string, string, int) Response {
			fallbackCalled = true
			return Response{}
		})
		response := search(context.Background(), "report", t.TempDir(), 10)
		assert.False(t, fallbackCalled)
		assert.Equal(t, ProviderEverything, response.Provider)
		if directErr != nil {
			require.ErrorIs(t, response.Err, directErr)
		} else {
			require.NoError(t, response.Err)
			assert.Equal(t, "found.txt", response.Results[0].Path)
		}
	}
}

func TestDirectEverythingFallsBackOnUnavailableAndPreservesFilteredQueries(t *testing.T) {
	for _, query := range []string{"report", "report type:file", `content:"needle"`} {
		fallbackCalled := false
		search := withDirectEverything(func(context.Context, string, int) ([]Result, error) {
			return nil, errors.New("not running")
		}, func(_ context.Context, gotQuery, root string, limit int) Response {
			fallbackCalled = true
			assert.Equal(t, query, gotQuery)
			return Response{Provider: ProviderLocal}
		})
		assert.Equal(t, ProviderLocal, search(context.Background(), query, t.TempDir(), 10).Provider)
		assert.True(t, fallbackCalled)
	}
}
