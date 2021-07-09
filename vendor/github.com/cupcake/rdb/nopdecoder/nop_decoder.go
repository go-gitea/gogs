package nopdecoder

// NopDecoder may be embedded in a real Decoder to avoid implementing methods.
type NopDecoder struct{}

func (d NopDecoder) StartRDB()                                       {}
func (d NopDecoder) StartDatabase(n int)                             {}
func (d NopDecoder) Aux(key, value []byte)                           {}
func (d NopDecoder) ResizeDatabase(dbSize, expiresSize uint32)       {}
func (d NopDecoder) EndDatabase(n int)                               {}
func (d NopDecoder) EndRDB()                                         {}
func (d NopDecoder) Set(key, value []byte, expiry int64)             {}
func (d NopDecoder) StartHash(key []byte, length, expiry int64)      {}
func (d NopDecoder) Hset(key, field, value []byte)                   {}
func (d NopDecoder) EndHash(key []byte)                              {}
func (d NopDecoder) StartSet(key []byte, cardinality, expiry int64)  {}
func (d NopDecoder) Sadd(key, member []byte)                         {}
func (d NopDecoder) EndSet(key []byte)                               {}
func (d NopDecoder) StartList(key []byte, length, expiry int64)      {}
func (d NopDecoder) Rpush(key, value []byte)                         {}
func (d NopDecoder) EndList(key []byte)                              {}
func (d NopDecoder) StartZSet(key []byte, cardinality, expiry int64) {}
func (d NopDecoder) Zadd(key []byte, score float64, member []byte)   {}
func (d NopDecoder) EndZSet(key []byte)                              {}
