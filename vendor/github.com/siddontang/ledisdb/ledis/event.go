package ledis

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/siddontang/go/hack"
)

var errInvalidEvent = errors.New("invalid event")

func formatEventKey(buf []byte, k []byte) ([]byte, error) {
	if len(k) < 2 {
		return nil, errInvalidEvent
	}

	buf = append(buf, fmt.Sprintf("DB:%2d ", k[0])...)
	buf = append(buf, fmt.Sprintf("%s ", TypeName[k[1]])...)

	db := new(DB)
	index, _, err := decodeDBIndex(k)
	if err != nil {
		return nil, err
	}
	db.setIndex(index)

	//to do format at respective place

	switch k[1] {
	case KVType:
		key, err := db.decodeKVKey(k)
		if err != nil {
			return nil, err
		}
		buf = strconv.AppendQuote(buf, hack.String(key))
	case HashType:
		key, field, err := db.hDecodeHashKey(k)
		if err != nil {
			return nil, err
		}

		buf = strconv.AppendQuote(buf, hack.String(key))
		buf = append(buf, ' ')
		buf = strconv.AppendQuote(buf, hack.String(field))
	case HSizeType:
		key, err := db.hDecodeSizeKey(k)
		if err != nil {
			return nil, err
		}

		buf = strconv.AppendQuote(buf, hack.String(key))
	case ListType:
		key, seq, err := db.lDecodeListKey(k)
		if err != nil {
			return nil, err
		}

		buf = strconv.AppendQuote(buf, hack.String(key))
		buf = append(buf, ' ')
		buf = strconv.AppendInt(buf, int64(seq), 10)
	case LMetaType:
		key, err := db.lDecodeMetaKey(k)
		if err != nil {
			return nil, err
		}

		buf = strconv.AppendQuote(buf, hack.String(key))
	case ZSetType:
		key, m, err := db.zDecodeSetKey(k)
		if err != nil {
			return nil, err
		}

		buf = strconv.AppendQuote(buf, hack.String(key))
		buf = append(buf, ' ')
		buf = strconv.AppendQuote(buf, hack.String(m))
	case ZSizeType:
		key, err := db.zDecodeSizeKey(k)
		if err != nil {
			return nil, err
		}

		buf = strconv.AppendQuote(buf, hack.String(key))
	case ZScoreType:
		key, m, score, err := db.zDecodeScoreKey(k)
		if err != nil {
			return nil, err
		}
		buf = strconv.AppendQuote(buf, hack.String(key))
		buf = append(buf, ' ')
		buf = strconv.AppendQuote(buf, hack.String(m))
		buf = append(buf, ' ')
		buf = strconv.AppendInt(buf, score, 10)
	case SetType:
		key, member, err := db.sDecodeSetKey(k)
		if err != nil {
			return nil, err
		}

		buf = strconv.AppendQuote(buf, hack.String(key))
		buf = append(buf, ' ')
		buf = strconv.AppendQuote(buf, hack.String(member))
	case SSizeType:
		key, err := db.sDecodeSizeKey(k)
		if err != nil {
			return nil, err
		}

		buf = strconv.AppendQuote(buf, hack.String(key))
	case ExpTimeType:
		tp, key, t, err := db.expDecodeTimeKey(k)
		if err != nil {
			return nil, err
		}

		buf = append(buf, TypeName[tp]...)
		buf = append(buf, ' ')
		buf = strconv.AppendQuote(buf, hack.String(key))
		buf = append(buf, ' ')
		buf = strconv.AppendInt(buf, t, 10)
	case ExpMetaType:
		tp, key, err := db.expDecodeMetaKey(k)
		if err != nil {
			return nil, err
		}

		buf = append(buf, TypeName[tp]...)
		buf = append(buf, ' ')
		buf = strconv.AppendQuote(buf, hack.String(key))
	default:
		return nil, errInvalidEvent
	}

	return buf, nil
}
