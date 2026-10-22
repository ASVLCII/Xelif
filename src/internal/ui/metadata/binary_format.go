package metadata

import (
	"debug/macho"
	"debug/pe"
	"encoding/binary"
	"io"
	"os"
)

type binaryFormat uint8

const (
	formatUnknown binaryFormat = iota
	formatELF
	formatPE
	formatMachO
)

// Gate the debug/* parsers on magic bytes before processing arbitrary preview
// inputs. In particular debug/pe accepts raw COFF, so arbitrary video bytes
// can otherwise be interpreted as large section and symbol table counts.
// This follows the upstream fix in yorukot/superfile#1552.
func detectBinaryFormat(path string) binaryFormat {
	file, err := os.Open(path)
	if err != nil {
		return formatUnknown
	}
	defer file.Close()
	var magic [4]byte
	if _, err := io.ReadFull(file, magic[:]); err != nil {
		return formatUnknown
	}
	switch {
	case magic == [4]byte{0x7f, 'E', 'L', 'F'}:
		return formatELF
	case magic[0] == 'M' && magic[1] == 'Z':
		return formatPE
	case isCOFFMachine(binary.LittleEndian.Uint16(magic[:2])):
		return formatPE
	case isMachOMagic(binary.BigEndian.Uint32(magic[:])):
		return formatMachO
	default:
		return formatUnknown
	}
}

func isCOFFMachine(machine uint16) bool {
	switch machine {
	case pe.IMAGE_FILE_MACHINE_I386, pe.IMAGE_FILE_MACHINE_AMD64,
		pe.IMAGE_FILE_MACHINE_ARM, pe.IMAGE_FILE_MACHINE_ARMNT, pe.IMAGE_FILE_MACHINE_ARM64,
		pe.IMAGE_FILE_MACHINE_RISCV32, pe.IMAGE_FILE_MACHINE_RISCV64, pe.IMAGE_FILE_MACHINE_RISCV128:
		return true
	default:
		return false
	}
}

const (
	machoCigam32  = 0xcefaedfe
	machoCigam64  = 0xcffaedfe
	machoCigamFat = 0xbebafeca
)

func isMachOMagic(magic uint32) bool {
	switch magic {
	case macho.Magic32, macho.Magic64, macho.MagicFat, machoCigam32, machoCigam64, machoCigamFat:
		return true
	default:
		return false
	}
}
