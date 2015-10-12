// +build gofuzz

package quicklz

import "encoding/binary"

func Fuzz(data []byte) int {

	if len(data) < 5 {
		return 0
	}

	level := (data[0] >> 2) & 0x3
	if level != 1 && level != 3 {
		return 0

	}

	ln := binary.LittleEndian.Uint32(data[1:])
	if ln > (1 << 21) {
		return 0
	}

	if b := Decompress(data); b == nil {
		return 0
	}

	return 1
}
