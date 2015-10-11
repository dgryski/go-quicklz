package quicklz

import (
	"bytes"
	"encoding/hex"
	"io/ioutil"
	"testing"
)

func TestCompress(t *testing.T) {

	in, _ := ioutil.ReadFile("testdata/alice-10k.txt")

	for i := 1; i < len(in); i++ {

		qz := Compress(in[:i], 1)

		out := Decompress(qz)
		if !bytes.Equal(in[:i], out) {
			offs := dump(t, "o", out, "i", in[:i])
			t.Log("\n" + hex.Dump(qz))
			t.Fatalf("roundtrip mismatch for length %d at offs %x", i, offs)
		}

	}
}

func dump(t *testing.T, s1 string, b1 []byte, s2 string, b2 []byte) int {
	t.Log("\n" + hex.Dump(b1))
	t.Log("\n" + hex.Dump(b2))
	for i, v := range b1 {
		if b2[i] != v {
			return i
		}
	}
	return -1
}
