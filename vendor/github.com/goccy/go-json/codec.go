package json

import (
	"fmt"
	"reflect"
	"sync/atomic"
	"unsafe"
)

const (
	maxAcceptableTypeAddrRange = 1024 * 1024 * 2 // 2 Mib
)

var (
	cachedDecoder    []decoder
	cachedDecoderMap unsafe.Pointer // map[uintptr]decoder
	baseTypeAddr     uintptr
	maxTypeAddr      uintptr
	typeAddrShift    uintptr
)

//go:linkname typelinks reflect.typelinks
func typelinks() ([]unsafe.Pointer, [][]int32)

//go:linkname rtypeOff reflect.rtypeOff
func rtypeOff(unsafe.Pointer, int32) unsafe.Pointer

func setupCodec() error {
	sections, offsets := typelinks()
	if len(sections) != 1 {
		return fmt.Errorf("failed to get sections")
	}
	if len(offsets) != 1 {
		return fmt.Errorf("failed to get offsets")
	}
	section := sections[0]
	offset := offsets[0]
	var (
		min         uintptr = uintptr(^uint(0))
		max         uintptr = 0
		isAligned64         = true
		isAligned32         = true
	)
	for i := 0; i < len(offset); i++ {
		typ := (*rtype)(rtypeOff(section, offset[i]))
		addr := uintptr(unsafe.Pointer(typ))
		if min > addr {
			min = addr
		}
		if max < addr {
			max = addr
		}
		if typ.Kind() == reflect.Ptr {
			addr = uintptr(unsafe.Pointer(typ.Elem()))
			if min > addr {
				min = addr
			}
			if max < addr {
				max = addr
			}
		}

		// check every address is aligned from the base address
		isAligned64 = isAligned64 && (addr-min)&63 == 0
		isAligned32 = isAligned32 && (addr-min)&31 == 0
	}
	addrRange := max - min
	if addrRange == 0 {
		return fmt.Errorf("failed to get address range of types")
	}
	if isAligned64 {
		typeAddrShift = 6
	} else if isAligned32 {
		typeAddrShift = 5
	}
	cacheSize := addrRange >> typeAddrShift
	if cacheSize > maxAcceptableTypeAddrRange {
		return fmt.Errorf("too big address range %d", addrRange)
	}
	cachedDecoder = make([]decoder, cacheSize)
	baseTypeAddr = min
	maxTypeAddr = max
	return nil
}

func init() {
	_ = setupCodec()
}

func loadDecoderMap() map[uintptr]decoder {
	p := atomic.LoadPointer(&cachedDecoderMap)
	return *(*map[uintptr]decoder)(unsafe.Pointer(&p))
}

func storeDecoder(typ uintptr, dec decoder, m map[uintptr]decoder) {
	newDecoderMap := make(map[uintptr]decoder, len(m)+1)
	newDecoderMap[typ] = dec

	for k, v := range m {
		newDecoderMap[k] = v
	}

	atomic.StorePointer(&cachedDecoderMap, *(*unsafe.Pointer)(unsafe.Pointer(&newDecoderMap)))
}
