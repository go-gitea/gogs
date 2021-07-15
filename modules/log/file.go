// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/util"
	jsoniter "github.com/json-iterator/go"
)

// FileLogger implements LoggerProvider.
// It writes messages by lines limit, file size limit, or time frequency.
type FileLogger struct {
	WriterLogger
	mw *MuxWriter
	// The opened file
	Filename string `json:"filename"`

	// Rotate at size
	Maxsize        int `json:"maxsize"`
	maxsizeCursize int

	// Rotate daily
	Daily         bool  `json:"daily"`
	Maxdays       int64 `json:"maxdays"`
	dailyOpenDate int

	Rotate bool `json:"rotate"`

	Compress         bool `json:"compress"`
	CompressionLevel int  `json:"compressionLevel"`

	startLock sync.Mutex // Only one log can write to the file
}

// MuxWriter an *os.File writer with locker.
type MuxWriter struct {
	mu    sync.Mutex
	fd    *os.File
	owner *FileLogger
}

// Write writes to os.File.
func (mw *MuxWriter) Write(b []byte) (int, error) {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	mw.owner.docheck(len(b))
	return mw.fd.Write(b)
}

// Close the internal writer
func (mw *MuxWriter) Close() error {
	return mw.fd.Close()
}

// SetFd sets os.File in writer.
func (mw *MuxWriter) SetFd(fd *os.File) {
	if mw.fd != nil {
		mw.fd.Close()
	}
	mw.fd = fd
}

// NewFileLogger create a FileLogger returning as LoggerProvider.
func NewFileLogger() LoggerProvider {
	log := &FileLogger{
		Filename:         "",
		Maxsize:          1 << 28, //256 MB
		Daily:            true,
		Maxdays:          7,
		Rotate:           true,
		Compress:         true,
		CompressionLevel: gzip.DefaultCompression,
	}
	log.Level = TRACE
	// use MuxWriter instead direct use os.File for lock write when rotate
	log.mw = new(MuxWriter)
	log.mw.owner = log

	return log
}

// Init file logger with json config.
// config like:
//	{
//	"filename":"log/gogs.log",
//	"maxsize":1<<30,
//	"daily":true,
//	"maxdays":15,
//	"rotate":true
//	}
func (log *FileLogger) Init(config string) error {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal([]byte(config), log); err != nil {
		return fmt.Errorf("Unable to parse JSON: %v", err)
	}
	if len(log.Filename) == 0 {
		return errors.New("config must have filename")
	}
	// set MuxWriter as Logger's io.Writer
	log.NewWriterLogger(log.mw)
	return log.StartLogger()
}

// StartLogger start file logger. create log file and set to locker-inside file writer.
func (log *FileLogger) StartLogger() error {
	fd, err := log.createLogFile()
	if err != nil {
		return err
	}
	log.mw.SetFd(fd)
	return log.initFd()
}

func (log *FileLogger) docheck(size int) {
	log.startLock.Lock()
	defer log.startLock.Unlock()
	if log.Rotate && ((log.Maxsize > 0 && log.maxsizeCursize >= log.Maxsize) ||
		(log.Daily && time.Now().Day() != log.dailyOpenDate)) {
		if err := log.DoRotate(); err != nil {
			fmt.Fprintf(os.Stderr, "FileLogger(%q): %s\n", log.Filename, err)
			return
		}
	}
	log.maxsizeCursize += size
}

func (log *FileLogger) createLogFile() (*os.File, error) {
	// Open the log file
	return os.OpenFile(log.Filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0660)
}

func (log *FileLogger) initFd() error {
	fd := log.mw.fd
	finfo, err := fd.Stat()
	if err != nil {
		return fmt.Errorf("get stat: %v", err)
	}
	log.maxsizeCursize = int(finfo.Size())
	log.dailyOpenDate = time.Now().Day()
	return nil
}

// DoRotate means it need to write file in new file.
// new file name like xx.log.2013-01-01.2
func (log *FileLogger) DoRotate() error {
	_, err := os.Lstat(log.Filename)
	if err == nil { // file exists
		// Find the next available number
		num := 1
		fname := ""
		for ; err == nil && num <= 999; num++ {
			fname = log.Filename + fmt.Sprintf(".%s.%03d", time.Now().Format("2006-01-02"), num)
			_, err = os.Lstat(fname)
			if log.Compress && err != nil {
				_, err = os.Lstat(fname + ".gz")
			}
		}
		// return error if the last file checked still existed
		if err == nil {
			return fmt.Errorf("rotate: cannot find free log number to rename %s", log.Filename)
		}

		fd := log.mw.fd
		fd.Close()

		// close fd before rename
		// Rename the file to its newfound home
		if err = util.Rename(log.Filename, fname); err != nil {
			return fmt.Errorf("Rotate: %v", err)
		}

		if log.Compress {
			go compressOldLogFile(fname, log.CompressionLevel)
		}

		// re-start logger
		if err = log.StartLogger(); err != nil {
			return fmt.Errorf("Rotate StartLogger: %v", err)
		}

		go log.deleteOldLog()
	}

	return nil
}

func compressOldLogFile(fname string, compressionLevel int) error {
	reader, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer reader.Close()
	buffer := bufio.NewReader(reader)
	fw, err := os.OpenFile(fname+".gz", os.O_WRONLY|os.O_CREATE, 0660)
	if err != nil {
		return err
	}
	defer fw.Close()
	zw, err := gzip.NewWriterLevel(fw, compressionLevel)
	if err != nil {
		return err
	}
	defer zw.Close()
	_, err = buffer.WriteTo(zw)
	if err != nil {
		zw.Close()
		fw.Close()
		util.Remove(fname + ".gz")
		return err
	}
	reader.Close()
	return util.Remove(fname)
}

func (log *FileLogger) deleteOldLog() {
	dir := filepath.Dir(log.Filename)
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) (returnErr error) {
		defer func() {
			if r := recover(); r != nil {
				returnErr = fmt.Errorf("Unable to delete old log '%s', error: %+v", path, r)
			}
		}()

		if !info.IsDir() && info.ModTime().Unix() < (time.Now().Unix()-60*60*24*log.Maxdays) {
			if strings.HasPrefix(filepath.Base(path), filepath.Base(log.Filename)) {

				if err := util.Remove(path); err != nil {
					returnErr = fmt.Errorf("Failed to remove %s: %v", path, err)
				}
			}
		}
		return returnErr
	})
}

// Flush flush file logger.
// there are no buffering messages in file logger in memory.
// flush file means sync file from disk.
func (log *FileLogger) Flush() {
	_ = log.mw.fd.Sync()
}

// ReleaseReopen releases and reopens log files
func (log *FileLogger) ReleaseReopen() error {
	closingErr := log.mw.fd.Close()
	startingErr := log.StartLogger()
	if startingErr != nil {
		if closingErr != nil {
			return fmt.Errorf("Error during closing: %v Error during starting: %v", closingErr, startingErr)
		}
		return startingErr
	}
	return closingErr
}

// GetName returns the default name for this implementation
func (log *FileLogger) GetName() string {
	return "file"
}

func init() {
	Register("file", NewFileLogger)
}
