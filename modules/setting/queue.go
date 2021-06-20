// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
)

// QueueSettings represent the settings for a queue from the ini
type QueueSettings struct {
	Name             string
	DataDir          string
	QueueLength      int `ini:"LENGTH"`
	BatchLength      int
	ConnectionString string
	Type             string
	Network          string
	Addresses        string
	Password         string
	QueueName        string
	SetName          string
	DBIndex          int
	WrapIfNecessary  bool
	MaxAttempts      int
	Timeout          time.Duration
	Workers          int
	MaxWorkers       int
	BlockTimeout     time.Duration
	BoostTimeout     time.Duration
	BoostWorkers     int
}

// Queue settings
var Queue = QueueSettings{}

// GetQueueSettings returns the queue settings for the appropriately named queue
func GetQueueSettings(name string) QueueSettings {
	q := QueueSettings{}
	sec := Cfg.Section("queue." + name)
	q.Name = name

	// DataDir is not directly inheritable
	q.DataDir = filepath.ToSlash(filepath.Join(Queue.DataDir, "common"))
	// QueueName is not directly inheritable either
	q.QueueName = name + Queue.QueueName
	for _, key := range sec.Keys() {
		switch key.Name() {
		case "DATADIR":
			q.DataDir = key.MustString(q.DataDir)
		case "QUEUE_NAME":
			q.QueueName = key.MustString(q.QueueName)
		case "SET_NAME":
			q.SetName = key.MustString(q.SetName)
		}
	}
	if len(q.SetName) == 0 && len(Queue.SetName) > 0 {
		q.SetName = q.QueueName + Queue.SetName
	}
	if !filepath.IsAbs(q.DataDir) {
		q.DataDir = filepath.ToSlash(filepath.Join(AppDataPath, q.DataDir))
	}
	_, _ = sec.NewKey("DATADIR", q.DataDir)

	// The rest are...
	q.QueueLength = sec.Key("LENGTH").MustInt(Queue.QueueLength)
	q.BatchLength = sec.Key("BATCH_LENGTH").MustInt(Queue.BatchLength)
	q.ConnectionString = sec.Key("CONN_STR").MustString(Queue.ConnectionString)
	q.Type = sec.Key("TYPE").MustString(Queue.Type)
	q.WrapIfNecessary = sec.Key("WRAP_IF_NECESSARY").MustBool(Queue.WrapIfNecessary)
	q.MaxAttempts = sec.Key("MAX_ATTEMPTS").MustInt(Queue.MaxAttempts)
	q.Timeout = sec.Key("TIMEOUT").MustDuration(Queue.Timeout)
	q.Workers = sec.Key("WORKERS").MustInt(Queue.Workers)
	q.MaxWorkers = sec.Key("MAX_WORKERS").MustInt(Queue.MaxWorkers)
	q.BlockTimeout = sec.Key("BLOCK_TIMEOUT").MustDuration(Queue.BlockTimeout)
	q.BoostTimeout = sec.Key("BOOST_TIMEOUT").MustDuration(Queue.BoostTimeout)
	q.BoostWorkers = sec.Key("BOOST_WORKERS").MustInt(Queue.BoostWorkers)

	q.Network, q.Addresses, q.Password, q.DBIndex, _ = ParseQueueConnStr(q.ConnectionString)
	return q
}

// NewQueueService sets up the default settings for Queues
// This is exported for tests to be able to use the queue
func NewQueueService() {
	sec := Cfg.Section("queue")
	Queue.DataDir = filepath.ToSlash(sec.Key("DATADIR").MustString("queues/"))
	if !filepath.IsAbs(Queue.DataDir) {
		Queue.DataDir = filepath.ToSlash(filepath.Join(AppDataPath, Queue.DataDir))
	}
	Queue.QueueLength = sec.Key("LENGTH").MustInt(20)
	Queue.BatchLength = sec.Key("BATCH_LENGTH").MustInt(20)
	Queue.ConnectionString = sec.Key("CONN_STR").MustString("")
	defaultType := sec.Key("TYPE").String()
	Queue.Type = sec.Key("TYPE").MustString("persistable-channel")
	Queue.Network, Queue.Addresses, Queue.Password, Queue.DBIndex, _ = ParseQueueConnStr(Queue.ConnectionString)
	Queue.WrapIfNecessary = sec.Key("WRAP_IF_NECESSARY").MustBool(true)
	Queue.MaxAttempts = sec.Key("MAX_ATTEMPTS").MustInt(10)
	Queue.Timeout = sec.Key("TIMEOUT").MustDuration(GracefulHammerTime + 30*time.Second)
	Queue.Workers = sec.Key("WORKERS").MustInt(0)
	Queue.MaxWorkers = sec.Key("MAX_WORKERS").MustInt(10)
	Queue.BlockTimeout = sec.Key("BLOCK_TIMEOUT").MustDuration(1 * time.Second)
	Queue.BoostTimeout = sec.Key("BOOST_TIMEOUT").MustDuration(5 * time.Minute)
	Queue.BoostWorkers = sec.Key("BOOST_WORKERS").MustInt(1)
	Queue.QueueName = sec.Key("QUEUE_NAME").MustString("_queue")
	Queue.SetName = sec.Key("SET_NAME").MustString("")

	// Now handle the old issue_indexer configuration
	section := Cfg.Section("queue.issue_indexer")
	sectionMap := map[string]bool{}
	for _, key := range section.Keys() {
		sectionMap[key.Name()] = true
	}
	if _, ok := sectionMap["TYPE"]; !ok && defaultType == "" {
		switch Indexer.IssueQueueType {
		case LevelQueueType:
			_, _ = section.NewKey("TYPE", "level")
		case ChannelQueueType:
			_, _ = section.NewKey("TYPE", "persistable-channel")
		case RedisQueueType:
			_, _ = section.NewKey("TYPE", "redis")
		case "":
			_, _ = section.NewKey("TYPE", "level")
		default:
			log.Fatal("Unsupported indexer queue type: %v",
				Indexer.IssueQueueType)
		}
	}
	if _, ok := sectionMap["LENGTH"]; !ok && Indexer.UpdateQueueLength != 0 {
		_, _ = section.NewKey("LENGTH", fmt.Sprintf("%d", Indexer.UpdateQueueLength))
	}
	if _, ok := sectionMap["BATCH_LENGTH"]; !ok && Indexer.IssueQueueBatchNumber != 0 {
		_, _ = section.NewKey("BATCH_LENGTH", fmt.Sprintf("%d", Indexer.IssueQueueBatchNumber))
	}
	if _, ok := sectionMap["DATADIR"]; !ok && Indexer.IssueQueueDir != "" {
		_, _ = section.NewKey("DATADIR", Indexer.IssueQueueDir)
	}
	if _, ok := sectionMap["CONN_STR"]; !ok && Indexer.IssueQueueConnStr != "" {
		_, _ = section.NewKey("CONN_STR", Indexer.IssueQueueConnStr)
	}

	// Handle the old mailer configuration
	section = Cfg.Section("queue.mailer")
	sectionMap = map[string]bool{}
	for _, key := range section.Keys() {
		sectionMap[key.Name()] = true
	}
	if _, ok := sectionMap["LENGTH"]; !ok {
		_, _ = section.NewKey("LENGTH", fmt.Sprintf("%d", Cfg.Section("mailer").Key("SEND_BUFFER_LEN").MustInt(100)))
	}

	// Handle the old test pull requests configuration
	// Please note this will be a unique queue
	section = Cfg.Section("queue.pr_patch_checker")
	sectionMap = map[string]bool{}
	for _, key := range section.Keys() {
		sectionMap[key.Name()] = true
	}
	if _, ok := sectionMap["LENGTH"]; !ok {
		_, _ = section.NewKey("LENGTH", fmt.Sprintf("%d", Repository.PullRequestQueueLength))
	}
}

// ParseQueueConnStr parses a queue connection string
func ParseQueueConnStr(connStr string) (network, addrs, password string, dbIdx int, err error) {
	fields := strings.Fields(connStr)
	for _, f := range fields {
		items := strings.SplitN(f, "=", 2)
		if len(items) < 2 {
			continue
		}
		switch strings.ToLower(items[0]) {
		case "network":
			network = items[1]
		case "addrs":
			addrs = items[1]
		case "password":
			password = items[1]
		case "db":
			dbIdx, err = strconv.Atoi(items[1])
			if err != nil {
				return
			}
		}
	}
	return
}
