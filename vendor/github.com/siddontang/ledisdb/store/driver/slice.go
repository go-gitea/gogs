package driver

type ISlice interface {
	Data() []byte
	Size() int
	Free()
}

type GoSlice []byte

func (s GoSlice) Data() []byte {
	return []byte(s)
}

func (s GoSlice) Size() int {
	return len(s)
}

func (s GoSlice) Free() {

}
