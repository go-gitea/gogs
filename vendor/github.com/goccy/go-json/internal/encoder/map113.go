// +build go1.13

package encoder

import "unsafe"

//go:linkname MapIterValue reflect.mapiterelem
func MapIterValue(it unsafe.Pointer) unsafe.Pointer
