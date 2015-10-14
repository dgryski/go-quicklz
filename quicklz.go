// Package quicklz implements QuickLZ compress
/*
Translation of http://www.quicklz.com/QuickLZ.java

Licensed under the GPL, like the original.
*/
package quicklz

import "errors"

// The QuickLZ protocol version
const (
	VersionMajor    = 1
	VersionMinor    = 5
	VersionRevision = 0
)

const (
	// Decrease pointers3 to increase compression speed of level 3. Do not
	// edit any other constants!
	hashValues            = 4096
	minOffset             = 2
	unconditionalMatchLen = 6
	uncompressedEnd       = 4
	cwordLen              = 4
	defaultHeaderLen      = 9
	pointers1             = 1
	pointers3             = 16
)

func headerLen(source []byte) int {
	if (source[0] & 2) == 2 {
		return 9
	}
	return 3
}

func sizeDecompressed(source []byte) (int, error) {
	if headerLen(source) == 9 {
		return fastRead(source, 5, 4)
	}
	return fastRead(source, 2, 1)

}

func sizeCompressed(source []byte) (int, error) {
	if headerLen(source) == 9 {
		return fastRead(source, 1, 4)
	}
	return fastRead(source, 1, 1)
}

func fastRead(a []byte, i, numbytes int) (int, error) {
	l := 0
	if len(a) < i+numbytes {
		return 0, ErrCorrupt
	}
	for j := 0; j < numbytes; j++ {
		l |= int(a[i+j]) << (uint(j) * 8)
	}
	return l, nil
}

func fastWrite(a []byte, i, value, numbytes int) {
	for j := 0; j < numbytes; j++ {
		a[i+j] = byte(value >> (uint(j) * 8))
	}
}

func writeHeader(dst []byte, level int, compressible bool, sizeCompressed int, sizeDecompressed int) {
	var cbit byte
	if compressible {
		cbit = 1
	}
	dst[0] = byte(2 | cbit)
	dst[0] |= byte(level << 2)
	dst[0] |= (1 << 6)
	dst[0] |= (0 << 4)
	fastWrite(dst, 1, sizeDecompressed, 4)
	fastWrite(dst, 5, sizeCompressed, 4)
}

// Compress compresses a byte slice.  Valid levels are 1 and 3.
func Compress(source []byte, level int) []byte {
	var src int
	var dst = defaultHeaderLen + cwordLen
	var cwordVal uint32 = 0x80000000
	var cwordPtr = defaultHeaderLen
	var destination = make([]byte, len(source)+400)
	var hashtable [][]int
	var cachetable = make([]int, hashValues)
	var hashCounter = make([]byte, hashValues)
	var d2 []byte
	var fetch = 0
	var lastMatchStart = (len(source) - unconditionalMatchLen - uncompressedEnd - 1)
	var lits = 0

	if level != 1 && level != 3 {
		panic("Go version only supports level 1 and 3")
	}

	hashtable = make([][]int, hashValues)

	hpointers := pointers1
	if level == 3 {
		hpointers = pointers3
	}

	for i := 0; i < hashValues; i++ {
		hashtable[i] = make([]int, hpointers)
	}

	if len(source) == 0 {
		return nil
	}

	if src <= lastMatchStart {
		fetch, _ = fastRead(source, src, 3)
	}

	for src <= lastMatchStart {
		if (cwordVal & 1) == 1 {
			if src > 3*(len(source)>>2) && dst > src-(src>>5) {
				d2 = make([]byte, len(source)+defaultHeaderLen)
				writeHeader(d2, level, false, len(source), len(source)+defaultHeaderLen)
				copy(d2[defaultHeaderLen:], source)
				return d2
			}

			fastWrite(destination, cwordPtr, int(cwordVal>>1)|0x80000000, 4)
			cwordPtr = dst
			dst += cwordLen
			cwordVal = 0x80000000
		}

		if level == 1 {
			hash := ((fetch >> 12) ^ fetch) & (hashValues - 1)
			o := hashtable[hash][0]
			cache := cachetable[hash] ^ fetch

			cachetable[hash] = fetch
			hashtable[hash][0] = src

			if cache == 0 && hashCounter[hash] != 0 && (src-o > minOffset || (src == o+1 && lits >= 3 && src > 3 && source[src] == source[src-3] && source[src] == source[src-2] && source[src] == source[src-1] && source[src] == source[src+1] && source[src] == source[src+2])) {
				cwordVal = ((cwordVal >> 1) | 0x80000000)
				if source[o+3] != source[src+3] {
					f := 3 - 2 | (hash << 4)
					destination[dst+0] = byte(f >> (0 * 8))
					destination[dst+1] = byte(f >> (1 * 8))
					src += 3
					dst += 2
				} else {
					oldSrc := src
					remaining := 255
					if ln := (len(source) - uncompressedEnd - src + 1 - 1); ln <= 255 {
						remaining = ln
					}

					src += 4
					if source[o+src-oldSrc] == source[src] {
						src++
						if source[o+src-oldSrc] == source[src] {
							src++
							for source[o+(src-oldSrc)] == source[src] && (src-oldSrc) < remaining {
								src++
							}
						}
					}

					matchlen := src - oldSrc

					hash <<= 4
					if matchlen < 18 {
						f := hash | (matchlen - 2)
						// Inline fastWrite
						destination[dst+0] = byte(f >> (0 * 8))
						destination[dst+1] = byte(f >> (1 * 8))
						dst += 2
					} else {
						f := hash | (matchlen << 16)
						fastWrite(destination, dst, f, 3)
						dst += 3
					}
				}
				lits = 0
				fetch, _ = fastRead(source, src, 3)
			} else {
				lits++
				hashCounter[hash] = 1
				destination[dst] = source[src]
				cwordVal = (cwordVal >> 1)
				src++
				dst++
				fetch = (fetch>>8)&0xffff | int(source[src+2])<<16
			}
		} else {
			fetch, _ = fastRead(source, src, 3)

			var o, offset2 int
			var matchlen, k, m int
			var c byte

			remaining := 255
			if ln := (len(source) - uncompressedEnd - src + 1 - 1); ln <= 255 {
				remaining = ln
			}

			hash := ((fetch >> 12) ^ fetch) & (hashValues - 1)

			c = hashCounter[hash]
			matchlen = 0
			offset2 = 0
			for k = 0; k < pointers3 && (int(c) > k || c < 0); k++ {

				o = hashtable[hash][k]
				if byte(fetch) == source[o] && byte(fetch>>8) == source[o+1] && byte(fetch>>16) == source[o+2] && o < src-minOffset {
					m = 3
					for source[o+m] == source[src+m] && m < remaining {
						m++
					}
					if (m > matchlen) || (m == matchlen && o > offset2) {
						offset2 = o
						matchlen = m
					}
				}
			}

			o = offset2
			hashtable[hash][c&(pointers3-1)] = src
			c++
			hashCounter[hash] = c

			if matchlen >= 3 && src-o < 131071 {
				offset := src - o
				for u := 1; u < matchlen; u++ {
					fetch, _ = fastRead(source, src+u, 3)
					hash = ((fetch >> 12) ^ fetch) & (hashValues - 1)
					c = hashCounter[hash]
					hashCounter[hash]++
					hashtable[hash][c&(pointers3-1)] = src + u
				}

				src += matchlen
				cwordVal = ((cwordVal >> 1) | 0x80000000)

				if matchlen == 3 && offset <= 63 {
					fastWrite(destination, dst, offset<<2, 1)
					dst++
				} else if matchlen == 3 && offset <= 16383 {
					fastWrite(destination, dst, (offset<<2)|1, 2)
					dst += 2
				} else if matchlen <= 18 && offset <= 1023 {
					fastWrite(destination, dst, ((matchlen-3)<<2)|(offset<<6)|2, 2)
					dst += 2
				} else if matchlen <= 33 {
					fastWrite(destination, dst, ((matchlen-2)<<2)|(offset<<7)|3, 3)
					dst += 3
				} else {
					fastWrite(destination, dst, ((matchlen-3)<<7)|(offset<<15)|3, 4)
					dst += 4
				}
			} else {
				destination[dst] = source[src]
				cwordVal = (cwordVal >> 1)
				src++
				dst++
			}
		}
	}

	for src <= len(source)-1 {
		if (cwordVal & 1) == 1 {
			fastWrite(destination, cwordPtr, int(cwordVal>>1)|0x80000000, 4)
			cwordPtr = dst
			dst += cwordLen
			cwordVal = 0x80000000
		}

		destination[dst] = source[src]
		src++
		dst++
		cwordVal = (cwordVal >> 1)
	}
	for (cwordVal & 1) != 1 {
		cwordVal = (cwordVal >> 1)
	}
	fastWrite(destination, cwordPtr, int(cwordVal>>1)|0x80000000, cwordLen)
	writeHeader(destination, level, true, len(source), dst)

	d2 = make([]byte, dst)
	copy(d2, destination)
	return d2
}

var (
	// ErrCorrupt indicates the input was corrupted
	ErrCorrupt = errors.New("quicklz: corrupt document")
	// ErrInvalidVersion indicates the compression version / level is not supported
	ErrInvalidVersion = errors.New("quicklz: unsupported compression version")
)

// Decompress decompresses a compressed byte slice.
func Decompress(source []byte) ([]byte, error) {
	size, err := sizeDecompressed(source)
	if err != nil || size < 0 {
		return nil, ErrCorrupt
	}
	src := headerLen(source)
	var dst int
	var cwordVal = 1
	destination := make([]byte, size)
	hashtable := make([]int, 4096)
	hashCounter := make([]byte, 4096)
	lastMatchStart := size - unconditionalMatchLen - uncompressedEnd - 1
	lastHashed := -1
	var hash int
	var fetch int

	level := (source[0] >> 2) & 0x3

	if level != 1 && level != 3 {
		return nil, ErrInvalidVersion
	}

	if (source[0] & 1) != 1 {
		d2 := make([]byte, size)
		l := headerLen(source)
		if len(source) < l {
			return nil, ErrCorrupt
		}
		copy(d2, source[l:])
		return d2, nil
	}

	for {
		if cwordVal == 1 {
			var err error
			cwordVal, err = fastRead(source, src, 4)
			if err != nil {
				return nil, ErrCorrupt
			}
			src += 4
			if dst <= lastMatchStart {
				if level == 1 {
					fetch, err = fastRead(source, src, 3)
				} else {
					fetch, err = fastRead(source, src, 4)
				}
				if err != nil {
					return nil, ErrCorrupt
				}
			}
		}

		if (cwordVal & 1) == 1 {
			var matchlen int
			var offset2 int

			cwordVal = cwordVal >> 1

			if level == 1 {
				hash = (fetch >> 4) & 0xfff
				offset2 = hashtable[hash]

				if (fetch & 0xf) != 0 {
					matchlen = (fetch & 0xf) + 2
					src += 2
				} else {
					if len(source) <= src+2 {
						return nil, ErrCorrupt
					}
					matchlen = int(source[src+2]) & 0xff
					src += 3
				}
			} else {
				var offset int

				if (fetch & 3) == 0 {
					offset = (fetch & 0xff) >> 2
					matchlen = 3
					src++
				} else if (fetch & 2) == 0 {
					offset = (fetch & 0xffff) >> 2
					matchlen = 3
					src += 2
				} else if (fetch & 1) == 0 {
					offset = (fetch & 0xffff) >> 6
					matchlen = ((fetch >> 2) & 15) + 3
					src += 2
				} else if (fetch & 127) != 3 {
					offset = (fetch >> 7) & 0x1ffff
					matchlen = ((fetch >> 2) & 0x1f) + 2
					src += 3
				} else {
					offset = (fetch >> 15)
					matchlen = ((fetch >> 7) & 255) + 3
					src += 4
				}
				offset2 = int(dst - offset)
			}

			if matchlen < 0 || offset2 < 0 || len(destination) <= dst+2 || len(destination) <= offset2+matchlen || len(destination) <= dst+matchlen {
				return nil, ErrCorrupt
			}

			destination[dst+0] = destination[offset2+0]
			destination[dst+1] = destination[offset2+1]
			destination[dst+2] = destination[offset2+2]

			for i := 3; i < matchlen; i++ {
				destination[dst+i] = destination[offset2+i]
			}
			dst += matchlen

			if level == 1 {
				fetch, err = fastRead(destination, lastHashed+1, 3) // destination[lastHashed + 1] | (destination[lastHashed + 2] << 8) | (destination[lastHashed + 3] << 16);
				if err != nil {
					return nil, ErrCorrupt
				}
				for lastHashed < dst-matchlen {
					lastHashed++
					hash = ((fetch >> 12) ^ fetch) & (hashValues - 1)
					hashtable[hash] = lastHashed
					hashCounter[hash] = 1
					if len(destination) <= lastHashed+3 {
						return nil, ErrCorrupt
					}
					fetch = (fetch >> 8 & 0xffff) | (int(destination[lastHashed+3]) << 16)
				}
				fetch, err = fastRead(source, src, 3)
				if err != nil {
					return nil, ErrCorrupt
				}
			} else {
				fetch, err = fastRead(source, src, 4)
				if err != nil {
					return nil, ErrCorrupt
				}
			}
			lastHashed = dst - 1
		} else {
			if dst <= lastMatchStart {
				destination[dst] = source[src]
				dst++
				src++
				cwordVal = cwordVal >> 1

				if level == 1 {
					for lastHashed < dst-3 {
						lastHashed++
						fetch2, err := fastRead(destination, lastHashed, 3)
						if err != nil {
							return nil, ErrCorrupt
						}
						hash = ((fetch2 >> 12) ^ fetch2) & (hashValues - 1)
						hashtable[hash] = lastHashed
						hashCounter[hash] = 1
					}
					if len(source) <= src+2 {
						return nil, ErrCorrupt
					}
					fetch = fetch>>8&0xffff | int(source[src+2])<<16
				} else {
					if len(source) <= src+3 {
						return nil, ErrCorrupt
					}
					fetch = fetch>>8&0xffff | int(source[src+2])<<16 | int(source[src+3])<<24
				}
			} else {
				for dst <= size-1 {
					if cwordVal == 1 {
						src += cwordLen
						cwordVal = 0x80000000
					}

					if len(destination) <= dst || len(source) <= src {
						return nil, ErrCorrupt
					}
					destination[dst] = source[src]
					dst++
					src++
					cwordVal = cwordVal >> 1
				}
				return destination, nil
			}
		}
	}
}
