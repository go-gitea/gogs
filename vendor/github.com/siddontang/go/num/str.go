package num

import (
	"strconv"
)

func ParseUint(s string) (uint, error) {
	if v, err := strconv.ParseUint(s, 10, 0); err != nil {
		return 0, err
	} else {
		return uint(v), nil
	}
}

func ParseUint8(s string) (uint8, error) {
	if v, err := strconv.ParseUint(s, 10, 8); err != nil {
		return 0, err
	} else {
		return uint8(v), nil
	}
}

func ParseUint16(s string) (uint16, error) {
	if v, err := strconv.ParseUint(s, 10, 16); err != nil {
		return 0, err
	} else {
		return uint16(v), nil
	}
}

func ParseUint32(s string) (uint32, error) {
	if v, err := strconv.ParseUint(s, 10, 32); err != nil {
		return 0, err
	} else {
		return uint32(v), nil
	}
}

func ParseUint64(s string) (uint64, error) {
	return strconv.ParseUint(s, 10, 64)
}

func ParseInt(s string) (int, error) {
	if v, err := strconv.ParseInt(s, 10, 0); err != nil {
		return 0, err
	} else {
		return int(v), nil
	}
}

func ParseInt8(s string) (int8, error) {
	if v, err := strconv.ParseInt(s, 10, 8); err != nil {
		return 0, err
	} else {
		return int8(v), nil
	}
}

func ParseInt16(s string) (int16, error) {
	if v, err := strconv.ParseInt(s, 10, 16); err != nil {
		return 0, err
	} else {
		return int16(v), nil
	}
}

func ParseInt32(s string) (int32, error) {
	if v, err := strconv.ParseInt(s, 10, 32); err != nil {
		return 0, err
	} else {
		return int32(v), nil
	}
}

func ParseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func FormatInt(v int) string {
	return strconv.FormatInt(int64(v), 10)
}

func FormatInt8(v int8) string {
	return strconv.FormatInt(int64(v), 10)
}

func FormatInt16(v int16) string {
	return strconv.FormatInt(int64(v), 10)
}

func FormatInt32(v int32) string {
	return strconv.FormatInt(int64(v), 10)
}

func FormatInt64(v int64) string {
	return strconv.FormatInt(int64(v), 10)
}

func FormatUint(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}

func FormatUint8(v uint8) string {
	return strconv.FormatUint(uint64(v), 10)
}

func FormatUint16(v uint16) string {
	return strconv.FormatUint(uint64(v), 10)
}

func FormatUint32(v uint32) string {
	return strconv.FormatUint(uint64(v), 10)
}

func FormatUint64(v uint64) string {
	return strconv.FormatUint(uint64(v), 10)
}

func FormatIntToSlice(v int) []byte {
	return strconv.AppendInt(nil, int64(v), 10)
}

func FormatInt8ToSlice(v int8) []byte {
	return strconv.AppendInt(nil, int64(v), 10)
}

func FormatInt16ToSlice(v int16) []byte {
	return strconv.AppendInt(nil, int64(v), 10)
}

func FormatInt32ToSlice(v int32) []byte {
	return strconv.AppendInt(nil, int64(v), 10)
}

func FormatInt64ToSlice(v int64) []byte {
	return strconv.AppendInt(nil, int64(v), 10)
}

func FormatUintToSlice(v uint) []byte {
	return strconv.AppendUint(nil, uint64(v), 10)
}

func FormatUint8ToSlice(v uint8) []byte {
	return strconv.AppendUint(nil, uint64(v), 10)
}

func FormatUint16ToSlice(v uint16) []byte {
	return strconv.AppendUint(nil, uint64(v), 10)
}

func FormatUint32ToSlice(v uint32) []byte {
	return strconv.AppendUint(nil, uint64(v), 10)
}

func FormatUint64ToSlice(v uint64) []byte {
	return strconv.AppendUint(nil, uint64(v), 10)
}
