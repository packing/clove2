package base

import "bytes"

func CombineBytes(bs ...[]byte) []byte {
	return bytes.Join(bs, []byte(""))
}
