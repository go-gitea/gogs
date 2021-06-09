// +build !race

package json

import "unsafe"

func decodeCompileToGetDecoder(typ *rtype) (decoder, error) {
	typeptr := uintptr(unsafe.Pointer(typ))
	if typeptr > maxTypeAddr {
		return decodeCompileToGetDecoderSlowPath(typeptr, typ)
	}

	index := (typeptr - baseTypeAddr) >> typeAddrShift
	if dec := cachedDecoder[index]; dec != nil {
		return dec, nil
	}

	dec, err := decodeCompileHead(typ, map[uintptr]decoder{})
	if err != nil {
		return nil, err
	}
	cachedDecoder[index] = dec
	return dec, nil
}
