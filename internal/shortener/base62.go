package shortener

import (
	"errors"
	"fmt"
	"math"
)

const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

const base = uint64(len(alphabet))

var (
	ErrInvalidCode = errors.New("invalid base62 code")

	ErrOverflow = errors.New("base62 value overflows uint64")
)

var decodeTable = func() [256]int8 {
	var t [256]int8
	for i := range t {
		t[i] = -1
	}
	for i := 0; i < len(alphabet); i++ {
		t[alphabet[i]] = int8(i)
	}
	return t
}()

func Encode(n uint64) string {
	if n == 0 {
		return alphabet[:1]
	}

	var buf [11]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = alphabet[n%base]
		n /= base
	}
	return string(buf[i:])
}

func Decode(s string) (uint64, error) {
	if s == "" {
		return 0, ErrInvalidCode
	}
	var n uint64
	for i := 0; i < len(s); i++ {
		v := decodeTable[s[i]]
		if v < 0 {
			return 0, fmt.Errorf("%w: unexpected character %q", ErrInvalidCode, s[i])
		}
		if n > (math.MaxUint64-uint64(v))/base {
			return 0, ErrOverflow
		}
		n = n*base + uint64(v)
	}
	return n, nil
}

func IsBase62(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if decodeTable[s[i]] < 0 {
			return false
		}
	}
	return true
}
