// +build rocksdb

package rocksdb

// #cgo LDFLAGS: -lrocksdb
// #include "rocksdb/c.h"
import "C"

type CompressionOpt int

const (
	NoCompression     = CompressionOpt(0)
	SnappyCompression = CompressionOpt(1)
	ZlibCompression   = CompressionOpt(2)
	Bz2Compression    = CompressionOpt(3)
	Lz4Compression    = CompressionOpt(4)
	Lz4hcCompression  = CompressionOpt(5)
)

type Options struct {
	Opt *C.rocksdb_options_t
}

type ReadOptions struct {
	Opt *C.rocksdb_readoptions_t
}

type WriteOptions struct {
	Opt *C.rocksdb_writeoptions_t
}

type BlockBasedTableOptions struct {
	Opt *C.rocksdb_block_based_table_options_t
}

func NewOptions() *Options {
	opt := C.rocksdb_options_create()
	return &Options{opt}
}

func NewReadOptions() *ReadOptions {
	opt := C.rocksdb_readoptions_create()
	return &ReadOptions{opt}
}

func NewWriteOptions() *WriteOptions {
	opt := C.rocksdb_writeoptions_create()
	return &WriteOptions{opt}
}

func NewBlockBasedTableOptions() *BlockBasedTableOptions {
	opt := C.rocksdb_block_based_options_create()
	return &BlockBasedTableOptions{opt}
}

func (o *Options) Close() {
	C.rocksdb_options_destroy(o.Opt)
}

func (o *Options) IncreaseParallelism(n int) {
	C.rocksdb_options_increase_parallelism(o.Opt, C.int(n))
}

func (o *Options) OptimizeLevelStyleCompaction(n int) {
	C.rocksdb_options_optimize_level_style_compaction(o.Opt, C.uint64_t(n))
}

func (o *Options) SetComparator(cmp *C.rocksdb_comparator_t) {
	C.rocksdb_options_set_comparator(o.Opt, cmp)
}

func (o *Options) SetErrorIfExists(error_if_exists bool) {
	eie := boolToUchar(error_if_exists)
	C.rocksdb_options_set_error_if_exists(o.Opt, eie)
}

func (o *Options) SetEnv(env *Env) {
	C.rocksdb_options_set_env(o.Opt, env.Env)
}

func (o *Options) SetWriteBufferSize(s int) {
	C.rocksdb_options_set_write_buffer_size(o.Opt, C.size_t(s))
}

func (o *Options) SetParanoidChecks(pc bool) {
	C.rocksdb_options_set_paranoid_checks(o.Opt, boolToUchar(pc))
}

func (o *Options) SetMaxOpenFiles(n int) {
	C.rocksdb_options_set_max_open_files(o.Opt, C.int(n))
}

func (o *Options) SetCompression(t CompressionOpt) {
	C.rocksdb_options_set_compression(o.Opt, C.int(t))
}

func (o *Options) SetCreateIfMissing(b bool) {
	C.rocksdb_options_set_create_if_missing(o.Opt, boolToUchar(b))
}

func (o *Options) SetMaxWriteBufferNumber(n int) {
	C.rocksdb_options_set_max_write_buffer_number(o.Opt, C.int(n))
}

func (o *Options) SetMaxBackgroundCompactions(n int) {
	C.rocksdb_options_set_max_background_compactions(o.Opt, C.int(n))
}

func (o *Options) SetMaxBackgroundFlushes(n int) {
	C.rocksdb_options_set_max_background_flushes(o.Opt, C.int(n))
}

func (o *Options) SetNumLevels(n int) {
	C.rocksdb_options_set_num_levels(o.Opt, C.int(n))
}

func (o *Options) SetLevel0FileNumCompactionTrigger(n int) {
	C.rocksdb_options_set_level0_file_num_compaction_trigger(o.Opt, C.int(n))
}

func (o *Options) SetLevel0SlowdownWritesTrigger(n int) {
	C.rocksdb_options_set_level0_slowdown_writes_trigger(o.Opt, C.int(n))
}

func (o *Options) SetLevel0StopWritesTrigger(n int) {
	C.rocksdb_options_set_level0_stop_writes_trigger(o.Opt, C.int(n))
}

func (o *Options) SetTargetFileSizeBase(n int) {
	C.rocksdb_options_set_target_file_size_base(o.Opt, C.uint64_t(uint64(n)))
}

func (o *Options) SetTargetFileSizeMultiplier(n int) {
	C.rocksdb_options_set_target_file_size_multiplier(o.Opt, C.int(n))
}

func (o *Options) SetMaxBytesForLevelBase(n int) {
	C.rocksdb_options_set_max_bytes_for_level_base(o.Opt, C.uint64_t(uint64(n)))
}

func (o *Options) SetMaxBytesForLevelMultiplier(n int) {
	C.rocksdb_options_set_max_bytes_for_level_multiplier(o.Opt, C.double(n))
}

func (o *Options) SetBlockBasedTableFactory(opt *BlockBasedTableOptions) {
	C.rocksdb_options_set_block_based_table_factory(o.Opt, opt.Opt)
}

func (o *Options) SetMinWriteBufferNumberToMerge(n int) {
	C.rocksdb_options_set_min_write_buffer_number_to_merge(o.Opt, C.int(n))
}

func (o *Options) DisableAutoCompactions(b bool) {
	C.rocksdb_options_set_disable_auto_compactions(o.Opt, boolToInt(b))
}

func (o *Options) UseFsync(b bool) {
	C.rocksdb_options_set_use_fsync(o.Opt, boolToInt(b))
}

func (o *Options) EnableStatistics(b bool) {
	if b {
		C.rocksdb_options_enable_statistics(o.Opt)
	}
}

func (o *Options) SetStatsDumpPeriodSec(n int) {
	C.rocksdb_options_set_stats_dump_period_sec(o.Opt, C.uint(n))
}

func (o *Options) SetMaxManifestFileSize(n int) {
	C.rocksdb_options_set_max_manifest_file_size(o.Opt, C.size_t(n))
}

func (o *BlockBasedTableOptions) Close() {
	C.rocksdb_block_based_options_destroy(o.Opt)
}

func (o *BlockBasedTableOptions) SetFilterPolicy(fp *FilterPolicy) {
	var policy *C.rocksdb_filterpolicy_t
	if fp != nil {
		policy = fp.Policy
	}
	C.rocksdb_block_based_options_set_filter_policy(o.Opt, policy)
}

func (o *BlockBasedTableOptions) SetBlockSize(s int) {
	C.rocksdb_block_based_options_set_block_size(o.Opt, C.size_t(s))
}

func (o *BlockBasedTableOptions) SetBlockRestartInterval(n int) {
	C.rocksdb_block_based_options_set_block_restart_interval(o.Opt, C.int(n))
}

func (o *BlockBasedTableOptions) SetCache(cache *Cache) {
	C.rocksdb_block_based_options_set_block_cache(o.Opt, cache.Cache)
}

func (ro *ReadOptions) Close() {
	C.rocksdb_readoptions_destroy(ro.Opt)
}

func (ro *ReadOptions) SetVerifyChecksums(b bool) {
	C.rocksdb_readoptions_set_verify_checksums(ro.Opt, boolToUchar(b))
}

func (ro *ReadOptions) SetFillCache(b bool) {
	C.rocksdb_readoptions_set_fill_cache(ro.Opt, boolToUchar(b))
}

func (ro *ReadOptions) SetSnapshot(snap *Snapshot) {
	var s *C.rocksdb_snapshot_t
	if snap != nil {
		s = snap.snap
	}
	C.rocksdb_readoptions_set_snapshot(ro.Opt, s)
}

func (wo *WriteOptions) Close() {
	C.rocksdb_writeoptions_destroy(wo.Opt)
}

func (wo *WriteOptions) SetSync(b bool) {
	C.rocksdb_writeoptions_set_sync(wo.Opt, boolToUchar(b))
}

func (wo *WriteOptions) DisableWAL(b bool) {
	C.rocksdb_writeoptions_disable_WAL(wo.Opt, boolToInt(b))
}
