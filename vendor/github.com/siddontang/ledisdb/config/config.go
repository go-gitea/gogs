package config

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"sync"

	"fmt"

	"github.com/pelletier/go-toml"
	"github.com/siddontang/go/ioutil2"
)

var (
	ErrNoConfigFile = errors.New("Running without a config file")
)

const (
	DefaultAddr string = "127.0.0.1:6380"

	DefaultDBName string = "goleveldb"

	DefaultDataDir string = "./var"

	KB int = 1024
	MB int = KB * 1024
	GB int = MB * 1024
)

type LevelDBConfig struct {
	Compression     bool `toml:"compression"`
	BlockSize       int  `toml:"block_size"`
	WriteBufferSize int  `toml:"write_buffer_size"`
	CacheSize       int  `toml:"cache_size"`
	MaxOpenFiles    int  `toml:"max_open_files"`
	MaxFileSize     int  `toml:"max_file_size"`
}

type RocksDBConfig struct {
	Compression                    int  `toml:"compression"`
	BlockSize                      int  `toml:"block_size"`
	WriteBufferSize                int  `toml:"write_buffer_size"`
	CacheSize                      int  `toml:"cache_size"`
	MaxOpenFiles                   int  `toml:"max_open_files"`
	MaxWriteBufferNum              int  `toml:"max_write_buffer_num"`
	MinWriteBufferNumberToMerge    int  `toml:"min_write_buffer_number_to_merge"`
	NumLevels                      int  `toml:"num_levels"`
	Level0FileNumCompactionTrigger int  `toml:"level0_file_num_compaction_trigger"`
	Level0SlowdownWritesTrigger    int  `toml:"level0_slowdown_writes_trigger"`
	Level0StopWritesTrigger        int  `toml:"level0_stop_writes_trigger"`
	TargetFileSizeBase             int  `toml:"target_file_size_base"`
	TargetFileSizeMultiplier       int  `toml:"target_file_size_multiplier"`
	MaxBytesForLevelBase           int  `toml:"max_bytes_for_level_base"`
	MaxBytesForLevelMultiplier     int  `toml:"max_bytes_for_level_multiplier"`
	DisableAutoCompactions         bool `toml:"disable_auto_compactions"`
	UseFsync                       bool `toml:"use_fsync"`
	MaxBackgroundCompactions       int  `toml:"max_background_compactions"`
	MaxBackgroundFlushes           int  `toml:"max_background_flushes"`
	EnableStatistics               bool `toml:"enable_statistics"`
	StatsDumpPeriodSec             int  `toml:"stats_dump_period_sec"`
	BackgroundThreads              int  `toml:"background_theads"`
	HighPriorityBackgroundThreads  int  `toml:"high_priority_background_threads"`
	DisableWAL                     bool `toml:"disable_wal"`
	MaxManifestFileSize            int  `toml:"max_manifest_file_size"`
}

type LMDBConfig struct {
	MapSize int  `toml:"map_size"`
	NoSync  bool `toml:"nosync"`
}

type ReplicationConfig struct {
	Path             string `toml:"path"`
	Sync             bool   `toml:"sync"`
	WaitSyncTime     int    `toml:"wait_sync_time"`
	WaitMaxSlaveAcks int    `toml:"wait_max_slave_acks"`
	ExpiredLogDays   int    `toml:"expired_log_days"`
	StoreName        string `toml:"store_name"`
	MaxLogFileSize   int64  `toml:"max_log_file_size"`
	MaxLogFileNum    int    `toml:"max_log_file_num"`
	SyncLog          int    `toml:"sync_log"`
	Compression      bool   `toml:"compression"`
	UseMmap          bool   `toml:"use_mmap"`
	MasterPassword   string `toml:"master_password"`
}

type SnapshotConfig struct {
	Path   string `toml:"path"`
	MaxNum int    `toml:"max_num"`
}

type TLS struct {
	Enabled     bool   `toml:"enabled"`
	Certificate string `toml:"certificate"`
	Key         string `toml:"key"`
}

type AuthMethod func(c *Config, password string) bool

type Config struct {
	m sync.RWMutex `toml:"-"`

	AuthPassword string `toml:"auth_password"`

	//AuthMethod custom authentication method
	AuthMethod AuthMethod `toml:"-"`

	FileName string `toml:"-"`

	// Addr can be empty to assign a local address dynamically
	Addr string `toml:"addr"`

	AddrUnixSocketPerm string `toml:"addr_unixsocketperm"`

	HttpAddr string `toml:"http_addr"`

	SlaveOf string `toml:"slaveof"`

	Readonly bool `toml:"readonly"`

	DataDir string `toml:"data_dir"`

	Databases int `toml:"databases"`

	DBName       string `toml:"db_name"`
	DBPath       string `toml:"db_path"`
	DBSyncCommit int    `toml:"db_sync_commit"`

	LevelDB LevelDBConfig `toml:"leveldb"`
	RocksDB RocksDBConfig `toml:"rocksdb"`

	LMDB LMDBConfig `toml:"lmdb"`

	AccessLog string `toml:"access_log"`

	UseReplication bool              `toml:"use_replication"`
	Replication    ReplicationConfig `toml:"replication"`

	Snapshot SnapshotConfig `toml:"snapshot"`

	ConnReadBufferSize    int `toml:"conn_read_buffer_size"`
	ConnWriteBufferSize   int `toml:"conn_write_buffer_size"`
	ConnKeepaliveInterval int `toml:"conn_keepalive_interval"`

	TTLCheckInterval int `toml:"ttl_check_interval"`

	//tls config
	TLS TLS `toml:"tls"`
}

func NewConfigWithFile(fileName string) (*Config, error) {
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	cfg, err := NewConfigWithData(data)
	if err != nil {
		return nil, err
	}

	cfg.FileName = fileName
	return cfg, nil
}

func NewConfigWithData(data []byte) (*Config, error) {
	cfg := NewConfigDefault()

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("newConfigwithData: unmarashal: %s", err)
	}

	cfg.adjust()

	return cfg, nil
}

func NewConfigDefault() *Config {
	cfg := new(Config)

	cfg.Addr = DefaultAddr
	cfg.HttpAddr = ""

	cfg.DataDir = DefaultDataDir

	cfg.DBName = DefaultDBName

	cfg.SlaveOf = ""
	cfg.Readonly = false

	// Disable Auth by default, by setting password to blank
	cfg.AuthPassword = ""

	// default databases number
	cfg.Databases = 16

	// disable access log
	cfg.AccessLog = ""

	cfg.LMDB.MapSize = 20 * MB
	cfg.LMDB.NoSync = true

	cfg.UseReplication = false
	cfg.Replication.WaitSyncTime = 500
	cfg.Replication.Compression = true
	cfg.Replication.WaitMaxSlaveAcks = 2
	cfg.Replication.SyncLog = 0
	cfg.Replication.UseMmap = true
	cfg.Snapshot.MaxNum = 1

	cfg.RocksDB.EnableStatistics = false
	cfg.RocksDB.UseFsync = false
	cfg.RocksDB.DisableAutoCompactions = false
	cfg.RocksDB.DisableWAL = false

	cfg.adjust()

	return cfg
}

func getDefault(d int, s int) int {
	if s <= 0 {
		return d
	}

	return s
}

func (cfg *Config) adjust() {
	cfg.LevelDB.adjust()

	cfg.RocksDB.adjust()

	cfg.Replication.ExpiredLogDays = getDefault(7, cfg.Replication.ExpiredLogDays)
	cfg.Replication.MaxLogFileNum = getDefault(50, cfg.Replication.MaxLogFileNum)
	cfg.ConnReadBufferSize = getDefault(4*KB, cfg.ConnReadBufferSize)
	cfg.ConnWriteBufferSize = getDefault(4*KB, cfg.ConnWriteBufferSize)
	cfg.TTLCheckInterval = getDefault(1, cfg.TTLCheckInterval)
	cfg.Databases = getDefault(16, cfg.Databases)
}

func (cfg *LevelDBConfig) adjust() {
	cfg.CacheSize = getDefault(4*MB, cfg.CacheSize)
	cfg.BlockSize = getDefault(4*KB, cfg.BlockSize)
	cfg.WriteBufferSize = getDefault(4*MB, cfg.WriteBufferSize)
	cfg.MaxOpenFiles = getDefault(1024, cfg.MaxOpenFiles)
	cfg.MaxFileSize = getDefault(32*MB, cfg.MaxFileSize)
}

func (cfg *RocksDBConfig) adjust() {
	cfg.CacheSize = getDefault(4*MB, cfg.CacheSize)
	cfg.BlockSize = getDefault(4*KB, cfg.BlockSize)
	cfg.WriteBufferSize = getDefault(4*MB, cfg.WriteBufferSize)
	cfg.MaxOpenFiles = getDefault(1024, cfg.MaxOpenFiles)
	cfg.MaxWriteBufferNum = getDefault(2, cfg.MaxWriteBufferNum)
	cfg.MinWriteBufferNumberToMerge = getDefault(1, cfg.MinWriteBufferNumberToMerge)
	cfg.NumLevels = getDefault(7, cfg.NumLevels)
	cfg.Level0FileNumCompactionTrigger = getDefault(4, cfg.Level0FileNumCompactionTrigger)
	cfg.Level0SlowdownWritesTrigger = getDefault(16, cfg.Level0SlowdownWritesTrigger)
	cfg.Level0StopWritesTrigger = getDefault(64, cfg.Level0StopWritesTrigger)
	cfg.TargetFileSizeBase = getDefault(32*MB, cfg.TargetFileSizeBase)
	cfg.TargetFileSizeMultiplier = getDefault(1, cfg.TargetFileSizeMultiplier)
	cfg.MaxBytesForLevelBase = getDefault(32*MB, cfg.MaxBytesForLevelBase)
	cfg.MaxBytesForLevelMultiplier = getDefault(1, cfg.MaxBytesForLevelMultiplier)
	cfg.MaxBackgroundCompactions = getDefault(1, cfg.MaxBackgroundCompactions)
	cfg.MaxBackgroundFlushes = getDefault(1, cfg.MaxBackgroundFlushes)
	cfg.StatsDumpPeriodSec = getDefault(3600, cfg.StatsDumpPeriodSec)
	cfg.BackgroundThreads = getDefault(2, cfg.BackgroundThreads)
	cfg.HighPriorityBackgroundThreads = getDefault(1, cfg.HighPriorityBackgroundThreads)
	cfg.MaxManifestFileSize = getDefault(20*MB, cfg.MaxManifestFileSize)
}

func (cfg *Config) Dump(w io.Writer) error {
	data, err := toml.Marshal(*cfg)
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}

	return nil
}

func (cfg *Config) DumpFile(fileName string) error {
	var b bytes.Buffer

	if err := cfg.Dump(&b); err != nil {
		return err
	}

	return ioutil2.WriteFileAtomic(fileName, b.Bytes(), 0644)
}

func (cfg *Config) Rewrite() error {
	if len(cfg.FileName) == 0 {
		return ErrNoConfigFile
	}

	return cfg.DumpFile(cfg.FileName)
}

func (cfg *Config) GetReadonly() bool {
	cfg.m.RLock()
	b := cfg.Readonly
	cfg.m.RUnlock()
	return b
}

func (cfg *Config) SetReadonly(b bool) {
	cfg.m.Lock()
	cfg.Readonly = b
	cfg.m.Unlock()
}
