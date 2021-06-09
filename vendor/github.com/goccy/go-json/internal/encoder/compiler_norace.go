// +build !race

package encoder

import (
	"unsafe"

	"github.com/goccy/go-json/internal/runtime"
)

func CompileToGetCodeSet(typeptr uintptr) (*OpcodeSet, error) {
	if typeptr > typeAddr.MaxTypeAddr {
		return compileToGetCodeSetSlowPath(typeptr)
	}
	index := (typeptr - typeAddr.BaseTypeAddr) >> typeAddr.AddrShift
	if codeSet := cachedOpcodeSets[index]; codeSet != nil {
		return codeSet, nil
	}

	// noescape trick for header.typ ( reflect.*rtype )
	copiedType := *(**runtime.Type)(unsafe.Pointer(&typeptr))

	noescapeKeyCode, err := compileHead(&compileContext{
		typ:                      copiedType,
		structTypeToCompiledCode: map[uintptr]*CompiledCode{},
	})
	if err != nil {
		return nil, err
	}
	escapeKeyCode, err := compileHead(&compileContext{
		typ:                      copiedType,
		structTypeToCompiledCode: map[uintptr]*CompiledCode{},
		escapeKey:                true,
	})
	if err != nil {
		return nil, err
	}
	noescapeKeyCode = copyOpcode(noescapeKeyCode)
	escapeKeyCode = copyOpcode(escapeKeyCode)
	codeLength := noescapeKeyCode.TotalLength()
	codeSet := &OpcodeSet{
		Type:            copiedType,
		NoescapeKeyCode: noescapeKeyCode,
		EscapeKeyCode:   escapeKeyCode,
		CodeLength:      codeLength,
	}
	cachedOpcodeSets[index] = codeSet
	return codeSet, nil
}
