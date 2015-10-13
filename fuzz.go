// +build gofuzz

package quicklz

func Fuzz(data []byte) int {

	if len(data) < 5 {
		return 0
	}

	level := (data[0] >> 2) & 0x3
	if level != 1 && level != 3 {
		return 0

	}

	ln, _ := sizeDecompressed(data)
	if ln > (1 << 21) {
		return 0
	}

	if _, err := Decompress(data); err != nil {
		return 0
	}

	return 1
}
