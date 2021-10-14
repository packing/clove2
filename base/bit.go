package base

func MaskBytes(raw []byte, mask byte, bitSize byte) []byte {
	var bitResvSize = 8 - bitSize
	dstb := make([]byte, len(raw)+1)
	if bitSize > 7 {
		return raw
	}
	var remainBits = mask >> bitResvSize << bitResvSize
	for i, v := range raw {
		newV := v >> bitSize
		if remainBits > 0 {
			newV |= remainBits
		}
		if i > 1 && i < len(raw)-1 {
			newV ^= mask
		}
		remainBits = v << bitResvSize
		dstb[i] = newV
	}

	endb := remainBits | (mask << bitSize >> bitSize)
	dstb[len(dstb)-1] = endb
	return dstb
}

func UnMaskBytes(raw []byte, bitSize byte) (byte, []byte) {
	var bitResvSize = 8 - bitSize
	srcb := make([]byte, len(raw)-1)
	var maskSrc = raw[0] >> bitResvSize << bitResvSize
	maskSrc |= raw[len(raw)-1] << bitSize >> bitSize
	for i, v := range raw {
		unmaskV := v
		if i > 1 && i < (len(raw)-2) {
			unmaskV ^= maskSrc
		}
		if i > 0 {
			srcb[i-1] |= unmaskV >> bitResvSize
		}

		if i == (len(raw) - 1) {
			break
		}
		srcb[i] = unmaskV << bitSize
	}
	return maskSrc, srcb
}

func ReadAsciiCode(raw []byte) byte {
	if len(raw) == 0 {
		return 0
	}
	if raw[0]>>7 != 0 {
		return 0
	}
	return raw[0]
}

func AbsForInt64(n int64) int64 {
	return (n ^ n>>63) - n>>63
}

func AbsForInt32(n int) int {
	return (n ^ n>>32) - n>>32
}
