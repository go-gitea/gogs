// +build mint

/*
 * MinIO Go Library for Amazon S3 Compatible Cloud Storage
 * Copyright 2015-2020 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/ioutil"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	jsoniter "github.com/json-iterator/go"
	log "github.com/sirupsen/logrus"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/encrypt"
	"github.com/minio/minio-go/v7/pkg/notification"
	"github.com/minio/minio-go/v7/pkg/tags"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyz01234569"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)
const (
	serverEndpoint = "SERVER_ENDPOINT"
	accessKey      = "ACCESS_KEY"
	secretKey      = "SECRET_KEY"
	enableHTTPS    = "ENABLE_HTTPS"
	enableKMS      = "ENABLE_KMS"
)

type mintJSONFormatter struct {
}

func (f *mintJSONFormatter) Format(entry *log.Entry) ([]byte, error) {
	data := make(log.Fields, len(entry.Data))
	for k, v := range entry.Data {
		switch v := v.(type) {
		case error:
			// Otherwise errors are ignored by `encoding/json`
			// https://github.com/sirupsen/logrus/issues/137
			data[k] = v.Error()
		default:
			data[k] = v
		}
	}
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	serialized, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal fields to JSON, %v", err)
	}
	return append(serialized, '\n'), nil
}

var readFull = func(r io.Reader, buf []byte) (n int, err error) {
	// ReadFull reads exactly len(buf) bytes from r into buf.
	// It returns the number of bytes copied and an error if
	// fewer bytes were read. The error is EOF only if no bytes
	// were read. If an EOF happens after reading some but not
	// all the bytes, ReadFull returns ErrUnexpectedEOF.
	// On return, n == len(buf) if and only if err == nil.
	// If r returns an error having read at least len(buf) bytes,
	// the error is dropped.
	for n < len(buf) && err == nil {
		var nn int
		nn, err = r.Read(buf[n:])
		// Some spurious io.Reader's return
		// io.ErrUnexpectedEOF when nn == 0
		// this behavior is undocumented
		// so we are on purpose not using io.ReadFull
		// implementation because this can lead
		// to custom handling, to avoid that
		// we simply modify the original io.ReadFull
		// implementation to avoid this issue.
		// io.ErrUnexpectedEOF with nn == 0 really
		// means that io.EOF
		if err == io.ErrUnexpectedEOF && nn == 0 {
			err = io.EOF
		}
		n += nn
	}
	if n >= len(buf) {
		err = nil
	} else if n > 0 && err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return
}

func cleanEmptyEntries(fields log.Fields) log.Fields {
	cleanFields := log.Fields{}
	for k, v := range fields {
		if v != "" {
			cleanFields[k] = v
		}
	}
	return cleanFields
}

// log successful test runs
func successLogger(testName string, function string, args map[string]interface{}, startTime time.Time) *log.Entry {
	// calculate the test case duration
	duration := time.Since(startTime)
	// log with the fields as per mint
	fields := log.Fields{"name": "minio-go: " + testName, "function": function, "args": args, "duration": duration.Nanoseconds() / 1000000, "status": "PASS"}
	return log.WithFields(cleanEmptyEntries(fields))
}

// As few of the features are not available in Gateway(s) currently, Check if err value is NotImplemented,
// and log as NA in that case and continue execution. Otherwise log as failure and return
func logError(testName string, function string, args map[string]interface{}, startTime time.Time, alert string, message string, err error) {
	// If server returns NotImplemented we assume it is gateway mode and hence log it as info and move on to next tests
	// Special case for ComposeObject API as it is implemented on client side and adds specific error details like `Error in upload-part-copy` in
	// addition to NotImplemented error returned from server
	if isErrNotImplemented(err) {
		ignoredLog(testName, function, args, startTime, message).Info()
	} else {
		failureLog(testName, function, args, startTime, alert, message, err).Fatal()
	}
}

// log failed test runs
func failureLog(testName string, function string, args map[string]interface{}, startTime time.Time, alert string, message string, err error) *log.Entry {
	// calculate the test case duration
	duration := time.Since(startTime)
	var fields log.Fields
	// log with the fields as per mint
	if err != nil {
		fields = log.Fields{"name": "minio-go: " + testName, "function": function, "args": args,
			"duration": duration.Nanoseconds() / 1000000, "status": "FAIL", "alert": alert, "message": message, "error": err}
	} else {
		fields = log.Fields{"name": "minio-go: " + testName, "function": function, "args": args,
			"duration": duration.Nanoseconds() / 1000000, "status": "FAIL", "alert": alert, "message": message}
	}
	return log.WithFields(cleanEmptyEntries(fields))
}

// log not applicable test runs
func ignoredLog(testName string, function string, args map[string]interface{}, startTime time.Time, alert string) *log.Entry {
	// calculate the test case duration
	duration := time.Since(startTime)
	// log with the fields as per mint
	fields := log.Fields{"name": "minio-go: " + testName, "function": function, "args": args,
		"duration": duration.Nanoseconds() / 1000000, "status": "NA", "alert": strings.Split(alert, " ")[0] + " is NotImplemented"}
	return log.WithFields(cleanEmptyEntries(fields))
}

// Delete objects in given bucket, recursively
func cleanupBucket(bucketName string, c *minio.Client) error {
	// Create a done channel to control 'ListObjectsV2' go routine.
	doneCh := make(chan struct{})
	// Exit cleanly upon return.
	defer close(doneCh)
	// Iterate over all objects in the bucket via listObjectsV2 and delete
	for objCh := range c.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{Recursive: true}) {
		if objCh.Err != nil {
			return objCh.Err
		}
		if objCh.Key != "" {
			err := c.RemoveObject(context.Background(), bucketName, objCh.Key, minio.RemoveObjectOptions{})
			if err != nil {
				return err
			}
		}
	}
	for objPartInfo := range c.ListIncompleteUploads(context.Background(), bucketName, "", true) {
		if objPartInfo.Err != nil {
			return objPartInfo.Err
		}
		if objPartInfo.Key != "" {
			err := c.RemoveIncompleteUpload(context.Background(), bucketName, objPartInfo.Key)
			if err != nil {
				return err
			}
		}
	}
	// objects are already deleted, clear the buckets now
	err := c.RemoveBucket(context.Background(), bucketName)
	if err != nil {
		return err
	}
	return err
}

func cleanupVersionedBucket(bucketName string, c *minio.Client) error {
	doneCh := make(chan struct{})
	defer close(doneCh)
	for obj := range c.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{WithVersions: true, Recursive: true}) {
		if obj.Err != nil {
			return obj.Err
		}
		if obj.Key != "" {
			err := c.RemoveObject(context.Background(), bucketName, obj.Key,
				minio.RemoveObjectOptions{VersionID: obj.VersionID, GovernanceBypass: true})
			if err != nil {
				return err
			}
		}
	}
	for objPartInfo := range c.ListIncompleteUploads(context.Background(), bucketName, "", true) {
		if objPartInfo.Err != nil {
			return objPartInfo.Err
		}
		if objPartInfo.Key != "" {
			err := c.RemoveIncompleteUpload(context.Background(), bucketName, objPartInfo.Key)
			if err != nil {
				return err
			}
		}
	}
	// objects are already deleted, clear the buckets now
	err := c.RemoveBucket(context.Background(), bucketName)
	if err != nil {
		for obj := range c.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{WithVersions: true, Recursive: true}) {
			log.Println("found", obj.Key, obj.VersionID)
		}
		return err
	}
	return err
}

func isErrNotImplemented(err error) bool {
	return minio.ToErrorResponse(err).Code == "NotImplemented"
}

func init() {
	// If server endpoint is not set, all tests default to
	// using https://play.min.io
	if os.Getenv(serverEndpoint) == "" {
		os.Setenv(serverEndpoint, "play.min.io")
		os.Setenv(accessKey, "Q3AM3UQ867SPQQA43P2F")
		os.Setenv(secretKey, "zuf+tfteSlswRu7BJ86wekitnifILbZam1KYY3TG")
		os.Setenv(enableHTTPS, "1")
	}
}

var mintDataDir = os.Getenv("MINT_DATA_DIR")

func getMintDataDirFilePath(filename string) (fp string) {
	if mintDataDir == "" {
		return
	}
	return filepath.Join(mintDataDir, filename)
}

func newRandomReader(seed, size int64) io.Reader {
	return io.LimitReader(rand.New(rand.NewSource(seed)), size)
}

func mustCrcReader(r io.Reader) uint32 {
	crc := crc32.NewIEEE()
	_, err := io.Copy(crc, r)
	if err != nil {
		panic(err)
	}
	return crc.Sum32()
}

func crcMatches(r io.Reader, want uint32) error {
	crc := crc32.NewIEEE()
	_, err := io.Copy(crc, r)
	if err != nil {
		panic(err)
	}
	got := crc.Sum32()
	if got != want {
		return fmt.Errorf("crc mismatch, want %x, got %x", want, got)
	}
	return nil
}

func crcMatchesName(r io.Reader, name string) error {
	want := dataFileCRC32[name]
	crc := crc32.NewIEEE()
	_, err := io.Copy(crc, r)
	if err != nil {
		panic(err)
	}
	got := crc.Sum32()
	if got != want {
		return fmt.Errorf("crc mismatch, want %x, got %x", want, got)
	}
	return nil
}

// read data from file if it exists or optionally create a buffer of particular size
func getDataReader(fileName string) io.ReadCloser {
	if mintDataDir == "" {
		size := int64(dataFileMap[fileName])
		if _, ok := dataFileCRC32[fileName]; !ok {
			dataFileCRC32[fileName] = mustCrcReader(newRandomReader(size, size))
		}
		return ioutil.NopCloser(newRandomReader(size, size))
	}
	reader, _ := os.Open(getMintDataDirFilePath(fileName))
	if _, ok := dataFileCRC32[fileName]; !ok {
		dataFileCRC32[fileName] = mustCrcReader(reader)
		reader.Close()
		reader, _ = os.Open(getMintDataDirFilePath(fileName))
	}
	return reader
}

// randString generates random names and prepends them with a known prefix.
func randString(n int, src rand.Source, prefix string) string {
	b := make([]byte, n)
	// A rand.Int63() generates 63 random bits, enough for letterIdxMax letters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}
	return prefix + string(b[0:30-len(prefix)])
}

var dataFileMap = map[string]int{
	"datafile-0-b":     0,
	"datafile-1-b":     1,
	"datafile-1-kB":    1 * humanize.KiByte,
	"datafile-10-kB":   10 * humanize.KiByte,
	"datafile-33-kB":   33 * humanize.KiByte,
	"datafile-100-kB":  100 * humanize.KiByte,
	"datafile-1.03-MB": 1056 * humanize.KiByte,
	"datafile-1-MB":    1 * humanize.MiByte,
	"datafile-5-MB":    5 * humanize.MiByte,
	"datafile-6-MB":    6 * humanize.MiByte,
	"datafile-11-MB":   11 * humanize.MiByte,
	"datafile-65-MB":   65 * humanize.MiByte,
	"datafile-129-MB":  129 * humanize.MiByte,
}

var dataFileCRC32 = map[string]uint32{}

func isFullMode() bool {
	return os.Getenv("MINT_MODE") == "full"
}

func getFuncName() string {
	return getFuncNameLoc(2)
}

func getFuncNameLoc(caller int) string {
	pc, _, _, _ := runtime.Caller(caller)
	return strings.TrimPrefix(runtime.FuncForPC(pc).Name(), "main.")
}

// Tests bucket re-create errors.
func testMakeBucketError() {
	region := "eu-central-1"

	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "MakeBucket(bucketName, region)"
	// initialize logging params
	args := map[string]interface{}{
		"bucketName": "",
		"region":     region,
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket in 'eu-central-1'.
	if err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: region}); err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket Failed", err)
		return
	}
	defer cleanupBucket(bucketName, c)

	if err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: region}); err == nil {
		logError(testName, function, args, startTime, "", "Bucket already exists", err)
		return
	}
	// Verify valid error response from server.
	if minio.ToErrorResponse(err).Code != "BucketAlreadyExists" &&
		minio.ToErrorResponse(err).Code != "BucketAlreadyOwnedByYou" {
		logError(testName, function, args, startTime, "", "Invalid error returned by server", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

func testMetadataSizeLimit() {
	startTime := time.Now()
	testName := getFuncName()
	function := "PutObject(bucketName, objectName, reader, objectSize, opts)"
	args := map[string]interface{}{
		"bucketName":        "",
		"objectName":        "",
		"opts.UserMetadata": "",
	}
	rand.Seed(startTime.Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client creation failed", err)
		return
	}
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "Make bucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	const HeaderSizeLimit = 8 * 1024
	const UserMetadataLimit = 2 * 1024

	// Meta-data greater than the 2 KB limit of AWS - PUT calls with this meta-data should fail
	metadata := make(map[string]string)
	metadata["X-Amz-Meta-Mint-Test"] = string(bytes.Repeat([]byte("m"), 1+UserMetadataLimit-len("X-Amz-Meta-Mint-Test")))
	args["metadata"] = fmt.Sprint(metadata)

	_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(nil), 0, minio.PutObjectOptions{UserMetadata: metadata})
	if err == nil {
		logError(testName, function, args, startTime, "", "Created object with user-defined metadata exceeding metadata size limits", nil)
		return
	}

	// Meta-data (headers) greater than the 8 KB limit of AWS - PUT calls with this meta-data should fail
	metadata = make(map[string]string)
	metadata["X-Amz-Mint-Test"] = string(bytes.Repeat([]byte("m"), 1+HeaderSizeLimit-len("X-Amz-Mint-Test")))
	args["metadata"] = fmt.Sprint(metadata)
	_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(nil), 0, minio.PutObjectOptions{UserMetadata: metadata})
	if err == nil {
		logError(testName, function, args, startTime, "", "Created object with headers exceeding header size limits", nil)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Tests various bucket supported formats.
func testMakeBucketRegions() {
	region := "eu-central-1"
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "MakeBucket(bucketName, region)"
	// initialize logging params
	args := map[string]interface{}{
		"bucketName": "",
		"region":     region,
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket in 'eu-central-1'.
	if err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: region}); err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	// Delete all objects and buckets
	if err = cleanupBucket(bucketName, c); err != nil {
		logError(testName, function, args, startTime, "", "CleanupBucket failed", err)
		return
	}

	// Make a new bucket with '.' in its name, in 'us-west-2'. This
	// request is internally staged into a path style instead of
	// virtual host style.
	region = "us-west-2"
	args["region"] = region
	if err = c.MakeBucket(context.Background(), bucketName+".withperiod", minio.MakeBucketOptions{Region: region}); err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	// Delete all objects and buckets
	if err = cleanupBucket(bucketName+".withperiod", c); err != nil {
		logError(testName, function, args, startTime, "", "CleanupBucket failed", err)
		return
	}
	successLogger(testName, function, args, startTime).Info()
}

// Test PutObject using a large data to trigger multipart readat
func testPutObjectReadAt() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "PutObject(bucketName, objectName, reader, opts)"
	args := map[string]interface{}{
		"bucketName": "",
		"objectName": "",
		"opts":       "objectContentType",
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "Make bucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	bufSize := dataFileMap["datafile-129-MB"]
	var reader = getDataReader("datafile-129-MB")
	defer reader.Close()

	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	// Object content type
	objectContentType := "binary/octet-stream"
	args["objectContentType"] = objectContentType

	_, err = c.PutObject(context.Background(), bucketName, objectName, reader, int64(bufSize), minio.PutObjectOptions{ContentType: objectContentType})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	// Read the data back
	r, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "Get Object failed", err)
		return
	}

	st, err := r.Stat()
	if err != nil {
		logError(testName, function, args, startTime, "", "Stat Object failed", err)
		return
	}
	if st.Size != int64(bufSize) {
		logError(testName, function, args, startTime, "", fmt.Sprintf("Number of bytes in stat does not match, expected %d got %d", bufSize, st.Size), err)
		return
	}
	if st.ContentType != objectContentType && st.ContentType != "application/octet-stream" {
		logError(testName, function, args, startTime, "", "Content types don't match", err)
		return
	}
	if err := crcMatchesName(r, "datafile-129-MB"); err != nil {
		logError(testName, function, args, startTime, "", "data CRC check failed", err)
		return
	}
	if err := r.Close(); err != nil {
		logError(testName, function, args, startTime, "", "Object Close failed", err)
		return
	}
	if err := r.Close(); err == nil {
		logError(testName, function, args, startTime, "", "Object is already closed, didn't return error on Close", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

func testListObjectVersions() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "ListObjectVersions(bucketName, prefix, recursive)"
	args := map[string]interface{}{
		"bucketName": "",
		"prefix":     "",
		"recursive":  "",
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1", ObjectLocking: true})
	if err != nil {
		logError(testName, function, args, startTime, "", "Make bucket failed", err)
		return
	}

	err = c.EnableVersioning(context.Background(), bucketName)
	if err != nil {
		logError(testName, function, args, startTime, "", "Enable versioning failed", err)
		return
	}

	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	bufSize := dataFileMap["datafile-10-kB"]
	var reader = getDataReader("datafile-10-kB")

	_, err = c.PutObject(context.Background(), bucketName, objectName, reader, int64(bufSize), minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}
	reader.Close()

	bufSize = dataFileMap["datafile-1-b"]
	reader = getDataReader("datafile-1-b")
	_, err = c.PutObject(context.Background(), bucketName, objectName, reader, int64(bufSize), minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}
	reader.Close()

	err = c.RemoveObject(context.Background(), bucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "Unexpected object deletion", err)
		return
	}

	var deleteMarkers, versions int

	objectsInfo := c.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{WithVersions: true, Recursive: true})
	for info := range objectsInfo {
		if info.Err != nil {
			logError(testName, function, args, startTime, "", "Unexpected error during listing objects", err)
			return
		}
		if info.Key != objectName {
			logError(testName, function, args, startTime, "", "Unexpected object name in listing objects", nil)
			return
		}
		if info.VersionID == "" {
			logError(testName, function, args, startTime, "", "Unexpected version id in listing objects", nil)
			return
		}
		if info.IsDeleteMarker {
			deleteMarkers++
			if !info.IsLatest {
				logError(testName, function, args, startTime, "", "Unexpected IsLatest field in listing objects", nil)
				return
			}
		} else {
			versions++
		}
	}

	if deleteMarkers != 1 {
		logError(testName, function, args, startTime, "", "Unexpected number of DeleteMarker elements in listing objects", nil)
		return
	}

	if versions != 2 {
		logError(testName, function, args, startTime, "", "Unexpected number of Version elements in listing objects", nil)
		return
	}

	// Delete all objects and their versions as long as the bucket itself
	if err = cleanupVersionedBucket(bucketName, c); err != nil {
		logError(testName, function, args, startTime, "", "CleanupBucket failed", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

func testStatObjectWithVersioning() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "StatObject"
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1", ObjectLocking: true})
	if err != nil {
		logError(testName, function, args, startTime, "", "Make bucket failed", err)
		return
	}

	err = c.EnableVersioning(context.Background(), bucketName)
	if err != nil {
		logError(testName, function, args, startTime, "", "Enable versioning failed", err)
		return
	}

	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	bufSize := dataFileMap["datafile-10-kB"]
	var reader = getDataReader("datafile-10-kB")

	_, err = c.PutObject(context.Background(), bucketName, objectName, reader, int64(bufSize), minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}
	reader.Close()

	bufSize = dataFileMap["datafile-1-b"]
	reader = getDataReader("datafile-1-b")
	_, err = c.PutObject(context.Background(), bucketName, objectName, reader, int64(bufSize), minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}
	reader.Close()

	objectsInfo := c.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{WithVersions: true, Recursive: true})

	var results []minio.ObjectInfo
	for info := range objectsInfo {
		if info.Err != nil {
			logError(testName, function, args, startTime, "", "Unexpected error during listing objects", err)
			return
		}
		results = append(results, info)
	}

	if len(results) != 2 {
		logError(testName, function, args, startTime, "", "Unexpected number of Version elements in listing objects", nil)
		return
	}

	for i := 0; i < len(results); i++ {
		opts := minio.StatObjectOptions{VersionID: results[i].VersionID}
		statInfo, err := c.StatObject(context.Background(), bucketName, objectName, opts)
		if err != nil {
			logError(testName, function, args, startTime, "", "error during HEAD object", err)
			return
		}
		if statInfo.VersionID == "" || statInfo.VersionID != results[i].VersionID {
			logError(testName, function, args, startTime, "", "error during HEAD object, unexpected version id", err)
			return
		}
		if statInfo.ETag != results[i].ETag {
			logError(testName, function, args, startTime, "", "error during HEAD object, unexpected ETag", err)
			return
		}
		if statInfo.LastModified.Unix() != results[i].LastModified.Unix() {
			logError(testName, function, args, startTime, "", "error during HEAD object, unexpected Last-Modified", err)
			return
		}
		if statInfo.Size != results[i].Size {
			logError(testName, function, args, startTime, "", "error during HEAD object, unexpected Content-Length", err)
			return
		}
	}

	// Delete all objects and their versions as long as the bucket itself
	if err = cleanupVersionedBucket(bucketName, c); err != nil {
		logError(testName, function, args, startTime, "", "CleanupBucket failed", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

func testGetObjectWithVersioning() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "GetObject()"
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1", ObjectLocking: true})
	if err != nil {
		logError(testName, function, args, startTime, "", "Make bucket failed", err)
		return
	}

	err = c.EnableVersioning(context.Background(), bucketName)
	if err != nil {
		logError(testName, function, args, startTime, "", "Enable versioning failed", err)
		return
	}

	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	// Save the contents of datafiles to check with GetObject() reader output later
	var buffers [][]byte
	var testFiles = []string{"datafile-1-b", "datafile-10-kB"}

	for _, testFile := range testFiles {
		r := getDataReader(testFile)
		buf, err := ioutil.ReadAll(r)
		if err != nil {
			logError(testName, function, args, startTime, "", "unexpected failure", err)
			return
		}
		r.Close()
		_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{})
		if err != nil {
			logError(testName, function, args, startTime, "", "PutObject failed", err)
			return
		}
		buffers = append(buffers, buf)
	}

	objectsInfo := c.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{WithVersions: true, Recursive: true})

	var results []minio.ObjectInfo
	for info := range objectsInfo {
		if info.Err != nil {
			logError(testName, function, args, startTime, "", "Unexpected error during listing objects", err)
			return
		}
		results = append(results, info)
	}

	if len(results) != 2 {
		logError(testName, function, args, startTime, "", "Unexpected number of Version elements in listing objects", nil)
		return
	}

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Size < results[j].Size
	})

	sort.SliceStable(buffers, func(i, j int) bool {
		return len(buffers[i]) < len(buffers[j])
	})

	for i := 0; i < len(results); i++ {
		opts := minio.GetObjectOptions{VersionID: results[i].VersionID}
		reader, err := c.GetObject(context.Background(), bucketName, objectName, opts)
		if err != nil {
			logError(testName, function, args, startTime, "", "error during  GET object", err)
			return
		}
		statInfo, err := reader.Stat()
		if err != nil {
			logError(testName, function, args, startTime, "", "error during calling reader.Stat()", err)
			return
		}
		if statInfo.ETag != results[i].ETag {
			logError(testName, function, args, startTime, "", "error during HEAD object, unexpected ETag", err)
			return
		}
		if statInfo.LastModified.Unix() != results[i].LastModified.Unix() {
			logError(testName, function, args, startTime, "", "error during HEAD object, unexpected Last-Modified", err)
			return
		}
		if statInfo.Size != results[i].Size {
			logError(testName, function, args, startTime, "", "error during HEAD object, unexpected Content-Length", err)
			return
		}

		tmpBuffer := bytes.NewBuffer([]byte{})
		_, err = io.Copy(tmpBuffer, reader)
		if err != nil {
			logError(testName, function, args, startTime, "", "unexpected io.Copy()", err)
			return
		}

		if !bytes.Equal(tmpBuffer.Bytes(), buffers[i]) {
			logError(testName, function, args, startTime, "", "unexpected content of GetObject()", err)
			return
		}
	}

	// Delete all objects and their versions as long as the bucket itself
	if err = cleanupVersionedBucket(bucketName, c); err != nil {
		logError(testName, function, args, startTime, "", "CleanupBucket failed", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

func testPutObjectWithVersioning() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "GetObject()"
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1", ObjectLocking: true})
	if err != nil {
		logError(testName, function, args, startTime, "", "Make bucket failed", err)
		return
	}

	err = c.EnableVersioning(context.Background(), bucketName)
	if err != nil {
		logError(testName, function, args, startTime, "", "Enable versioning failed", err)
		return
	}

	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	const n = 10
	// Read input...

	// Save the data concurrently.
	var wg sync.WaitGroup
	wg.Add(n)
	var buffers = make([][]byte, n)
	var errs [n]error
	for i := 0; i < n; i++ {
		r := newRandomReader(int64((1<<20)*i+i), int64(i))
		buf, err := ioutil.ReadAll(r)
		if err != nil {
			logError(testName, function, args, startTime, "", "unexpected failure", err)
			return
		}
		buffers[i] = buf

		go func(i int) {
			defer wg.Done()
			_, errs[i] = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{PartSize: 5 << 20})
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			logError(testName, function, args, startTime, "", "PutObject failed", err)
			return
		}
	}

	objectsInfo := c.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{WithVersions: true, Recursive: true})
	var results []minio.ObjectInfo
	for info := range objectsInfo {
		if info.Err != nil {
			logError(testName, function, args, startTime, "", "Unexpected error during listing objects", err)
			return
		}
		results = append(results, info)
	}

	if len(results) != n {
		logError(testName, function, args, startTime, "", "Unexpected number of Version elements in listing objects", nil)
		return
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Size < results[j].Size
	})

	sort.Slice(buffers, func(i, j int) bool {
		return len(buffers[i]) < len(buffers[j])
	})

	for i := 0; i < len(results); i++ {
		opts := minio.GetObjectOptions{VersionID: results[i].VersionID}
		reader, err := c.GetObject(context.Background(), bucketName, objectName, opts)
		if err != nil {
			logError(testName, function, args, startTime, "", "error during  GET object", err)
			return
		}
		statInfo, err := reader.Stat()
		if err != nil {
			logError(testName, function, args, startTime, "", "error during calling reader.Stat()", err)
			return
		}
		if statInfo.ETag != results[i].ETag {
			logError(testName, function, args, startTime, "", "error during HEAD object, unexpected ETag", err)
			return
		}
		if statInfo.LastModified.Unix() != results[i].LastModified.Unix() {
			logError(testName, function, args, startTime, "", "error during HEAD object, unexpected Last-Modified", err)
			return
		}
		if statInfo.Size != results[i].Size {
			logError(testName, function, args, startTime, "", "error during HEAD object, unexpected Content-Length", err)
			return
		}

		tmpBuffer := bytes.NewBuffer([]byte{})
		_, err = io.Copy(tmpBuffer, reader)
		if err != nil {
			logError(testName, function, args, startTime, "", "unexpected io.Copy()", err)
			return
		}

		if !bytes.Equal(tmpBuffer.Bytes(), buffers[i]) {
			logError(testName, function, args, startTime, "", "unexpected content of GetObject()", err)
			return
		}
	}

	// Delete all objects and their versions as long as the bucket itself
	if err = cleanupVersionedBucket(bucketName, c); err != nil {
		logError(testName, function, args, startTime, "", "CleanupBucket failed", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

func testCopyObjectWithVersioning() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObject()"
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1", ObjectLocking: true})
	if err != nil {
		logError(testName, function, args, startTime, "", "Make bucket failed", err)
		return
	}

	err = c.EnableVersioning(context.Background(), bucketName)
	if err != nil {
		logError(testName, function, args, startTime, "", "Enable versioning failed", err)
		return
	}

	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	var testFiles = []string{"datafile-1-b", "datafile-10-kB"}
	for _, testFile := range testFiles {
		r := getDataReader(testFile)
		buf, err := ioutil.ReadAll(r)
		if err != nil {
			logError(testName, function, args, startTime, "", "unexpected failure", err)
			return
		}
		r.Close()
		_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{})
		if err != nil {
			logError(testName, function, args, startTime, "", "PutObject failed", err)
			return
		}
	}

	objectsInfo := c.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{WithVersions: true, Recursive: true})
	var infos []minio.ObjectInfo
	for info := range objectsInfo {
		if info.Err != nil {
			logError(testName, function, args, startTime, "", "Unexpected error during listing objects", err)
			return
		}
		infos = append(infos, info)
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Size < infos[j].Size
	})

	reader, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{VersionID: infos[0].VersionID})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject of the oldest version content failed", err)
		return
	}

	oldestContent, err := ioutil.ReadAll(reader)
	if err != nil {
		logError(testName, function, args, startTime, "", "Reading the oldest object version failed", err)
		return
	}

	// Copy Source
	srcOpts := minio.CopySrcOptions{
		Bucket:    bucketName,
		Object:    objectName,
		VersionID: infos[0].VersionID,
	}
	args["src"] = srcOpts

	dstOpts := minio.CopyDestOptions{
		Bucket: bucketName,
		Object: objectName + "-copy",
	}
	args["dst"] = dstOpts

	// Perform the Copy
	if _, err = c.CopyObject(context.Background(), dstOpts, srcOpts); err != nil {
		logError(testName, function, args, startTime, "", "CopyObject failed", err)
		return
	}

	// Destination object
	readerCopy, err := c.GetObject(context.Background(), bucketName, objectName+"-copy", minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}
	defer readerCopy.Close()

	newestContent, err := ioutil.ReadAll(readerCopy)
	if err != nil {
		logError(testName, function, args, startTime, "", "Reading from GetObject reader failed", err)
		return
	}

	if len(newestContent) == 0 || !bytes.Equal(oldestContent, newestContent) {
		logError(testName, function, args, startTime, "", "Unexpected destination object content", err)
		return
	}

	// Delete all objects and their versions as long as the bucket itself
	if err = cleanupVersionedBucket(bucketName, c); err != nil {
		logError(testName, function, args, startTime, "", "CleanupBucket failed", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

func testConcurrentCopyObjectWithVersioning() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObject()"
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1", ObjectLocking: true})
	if err != nil {
		logError(testName, function, args, startTime, "", "Make bucket failed", err)
		return
	}

	err = c.EnableVersioning(context.Background(), bucketName)
	if err != nil {
		logError(testName, function, args, startTime, "", "Enable versioning failed", err)
		return
	}

	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	var testFiles = []string{"datafile-10-kB"}
	for _, testFile := range testFiles {
		r := getDataReader(testFile)
		buf, err := ioutil.ReadAll(r)
		if err != nil {
			logError(testName, function, args, startTime, "", "unexpected failure", err)
			return
		}
		r.Close()
		_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{})
		if err != nil {
			logError(testName, function, args, startTime, "", "PutObject failed", err)
			return
		}
	}

	objectsInfo := c.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{WithVersions: true, Recursive: true})
	var infos []minio.ObjectInfo
	for info := range objectsInfo {
		if info.Err != nil {
			logError(testName, function, args, startTime, "", "Unexpected error during listing objects", err)
			return
		}
		infos = append(infos, info)
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Size < infos[j].Size
	})

	reader, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{VersionID: infos[0].VersionID})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject of the oldest version content failed", err)
		return
	}

	oldestContent, err := ioutil.ReadAll(reader)
	if err != nil {
		logError(testName, function, args, startTime, "", "Reading the oldest object version failed", err)
		return
	}

	// Copy Source
	srcOpts := minio.CopySrcOptions{
		Bucket:    bucketName,
		Object:    objectName,
		VersionID: infos[0].VersionID,
	}
	args["src"] = srcOpts

	dstOpts := minio.CopyDestOptions{
		Bucket: bucketName,
		Object: objectName + "-copy",
	}
	args["dst"] = dstOpts

	// Perform the Copy concurrently
	const n = 10
	var wg sync.WaitGroup
	wg.Add(n)
	var errs [n]error
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			_, errs[i] = c.CopyObject(context.Background(), dstOpts, srcOpts)
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			logError(testName, function, args, startTime, "", "CopyObject failed", err)
			return
		}
	}

	objectsInfo = c.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{WithVersions: true, Recursive: false, Prefix: dstOpts.Object})
	infos = []minio.ObjectInfo{}
	for info := range objectsInfo {
		// Destination object
		readerCopy, err := c.GetObject(context.Background(), bucketName, objectName+"-copy", minio.GetObjectOptions{VersionID: info.VersionID})
		if err != nil {
			logError(testName, function, args, startTime, "", "GetObject failed", err)
			return
		}
		defer readerCopy.Close()

		newestContent, err := ioutil.ReadAll(readerCopy)
		if err != nil {
			logError(testName, function, args, startTime, "", "Reading from GetObject reader failed", err)
			return
		}

		if len(newestContent) == 0 || !bytes.Equal(oldestContent, newestContent) {
			logError(testName, function, args, startTime, "", "Unexpected destination object content", err)
			return
		}
		infos = append(infos, info)
	}

	if len(infos) != n {
		logError(testName, function, args, startTime, "", "Unexpected number of Version elements in listing objects", nil)
		return
	}

	// Delete all objects and their versions as long as the bucket itself
	if err = cleanupVersionedBucket(bucketName, c); err != nil {
		logError(testName, function, args, startTime, "", "CleanupBucket failed", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

func testComposeObjectWithVersioning() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "ComposeObject()"
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1", ObjectLocking: true})
	if err != nil {
		logError(testName, function, args, startTime, "", "Make bucket failed", err)
		return
	}

	err = c.EnableVersioning(context.Background(), bucketName)
	if err != nil {
		logError(testName, function, args, startTime, "", "Enable versioning failed", err)
		return
	}

	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	// var testFiles = []string{"datafile-5-MB", "datafile-10-kB"}
	var testFiles = []string{"datafile-5-MB", "datafile-10-kB"}
	var testFilesBytes [][]byte

	for _, testFile := range testFiles {
		r := getDataReader(testFile)
		buf, err := ioutil.ReadAll(r)
		if err != nil {
			logError(testName, function, args, startTime, "", "unexpected failure", err)
			return
		}
		r.Close()
		_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{})
		if err != nil {
			logError(testName, function, args, startTime, "", "PutObject failed", err)
			return
		}
		testFilesBytes = append(testFilesBytes, buf)
	}

	objectsInfo := c.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{WithVersions: true, Recursive: true})

	var results []minio.ObjectInfo
	for info := range objectsInfo {
		if info.Err != nil {
			logError(testName, function, args, startTime, "", "Unexpected error during listing objects", err)
			return
		}
		results = append(results, info)
	}

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Size > results[j].Size
	})

	// Source objects to concatenate. We also specify decryption
	// key for each
	src1 := minio.CopySrcOptions{
		Bucket:    bucketName,
		Object:    objectName,
		VersionID: results[0].VersionID,
	}

	src2 := minio.CopySrcOptions{
		Bucket:    bucketName,
		Object:    objectName,
		VersionID: results[1].VersionID,
	}

	dst := minio.CopyDestOptions{
		Bucket: bucketName,
		Object: objectName + "-copy",
	}

	_, err = c.ComposeObject(context.Background(), dst, src1, src2)
	if err != nil {
		logError(testName, function, args, startTime, "", "ComposeObject failed", err)
		return
	}

	// Destination object
	readerCopy, err := c.GetObject(context.Background(), bucketName, objectName+"-copy", minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject of the copy object failed", err)
		return
	}
	defer readerCopy.Close()

	copyContentBytes, err := ioutil.ReadAll(readerCopy)
	if err != nil {
		logError(testName, function, args, startTime, "", "Reading from the copy object reader failed", err)
		return
	}

	var expectedContent []byte
	for _, fileBytes := range testFilesBytes {
		expectedContent = append(expectedContent, fileBytes...)
	}

	if len(copyContentBytes) == 0 || !bytes.Equal(copyContentBytes, expectedContent) {
		logError(testName, function, args, startTime, "", "Unexpected destination object content", err)
		return
	}

	// Delete all objects and their versions as long as the bucket itself
	if err = cleanupVersionedBucket(bucketName, c); err != nil {
		logError(testName, function, args, startTime, "", "CleanupBucket failed", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

func testRemoveObjectWithVersioning() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "DeleteObject()"
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1", ObjectLocking: true})
	if err != nil {
		logError(testName, function, args, startTime, "", "Make bucket failed", err)
		return
	}

	err = c.EnableVersioning(context.Background(), bucketName)
	if err != nil {
		logError(testName, function, args, startTime, "", "Enable versioning failed", err)
		return
	}

	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	_, err = c.PutObject(context.Background(), bucketName, objectName, getDataReader("datafile-10-kB"), int64(dataFileMap["datafile-10-kB"]), minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	objectsInfo := c.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{WithVersions: true, Recursive: true})
	var version minio.ObjectInfo
	for info := range objectsInfo {
		if info.Err != nil {
			logError(testName, function, args, startTime, "", "Unexpected error during listing objects", err)
			return
		}
		version = info
		break
	}

	err = c.RemoveObject(context.Background(), bucketName, objectName, minio.RemoveObjectOptions{VersionID: version.VersionID})
	if err != nil {
		logError(testName, function, args, startTime, "", "DeleteObject failed", err)
		return
	}

	objectsInfo = c.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{WithVersions: true, Recursive: true})
	for range objectsInfo {
		logError(testName, function, args, startTime, "", "Unexpected versioning info, should not have any one ", err)
		return
	}

	err = c.RemoveBucket(context.Background(), bucketName)
	if err != nil {
		logError(testName, function, args, startTime, "", "CleanupBucket failed", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

func testRemoveObjectsWithVersioning() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "DeleteObjects()"
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1", ObjectLocking: true})
	if err != nil {
		logError(testName, function, args, startTime, "", "Make bucket failed", err)
		return
	}

	err = c.EnableVersioning(context.Background(), bucketName)
	if err != nil {
		logError(testName, function, args, startTime, "", "Enable versioning failed", err)
		return
	}

	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	_, err = c.PutObject(context.Background(), bucketName, objectName, getDataReader("datafile-10-kB"), int64(dataFileMap["datafile-10-kB"]), minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	objectsVersions := make(chan minio.ObjectInfo)
	go func() {
		objectsVersionsInfo := c.ListObjects(context.Background(), bucketName,
			minio.ListObjectsOptions{WithVersions: true, Recursive: true})
		for info := range objectsVersionsInfo {
			if info.Err != nil {
				logError(testName, function, args, startTime, "", "Unexpected error during listing objects", err)
				return
			}
			objectsVersions <- info
		}
		close(objectsVersions)
	}()

	removeErrors := c.RemoveObjects(context.Background(), bucketName, objectsVersions, minio.RemoveObjectsOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "DeleteObjects call failed", err)
		return
	}

	for e := range removeErrors {
		if e.Err != nil {
			logError(testName, function, args, startTime, "", "Single delete operation failed", err)
			return
		}
	}

	objectsVersionsInfo := c.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{WithVersions: true, Recursive: true})
	for range objectsVersionsInfo {
		logError(testName, function, args, startTime, "", "Unexpected versioning info, should not have any one ", err)
		return
	}

	err = c.RemoveBucket(context.Background(), bucketName)
	if err != nil {
		logError(testName, function, args, startTime, "", "CleanupBucket failed", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

func testObjectTaggingWithVersioning() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "{Get,Set,Remove}ObjectTagging()"
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1", ObjectLocking: true})
	if err != nil {
		logError(testName, function, args, startTime, "", "Make bucket failed", err)
		return
	}

	err = c.EnableVersioning(context.Background(), bucketName)
	if err != nil {
		logError(testName, function, args, startTime, "", "Enable versioning failed", err)
		return
	}

	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	for _, file := range []string{"datafile-1-b", "datafile-10-kB"} {
		_, err = c.PutObject(context.Background(), bucketName, objectName, getDataReader(file), int64(dataFileMap[file]), minio.PutObjectOptions{})
		if err != nil {
			logError(testName, function, args, startTime, "", "PutObject failed", err)
			return
		}
	}

	versionsInfo := c.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{WithVersions: true, Recursive: true})

	var versions []minio.ObjectInfo
	for info := range versionsInfo {
		if info.Err != nil {
			logError(testName, function, args, startTime, "", "Unexpected error during listing objects", err)
			return
		}
		versions = append(versions, info)
	}

	sort.SliceStable(versions, func(i, j int) bool {
		return versions[i].Size < versions[j].Size
	})

	tagsV1 := map[string]string{"key1": "val1"}
	t1, err := tags.MapToObjectTags(tagsV1)
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObjectTagging (1) failed", err)
		return
	}

	err = c.PutObjectTagging(context.Background(), bucketName, objectName, t1, minio.PutObjectTaggingOptions{VersionID: versions[0].VersionID})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObjectTagging (1) failed", err)
		return
	}

	tagsV2 := map[string]string{"key2": "val2"}
	t2, err := tags.MapToObjectTags(tagsV2)
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObjectTagging (1) failed", err)
		return
	}

	err = c.PutObjectTagging(context.Background(), bucketName, objectName, t2, minio.PutObjectTaggingOptions{VersionID: versions[1].VersionID})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObjectTagging (2) failed", err)
		return
	}

	tagsEqual := func(tags1, tags2 map[string]string) bool {
		for k1, v1 := range tags1 {
			v2, found := tags2[k1]
			if found {
				if v1 != v2 {
					return false
				}
			}
		}
		return true
	}

	gotTagsV1, err := c.GetObjectTagging(context.Background(), bucketName, objectName, minio.GetObjectTaggingOptions{VersionID: versions[0].VersionID})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObjectTagging failed", err)
		return
	}

	if !tagsEqual(t1.ToMap(), gotTagsV1.ToMap()) {
		logError(testName, function, args, startTime, "", "Unexpected tags content (1)", err)
		return
	}

	gotTagsV2, err := c.GetObjectTagging(context.Background(), bucketName, objectName, minio.GetObjectTaggingOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObjectTaggingContext failed", err)
		return
	}

	if !tagsEqual(t2.ToMap(), gotTagsV2.ToMap()) {
		logError(testName, function, args, startTime, "", "Unexpected tags content (2)", err)
		return
	}

	err = c.RemoveObjectTagging(context.Background(), bucketName, objectName, minio.RemoveObjectTaggingOptions{VersionID: versions[0].VersionID})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObjectTagging (2) failed", err)
		return
	}

	emptyTags, err := c.GetObjectTagging(context.Background(), bucketName, objectName,
		minio.GetObjectTaggingOptions{VersionID: versions[0].VersionID})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObjectTagging failed", err)
		return
	}

	if len(emptyTags.ToMap()) != 0 {
		logError(testName, function, args, startTime, "", "Unexpected tags content (2)", err)
		return
	}

	// Delete all objects and their versions as long as the bucket itself
	if err = cleanupVersionedBucket(bucketName, c); err != nil {
		logError(testName, function, args, startTime, "", "CleanupBucket failed", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Test PutObject using a large data to trigger multipart readat
func testPutObjectWithMetadata() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "PutObject(bucketName, objectName, reader,size, opts)"
	args := map[string]interface{}{
		"bucketName": "",
		"objectName": "",
		"opts":       "minio.PutObjectOptions{UserMetadata: metadata, Progress: progress}",
	}

	if !isFullMode() {
		ignoredLog(testName, function, args, startTime, "Skipping functional tests for short/quick runs").Info()
		return
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "Make bucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	bufSize := dataFileMap["datafile-129-MB"]
	var reader = getDataReader("datafile-129-MB")
	defer reader.Close()

	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	// Object custom metadata
	customContentType := "custom/contenttype"

	args["metadata"] = map[string][]string{
		"Content-Type":         {customContentType},
		"X-Amz-Meta-CustomKey": {"extra  spaces  in   value"},
	}

	_, err = c.PutObject(context.Background(), bucketName, objectName, reader, int64(bufSize), minio.PutObjectOptions{
		ContentType: customContentType})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	// Read the data back
	r, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}

	st, err := r.Stat()
	if err != nil {
		logError(testName, function, args, startTime, "", "Stat failed", err)
		return
	}
	if st.Size != int64(bufSize) {
		logError(testName, function, args, startTime, "", "Number of bytes returned by PutObject does not match GetObject, expected "+string(bufSize)+" got "+string(st.Size), err)
		return
	}
	if st.ContentType != customContentType && st.ContentType != "application/octet-stream" {
		logError(testName, function, args, startTime, "", "ContentType does not match, expected "+customContentType+" got "+st.ContentType, err)
		return
	}
	if err := crcMatchesName(r, "datafile-129-MB"); err != nil {
		logError(testName, function, args, startTime, "", "data CRC check failed", err)
		return
	}
	if err := r.Close(); err != nil {
		logError(testName, function, args, startTime, "", "Object Close failed", err)
		return
	}
	if err := r.Close(); err == nil {
		logError(testName, function, args, startTime, "", "Object already closed, should respond with error", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

func testPutObjectWithContentLanguage() {
	// initialize logging params
	objectName := "test-object"
	startTime := time.Now()
	testName := getFuncName()
	function := "PutObject(bucketName, objectName, reader, size, opts)"
	args := map[string]interface{}{
		"bucketName": "",
		"objectName": objectName,
		"size":       -1,
		"opts":       "",
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName
	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	data := []byte{}
	_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(data), int64(0), minio.PutObjectOptions{
		ContentLanguage: "en",
	})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	objInfo, err := c.StatObject(context.Background(), bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject failed", err)
		return
	}

	if objInfo.Metadata.Get("Content-Language") != "en" {
		logError(testName, function, args, startTime, "", "Expected content-language 'en' doesn't match with StatObject return value", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Test put object with streaming signature.
func testPutObjectStreaming() {
	// initialize logging params
	objectName := "test-object"
	startTime := time.Now()
	testName := getFuncName()
	function := "PutObject(bucketName, objectName, reader,size,opts)"
	args := map[string]interface{}{
		"bucketName": "",
		"objectName": objectName,
		"size":       -1,
		"opts":       "",
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName
	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Upload an object.
	sizes := []int64{0, 64*1024 - 1, 64 * 1024}

	for _, size := range sizes {
		data := newRandomReader(size, size)
		ui, err := c.PutObject(context.Background(), bucketName, objectName, data, int64(size), minio.PutObjectOptions{})
		if err != nil {
			logError(testName, function, args, startTime, "", "PutObjectStreaming failed", err)
			return
		}

		if ui.Size != size {
			logError(testName, function, args, startTime, "", "PutObjectStreaming result has unexpected size", nil)
			return
		}

		objInfo, err := c.StatObject(context.Background(), bucketName, objectName, minio.StatObjectOptions{})
		if err != nil {
			logError(testName, function, args, startTime, "", "StatObject failed", err)
			return
		}
		if objInfo.Size != size {
			logError(testName, function, args, startTime, "", "Unexpected size", err)
			return
		}

	}

	successLogger(testName, function, args, startTime).Info()
}

// Test get object seeker from the end, using whence set to '2'.
func testGetObjectSeekEnd() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "GetObject(bucketName, objectName)"
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Generate 33K of data.
	bufSize := dataFileMap["datafile-33-kB"]
	var reader = getDataReader("datafile-33-kB")
	defer reader.Close()

	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	buf, err := ioutil.ReadAll(reader)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAll failed", err)
		return
	}

	_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	// Read the data back
	r, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}

	st, err := r.Stat()
	if err != nil {
		logError(testName, function, args, startTime, "", "Stat failed", err)
		return
	}

	if st.Size != int64(bufSize) {
		logError(testName, function, args, startTime, "", "Number of bytes read does not match, expected "+string(int64(bufSize))+" got "+string(st.Size), err)
		return
	}

	pos, err := r.Seek(-100, 2)
	if err != nil {
		logError(testName, function, args, startTime, "", "Object Seek failed", err)
		return
	}
	if pos != st.Size-100 {
		logError(testName, function, args, startTime, "", "Incorrect position", err)
		return
	}
	buf2 := make([]byte, 100)
	m, err := readFull(r, buf2)
	if err != nil {
		logError(testName, function, args, startTime, "", "Error reading through readFull", err)
		return
	}
	if m != len(buf2) {
		logError(testName, function, args, startTime, "", "Number of bytes dont match, expected "+string(len(buf2))+" got "+string(m), err)
		return
	}
	hexBuf1 := fmt.Sprintf("%02x", buf[len(buf)-100:])
	hexBuf2 := fmt.Sprintf("%02x", buf2[:m])
	if hexBuf1 != hexBuf2 {
		logError(testName, function, args, startTime, "", "Values at same index dont match", err)
		return
	}
	pos, err = r.Seek(-100, 2)
	if err != nil {
		logError(testName, function, args, startTime, "", "Object Seek failed", err)
		return
	}
	if pos != st.Size-100 {
		logError(testName, function, args, startTime, "", "Incorrect position", err)
		return
	}
	if err = r.Close(); err != nil {
		logError(testName, function, args, startTime, "", "ObjectClose failed", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Test get object reader to not throw error on being closed twice.
func testGetObjectClosedTwice() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "GetObject(bucketName, objectName)"
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Generate 33K of data.
	bufSize := dataFileMap["datafile-33-kB"]
	var reader = getDataReader("datafile-33-kB")
	defer reader.Close()

	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	_, err = c.PutObject(context.Background(), bucketName, objectName, reader, int64(bufSize), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	// Read the data back
	r, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}

	st, err := r.Stat()
	if err != nil {
		logError(testName, function, args, startTime, "", "Stat failed", err)
		return
	}
	if st.Size != int64(bufSize) {
		logError(testName, function, args, startTime, "", "Number of bytes in stat does not match, expected "+string(int64(bufSize))+" got "+string(st.Size), err)
		return
	}
	if err := crcMatchesName(r, "datafile-33-kB"); err != nil {
		logError(testName, function, args, startTime, "", "data CRC check failed", err)
		return
	}
	if err := r.Close(); err != nil {
		logError(testName, function, args, startTime, "", "Object Close failed", err)
		return
	}
	if err := r.Close(); err == nil {
		logError(testName, function, args, startTime, "", "Already closed object. No error returned", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Test RemoveObjects request where context cancels after timeout
func testRemoveObjectsContext() {
	// Initialize logging params.
	startTime := time.Now()
	testName := getFuncName()
	function := "RemoveObjects(ctx, bucketName, objectsCh)"
	args := map[string]interface{}{
		"bucketName": "",
	}

	// Seed random based on current tie.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")
	// Enable tracing, write to stdout.
	// c.TraceOn(os.Stderr)

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Generate put data.
	r := bytes.NewReader(bytes.Repeat([]byte("a"), 8))

	// Multi remove of 20 objects.
	nrObjects := 20
	objectsCh := make(chan minio.ObjectInfo)
	go func() {
		defer close(objectsCh)
		for i := 0; i < nrObjects; i++ {
			objectName := "sample" + strconv.Itoa(i) + ".txt"
			info, err := c.PutObject(context.Background(), bucketName, objectName, r, 8,
				minio.PutObjectOptions{ContentType: "application/octet-stream"})
			if err != nil {
				logError(testName, function, args, startTime, "", "PutObject failed", err)
				continue
			}
			objectsCh <- minio.ObjectInfo{
				Key:       info.Key,
				VersionID: info.VersionID,
			}
		}
	}()
	// Set context to cancel in 1 nanosecond.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	args["ctx"] = ctx
	defer cancel()

	// Call RemoveObjects API with short timeout.
	errorCh := c.RemoveObjects(ctx, bucketName, objectsCh, minio.RemoveObjectsOptions{})
	// Check for error.
	select {
	case r := <-errorCh:
		if r.Err == nil {
			logError(testName, function, args, startTime, "", "RemoveObjects should fail on short timeout", err)
			return
		}
	}
	// Set context with longer timeout.
	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Hour)
	args["ctx"] = ctx
	defer cancel()
	// Perform RemoveObjects with the longer timeout. Expect the removals to succeed.
	errorCh = c.RemoveObjects(ctx, bucketName, objectsCh, minio.RemoveObjectsOptions{})
	select {
	case r, more := <-errorCh:
		if more || r.Err != nil {
			logError(testName, function, args, startTime, "", "Unexpected error", r.Err)
			return
		}
	}

	successLogger(testName, function, args, startTime).Info()
}

// Test removing multiple objects with Remove API
func testRemoveMultipleObjects() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "RemoveObjects(bucketName, objectsCh)"
	args := map[string]interface{}{
		"bucketName": "",
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Enable tracing, write to stdout.
	// c.TraceOn(os.Stderr)

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	r := bytes.NewReader(bytes.Repeat([]byte("a"), 8))

	// Multi remove of 1100 objects
	nrObjects := 200

	objectsCh := make(chan minio.ObjectInfo)

	go func() {
		defer close(objectsCh)
		// Upload objects and send them to objectsCh
		for i := 0; i < nrObjects; i++ {
			objectName := "sample" + strconv.Itoa(i) + ".txt"
			info, err := c.PutObject(context.Background(), bucketName, objectName, r, 8,
				minio.PutObjectOptions{ContentType: "application/octet-stream"})
			if err != nil {
				logError(testName, function, args, startTime, "", "PutObject failed", err)
				continue
			}
			objectsCh <- minio.ObjectInfo{
				Key:       info.Key,
				VersionID: info.VersionID,
			}
		}
	}()

	// Call RemoveObjects API
	errorCh := c.RemoveObjects(context.Background(), bucketName, objectsCh, minio.RemoveObjectsOptions{})

	// Check if errorCh doesn't receive any error
	select {
	case r, more := <-errorCh:
		if more {
			logError(testName, function, args, startTime, "", "Unexpected error", r.Err)
			return
		}
	}

	successLogger(testName, function, args, startTime).Info()
}

// Tests FPutObject of a big file to trigger multipart
func testFPutObjectMultipart() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "FPutObject(bucketName, objectName, fileName, opts)"
	args := map[string]interface{}{
		"bucketName": "",
		"objectName": "",
		"fileName":   "",
		"opts":       "",
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Upload 4 parts to utilize all 3 'workers' in multipart and still have a part to upload.
	var fileName = getMintDataDirFilePath("datafile-129-MB")
	if fileName == "" {
		// Make a temp file with minPartSize bytes of data.
		file, err := ioutil.TempFile(os.TempDir(), "FPutObjectTest")
		if err != nil {
			logError(testName, function, args, startTime, "", "TempFile creation failed", err)
			return
		}
		// Upload 2 parts to utilize all 3 'workers' in multipart and still have a part to upload.
		if _, err = io.Copy(file, getDataReader("datafile-129-MB")); err != nil {
			logError(testName, function, args, startTime, "", "Copy failed", err)
			return
		}
		if err = file.Close(); err != nil {
			logError(testName, function, args, startTime, "", "File Close failed", err)
			return
		}
		fileName = file.Name()
		args["fileName"] = fileName
	}
	totalSize := dataFileMap["datafile-129-MB"]
	// Set base object name
	objectName := bucketName + "FPutObject" + "-standard"
	args["objectName"] = objectName

	objectContentType := "testapplication/octet-stream"
	args["objectContentType"] = objectContentType

	// Perform standard FPutObject with contentType provided (Expecting application/octet-stream)
	_, err = c.FPutObject(context.Background(), bucketName, objectName, fileName, minio.PutObjectOptions{ContentType: objectContentType})
	if err != nil {
		logError(testName, function, args, startTime, "", "FPutObject failed", err)
		return
	}

	r, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}
	objInfo, err := r.Stat()
	if err != nil {
		logError(testName, function, args, startTime, "", "Unexpected error", err)
		return
	}
	if objInfo.Size != int64(totalSize) {
		logError(testName, function, args, startTime, "", "Number of bytes does not match, expected "+string(int64(totalSize))+" got "+string(objInfo.Size), err)
		return
	}
	if objInfo.ContentType != objectContentType && objInfo.ContentType != "application/octet-stream" {
		logError(testName, function, args, startTime, "", "ContentType doesn't match", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Tests FPutObject with null contentType (default = application/octet-stream)
func testFPutObject() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "FPutObject(bucketName, objectName, fileName, opts)"

	args := map[string]interface{}{
		"bucketName": "",
		"objectName": "",
		"fileName":   "",
		"opts":       "",
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	location := "us-east-1"

	// Make a new bucket.
	args["bucketName"] = bucketName
	args["location"] = location
	function = "MakeBucket(bucketName, location)"
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: location})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Upload 3 parts worth of data to use all 3 of multiparts 'workers' and have an extra part.
	// Use different data in part for multipart tests to check parts are uploaded in correct order.
	var fName = getMintDataDirFilePath("datafile-129-MB")
	if fName == "" {
		// Make a temp file with minPartSize bytes of data.
		file, err := ioutil.TempFile(os.TempDir(), "FPutObjectTest")
		if err != nil {
			logError(testName, function, args, startTime, "", "TempFile creation failed", err)
			return
		}

		// Upload 3 parts to utilize all 3 'workers' in multipart and still have a part to upload.
		if _, err = io.Copy(file, getDataReader("datafile-129-MB")); err != nil {
			logError(testName, function, args, startTime, "", "File copy failed", err)
			return
		}
		// Close the file pro-actively for windows.
		if err = file.Close(); err != nil {
			logError(testName, function, args, startTime, "", "File close failed", err)
			return
		}
		defer os.Remove(file.Name())
		fName = file.Name()
	}

	// Set base object name
	function = "FPutObject(bucketName, objectName, fileName, opts)"
	objectName := bucketName + "FPutObject"
	args["objectName"] = objectName + "-standard"
	args["fileName"] = fName
	args["opts"] = minio.PutObjectOptions{ContentType: "application/octet-stream"}

	// Perform standard FPutObject with contentType provided (Expecting application/octet-stream)
	ui, err := c.FPutObject(context.Background(), bucketName, objectName+"-standard", fName, minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "FPutObject failed", err)
		return
	}

	if ui.Size != int64(dataFileMap["datafile-129-MB"]) {
		logError(testName, function, args, startTime, "", "FPutObject returned an unexpected upload size", err)
		return
	}

	// Perform FPutObject with no contentType provided (Expecting application/octet-stream)
	args["objectName"] = objectName + "-Octet"
	_, err = c.FPutObject(context.Background(), bucketName, objectName+"-Octet", fName, minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "File close failed", err)
		return
	}

	srcFile, err := os.Open(fName)
	if err != nil {
		logError(testName, function, args, startTime, "", "File open failed", err)
		return
	}
	defer srcFile.Close()
	// Add extension to temp file name
	tmpFile, err := os.Create(fName + ".gtar")
	if err != nil {
		logError(testName, function, args, startTime, "", "File create failed", err)
		return
	}
	_, err = io.Copy(tmpFile, srcFile)
	if err != nil {
		logError(testName, function, args, startTime, "", "File copy failed", err)
		return
	}
	tmpFile.Close()

	// Perform FPutObject with no contentType provided (Expecting application/x-gtar)
	args["objectName"] = objectName + "-GTar"
	args["opts"] = minio.PutObjectOptions{}
	_, err = c.FPutObject(context.Background(), bucketName, objectName+"-GTar", fName+".gtar", minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "FPutObject failed", err)
		return
	}

	// Check headers
	function = "StatObject(bucketName, objectName, opts)"
	args["objectName"] = objectName + "-standard"
	rStandard, err := c.StatObject(context.Background(), bucketName, objectName+"-standard", minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject failed", err)
		return
	}
	if rStandard.ContentType != "application/octet-stream" {
		logError(testName, function, args, startTime, "", "ContentType does not match, expected application/octet-stream, got "+rStandard.ContentType, err)
		return
	}

	function = "StatObject(bucketName, objectName, opts)"
	args["objectName"] = objectName + "-Octet"
	rOctet, err := c.StatObject(context.Background(), bucketName, objectName+"-Octet", minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject failed", err)
		return
	}
	if rOctet.ContentType != "application/octet-stream" {
		logError(testName, function, args, startTime, "", "ContentType does not match, expected application/octet-stream, got "+rOctet.ContentType, err)
		return
	}

	function = "StatObject(bucketName, objectName, opts)"
	args["objectName"] = objectName + "-GTar"
	rGTar, err := c.StatObject(context.Background(), bucketName, objectName+"-GTar", minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject failed", err)
		return
	}
	if rGTar.ContentType != "application/x-gtar" && rGTar.ContentType != "application/octet-stream" {
		logError(testName, function, args, startTime, "", "ContentType does not match, expected application/x-gtar or application/octet-stream, got "+rGTar.ContentType, err)
		return
	}

	os.Remove(fName + ".gtar")
	successLogger(testName, function, args, startTime).Info()
}

// Tests FPutObject request when context cancels after timeout
func testFPutObjectContext() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "FPutObject(bucketName, objectName, fileName, opts)"
	args := map[string]interface{}{
		"bucketName": "",
		"objectName": "",
		"fileName":   "",
		"opts":       "",
	}
	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Upload 1 parts worth of data to use multipart upload.
	// Use different data in part for multipart tests to check parts are uploaded in correct order.
	var fName = getMintDataDirFilePath("datafile-1-MB")
	if fName == "" {
		// Make a temp file with 1 MiB bytes of data.
		file, err := ioutil.TempFile(os.TempDir(), "FPutObjectContextTest")
		if err != nil {
			logError(testName, function, args, startTime, "", "TempFile creation failed", err)
			return
		}

		// Upload 1 parts to trigger multipart upload
		if _, err = io.Copy(file, getDataReader("datafile-1-MB")); err != nil {
			logError(testName, function, args, startTime, "", "File copy failed", err)
			return
		}
		// Close the file pro-actively for windows.
		if err = file.Close(); err != nil {
			logError(testName, function, args, startTime, "", "File close failed", err)
			return
		}
		defer os.Remove(file.Name())
		fName = file.Name()
	}

	// Set base object name
	objectName := bucketName + "FPutObjectContext"
	args["objectName"] = objectName
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	args["ctx"] = ctx
	defer cancel()

	// Perform FPutObject with contentType provided (Expecting application/octet-stream)
	_, err = c.FPutObject(ctx, bucketName, objectName+"-Shorttimeout", fName, minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err == nil {
		logError(testName, function, args, startTime, "", "FPutObject should fail on short timeout", err)
		return
	}
	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()
	// Perform FPutObject with a long timeout. Expect the put object to succeed
	_, err = c.FPutObject(ctx, bucketName, objectName+"-Longtimeout", fName, minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "FPutObject shouldn't fail on long timeout", err)
		return
	}

	_, err = c.StatObject(context.Background(), bucketName, objectName+"-Longtimeout", minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject failed", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()

}

// Tests FPutObject request when context cancels after timeout
func testFPutObjectContextV2() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "FPutObjectContext(ctx, bucketName, objectName, fileName, opts)"
	args := map[string]interface{}{
		"bucketName": "",
		"objectName": "",
		"opts":       "minio.PutObjectOptions{ContentType:objectContentType}",
	}
	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV2(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Upload 1 parts worth of data to use multipart upload.
	// Use different data in part for multipart tests to check parts are uploaded in correct order.
	var fName = getMintDataDirFilePath("datafile-1-MB")
	if fName == "" {
		// Make a temp file with 1 MiB bytes of data.
		file, err := ioutil.TempFile(os.TempDir(), "FPutObjectContextTest")
		if err != nil {
			logError(testName, function, args, startTime, "", "Temp file creation failed", err)
			return
		}

		// Upload 1 parts to trigger multipart upload
		if _, err = io.Copy(file, getDataReader("datafile-1-MB")); err != nil {
			logError(testName, function, args, startTime, "", "File copy failed", err)
			return
		}

		// Close the file pro-actively for windows.
		if err = file.Close(); err != nil {
			logError(testName, function, args, startTime, "", "File close failed", err)
			return
		}
		defer os.Remove(file.Name())
		fName = file.Name()
	}

	// Set base object name
	objectName := bucketName + "FPutObjectContext"
	args["objectName"] = objectName

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	args["ctx"] = ctx
	defer cancel()

	// Perform FPutObject with contentType provided (Expecting application/octet-stream)
	_, err = c.FPutObject(ctx, bucketName, objectName+"-Shorttimeout", fName, minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err == nil {
		logError(testName, function, args, startTime, "", "FPutObject should fail on short timeout", err)
		return
	}
	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()
	// Perform FPutObject with a long timeout. Expect the put object to succeed
	_, err = c.FPutObject(ctx, bucketName, objectName+"-Longtimeout", fName, minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "FPutObject shouldn't fail on longer timeout", err)
		return
	}

	_, err = c.StatObject(context.Background(), bucketName, objectName+"-Longtimeout", minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject failed", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()

}

// Test validates putObject with context to see if request cancellation is honored.
func testPutObjectContext() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "PutObject(ctx, bucketName, objectName, fileName, opts)"
	args := map[string]interface{}{
		"ctx":        "",
		"bucketName": "",
		"objectName": "",
		"opts":       "",
	}

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Make a new bucket.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket call failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	bufSize := dataFileMap["datafile-33-kB"]
	var reader = getDataReader("datafile-33-kB")
	defer reader.Close()
	objectName := fmt.Sprintf("test-file-%v", rand.Uint32())
	args["objectName"] = objectName

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	cancel()
	args["ctx"] = ctx
	args["opts"] = minio.PutObjectOptions{ContentType: "binary/octet-stream"}

	_, err = c.PutObject(ctx, bucketName, objectName, reader, int64(bufSize), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err == nil {
		logError(testName, function, args, startTime, "", "PutObject should fail on short timeout", err)
		return
	}

	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Hour)
	args["ctx"] = ctx

	defer cancel()
	reader = getDataReader("datafile-33-kB")
	defer reader.Close()
	_, err = c.PutObject(ctx, bucketName, objectName, reader, int64(bufSize), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject with long timeout failed", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()

}

// Tests get object ReaderSeeker interface methods.
func testGetObjectReadSeekFunctional() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "GetObject(bucketName, objectName)"
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer func() {
		// Delete all objects and buckets
		if err = cleanupBucket(bucketName, c); err != nil {
			logError(testName, function, args, startTime, "", "CleanupBucket failed", err)
			return
		}
	}()

	// Generate 33K of data.
	bufSize := dataFileMap["datafile-33-kB"]
	var reader = getDataReader("datafile-33-kB")
	defer reader.Close()

	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	buf, err := ioutil.ReadAll(reader)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAll failed", err)
		return
	}

	// Save the data
	_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	// Read the data back
	r, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}

	st, err := r.Stat()
	if err != nil {
		logError(testName, function, args, startTime, "", "Stat object failed", err)
		return
	}

	if st.Size != int64(bufSize) {
		logError(testName, function, args, startTime, "", "Number of bytes does not match, expected "+string(int64(bufSize))+", got "+string(st.Size), err)
		return
	}

	// This following function helps us to compare data from the reader after seek
	// with the data from the original buffer
	cmpData := func(r io.Reader, start, end int) {
		if end-start == 0 {
			return
		}
		buffer := bytes.NewBuffer([]byte{})
		if _, err := io.CopyN(buffer, r, int64(bufSize)); err != nil {
			if err != io.EOF {
				logError(testName, function, args, startTime, "", "CopyN failed", err)
				return
			}
		}
		if !bytes.Equal(buf[start:end], buffer.Bytes()) {
			logError(testName, function, args, startTime, "", "Incorrect read bytes v/s original buffer", err)
			return
		}
	}

	// Generic seek error for errors other than io.EOF
	seekErr := errors.New("seek error")

	testCases := []struct {
		offset    int64
		whence    int
		pos       int64
		err       error
		shouldCmp bool
		start     int
		end       int
	}{
		// Start from offset 0, fetch data and compare
		{0, 0, 0, nil, true, 0, 0},
		// Start from offset 2048, fetch data and compare
		{2048, 0, 2048, nil, true, 2048, bufSize},
		// Start from offset larger than possible
		{int64(bufSize) + 1024, 0, 0, seekErr, false, 0, 0},
		// Move to offset 0 without comparing
		{0, 0, 0, nil, false, 0, 0},
		// Move one step forward and compare
		{1, 1, 1, nil, true, 1, bufSize},
		// Move larger than possible
		{int64(bufSize), 1, 0, seekErr, false, 0, 0},
		// Provide negative offset with CUR_SEEK
		{int64(-1), 1, 0, seekErr, false, 0, 0},
		// Test with whence SEEK_END and with positive offset
		{1024, 2, int64(bufSize) - 1024, io.EOF, true, 0, 0},
		// Test with whence SEEK_END and with negative offset
		{-1024, 2, int64(bufSize) - 1024, nil, true, bufSize - 1024, bufSize},
		// Test with whence SEEK_END and with large negative offset
		{-int64(bufSize) * 2, 2, 0, seekErr, true, 0, 0},
	}

	for i, testCase := range testCases {
		// Perform seek operation
		n, err := r.Seek(testCase.offset, testCase.whence)
		// We expect an error
		if testCase.err == seekErr && err == nil {
			logError(testName, function, args, startTime, "", "Test "+string(i+1)+", unexpected err value: expected: "+testCase.err.Error()+", found: "+err.Error(), err)
			return
		}
		// We expect a specific error
		if testCase.err != seekErr && testCase.err != err {
			logError(testName, function, args, startTime, "", "Test "+string(i+1)+", unexpected err value: expected: "+testCase.err.Error()+", found: "+err.Error(), err)
			return
		}
		// If we expect an error go to the next loop
		if testCase.err != nil {
			continue
		}
		// Check the returned seek pos
		if n != testCase.pos {
			logError(testName, function, args, startTime, "", "Test "+string(i+1)+", number of bytes seeked does not match, expected "+string(testCase.pos)+", got "+string(n), err)
			return
		}
		// Compare only if shouldCmp is activated
		if testCase.shouldCmp {
			cmpData(r, testCase.start, testCase.end)
		}
	}
	successLogger(testName, function, args, startTime).Info()
}

// Tests get object ReaderAt interface methods.
func testGetObjectReadAtFunctional() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "GetObject(bucketName, objectName)"
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Generate 33K of data.
	bufSize := dataFileMap["datafile-33-kB"]
	var reader = getDataReader("datafile-33-kB")
	defer reader.Close()

	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	buf, err := ioutil.ReadAll(reader)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAll failed", err)
		return
	}

	// Save the data
	_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	// read the data back
	r, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}
	offset := int64(2048)

	// read directly
	buf1 := make([]byte, 512)
	buf2 := make([]byte, 512)
	buf3 := make([]byte, 512)
	buf4 := make([]byte, 512)

	// Test readAt before stat is called such that objectInfo doesn't change.
	m, err := r.ReadAt(buf1, offset)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAt failed", err)
		return
	}
	if m != len(buf1) {
		logError(testName, function, args, startTime, "", "ReadAt read shorter bytes before reaching EOF, expected "+string(len(buf1))+", got "+string(m), err)
		return
	}
	if !bytes.Equal(buf1, buf[offset:offset+512]) {
		logError(testName, function, args, startTime, "", "Incorrect read between two ReadAt from same offset", err)
		return
	}
	offset += 512

	st, err := r.Stat()
	if err != nil {
		logError(testName, function, args, startTime, "", "Stat failed", err)
		return
	}

	if st.Size != int64(bufSize) {
		logError(testName, function, args, startTime, "", "Number of bytes in stat does not match, expected "+string(int64(bufSize))+", got "+string(st.Size), err)
		return
	}

	m, err = r.ReadAt(buf2, offset)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAt failed", err)
		return
	}
	if m != len(buf2) {
		logError(testName, function, args, startTime, "", "ReadAt read shorter bytes before reaching EOF, expected "+string(len(buf2))+", got "+string(m), err)
		return
	}
	if !bytes.Equal(buf2, buf[offset:offset+512]) {
		logError(testName, function, args, startTime, "", "Incorrect read between two ReadAt from same offset", err)
		return
	}

	offset += 512
	m, err = r.ReadAt(buf3, offset)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAt failed", err)
		return
	}
	if m != len(buf3) {
		logError(testName, function, args, startTime, "", "ReadAt read shorter bytes before reaching EOF, expected "+string(len(buf3))+", got "+string(m), err)
		return
	}
	if !bytes.Equal(buf3, buf[offset:offset+512]) {
		logError(testName, function, args, startTime, "", "Incorrect read between two ReadAt from same offset", err)
		return
	}
	offset += 512
	m, err = r.ReadAt(buf4, offset)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAt failed", err)
		return
	}
	if m != len(buf4) {
		logError(testName, function, args, startTime, "", "ReadAt read shorter bytes before reaching EOF, expected "+string(len(buf4))+", got "+string(m), err)
		return
	}
	if !bytes.Equal(buf4, buf[offset:offset+512]) {
		logError(testName, function, args, startTime, "", "Incorrect read between two ReadAt from same offset", err)
		return
	}

	buf5 := make([]byte, len(buf))
	// Read the whole object.
	m, err = r.ReadAt(buf5, 0)
	if err != nil {
		if err != io.EOF {
			logError(testName, function, args, startTime, "", "ReadAt failed", err)
			return
		}
	}
	if m != len(buf5) {
		logError(testName, function, args, startTime, "", "ReadAt read shorter bytes before reaching EOF, expected "+string(len(buf5))+", got "+string(m), err)
		return
	}
	if !bytes.Equal(buf, buf5) {
		logError(testName, function, args, startTime, "", "Incorrect data read in GetObject, than what was previously uploaded", err)
		return
	}

	buf6 := make([]byte, len(buf)+1)
	// Read the whole object and beyond.
	_, err = r.ReadAt(buf6, 0)
	if err != nil {
		if err != io.EOF {
			logError(testName, function, args, startTime, "", "ReadAt failed", err)
			return
		}
	}

	successLogger(testName, function, args, startTime).Info()
}

// Reproduces issue https://github.com/minio/minio-go/issues/1137
func testGetObjectReadAtWhenEOFWasReached() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "GetObject(bucketName, objectName)"
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Generate 33K of data.
	bufSize := dataFileMap["datafile-33-kB"]
	var reader = getDataReader("datafile-33-kB")
	defer reader.Close()

	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	buf, err := ioutil.ReadAll(reader)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAll failed", err)
		return
	}

	// Save the data
	_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	// read the data back
	r, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	// read directly
	buf1 := make([]byte, len(buf))
	buf2 := make([]byte, 512)

	m, err := r.Read(buf1)
	if err != nil {
		if err != io.EOF {
			logError(testName, function, args, startTime, "", "Read failed", err)
			return
		}
	}
	if m != len(buf1) {
		logError(testName, function, args, startTime, "", "Read read shorter bytes before reaching EOF, expected "+string(len(buf1))+", got "+string(m), err)
		return
	}
	if !bytes.Equal(buf1, buf) {
		logError(testName, function, args, startTime, "", "Incorrect count of Read data", err)
		return
	}

	st, err := r.Stat()
	if err != nil {
		logError(testName, function, args, startTime, "", "Stat failed", err)
		return
	}

	if st.Size != int64(bufSize) {
		logError(testName, function, args, startTime, "", "Number of bytes in stat does not match, expected "+string(int64(bufSize))+", got "+string(st.Size), err)
		return
	}

	m, err = r.ReadAt(buf2, 512)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAt failed", err)
		return
	}
	if m != len(buf2) {
		logError(testName, function, args, startTime, "", "ReadAt read shorter bytes before reaching EOF, expected "+string(len(buf2))+", got "+string(m), err)
		return
	}
	if !bytes.Equal(buf2, buf[512:1024]) {
		logError(testName, function, args, startTime, "", "Incorrect count of ReadAt data", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Test Presigned Post Policy
func testPresignedPostPolicy() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "PresignedPostPolicy(policy)"
	args := map[string]interface{}{
		"policy": "",
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")

	// Make a new bucket in 'us-east-1' (source bucket).
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Generate 33K of data.
	var reader = getDataReader("datafile-33-kB")
	defer reader.Close()

	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	// Azure requires the key to not start with a number
	metadataKey := randString(60, rand.NewSource(time.Now().UnixNano()), "user")
	metadataValue := randString(60, rand.NewSource(time.Now().UnixNano()), "")

	buf, err := ioutil.ReadAll(reader)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAll failed", err)
		return
	}

	// Save the data
	_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	policy := minio.NewPostPolicy()

	if err := policy.SetBucket(""); err == nil {
		logError(testName, function, args, startTime, "", "SetBucket did not fail for invalid conditions", err)
		return
	}
	if err := policy.SetKey(""); err == nil {
		logError(testName, function, args, startTime, "", "SetKey did not fail for invalid conditions", err)
		return
	}
	if err := policy.SetKeyStartsWith(""); err == nil {
		logError(testName, function, args, startTime, "", "SetKeyStartsWith did not fail for invalid conditions", err)
		return
	}
	if err := policy.SetExpires(time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)); err == nil {
		logError(testName, function, args, startTime, "", "SetExpires did not fail for invalid conditions", err)
		return
	}
	if err := policy.SetContentType(""); err == nil {
		logError(testName, function, args, startTime, "", "SetContentType did not fail for invalid conditions", err)
		return
	}
	if err := policy.SetContentTypeStartsWith(""); err == nil {
		logError(testName, function, args, startTime, "", "SetContentTypeStartsWith did not fail for invalid conditions", err)
		return
	}
	if err := policy.SetContentLengthRange(1024*1024, 1024); err == nil {
		logError(testName, function, args, startTime, "", "SetContentLengthRange did not fail for invalid conditions", err)
		return
	}
	if err := policy.SetUserMetadata("", ""); err == nil {
		logError(testName, function, args, startTime, "", "SetUserMetadata did not fail for invalid conditions", err)
		return
	}

	policy.SetBucket(bucketName)
	policy.SetKey(objectName)
	policy.SetExpires(time.Now().UTC().AddDate(0, 0, 10)) // expires in 10 days
	policy.SetContentType("binary/octet-stream")
	policy.SetContentLengthRange(10, 1024*1024)
	policy.SetUserMetadata(metadataKey, metadataValue)
	args["policy"] = policy.String()

	presignedPostPolicyURL, formData, err := c.PresignedPostPolicy(context.Background(), policy)
	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedPostPolicy failed", err)
		return
	}

	var formBuf bytes.Buffer
	writer := multipart.NewWriter(&formBuf)
	for k, v := range formData {
		writer.WriteField(k, v)
	}

	// Get a 33KB file to upload and test if set post policy works
	var filePath = getMintDataDirFilePath("datafile-33-kB")
	if filePath == "" {
		// Make a temp file with 33 KB data.
		file, err := ioutil.TempFile(os.TempDir(), "PresignedPostPolicyTest")
		if err != nil {
			logError(testName, function, args, startTime, "", "TempFile creation failed", err)
			return
		}
		if _, err = io.Copy(file, getDataReader("datafile-33-kB")); err != nil {
			logError(testName, function, args, startTime, "", "Copy failed", err)
			return
		}
		if err = file.Close(); err != nil {
			logError(testName, function, args, startTime, "", "File Close failed", err)
			return
		}
		filePath = file.Name()
	}

	// add file to post request
	f, err := os.Open(filePath)
	defer f.Close()
	if err != nil {
		logError(testName, function, args, startTime, "", "File open failed", err)
		return
	}
	w, err := writer.CreateFormFile("file", filePath)
	if err != nil {
		logError(testName, function, args, startTime, "", "CreateFormFile failed", err)
		return
	}

	_, err = io.Copy(w, f)
	if err != nil {
		logError(testName, function, args, startTime, "", "Copy failed", err)
		return
	}
	writer.Close()

	transport, err := minio.DefaultTransport(mustParseBool(os.Getenv(enableHTTPS)))
	if err != nil {
		logError(testName, function, args, startTime, "", "DefaultTransport failed", err)
		return
	}

	httpClient := &http.Client{
		// Setting a sensible time out of 30secs to wait for response
		// headers. Request is pro-actively canceled after 30secs
		// with no response.
		Timeout:   30 * time.Second,
		Transport: transport,
	}

	req, err := http.NewRequest(http.MethodPost, presignedPostPolicyURL.String(), bytes.NewReader(formBuf.Bytes()))
	if err != nil {
		logError(testName, function, args, startTime, "", "Http request failed", err)
		return
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// make post request with correct form data
	res, err := httpClient.Do(req)
	if err != nil {
		logError(testName, function, args, startTime, "", "Http request failed", err)
		return
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		logError(testName, function, args, startTime, "", "Http request failed", errors.New(res.Status))
		return
	}

	// expected path should be absolute path of the object
	var scheme string
	if mustParseBool(os.Getenv(enableHTTPS)) {
		scheme = "https://"
	} else {
		scheme = "http://"
	}

	expectedLocation := scheme + os.Getenv(serverEndpoint) + "/" + bucketName + "/" + objectName
	expectedLocationBucketDNS := scheme + bucketName + "." + os.Getenv(serverEndpoint) + "/" + objectName

	if val, ok := res.Header["Location"]; ok {
		if val[0] != expectedLocation && val[0] != expectedLocationBucketDNS {
			logError(testName, function, args, startTime, "", "Location in header response is incorrect", err)
			return
		}
	} else {
		logError(testName, function, args, startTime, "", "Location not found in header response", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Tests copy object
func testCopyObject() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObject(dst, src)"
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")

	// Make a new bucket in 'us-east-1' (source bucket).
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Make a new bucket in 'us-east-1' (destination bucket).
	err = c.MakeBucket(context.Background(), bucketName+"-copy", minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}
	defer cleanupBucket(bucketName+"-copy", c)

	// Generate 33K of data.
	bufSize := dataFileMap["datafile-33-kB"]
	var reader = getDataReader("datafile-33-kB")

	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	_, err = c.PutObject(context.Background(), bucketName, objectName, reader, int64(bufSize), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	r, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}
	// Check the various fields of source object against destination object.
	objInfo, err := r.Stat()
	if err != nil {
		logError(testName, function, args, startTime, "", "Stat failed", err)
		return
	}

	// Copy Source
	src := minio.CopySrcOptions{
		Bucket: bucketName,
		Object: objectName,
		// Set copy conditions.
		MatchETag:          objInfo.ETag,
		MatchModifiedSince: time.Date(2014, time.April, 0, 0, 0, 0, 0, time.UTC),
	}
	args["src"] = src

	dst := minio.CopyDestOptions{
		Bucket: bucketName + "-copy",
		Object: objectName + "-copy",
	}

	// Perform the Copy
	if _, err = c.CopyObject(context.Background(), dst, src); err != nil {
		logError(testName, function, args, startTime, "", "CopyObject failed", err)
		return
	}

	// Source object
	r, err = c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}

	// Destination object
	readerCopy, err := c.GetObject(context.Background(), bucketName+"-copy", objectName+"-copy", minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}

	// Check the various fields of source object against destination object.
	objInfo, err = r.Stat()
	if err != nil {
		logError(testName, function, args, startTime, "", "Stat failed", err)
		return
	}
	objInfoCopy, err := readerCopy.Stat()
	if err != nil {
		logError(testName, function, args, startTime, "", "Stat failed", err)
		return
	}
	if objInfo.Size != objInfoCopy.Size {
		logError(testName, function, args, startTime, "", "Number of bytes does not match, expected "+string(objInfoCopy.Size)+", got "+string(objInfo.Size), err)
		return
	}

	if err := crcMatchesName(r, "datafile-33-kB"); err != nil {
		logError(testName, function, args, startTime, "", "data CRC check failed", err)
		return
	}
	if err := crcMatchesName(readerCopy, "datafile-33-kB"); err != nil {
		logError(testName, function, args, startTime, "", "copy data CRC check failed", err)
		return
	}
	// Close all the get readers before proceeding with CopyObject operations.
	r.Close()
	readerCopy.Close()

	// CopyObject again but with wrong conditions
	src = minio.CopySrcOptions{
		Bucket:               bucketName,
		Object:               objectName,
		MatchUnmodifiedSince: time.Date(2014, time.April, 0, 0, 0, 0, 0, time.UTC),
		NoMatchETag:          objInfo.ETag,
	}

	// Perform the Copy which should fail
	_, err = c.CopyObject(context.Background(), dst, src)
	if err == nil {
		logError(testName, function, args, startTime, "", "CopyObject did not fail for invalid conditions", err)
		return
	}

	src = minio.CopySrcOptions{
		Bucket: bucketName,
		Object: objectName,
	}

	dst = minio.CopyDestOptions{
		Bucket:          bucketName,
		Object:          objectName,
		ReplaceMetadata: true,
		UserMetadata: map[string]string{
			"Copy": "should be same",
		},
	}
	args["dst"] = dst
	args["src"] = src

	_, err = c.CopyObject(context.Background(), dst, src)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObject shouldn't fail", err)
		return
	}

	oi, err := c.StatObject(context.Background(), bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject failed", err)
		return
	}

	stOpts := minio.StatObjectOptions{}
	stOpts.SetMatchETag(oi.ETag)
	objInfo, err = c.StatObject(context.Background(), bucketName, objectName, stOpts)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObject ETag should match and not fail", err)
		return
	}

	if objInfo.Metadata.Get("x-amz-meta-copy") != "should be same" {
		logError(testName, function, args, startTime, "", "CopyObject modified metadata should match", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Tests SSE-C get object ReaderSeeker interface methods.
func testSSECEncryptedGetObjectReadSeekFunctional() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "GetObject(bucketName, objectName)"
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer func() {
		// Delete all objects and buckets
		if err = cleanupBucket(bucketName, c); err != nil {
			logError(testName, function, args, startTime, "", "CleanupBucket failed", err)
			return
		}
	}()

	// Generate 129MiB of data.
	bufSize := dataFileMap["datafile-129-MB"]
	var reader = getDataReader("datafile-129-MB")
	defer reader.Close()

	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	buf, err := ioutil.ReadAll(reader)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAll failed", err)
		return
	}

	// Save the data
	_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{
		ContentType:          "binary/octet-stream",
		ServerSideEncryption: encrypt.DefaultPBKDF([]byte("correct horse battery staple"), []byte(bucketName+objectName)),
	})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	// Read the data back
	r, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{
		ServerSideEncryption: encrypt.DefaultPBKDF([]byte("correct horse battery staple"), []byte(bucketName+objectName)),
	})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}
	defer r.Close()

	st, err := r.Stat()
	if err != nil {
		logError(testName, function, args, startTime, "", "Stat object failed", err)
		return
	}

	if st.Size != int64(bufSize) {
		logError(testName, function, args, startTime, "", "Number of bytes does not match, expected "+string(int64(bufSize))+", got "+string(st.Size), err)
		return
	}

	// This following function helps us to compare data from the reader after seek
	// with the data from the original buffer
	cmpData := func(r io.Reader, start, end int) {
		if end-start == 0 {
			return
		}
		buffer := bytes.NewBuffer([]byte{})
		if _, err := io.CopyN(buffer, r, int64(bufSize)); err != nil {
			if err != io.EOF {
				logError(testName, function, args, startTime, "", "CopyN failed", err)
				return
			}
		}
		if !bytes.Equal(buf[start:end], buffer.Bytes()) {
			logError(testName, function, args, startTime, "", "Incorrect read bytes v/s original buffer", err)
			return
		}
	}

	testCases := []struct {
		offset    int64
		whence    int
		pos       int64
		err       error
		shouldCmp bool
		start     int
		end       int
	}{
		// Start from offset 0, fetch data and compare
		{0, 0, 0, nil, true, 0, 0},
		// Start from offset 2048, fetch data and compare
		{2048, 0, 2048, nil, true, 2048, bufSize},
		// Start from offset larger than possible
		{int64(bufSize) + 1024, 0, 0, io.EOF, false, 0, 0},
		// Move to offset 0 without comparing
		{0, 0, 0, nil, false, 0, 0},
		// Move one step forward and compare
		{1, 1, 1, nil, true, 1, bufSize},
		// Move larger than possible
		{int64(bufSize), 1, 0, io.EOF, false, 0, 0},
		// Provide negative offset with CUR_SEEK
		{int64(-1), 1, 0, fmt.Errorf("Negative position not allowed for 1"), false, 0, 0},
		// Test with whence SEEK_END and with positive offset
		{1024, 2, 0, io.EOF, false, 0, 0},
		// Test with whence SEEK_END and with negative offset
		{-1024, 2, int64(bufSize) - 1024, nil, true, bufSize - 1024, bufSize},
		// Test with whence SEEK_END and with large negative offset
		{-int64(bufSize) * 2, 2, 0, fmt.Errorf("Seeking at negative offset not allowed for 2"), false, 0, 0},
		// Test with invalid whence
		{0, 3, 0, fmt.Errorf("Invalid whence 3"), false, 0, 0},
	}

	for i, testCase := range testCases {
		// Perform seek operation
		n, err := r.Seek(testCase.offset, testCase.whence)
		if err != nil && testCase.err == nil {
			// We expected success.
			logError(testName, function, args, startTime, "",
				fmt.Sprintf("Test %d, unexpected err value: expected: %s, found: %s", i+1, testCase.err, err), err)
			return
		}
		if err == nil && testCase.err != nil {
			// We expected failure, but got success.
			logError(testName, function, args, startTime, "",
				fmt.Sprintf("Test %d, unexpected err value: expected: %s, found: %s", i+1, testCase.err, err), err)
			return
		}
		if err != nil && testCase.err != nil {
			if err.Error() != testCase.err.Error() {
				// We expect a specific error
				logError(testName, function, args, startTime, "",
					fmt.Sprintf("Test %d, unexpected err value: expected: %s, found: %s", i+1, testCase.err, err), err)
				return
			}
		}
		// Check the returned seek pos
		if n != testCase.pos {
			logError(testName, function, args, startTime, "",
				fmt.Sprintf("Test %d, number of bytes seeked does not match, expected %d, got %d", i+1, testCase.pos, n), err)
			return
		}
		// Compare only if shouldCmp is activated
		if testCase.shouldCmp {
			cmpData(r, testCase.start, testCase.end)
		}
	}

	successLogger(testName, function, args, startTime).Info()
}

// Tests SSE-S3 get object ReaderSeeker interface methods.
func testSSES3EncryptedGetObjectReadSeekFunctional() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "GetObject(bucketName, objectName)"
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer func() {
		// Delete all objects and buckets
		if err = cleanupBucket(bucketName, c); err != nil {
			logError(testName, function, args, startTime, "", "CleanupBucket failed", err)
			return
		}
	}()

	// Generate 129MiB of data.
	bufSize := dataFileMap["datafile-129-MB"]
	var reader = getDataReader("datafile-129-MB")
	defer reader.Close()

	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	buf, err := ioutil.ReadAll(reader)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAll failed", err)
		return
	}

	// Save the data
	_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{
		ContentType:          "binary/octet-stream",
		ServerSideEncryption: encrypt.NewSSE(),
	})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	// Read the data back
	r, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}
	defer r.Close()

	st, err := r.Stat()
	if err != nil {
		logError(testName, function, args, startTime, "", "Stat object failed", err)
		return
	}

	if st.Size != int64(bufSize) {
		logError(testName, function, args, startTime, "", "Number of bytes does not match, expected "+string(int64(bufSize))+", got "+string(st.Size), err)
		return
	}

	// This following function helps us to compare data from the reader after seek
	// with the data from the original buffer
	cmpData := func(r io.Reader, start, end int) {
		if end-start == 0 {
			return
		}
		buffer := bytes.NewBuffer([]byte{})
		if _, err := io.CopyN(buffer, r, int64(bufSize)); err != nil {
			if err != io.EOF {
				logError(testName, function, args, startTime, "", "CopyN failed", err)
				return
			}
		}
		if !bytes.Equal(buf[start:end], buffer.Bytes()) {
			logError(testName, function, args, startTime, "", "Incorrect read bytes v/s original buffer", err)
			return
		}
	}

	testCases := []struct {
		offset    int64
		whence    int
		pos       int64
		err       error
		shouldCmp bool
		start     int
		end       int
	}{
		// Start from offset 0, fetch data and compare
		{0, 0, 0, nil, true, 0, 0},
		// Start from offset 2048, fetch data and compare
		{2048, 0, 2048, nil, true, 2048, bufSize},
		// Start from offset larger than possible
		{int64(bufSize) + 1024, 0, 0, io.EOF, false, 0, 0},
		// Move to offset 0 without comparing
		{0, 0, 0, nil, false, 0, 0},
		// Move one step forward and compare
		{1, 1, 1, nil, true, 1, bufSize},
		// Move larger than possible
		{int64(bufSize), 1, 0, io.EOF, false, 0, 0},
		// Provide negative offset with CUR_SEEK
		{int64(-1), 1, 0, fmt.Errorf("Negative position not allowed for 1"), false, 0, 0},
		// Test with whence SEEK_END and with positive offset
		{1024, 2, 0, io.EOF, false, 0, 0},
		// Test with whence SEEK_END and with negative offset
		{-1024, 2, int64(bufSize) - 1024, nil, true, bufSize - 1024, bufSize},
		// Test with whence SEEK_END and with large negative offset
		{-int64(bufSize) * 2, 2, 0, fmt.Errorf("Seeking at negative offset not allowed for 2"), false, 0, 0},
		// Test with invalid whence
		{0, 3, 0, fmt.Errorf("Invalid whence 3"), false, 0, 0},
	}

	for i, testCase := range testCases {
		// Perform seek operation
		n, err := r.Seek(testCase.offset, testCase.whence)
		if err != nil && testCase.err == nil {
			// We expected success.
			logError(testName, function, args, startTime, "",
				fmt.Sprintf("Test %d, unexpected err value: expected: %s, found: %s", i+1, testCase.err, err), err)
			return
		}
		if err == nil && testCase.err != nil {
			// We expected failure, but got success.
			logError(testName, function, args, startTime, "",
				fmt.Sprintf("Test %d, unexpected err value: expected: %s, found: %s", i+1, testCase.err, err), err)
			return
		}
		if err != nil && testCase.err != nil {
			if err.Error() != testCase.err.Error() {
				// We expect a specific error
				logError(testName, function, args, startTime, "",
					fmt.Sprintf("Test %d, unexpected err value: expected: %s, found: %s", i+1, testCase.err, err), err)
				return
			}
		}
		// Check the returned seek pos
		if n != testCase.pos {
			logError(testName, function, args, startTime, "",
				fmt.Sprintf("Test %d, number of bytes seeked does not match, expected %d, got %d", i+1, testCase.pos, n), err)
			return
		}
		// Compare only if shouldCmp is activated
		if testCase.shouldCmp {
			cmpData(r, testCase.start, testCase.end)
		}
	}

	successLogger(testName, function, args, startTime).Info()
}

// Tests SSE-C get object ReaderAt interface methods.
func testSSECEncryptedGetObjectReadAtFunctional() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "GetObject(bucketName, objectName)"
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Generate 129MiB of data.
	bufSize := dataFileMap["datafile-129-MB"]
	var reader = getDataReader("datafile-129-MB")
	defer reader.Close()

	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	buf, err := ioutil.ReadAll(reader)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAll failed", err)
		return
	}

	// Save the data
	_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{
		ContentType:          "binary/octet-stream",
		ServerSideEncryption: encrypt.DefaultPBKDF([]byte("correct horse battery staple"), []byte(bucketName+objectName)),
	})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	// read the data back
	r, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{
		ServerSideEncryption: encrypt.DefaultPBKDF([]byte("correct horse battery staple"), []byte(bucketName+objectName)),
	})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}
	defer r.Close()

	offset := int64(2048)

	// read directly
	buf1 := make([]byte, 512)
	buf2 := make([]byte, 512)
	buf3 := make([]byte, 512)
	buf4 := make([]byte, 512)

	// Test readAt before stat is called such that objectInfo doesn't change.
	m, err := r.ReadAt(buf1, offset)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAt failed", err)
		return
	}
	if m != len(buf1) {
		logError(testName, function, args, startTime, "", "ReadAt read shorter bytes before reaching EOF, expected "+string(len(buf1))+", got "+string(m), err)
		return
	}
	if !bytes.Equal(buf1, buf[offset:offset+512]) {
		logError(testName, function, args, startTime, "", "Incorrect read between two ReadAt from same offset", err)
		return
	}
	offset += 512

	st, err := r.Stat()
	if err != nil {
		logError(testName, function, args, startTime, "", "Stat failed", err)
		return
	}

	if st.Size != int64(bufSize) {
		logError(testName, function, args, startTime, "", "Number of bytes in stat does not match, expected "+string(int64(bufSize))+", got "+string(st.Size), err)
		return
	}

	m, err = r.ReadAt(buf2, offset)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAt failed", err)
		return
	}
	if m != len(buf2) {
		logError(testName, function, args, startTime, "", "ReadAt read shorter bytes before reaching EOF, expected "+string(len(buf2))+", got "+string(m), err)
		return
	}
	if !bytes.Equal(buf2, buf[offset:offset+512]) {
		logError(testName, function, args, startTime, "", "Incorrect read between two ReadAt from same offset", err)
		return
	}
	offset += 512
	m, err = r.ReadAt(buf3, offset)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAt failed", err)
		return
	}
	if m != len(buf3) {
		logError(testName, function, args, startTime, "", "ReadAt read shorter bytes before reaching EOF, expected "+string(len(buf3))+", got "+string(m), err)
		return
	}
	if !bytes.Equal(buf3, buf[offset:offset+512]) {
		logError(testName, function, args, startTime, "", "Incorrect read between two ReadAt from same offset", err)
		return
	}
	offset += 512
	m, err = r.ReadAt(buf4, offset)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAt failed", err)
		return
	}
	if m != len(buf4) {
		logError(testName, function, args, startTime, "", "ReadAt read shorter bytes before reaching EOF, expected "+string(len(buf4))+", got "+string(m), err)
		return
	}
	if !bytes.Equal(buf4, buf[offset:offset+512]) {
		logError(testName, function, args, startTime, "", "Incorrect read between two ReadAt from same offset", err)
		return
	}

	buf5 := make([]byte, len(buf))
	// Read the whole object.
	m, err = r.ReadAt(buf5, 0)
	if err != nil {
		if err != io.EOF {
			logError(testName, function, args, startTime, "", "ReadAt failed", err)
			return
		}
	}
	if m != len(buf5) {
		logError(testName, function, args, startTime, "", "ReadAt read shorter bytes before reaching EOF, expected "+string(len(buf5))+", got "+string(m), err)
		return
	}
	if !bytes.Equal(buf, buf5) {
		logError(testName, function, args, startTime, "", "Incorrect data read in GetObject, than what was previously uploaded", err)
		return
	}

	buf6 := make([]byte, len(buf)+1)
	// Read the whole object and beyond.
	_, err = r.ReadAt(buf6, 0)
	if err != nil {
		if err != io.EOF {
			logError(testName, function, args, startTime, "", "ReadAt failed", err)
			return
		}
	}

	successLogger(testName, function, args, startTime).Info()
}

// Tests SSE-S3 get object ReaderAt interface methods.
func testSSES3EncryptedGetObjectReadAtFunctional() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "GetObject(bucketName, objectName)"
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Generate 129MiB of data.
	bufSize := dataFileMap["datafile-129-MB"]
	var reader = getDataReader("datafile-129-MB")
	defer reader.Close()

	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	buf, err := ioutil.ReadAll(reader)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAll failed", err)
		return
	}

	// Save the data
	_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{
		ContentType:          "binary/octet-stream",
		ServerSideEncryption: encrypt.NewSSE(),
	})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	// read the data back
	r, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}
	defer r.Close()

	offset := int64(2048)

	// read directly
	buf1 := make([]byte, 512)
	buf2 := make([]byte, 512)
	buf3 := make([]byte, 512)
	buf4 := make([]byte, 512)

	// Test readAt before stat is called such that objectInfo doesn't change.
	m, err := r.ReadAt(buf1, offset)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAt failed", err)
		return
	}
	if m != len(buf1) {
		logError(testName, function, args, startTime, "", "ReadAt read shorter bytes before reaching EOF, expected "+string(len(buf1))+", got "+string(m), err)
		return
	}
	if !bytes.Equal(buf1, buf[offset:offset+512]) {
		logError(testName, function, args, startTime, "", "Incorrect read between two ReadAt from same offset", err)
		return
	}
	offset += 512

	st, err := r.Stat()
	if err != nil {
		logError(testName, function, args, startTime, "", "Stat failed", err)
		return
	}

	if st.Size != int64(bufSize) {
		logError(testName, function, args, startTime, "", "Number of bytes in stat does not match, expected "+string(int64(bufSize))+", got "+string(st.Size), err)
		return
	}

	m, err = r.ReadAt(buf2, offset)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAt failed", err)
		return
	}
	if m != len(buf2) {
		logError(testName, function, args, startTime, "", "ReadAt read shorter bytes before reaching EOF, expected "+string(len(buf2))+", got "+string(m), err)
		return
	}
	if !bytes.Equal(buf2, buf[offset:offset+512]) {
		logError(testName, function, args, startTime, "", "Incorrect read between two ReadAt from same offset", err)
		return
	}
	offset += 512
	m, err = r.ReadAt(buf3, offset)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAt failed", err)
		return
	}
	if m != len(buf3) {
		logError(testName, function, args, startTime, "", "ReadAt read shorter bytes before reaching EOF, expected "+string(len(buf3))+", got "+string(m), err)
		return
	}
	if !bytes.Equal(buf3, buf[offset:offset+512]) {
		logError(testName, function, args, startTime, "", "Incorrect read between two ReadAt from same offset", err)
		return
	}
	offset += 512
	m, err = r.ReadAt(buf4, offset)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAt failed", err)
		return
	}
	if m != len(buf4) {
		logError(testName, function, args, startTime, "", "ReadAt read shorter bytes before reaching EOF, expected "+string(len(buf4))+", got "+string(m), err)
		return
	}
	if !bytes.Equal(buf4, buf[offset:offset+512]) {
		logError(testName, function, args, startTime, "", "Incorrect read between two ReadAt from same offset", err)
		return
	}

	buf5 := make([]byte, len(buf))
	// Read the whole object.
	m, err = r.ReadAt(buf5, 0)
	if err != nil {
		if err != io.EOF {
			logError(testName, function, args, startTime, "", "ReadAt failed", err)
			return
		}
	}
	if m != len(buf5) {
		logError(testName, function, args, startTime, "", "ReadAt read shorter bytes before reaching EOF, expected "+string(len(buf5))+", got "+string(m), err)
		return
	}
	if !bytes.Equal(buf, buf5) {
		logError(testName, function, args, startTime, "", "Incorrect data read in GetObject, than what was previously uploaded", err)
		return
	}

	buf6 := make([]byte, len(buf)+1)
	// Read the whole object and beyond.
	_, err = r.ReadAt(buf6, 0)
	if err != nil {
		if err != io.EOF {
			logError(testName, function, args, startTime, "", "ReadAt failed", err)
			return
		}
	}

	successLogger(testName, function, args, startTime).Info()
}

// testSSECEncryptionPutGet tests encryption with customer provided encryption keys
func testSSECEncryptionPutGet() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "PutEncryptedObject(bucketName, objectName, reader, sse)"
	args := map[string]interface{}{
		"bucketName": "",
		"objectName": "",
		"sse":        "",
	}
	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	testCases := []struct {
		buf []byte
	}{
		{buf: bytes.Repeat([]byte("F"), 1)},
		{buf: bytes.Repeat([]byte("F"), 15)},
		{buf: bytes.Repeat([]byte("F"), 16)},
		{buf: bytes.Repeat([]byte("F"), 17)},
		{buf: bytes.Repeat([]byte("F"), 31)},
		{buf: bytes.Repeat([]byte("F"), 32)},
		{buf: bytes.Repeat([]byte("F"), 33)},
		{buf: bytes.Repeat([]byte("F"), 1024)},
		{buf: bytes.Repeat([]byte("F"), 1024*2)},
		{buf: bytes.Repeat([]byte("F"), 1024*1024)},
	}

	const password = "correct horse battery staple" // https://xkcd.com/936/

	for i, testCase := range testCases {
		// Generate a random object name
		objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
		args["objectName"] = objectName

		// Secured object
		sse := encrypt.DefaultPBKDF([]byte(password), []byte(bucketName+objectName))
		args["sse"] = sse

		// Put encrypted data
		_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(testCase.buf), int64(len(testCase.buf)), minio.PutObjectOptions{ServerSideEncryption: sse})
		if err != nil {
			logError(testName, function, args, startTime, "", "PutEncryptedObject failed", err)
			return
		}

		// Read the data back
		r, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{ServerSideEncryption: sse})
		if err != nil {
			logError(testName, function, args, startTime, "", "GetEncryptedObject failed", err)
			return
		}
		defer r.Close()

		// Compare the sent object with the received one
		recvBuffer := bytes.NewBuffer([]byte{})
		if _, err = io.Copy(recvBuffer, r); err != nil {
			logError(testName, function, args, startTime, "", "Test "+string(i+1)+", error: "+err.Error(), err)
			return
		}
		if recvBuffer.Len() != len(testCase.buf) {
			logError(testName, function, args, startTime, "", "Test "+string(i+1)+", Number of bytes of received object does not match, expected "+string(len(testCase.buf))+", got "+string(recvBuffer.Len()), err)
			return
		}
		if !bytes.Equal(testCase.buf, recvBuffer.Bytes()) {
			logError(testName, function, args, startTime, "", "Test "+string(i+1)+", Encrypted sent is not equal to decrypted, expected "+string(testCase.buf)+", got "+string(recvBuffer.Bytes()), err)
			return
		}

		successLogger(testName, function, args, startTime).Info()

	}

	successLogger(testName, function, args, startTime).Info()
}

// TestEncryptionFPut tests encryption with customer specified encryption keys
func testSSECEncryptionFPut() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "FPutEncryptedObject(bucketName, objectName, filePath, contentType, sse)"
	args := map[string]interface{}{
		"bucketName":  "",
		"objectName":  "",
		"filePath":    "",
		"contentType": "",
		"sse":         "",
	}
	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Object custom metadata
	customContentType := "custom/contenttype"
	args["metadata"] = customContentType

	testCases := []struct {
		buf []byte
	}{
		{buf: bytes.Repeat([]byte("F"), 0)},
		{buf: bytes.Repeat([]byte("F"), 1)},
		{buf: bytes.Repeat([]byte("F"), 15)},
		{buf: bytes.Repeat([]byte("F"), 16)},
		{buf: bytes.Repeat([]byte("F"), 17)},
		{buf: bytes.Repeat([]byte("F"), 31)},
		{buf: bytes.Repeat([]byte("F"), 32)},
		{buf: bytes.Repeat([]byte("F"), 33)},
		{buf: bytes.Repeat([]byte("F"), 1024)},
		{buf: bytes.Repeat([]byte("F"), 1024*2)},
		{buf: bytes.Repeat([]byte("F"), 1024*1024)},
	}

	const password = "correct horse battery staple" // https://xkcd.com/936/
	for i, testCase := range testCases {
		// Generate a random object name
		objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
		args["objectName"] = objectName

		// Secured object
		sse := encrypt.DefaultPBKDF([]byte(password), []byte(bucketName+objectName))
		args["sse"] = sse

		// Generate a random file name.
		fileName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
		file, err := os.Create(fileName)
		if err != nil {
			logError(testName, function, args, startTime, "", "file create failed", err)
			return
		}
		_, err = file.Write(testCase.buf)
		if err != nil {
			logError(testName, function, args, startTime, "", "file write failed", err)
			return
		}
		file.Close()
		// Put encrypted data
		if _, err = c.FPutObject(context.Background(), bucketName, objectName, fileName, minio.PutObjectOptions{ServerSideEncryption: sse}); err != nil {
			logError(testName, function, args, startTime, "", "FPutEncryptedObject failed", err)
			return
		}

		// Read the data back
		r, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{ServerSideEncryption: sse})
		if err != nil {
			logError(testName, function, args, startTime, "", "GetEncryptedObject failed", err)
			return
		}
		defer r.Close()

		// Compare the sent object with the received one
		recvBuffer := bytes.NewBuffer([]byte{})
		if _, err = io.Copy(recvBuffer, r); err != nil {
			logError(testName, function, args, startTime, "", "Test "+string(i+1)+", error: "+err.Error(), err)
			return
		}
		if recvBuffer.Len() != len(testCase.buf) {
			logError(testName, function, args, startTime, "", "Test "+string(i+1)+", Number of bytes of received object does not match, expected "+string(len(testCase.buf))+", got "+string(recvBuffer.Len()), err)
			return
		}
		if !bytes.Equal(testCase.buf, recvBuffer.Bytes()) {
			logError(testName, function, args, startTime, "", "Test "+string(i+1)+", Encrypted sent is not equal to decrypted, expected "+string(testCase.buf)+", got "+string(recvBuffer.Bytes()), err)
			return
		}

		os.Remove(fileName)
	}

	successLogger(testName, function, args, startTime).Info()
}

// testSSES3EncryptionPutGet tests SSE-S3 encryption
func testSSES3EncryptionPutGet() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "PutEncryptedObject(bucketName, objectName, reader, sse)"
	args := map[string]interface{}{
		"bucketName": "",
		"objectName": "",
		"sse":        "",
	}
	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	testCases := []struct {
		buf []byte
	}{
		{buf: bytes.Repeat([]byte("F"), 1)},
		{buf: bytes.Repeat([]byte("F"), 15)},
		{buf: bytes.Repeat([]byte("F"), 16)},
		{buf: bytes.Repeat([]byte("F"), 17)},
		{buf: bytes.Repeat([]byte("F"), 31)},
		{buf: bytes.Repeat([]byte("F"), 32)},
		{buf: bytes.Repeat([]byte("F"), 33)},
		{buf: bytes.Repeat([]byte("F"), 1024)},
		{buf: bytes.Repeat([]byte("F"), 1024*2)},
		{buf: bytes.Repeat([]byte("F"), 1024*1024)},
	}

	for i, testCase := range testCases {
		// Generate a random object name
		objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
		args["objectName"] = objectName

		// Secured object
		sse := encrypt.NewSSE()
		args["sse"] = sse

		// Put encrypted data
		_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(testCase.buf), int64(len(testCase.buf)), minio.PutObjectOptions{ServerSideEncryption: sse})
		if err != nil {
			logError(testName, function, args, startTime, "", "PutEncryptedObject failed", err)
			return
		}

		// Read the data back without any encryption headers
		r, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
		if err != nil {
			logError(testName, function, args, startTime, "", "GetEncryptedObject failed", err)
			return
		}
		defer r.Close()

		// Compare the sent object with the received one
		recvBuffer := bytes.NewBuffer([]byte{})
		if _, err = io.Copy(recvBuffer, r); err != nil {
			logError(testName, function, args, startTime, "", "Test "+string(i+1)+", error: "+err.Error(), err)
			return
		}
		if recvBuffer.Len() != len(testCase.buf) {
			logError(testName, function, args, startTime, "", "Test "+string(i+1)+", Number of bytes of received object does not match, expected "+string(len(testCase.buf))+", got "+string(recvBuffer.Len()), err)
			return
		}
		if !bytes.Equal(testCase.buf, recvBuffer.Bytes()) {
			logError(testName, function, args, startTime, "", "Test "+string(i+1)+", Encrypted sent is not equal to decrypted, expected "+string(testCase.buf)+", got "+string(recvBuffer.Bytes()), err)
			return
		}

		successLogger(testName, function, args, startTime).Info()

	}

	successLogger(testName, function, args, startTime).Info()
}

// TestSSES3EncryptionFPut tests server side encryption
func testSSES3EncryptionFPut() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "FPutEncryptedObject(bucketName, objectName, filePath, contentType, sse)"
	args := map[string]interface{}{
		"bucketName":  "",
		"objectName":  "",
		"filePath":    "",
		"contentType": "",
		"sse":         "",
	}
	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Object custom metadata
	customContentType := "custom/contenttype"
	args["metadata"] = customContentType

	testCases := []struct {
		buf []byte
	}{
		{buf: bytes.Repeat([]byte("F"), 0)},
		{buf: bytes.Repeat([]byte("F"), 1)},
		{buf: bytes.Repeat([]byte("F"), 15)},
		{buf: bytes.Repeat([]byte("F"), 16)},
		{buf: bytes.Repeat([]byte("F"), 17)},
		{buf: bytes.Repeat([]byte("F"), 31)},
		{buf: bytes.Repeat([]byte("F"), 32)},
		{buf: bytes.Repeat([]byte("F"), 33)},
		{buf: bytes.Repeat([]byte("F"), 1024)},
		{buf: bytes.Repeat([]byte("F"), 1024*2)},
		{buf: bytes.Repeat([]byte("F"), 1024*1024)},
	}

	for i, testCase := range testCases {
		// Generate a random object name
		objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
		args["objectName"] = objectName

		// Secured object
		sse := encrypt.NewSSE()
		args["sse"] = sse

		// Generate a random file name.
		fileName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
		file, err := os.Create(fileName)
		if err != nil {
			logError(testName, function, args, startTime, "", "file create failed", err)
			return
		}
		_, err = file.Write(testCase.buf)
		if err != nil {
			logError(testName, function, args, startTime, "", "file write failed", err)
			return
		}
		file.Close()
		// Put encrypted data
		if _, err = c.FPutObject(context.Background(), bucketName, objectName, fileName, minio.PutObjectOptions{ServerSideEncryption: sse}); err != nil {
			logError(testName, function, args, startTime, "", "FPutEncryptedObject failed", err)
			return
		}

		// Read the data back
		r, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
		if err != nil {
			logError(testName, function, args, startTime, "", "GetEncryptedObject failed", err)
			return
		}
		defer r.Close()

		// Compare the sent object with the received one
		recvBuffer := bytes.NewBuffer([]byte{})
		if _, err = io.Copy(recvBuffer, r); err != nil {
			logError(testName, function, args, startTime, "", "Test "+string(i+1)+", error: "+err.Error(), err)
			return
		}
		if recvBuffer.Len() != len(testCase.buf) {
			logError(testName, function, args, startTime, "", "Test "+string(i+1)+", Number of bytes of received object does not match, expected "+string(len(testCase.buf))+", got "+string(recvBuffer.Len()), err)
			return
		}
		if !bytes.Equal(testCase.buf, recvBuffer.Bytes()) {
			logError(testName, function, args, startTime, "", "Test "+string(i+1)+", Encrypted sent is not equal to decrypted, expected "+string(testCase.buf)+", got "+string(recvBuffer.Bytes()), err)
			return
		}

		os.Remove(fileName)
	}

	successLogger(testName, function, args, startTime).Info()
}

func testBucketNotification() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "SetBucketNotification(bucketName)"
	args := map[string]interface{}{
		"bucketName": "",
	}

	if os.Getenv("NOTIFY_BUCKET") == "" ||
		os.Getenv("NOTIFY_SERVICE") == "" ||
		os.Getenv("NOTIFY_REGION") == "" ||
		os.Getenv("NOTIFY_ACCOUNTID") == "" ||
		os.Getenv("NOTIFY_RESOURCE") == "" {
		ignoredLog(testName, function, args, startTime, "Skipped notification test as it is not configured").Info()
		return
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable to debug
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	bucketName := os.Getenv("NOTIFY_BUCKET")
	args["bucketName"] = bucketName

	topicArn := notification.NewArn("aws", os.Getenv("NOTIFY_SERVICE"), os.Getenv("NOTIFY_REGION"), os.Getenv("NOTIFY_ACCOUNTID"), os.Getenv("NOTIFY_RESOURCE"))
	queueArn := notification.NewArn("aws", "dummy-service", "dummy-region", "dummy-accountid", "dummy-resource")

	topicConfig := notification.NewConfig(topicArn)
	topicConfig.AddEvents(notification.ObjectCreatedAll, notification.ObjectRemovedAll)
	topicConfig.AddFilterSuffix("jpg")

	queueConfig := notification.NewConfig(queueArn)
	queueConfig.AddEvents(notification.ObjectCreatedAll)
	queueConfig.AddFilterPrefix("photos/")

	config := notification.Configuration{}
	config.AddTopic(topicConfig)

	// Add the same topicConfig again, should have no effect
	// because it is duplicated
	config.AddTopic(topicConfig)
	if len(config.TopicConfigs) != 1 {
		logError(testName, function, args, startTime, "", "Duplicate entry added", err)
		return
	}

	// Add and remove a queue config
	config.AddQueue(queueConfig)
	config.RemoveQueueByArn(queueArn)

	err = c.SetBucketNotification(context.Background(), bucketName, config)
	if err != nil {
		logError(testName, function, args, startTime, "", "SetBucketNotification failed", err)
		return
	}

	config, err = c.GetBucketNotification(context.Background(), bucketName)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetBucketNotification failed", err)
		return
	}

	if len(config.TopicConfigs) != 1 {
		logError(testName, function, args, startTime, "", "Topic config is empty", err)
		return
	}

	if config.TopicConfigs[0].Filter.S3Key.FilterRules[0].Value != "jpg" {
		logError(testName, function, args, startTime, "", "Couldn't get the suffix", err)
		return
	}

	err = c.RemoveAllBucketNotification(context.Background(), bucketName)
	if err != nil {
		logError(testName, function, args, startTime, "", "RemoveAllBucketNotification failed", err)
		return
	}

	// Delete all objects and buckets
	if err = cleanupBucket(bucketName, c); err != nil {
		logError(testName, function, args, startTime, "", "CleanupBucket failed", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Tests comprehensive list of all methods.
func testFunctional() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "testFunctional()"
	functionAll := ""
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, nil, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable to debug
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")

	// Make a new bucket.
	function = "MakeBucket(bucketName, region)"
	functionAll = "MakeBucket(bucketName, region)"
	args["bucketName"] = bucketName
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})

	defer cleanupBucket(bucketName, c)
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	// Generate a random file name.
	fileName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	file, err := os.Create(fileName)
	if err != nil {
		logError(testName, function, args, startTime, "", "File creation failed", err)
		return
	}
	for i := 0; i < 3; i++ {
		buf := make([]byte, rand.Intn(1<<19))
		_, err = file.Write(buf)
		if err != nil {
			logError(testName, function, args, startTime, "", "File write failed", err)
			return
		}
	}
	file.Close()

	// Verify if bucket exits and you have access.
	var exists bool
	function = "BucketExists(bucketName)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName": bucketName,
	}
	exists, err = c.BucketExists(context.Background(), bucketName)

	if err != nil {
		logError(testName, function, args, startTime, "", "BucketExists failed", err)
		return
	}
	if !exists {
		logError(testName, function, args, startTime, "", "Could not find the bucket", err)
		return
	}

	// Asserting the default bucket policy.
	function = "GetBucketPolicy(ctx, bucketName)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName": bucketName,
	}
	nilPolicy, err := c.GetBucketPolicy(context.Background(), bucketName)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetBucketPolicy failed", err)
		return
	}
	if nilPolicy != "" {
		logError(testName, function, args, startTime, "", "policy should be set to nil", err)
		return
	}

	// Set the bucket policy to 'public readonly'.
	function = "SetBucketPolicy(bucketName, readOnlyPolicy)"
	functionAll += ", " + function

	readOnlyPolicy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:ListBucket"],"Resource":["arn:aws:s3:::` + bucketName + `"]}]}`
	args = map[string]interface{}{
		"bucketName":   bucketName,
		"bucketPolicy": readOnlyPolicy,
	}

	err = c.SetBucketPolicy(context.Background(), bucketName, readOnlyPolicy)
	if err != nil {
		logError(testName, function, args, startTime, "", "SetBucketPolicy failed", err)
		return
	}
	// should return policy `readonly`.
	function = "GetBucketPolicy(ctx, bucketName)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName": bucketName,
	}
	_, err = c.GetBucketPolicy(context.Background(), bucketName)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetBucketPolicy failed", err)
		return
	}

	// Make the bucket 'public writeonly'.
	function = "SetBucketPolicy(bucketName, writeOnlyPolicy)"
	functionAll += ", " + function

	writeOnlyPolicy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:ListBucketMultipartUploads"],"Resource":["arn:aws:s3:::` + bucketName + `"]}]}`
	args = map[string]interface{}{
		"bucketName":   bucketName,
		"bucketPolicy": writeOnlyPolicy,
	}
	err = c.SetBucketPolicy(context.Background(), bucketName, writeOnlyPolicy)

	if err != nil {
		logError(testName, function, args, startTime, "", "SetBucketPolicy failed", err)
		return
	}
	// should return policy `writeonly`.
	function = "GetBucketPolicy(ctx, bucketName)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName": bucketName,
	}

	_, err = c.GetBucketPolicy(context.Background(), bucketName)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetBucketPolicy failed", err)
		return
	}

	// Make the bucket 'public read/write'.
	function = "SetBucketPolicy(bucketName, readWritePolicy)"
	functionAll += ", " + function

	readWritePolicy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:ListBucket","s3:ListBucketMultipartUploads"],"Resource":["arn:aws:s3:::` + bucketName + `"]}]}`

	args = map[string]interface{}{
		"bucketName":   bucketName,
		"bucketPolicy": readWritePolicy,
	}
	err = c.SetBucketPolicy(context.Background(), bucketName, readWritePolicy)

	if err != nil {
		logError(testName, function, args, startTime, "", "SetBucketPolicy failed", err)
		return
	}
	// should return policy `readwrite`.
	function = "GetBucketPolicy(bucketName)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName": bucketName,
	}
	_, err = c.GetBucketPolicy(context.Background(), bucketName)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetBucketPolicy failed", err)
		return
	}

	// List all buckets.
	function = "ListBuckets()"
	functionAll += ", " + function
	args = nil
	buckets, err := c.ListBuckets(context.Background())

	if len(buckets) == 0 {
		logError(testName, function, args, startTime, "", "Found bucket list to be empty", err)
		return
	}
	if err != nil {
		logError(testName, function, args, startTime, "", "ListBuckets failed", err)
		return
	}

	// Verify if previously created bucket is listed in list buckets.
	bucketFound := false
	for _, bucket := range buckets {
		if bucket.Name == bucketName {
			bucketFound = true
		}
	}

	// If bucket not found error out.
	if !bucketFound {
		logError(testName, function, args, startTime, "", "Bucket: "+bucketName+" not found", err)
		return
	}

	objectName := bucketName + "unique"

	// Generate data
	buf := bytes.Repeat([]byte("f"), 1<<19)

	function = "PutObject(bucketName, objectName, reader, contentType)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName":  bucketName,
		"objectName":  objectName,
		"contentType": "",
	}

	_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	args = map[string]interface{}{
		"bucketName":  bucketName,
		"objectName":  objectName + "-nolength",
		"contentType": "binary/octet-stream",
	}

	_, err = c.PutObject(context.Background(), bucketName, objectName+"-nolength", bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	// Instantiate a done channel to close all listing.
	doneCh := make(chan struct{})
	defer close(doneCh)

	objFound := false
	isRecursive := true // Recursive is true.

	function = "ListObjects(bucketName, objectName, isRecursive, doneCh)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName":  bucketName,
		"objectName":  objectName,
		"isRecursive": isRecursive,
	}

	for obj := range c.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{UseV1: true, Prefix: objectName, Recursive: true}) {
		if obj.Key == objectName {
			objFound = true
			break
		}
	}
	if !objFound {
		logError(testName, function, args, startTime, "", "Object "+objectName+" not found", err)
		return
	}

	objFound = false
	isRecursive = true // Recursive is true.
	function = "ListObjects()"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName":  bucketName,
		"objectName":  objectName,
		"isRecursive": isRecursive,
	}

	for obj := range c.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{Prefix: objectName, Recursive: isRecursive}) {
		if obj.Key == objectName {
			objFound = true
			break
		}
	}
	if !objFound {
		logError(testName, function, args, startTime, "", "Object "+objectName+" not found", err)
		return
	}

	incompObjNotFound := true

	function = "ListIncompleteUploads(bucketName, objectName, isRecursive, doneCh)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName":  bucketName,
		"objectName":  objectName,
		"isRecursive": isRecursive,
	}

	for objIncompl := range c.ListIncompleteUploads(context.Background(), bucketName, objectName, isRecursive) {
		if objIncompl.Key != "" {
			incompObjNotFound = false
			break
		}
	}
	if !incompObjNotFound {
		logError(testName, function, args, startTime, "", "Unexpected dangling incomplete upload found", err)
		return
	}

	function = "GetObject(bucketName, objectName)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName": bucketName,
		"objectName": objectName,
	}
	newReader, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})

	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}

	newReadBytes, err := ioutil.ReadAll(newReader)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAll failed", err)
		return
	}

	if !bytes.Equal(newReadBytes, buf) {
		logError(testName, function, args, startTime, "", "GetObject bytes mismatch", err)
		return
	}
	newReader.Close()

	function = "FGetObject(bucketName, objectName, fileName)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName": bucketName,
		"objectName": objectName,
		"fileName":   fileName + "-f",
	}
	err = c.FGetObject(context.Background(), bucketName, objectName, fileName+"-f", minio.GetObjectOptions{})

	if err != nil {
		logError(testName, function, args, startTime, "", "FGetObject failed", err)
		return
	}

	function = "PresignedHeadObject(bucketName, objectName, expires, reqParams)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName": bucketName,
		"objectName": "",
		"expires":    3600 * time.Second,
	}
	if _, err = c.PresignedHeadObject(context.Background(), bucketName, "", 3600*time.Second, nil); err == nil {
		logError(testName, function, args, startTime, "", "PresignedHeadObject success", err)
		return
	}

	// Generate presigned HEAD object url.
	function = "PresignedHeadObject(bucketName, objectName, expires, reqParams)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName": bucketName,
		"objectName": objectName,
		"expires":    3600 * time.Second,
	}
	presignedHeadURL, err := c.PresignedHeadObject(context.Background(), bucketName, objectName, 3600*time.Second, nil)

	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedHeadObject failed", err)
		return
	}

	transport, err := minio.DefaultTransport(mustParseBool(os.Getenv(enableHTTPS)))
	if err != nil {
		logError(testName, function, args, startTime, "", "DefaultTransport failed", err)
		return
	}

	httpClient := &http.Client{
		// Setting a sensible time out of 30secs to wait for response
		// headers. Request is pro-actively canceled after 30secs
		// with no response.
		Timeout:   30 * time.Second,
		Transport: transport,
	}

	req, err := http.NewRequest(http.MethodHead, presignedHeadURL.String(), nil)
	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedHeadObject request was incorrect", err)
		return
	}

	// Verify if presigned url works.
	resp, err := httpClient.Do(req)
	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedHeadObject response incorrect", err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		logError(testName, function, args, startTime, "", "PresignedHeadObject response incorrect, status "+string(resp.StatusCode), err)
		return
	}
	if resp.Header.Get("ETag") == "" {
		logError(testName, function, args, startTime, "", "PresignedHeadObject response incorrect", err)
		return
	}
	resp.Body.Close()

	function = "PresignedGetObject(bucketName, objectName, expires, reqParams)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName": bucketName,
		"objectName": "",
		"expires":    3600 * time.Second,
	}
	_, err = c.PresignedGetObject(context.Background(), bucketName, "", 3600*time.Second, nil)
	if err == nil {
		logError(testName, function, args, startTime, "", "PresignedGetObject success", err)
		return
	}

	// Generate presigned GET object url.
	function = "PresignedGetObject(bucketName, objectName, expires, reqParams)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName": bucketName,
		"objectName": objectName,
		"expires":    3600 * time.Second,
	}
	presignedGetURL, err := c.PresignedGetObject(context.Background(), bucketName, objectName, 3600*time.Second, nil)

	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedGetObject failed", err)
		return
	}

	// Verify if presigned url works.
	req, err = http.NewRequest(http.MethodGet, presignedGetURL.String(), nil)
	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedGetObject request incorrect", err)
		return
	}

	resp, err = httpClient.Do(req)
	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedGetObject response incorrect", err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		logError(testName, function, args, startTime, "", "PresignedGetObject response incorrect, status "+string(resp.StatusCode), err)
		return
	}
	newPresignedBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedGetObject response incorrect", err)
		return
	}
	resp.Body.Close()
	if !bytes.Equal(newPresignedBytes, buf) {
		logError(testName, function, args, startTime, "", "PresignedGetObject response incorrect", err)
		return
	}

	// Set request parameters.
	reqParams := make(url.Values)
	reqParams.Set("response-content-disposition", "attachment; filename=\"test.txt\"")
	args = map[string]interface{}{
		"bucketName": bucketName,
		"objectName": objectName,
		"expires":    3600 * time.Second,
		"reqParams":  reqParams,
	}
	presignedGetURL, err = c.PresignedGetObject(context.Background(), bucketName, objectName, 3600*time.Second, reqParams)

	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedGetObject failed", err)
		return
	}

	// Verify if presigned url works.
	req, err = http.NewRequest(http.MethodGet, presignedGetURL.String(), nil)
	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedGetObject request incorrect", err)
		return
	}

	resp, err = httpClient.Do(req)
	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedGetObject response incorrect", err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		logError(testName, function, args, startTime, "", "PresignedGetObject response incorrect, status "+string(resp.StatusCode), err)
		return
	}
	newPresignedBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedGetObject response incorrect", err)
		return
	}
	if !bytes.Equal(newPresignedBytes, buf) {
		logError(testName, function, args, startTime, "", "Bytes mismatch for presigned GET URL", err)
		return
	}
	if resp.Header.Get("Content-Disposition") != "attachment; filename=\"test.txt\"" {
		logError(testName, function, args, startTime, "", "wrong Content-Disposition received "+string(resp.Header.Get("Content-Disposition")), err)
		return
	}

	function = "PresignedPutObject(bucketName, objectName, expires)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName": bucketName,
		"objectName": "",
		"expires":    3600 * time.Second,
	}
	_, err = c.PresignedPutObject(context.Background(), bucketName, "", 3600*time.Second)
	if err == nil {
		logError(testName, function, args, startTime, "", "PresignedPutObject success", err)
		return
	}

	function = "PresignedPutObject(bucketName, objectName, expires)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName": bucketName,
		"objectName": objectName + "-presigned",
		"expires":    3600 * time.Second,
	}
	presignedPutURL, err := c.PresignedPutObject(context.Background(), bucketName, objectName+"-presigned", 3600*time.Second)

	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedPutObject failed", err)
		return
	}

	buf = bytes.Repeat([]byte("g"), 1<<19)

	req, err = http.NewRequest(http.MethodPut, presignedPutURL.String(), bytes.NewReader(buf))
	if err != nil {
		logError(testName, function, args, startTime, "", "Couldn't make HTTP request with PresignedPutObject URL", err)
		return
	}

	resp, err = httpClient.Do(req)
	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedPutObject failed", err)
		return
	}

	newReader, err = c.GetObject(context.Background(), bucketName, objectName+"-presigned", minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject after PresignedPutObject failed", err)
		return
	}

	newReadBytes, err = ioutil.ReadAll(newReader)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAll after GetObject failed", err)
		return
	}

	if !bytes.Equal(newReadBytes, buf) {
		logError(testName, function, args, startTime, "", "Bytes mismatch", err)
		return
	}

	function = "RemoveObject(bucketName, objectName)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName": bucketName,
		"objectName": objectName,
	}
	err = c.RemoveObject(context.Background(), bucketName, objectName, minio.RemoveObjectOptions{})

	if err != nil {
		logError(testName, function, args, startTime, "", "RemoveObject failed", err)
		return
	}
	args["objectName"] = objectName + "-f"
	err = c.RemoveObject(context.Background(), bucketName, objectName+"-f", minio.RemoveObjectOptions{})

	if err != nil {
		logError(testName, function, args, startTime, "", "RemoveObject failed", err)
		return
	}

	args["objectName"] = objectName + "-nolength"
	err = c.RemoveObject(context.Background(), bucketName, objectName+"-nolength", minio.RemoveObjectOptions{})

	if err != nil {
		logError(testName, function, args, startTime, "", "RemoveObject failed", err)
		return
	}

	args["objectName"] = objectName + "-presigned"
	err = c.RemoveObject(context.Background(), bucketName, objectName+"-presigned", minio.RemoveObjectOptions{})

	if err != nil {
		logError(testName, function, args, startTime, "", "RemoveObject failed", err)
		return
	}

	function = "RemoveBucket(bucketName)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName": bucketName,
	}
	err = c.RemoveBucket(context.Background(), bucketName)

	if err != nil {
		logError(testName, function, args, startTime, "", "RemoveBucket failed", err)
		return
	}
	err = c.RemoveBucket(context.Background(), bucketName)
	if err == nil {
		logError(testName, function, args, startTime, "", "RemoveBucket did not fail for invalid bucket name", err)
		return
	}
	if err.Error() != "The specified bucket does not exist" {
		logError(testName, function, args, startTime, "", "RemoveBucket failed", err)
		return
	}

	os.Remove(fileName)
	os.Remove(fileName + "-f")
	successLogger(testName, functionAll, args, startTime).Info()
}

// Test for validating GetObject Reader* methods functioning when the
// object is modified in the object store.
func testGetObjectModified() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "GetObject(bucketName, objectName)"
	args := map[string]interface{}{}

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Make a new bucket.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Upload an object.
	objectName := "myobject"
	args["objectName"] = objectName
	content := "helloworld"
	_, err = c.PutObject(context.Background(), bucketName, objectName, strings.NewReader(content), int64(len(content)), minio.PutObjectOptions{ContentType: "application/text"})
	if err != nil {
		logError(testName, function, args, startTime, "", "Failed to upload "+objectName+", to bucket "+bucketName, err)
		return
	}

	defer c.RemoveObject(context.Background(), bucketName, objectName, minio.RemoveObjectOptions{})

	reader, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "Failed to GetObject "+objectName+", from bucket "+bucketName, err)
		return
	}
	defer reader.Close()

	// Read a few bytes of the object.
	b := make([]byte, 5)
	n, err := reader.ReadAt(b, 0)
	if err != nil {
		logError(testName, function, args, startTime, "", "Failed to read object "+objectName+", from bucket "+bucketName+" at an offset", err)
		return
	}

	// Upload different contents to the same object while object is being read.
	newContent := "goodbyeworld"
	_, err = c.PutObject(context.Background(), bucketName, objectName, strings.NewReader(newContent), int64(len(newContent)), minio.PutObjectOptions{ContentType: "application/text"})
	if err != nil {
		logError(testName, function, args, startTime, "", "Failed to upload "+objectName+", to bucket "+bucketName, err)
		return
	}

	// Confirm that a Stat() call in between doesn't change the Object's cached etag.
	_, err = reader.Stat()
	expectedError := "At least one of the pre-conditions you specified did not hold"
	if err.Error() != expectedError {
		logError(testName, function, args, startTime, "", "Expected Stat to fail with error "+expectedError+", but received "+err.Error(), err)
		return
	}

	// Read again only to find object contents have been modified since last read.
	_, err = reader.ReadAt(b, int64(n))
	if err.Error() != expectedError {
		logError(testName, function, args, startTime, "", "Expected ReadAt to fail with error "+expectedError+", but received "+err.Error(), err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Test validates putObject to upload a file seeked at a given offset.
func testPutObjectUploadSeekedObject() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "PutObject(bucketName, objectName, fileToUpload, contentType)"
	args := map[string]interface{}{
		"bucketName":   "",
		"objectName":   "",
		"fileToUpload": "",
		"contentType":  "binary/octet-stream",
	}

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Make a new bucket.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}
	defer cleanupBucket(bucketName, c)

	var tempfile *os.File

	if fileName := getMintDataDirFilePath("datafile-100-kB"); fileName != "" {
		tempfile, err = os.Open(fileName)
		if err != nil {
			logError(testName, function, args, startTime, "", "File open failed", err)
			return
		}
		args["fileToUpload"] = fileName
	} else {
		tempfile, err = ioutil.TempFile("", "minio-go-upload-test-")
		if err != nil {
			logError(testName, function, args, startTime, "", "TempFile create failed", err)
			return
		}
		args["fileToUpload"] = tempfile.Name()

		// Generate 100kB data
		if _, err = io.Copy(tempfile, getDataReader("datafile-100-kB")); err != nil {
			logError(testName, function, args, startTime, "", "File copy failed", err)
			return
		}

		defer os.Remove(tempfile.Name())

		// Seek back to the beginning of the file.
		tempfile.Seek(0, 0)
	}
	var length = 100 * humanize.KiByte
	objectName := fmt.Sprintf("test-file-%v", rand.Uint32())
	args["objectName"] = objectName

	offset := length / 2
	if _, err = tempfile.Seek(int64(offset), 0); err != nil {
		logError(testName, function, args, startTime, "", "TempFile seek failed", err)
		return
	}

	_, err = c.PutObject(context.Background(), bucketName, objectName, tempfile, int64(length-offset), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}
	tempfile.Close()

	obj, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}
	defer obj.Close()

	n, err := obj.Seek(int64(offset), 0)
	if err != nil {
		logError(testName, function, args, startTime, "", "Seek failed", err)
		return
	}
	if n != int64(offset) {
		logError(testName, function, args, startTime, "", fmt.Sprintf("Invalid offset returned, expected %d got %d", int64(offset), n), err)
		return
	}

	_, err = c.PutObject(context.Background(), bucketName, objectName+"getobject", obj, int64(length-offset), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}
	st, err := c.StatObject(context.Background(), bucketName, objectName+"getobject", minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject failed", err)
		return
	}
	if st.Size != int64(length-offset) {
		logError(testName, function, args, startTime, "", fmt.Sprintf("Invalid offset returned, expected %d got %d", int64(length-offset), n), err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Tests bucket re-create errors.
func testMakeBucketErrorV2() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "MakeBucket(bucketName, region)"
	args := map[string]interface{}{
		"bucketName": "",
		"region":     "eu-west-1",
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV2(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v2 client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	region := "eu-west-1"
	args["bucketName"] = bucketName
	args["region"] = region

	// Make a new bucket in 'eu-west-1'.
	if err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: region}); err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	if err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: region}); err == nil {
		logError(testName, function, args, startTime, "", "MakeBucket did not fail for existing bucket name", err)
		return
	}
	// Verify valid error response from server.
	if minio.ToErrorResponse(err).Code != "BucketAlreadyExists" &&
		minio.ToErrorResponse(err).Code != "BucketAlreadyOwnedByYou" {
		logError(testName, function, args, startTime, "", "Invalid error returned by server", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Test get object reader to not throw error on being closed twice.
func testGetObjectClosedTwiceV2() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "MakeBucket(bucketName, region)"
	args := map[string]interface{}{
		"bucketName": "",
		"region":     "eu-west-1",
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV2(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v2 client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Generate 33K of data.
	bufSize := dataFileMap["datafile-33-kB"]
	var reader = getDataReader("datafile-33-kB")
	defer reader.Close()

	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	_, err = c.PutObject(context.Background(), bucketName, objectName, reader, int64(bufSize), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	// Read the data back
	r, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}

	st, err := r.Stat()
	if err != nil {
		logError(testName, function, args, startTime, "", "Stat failed", err)
		return
	}

	if st.Size != int64(bufSize) {
		logError(testName, function, args, startTime, "", "Number of bytes does not match, expected "+string(bufSize)+" got "+string(st.Size), err)
		return
	}
	if err := r.Close(); err != nil {
		logError(testName, function, args, startTime, "", "Stat failed", err)
		return
	}
	if err := r.Close(); err == nil {
		logError(testName, function, args, startTime, "", "Object is already closed, should return error", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Tests FPutObject hidden contentType setting
func testFPutObjectV2() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "FPutObject(bucketName, objectName, fileName, opts)"
	args := map[string]interface{}{
		"bucketName": "",
		"objectName": "",
		"fileName":   "",
		"opts":       "",
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV2(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v2 client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Make a temp file with 11*1024*1024 bytes of data.
	file, err := ioutil.TempFile(os.TempDir(), "FPutObjectTest")
	if err != nil {
		logError(testName, function, args, startTime, "", "TempFile creation failed", err)
		return
	}

	r := bytes.NewReader(bytes.Repeat([]byte("b"), 11*1024*1024))
	n, err := io.CopyN(file, r, 11*1024*1024)
	if err != nil {
		logError(testName, function, args, startTime, "", "Copy failed", err)
		return
	}
	if n != int64(11*1024*1024) {
		logError(testName, function, args, startTime, "", "Number of bytes does not match, expected "+string(int64(11*1024*1024))+" got "+string(n), err)
		return
	}

	// Close the file pro-actively for windows.
	err = file.Close()
	if err != nil {
		logError(testName, function, args, startTime, "", "File close failed", err)
		return
	}

	// Set base object name
	objectName := bucketName + "FPutObject"
	args["objectName"] = objectName
	args["fileName"] = file.Name()

	// Perform standard FPutObject with contentType provided (Expecting application/octet-stream)
	_, err = c.FPutObject(context.Background(), bucketName, objectName+"-standard", file.Name(), minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "FPutObject failed", err)
		return
	}

	// Perform FPutObject with no contentType provided (Expecting application/octet-stream)
	args["objectName"] = objectName + "-Octet"
	args["contentType"] = ""

	_, err = c.FPutObject(context.Background(), bucketName, objectName+"-Octet", file.Name(), minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "FPutObject failed", err)
		return
	}

	// Add extension to temp file name
	fileName := file.Name()
	err = os.Rename(fileName, fileName+".gtar")
	if err != nil {
		logError(testName, function, args, startTime, "", "Rename failed", err)
		return
	}

	// Perform FPutObject with no contentType provided (Expecting application/x-gtar)
	args["objectName"] = objectName + "-Octet"
	args["contentType"] = ""
	args["fileName"] = fileName + ".gtar"

	_, err = c.FPutObject(context.Background(), bucketName, objectName+"-GTar", fileName+".gtar", minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "FPutObject failed", err)
		return
	}

	// Check headers and sizes
	rStandard, err := c.StatObject(context.Background(), bucketName, objectName+"-standard", minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject failed", err)
		return
	}

	if rStandard.Size != 11*1024*1024 {
		logError(testName, function, args, startTime, "", "Unexpected size", nil)
		return
	}

	if rStandard.ContentType != "application/octet-stream" {
		logError(testName, function, args, startTime, "", "Content-Type headers mismatched, expected: application/octet-stream , got "+rStandard.ContentType, err)
		return
	}

	rOctet, err := c.StatObject(context.Background(), bucketName, objectName+"-Octet", minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject failed", err)
		return
	}
	if rOctet.ContentType != "application/octet-stream" {
		logError(testName, function, args, startTime, "", "Content-Type headers mismatched, expected: application/octet-stream , got "+rOctet.ContentType, err)
		return
	}

	if rOctet.Size != 11*1024*1024 {
		logError(testName, function, args, startTime, "", "Unexpected size", nil)
		return
	}

	rGTar, err := c.StatObject(context.Background(), bucketName, objectName+"-GTar", minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject failed", err)
		return
	}
	if rGTar.Size != 11*1024*1024 {
		logError(testName, function, args, startTime, "", "Unexpected size", nil)
		return
	}
	if rGTar.ContentType != "application/x-gtar" && rGTar.ContentType != "application/octet-stream" {
		logError(testName, function, args, startTime, "", "Content-Type headers mismatched, expected: application/x-gtar , got "+rGTar.ContentType, err)
		return
	}

	os.Remove(fileName + ".gtar")
	successLogger(testName, function, args, startTime).Info()
}

// Tests various bucket supported formats.
func testMakeBucketRegionsV2() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "MakeBucket(bucketName, region)"
	args := map[string]interface{}{
		"bucketName": "",
		"region":     "eu-west-1",
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV2(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v2 client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket in 'eu-central-1'.
	if err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "eu-west-1"}); err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	if err = cleanupBucket(bucketName, c); err != nil {
		logError(testName, function, args, startTime, "", "CleanupBucket failed while removing bucket recursively", err)
		return
	}

	// Make a new bucket with '.' in its name, in 'us-west-2'. This
	// request is internally staged into a path style instead of
	// virtual host style.
	if err = c.MakeBucket(context.Background(), bucketName+".withperiod", minio.MakeBucketOptions{Region: "us-west-2"}); err != nil {
		args["bucketName"] = bucketName + ".withperiod"
		args["region"] = "us-west-2"
		logError(testName, function, args, startTime, "", "MakeBucket test with a bucket name with period, '.', failed", err)
		return
	}

	// Delete all objects and buckets
	if err = cleanupBucket(bucketName+".withperiod", c); err != nil {
		logError(testName, function, args, startTime, "", "CleanupBucket failed while removing bucket recursively", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Tests get object ReaderSeeker interface methods.
func testGetObjectReadSeekFunctionalV2() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "GetObject(bucketName, objectName)"
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV2(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v2 client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Generate 33K of data.
	bufSize := dataFileMap["datafile-33-kB"]
	var reader = getDataReader("datafile-33-kB")
	defer reader.Close()

	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	buf, err := ioutil.ReadAll(reader)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAll failed", err)
		return
	}

	// Save the data.
	_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(bufSize), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	// Read the data back
	r, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}
	defer r.Close()

	st, err := r.Stat()
	if err != nil {
		logError(testName, function, args, startTime, "", "Stat failed", err)
		return
	}

	if st.Size != int64(bufSize) {
		logError(testName, function, args, startTime, "", "Number of bytes in stat does not match, expected "+string(int64(bufSize))+" got "+string(st.Size), err)
		return
	}

	offset := int64(2048)
	n, err := r.Seek(offset, 0)
	if err != nil {
		logError(testName, function, args, startTime, "", "Seek failed", err)
		return
	}
	if n != offset {
		logError(testName, function, args, startTime, "", "Number of seeked bytes does not match, expected "+string(offset)+" got "+string(n), err)
		return
	}
	n, err = r.Seek(0, 1)
	if err != nil {
		logError(testName, function, args, startTime, "", "Seek failed", err)
		return
	}
	if n != offset {
		logError(testName, function, args, startTime, "", "Number of seeked bytes does not match, expected "+string(offset)+" got "+string(n), err)
		return
	}
	_, err = r.Seek(offset, 2)
	if err == nil {
		logError(testName, function, args, startTime, "", "Seek on positive offset for whence '2' should error out", err)
		return
	}
	n, err = r.Seek(-offset, 2)
	if err != nil {
		logError(testName, function, args, startTime, "", "Seek failed", err)
		return
	}
	if n != st.Size-offset {
		logError(testName, function, args, startTime, "", "Number of seeked bytes does not match, expected "+string(st.Size-offset)+" got "+string(n), err)
		return
	}

	var buffer1 bytes.Buffer
	if _, err = io.CopyN(&buffer1, r, st.Size); err != nil {
		if err != io.EOF {
			logError(testName, function, args, startTime, "", "Copy failed", err)
			return
		}
	}
	if !bytes.Equal(buf[len(buf)-int(offset):], buffer1.Bytes()) {
		logError(testName, function, args, startTime, "", "Incorrect read bytes v/s original buffer", err)
		return
	}

	// Seek again and read again.
	n, err = r.Seek(offset-1, 0)
	if err != nil {
		logError(testName, function, args, startTime, "", "Seek failed", err)
		return
	}
	if n != (offset - 1) {
		logError(testName, function, args, startTime, "", "Number of seeked bytes does not match, expected "+string(offset-1)+" got "+string(n), err)
		return
	}

	var buffer2 bytes.Buffer
	if _, err = io.CopyN(&buffer2, r, st.Size); err != nil {
		if err != io.EOF {
			logError(testName, function, args, startTime, "", "Copy failed", err)
			return
		}
	}
	// Verify now lesser bytes.
	if !bytes.Equal(buf[2047:], buffer2.Bytes()) {
		logError(testName, function, args, startTime, "", "Incorrect read bytes v/s original buffer", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Tests get object ReaderAt interface methods.
func testGetObjectReadAtFunctionalV2() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "GetObject(bucketName, objectName)"
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV2(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v2 client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Generate 33K of data.
	bufSize := dataFileMap["datafile-33-kB"]
	var reader = getDataReader("datafile-33-kB")
	defer reader.Close()

	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	buf, err := ioutil.ReadAll(reader)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAll failed", err)
		return
	}

	// Save the data
	_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(bufSize), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	// Read the data back
	r, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}
	defer r.Close()

	st, err := r.Stat()
	if err != nil {
		logError(testName, function, args, startTime, "", "Stat failed", err)
		return
	}

	if st.Size != int64(bufSize) {
		logError(testName, function, args, startTime, "", "Number of bytes does not match, expected "+string(bufSize)+" got "+string(st.Size), err)
		return
	}

	offset := int64(2048)

	// Read directly
	buf2 := make([]byte, 512)
	buf3 := make([]byte, 512)
	buf4 := make([]byte, 512)

	m, err := r.ReadAt(buf2, offset)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAt failed", err)
		return
	}
	if m != len(buf2) {
		logError(testName, function, args, startTime, "", "ReadAt read shorter bytes before reaching EOF, expected "+string(len(buf2))+" got "+string(m), err)
		return
	}
	if !bytes.Equal(buf2, buf[offset:offset+512]) {
		logError(testName, function, args, startTime, "", "Incorrect read between two ReadAt from same offset", err)
		return
	}
	offset += 512
	m, err = r.ReadAt(buf3, offset)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAt failed", err)
		return
	}
	if m != len(buf3) {
		logError(testName, function, args, startTime, "", "ReadAt read shorter bytes before reaching EOF, expected "+string(len(buf3))+" got "+string(m), err)
		return
	}
	if !bytes.Equal(buf3, buf[offset:offset+512]) {
		logError(testName, function, args, startTime, "", "Incorrect read between two ReadAt from same offset", err)
		return
	}
	offset += 512
	m, err = r.ReadAt(buf4, offset)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAt failed", err)
		return
	}
	if m != len(buf4) {
		logError(testName, function, args, startTime, "", "ReadAt read shorter bytes before reaching EOF, expected "+string(len(buf4))+" got "+string(m), err)
		return
	}
	if !bytes.Equal(buf4, buf[offset:offset+512]) {
		logError(testName, function, args, startTime, "", "Incorrect read between two ReadAt from same offset", err)
		return
	}

	buf5 := make([]byte, bufSize)
	// Read the whole object.
	m, err = r.ReadAt(buf5, 0)
	if err != nil {
		if err != io.EOF {
			logError(testName, function, args, startTime, "", "ReadAt failed", err)
			return
		}
	}
	if m != len(buf5) {
		logError(testName, function, args, startTime, "", "ReadAt read shorter bytes before reaching EOF, expected "+string(len(buf5))+" got "+string(m), err)
		return
	}
	if !bytes.Equal(buf, buf5) {
		logError(testName, function, args, startTime, "", "Incorrect data read in GetObject, than what was previously uploaded", err)
		return
	}

	buf6 := make([]byte, bufSize+1)
	// Read the whole object and beyond.
	_, err = r.ReadAt(buf6, 0)
	if err != nil {
		if err != io.EOF {
			logError(testName, function, args, startTime, "", "ReadAt failed", err)
			return
		}
	}

	successLogger(testName, function, args, startTime).Info()
}

// Tests copy object
func testCopyObjectV2() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObject(destination, source)"
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV2(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v2 client object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")

	// Make a new bucket in 'us-east-1' (source bucket).
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}
	defer cleanupBucket(bucketName, c)

	// Make a new bucket in 'us-east-1' (destination bucket).
	err = c.MakeBucket(context.Background(), bucketName+"-copy", minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}
	defer cleanupBucket(bucketName+"-copy", c)

	// Generate 33K of data.
	bufSize := dataFileMap["datafile-33-kB"]
	var reader = getDataReader("datafile-33-kB")
	defer reader.Close()

	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	_, err = c.PutObject(context.Background(), bucketName, objectName, reader, int64(bufSize), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	r, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}
	// Check the various fields of source object against destination object.
	objInfo, err := r.Stat()
	if err != nil {
		logError(testName, function, args, startTime, "", "Stat failed", err)
		return
	}
	r.Close()

	// Copy Source
	src := minio.CopySrcOptions{
		Bucket:             bucketName,
		Object:             objectName,
		MatchModifiedSince: time.Date(2014, time.April, 0, 0, 0, 0, 0, time.UTC),
		MatchETag:          objInfo.ETag,
	}
	args["source"] = src

	// Set copy conditions.
	dst := minio.CopyDestOptions{
		Bucket: bucketName + "-copy",
		Object: objectName + "-copy",
	}
	args["destination"] = dst

	// Perform the Copy
	_, err = c.CopyObject(context.Background(), dst, src)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObject failed", err)
		return
	}

	// Source object
	r, err = c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}
	// Destination object
	readerCopy, err := c.GetObject(context.Background(), bucketName+"-copy", objectName+"-copy", minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}
	// Check the various fields of source object against destination object.
	objInfo, err = r.Stat()
	if err != nil {
		logError(testName, function, args, startTime, "", "Stat failed", err)
		return
	}
	objInfoCopy, err := readerCopy.Stat()
	if err != nil {
		logError(testName, function, args, startTime, "", "Stat failed", err)
		return
	}
	if objInfo.Size != objInfoCopy.Size {
		logError(testName, function, args, startTime, "", "Number of bytes does not match, expected "+string(objInfoCopy.Size)+" got "+string(objInfo.Size), err)
		return
	}

	// Close all the readers.
	r.Close()
	readerCopy.Close()

	// CopyObject again but with wrong conditions
	src = minio.CopySrcOptions{
		Bucket:               bucketName,
		Object:               objectName,
		MatchUnmodifiedSince: time.Date(2014, time.April, 0, 0, 0, 0, 0, time.UTC),
		NoMatchETag:          objInfo.ETag,
	}

	// Perform the Copy which should fail
	_, err = c.CopyObject(context.Background(), dst, src)
	if err == nil {
		logError(testName, function, args, startTime, "", "CopyObject did not fail for invalid conditions", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

func testComposeObjectErrorCasesWrapper(c *minio.Client) {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "ComposeObject(destination, sourceList)"
	args := map[string]interface{}{}

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")

	// Make a new bucket in 'us-east-1' (source bucket).
	err := c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})

	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Test that more than 10K source objects cannot be
	// concatenated.
	srcArr := [10001]minio.CopySrcOptions{}
	srcSlice := srcArr[:]
	dst := minio.CopyDestOptions{
		Bucket: bucketName,
		Object: "object",
	}

	args["destination"] = dst
	// Just explain about srcArr in args["sourceList"]
	// to stop having 10,001 null headers logged
	args["sourceList"] = "source array of 10,001 elements"
	if _, err := c.ComposeObject(context.Background(), dst, srcSlice...); err == nil {
		logError(testName, function, args, startTime, "", "Expected error in ComposeObject", err)
		return
	} else if err.Error() != "There must be as least one and up to 10000 source objects." {
		logError(testName, function, args, startTime, "", "Got unexpected error", err)
		return
	}

	// Create a source with invalid offset spec and check that
	// error is returned:
	// 1. Create the source object.
	const badSrcSize = 5 * 1024 * 1024
	buf := bytes.Repeat([]byte("1"), badSrcSize)
	_, err = c.PutObject(context.Background(), bucketName, "badObject", bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}
	// 2. Set invalid range spec on the object (going beyond
	// object size)
	badSrc := minio.CopySrcOptions{
		Bucket:     bucketName,
		Object:     "badObject",
		MatchRange: true,
		Start:      1,
		End:        badSrcSize,
	}

	// 3. ComposeObject call should fail.
	if _, err := c.ComposeObject(context.Background(), dst, badSrc); err == nil {
		logError(testName, function, args, startTime, "", "ComposeObject expected to fail", err)
		return
	} else if !strings.Contains(err.Error(), "has invalid segment-to-copy") {
		logError(testName, function, args, startTime, "", "Got invalid error", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Test expected error cases
func testComposeObjectErrorCasesV2() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "ComposeObject(destination, sourceList)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV2(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v2 client object creation failed", err)
		return
	}

	testComposeObjectErrorCasesWrapper(c)
}

func testComposeMultipleSources(c *minio.Client) {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "ComposeObject(destination, sourceList)"
	args := map[string]interface{}{
		"destination": "",
		"sourceList":  "",
	}

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	// Make a new bucket in 'us-east-1' (source bucket).
	err := c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Upload a small source object
	const srcSize = 1024 * 1024 * 5
	buf := bytes.Repeat([]byte("1"), srcSize)
	_, err = c.PutObject(context.Background(), bucketName, "srcObject", bytes.NewReader(buf), int64(srcSize), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	// We will append 10 copies of the object.
	srcs := []minio.CopySrcOptions{}
	for i := 0; i < 10; i++ {
		srcs = append(srcs, minio.CopySrcOptions{
			Bucket: bucketName,
			Object: "srcObject",
		})
	}

	// make the last part very small
	srcs[9].MatchRange = true

	args["sourceList"] = srcs

	dst := minio.CopyDestOptions{
		Bucket: bucketName,
		Object: "dstObject",
	}
	args["destination"] = dst

	ui, err := c.ComposeObject(context.Background(), dst, srcs...)
	if err != nil {
		logError(testName, function, args, startTime, "", "ComposeObject failed", err)
		return
	}

	if ui.Size != 9*srcSize+1 {
		logError(testName, function, args, startTime, "", "ComposeObject returned unexpected size", err)
		return
	}

	objProps, err := c.StatObject(context.Background(), bucketName, "dstObject", minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject failed", err)
		return
	}

	if objProps.Size != 9*srcSize+1 {
		logError(testName, function, args, startTime, "", "Size mismatched! Expected "+string(10000*srcSize)+" got "+string(objProps.Size), err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Test concatenating multiple 10K objects V2
func testCompose10KSourcesV2() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "ComposeObject(destination, sourceList)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV2(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v2 client object creation failed", err)
		return
	}

	testComposeMultipleSources(c)
}

func testEncryptedEmptyObject() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "PutObject(bucketName, objectName, reader, objectSize, opts)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v4 client object creation failed", err)
		return
	}

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName
	// Make a new bucket in 'us-east-1' (source bucket).
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	sse := encrypt.DefaultPBKDF([]byte("correct horse battery staple"), []byte(bucketName+"object"))

	// 1. create an sse-c encrypted object to copy by uploading
	const srcSize = 0
	var buf []byte // Empty buffer
	args["objectName"] = "object"
	_, err = c.PutObject(context.Background(), bucketName, "object", bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{ServerSideEncryption: sse})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject call failed", err)
		return
	}

	// 2. Test CopyObject for an empty object
	src := minio.CopySrcOptions{
		Bucket:     bucketName,
		Object:     "object",
		Encryption: sse,
	}

	dst := minio.CopyDestOptions{
		Bucket:     bucketName,
		Object:     "new-object",
		Encryption: sse,
	}

	if _, err = c.CopyObject(context.Background(), dst, src); err != nil {
		function = "CopyObject(dst, src)"
		logError(testName, function, map[string]interface{}{}, startTime, "", "CopyObject failed", err)
		return
	}

	// 3. Test Key rotation
	newSSE := encrypt.DefaultPBKDF([]byte("Don't Panic"), []byte(bucketName+"new-object"))
	src = minio.CopySrcOptions{
		Bucket:     bucketName,
		Object:     "new-object",
		Encryption: sse,
	}

	dst = minio.CopyDestOptions{
		Bucket:     bucketName,
		Object:     "new-object",
		Encryption: newSSE,
	}

	if _, err = c.CopyObject(context.Background(), dst, src); err != nil {
		function = "CopyObject(dst, src)"
		logError(testName, function, map[string]interface{}{}, startTime, "", "CopyObject with key rotation failed", err)
		return
	}

	// 4. Download the object.
	reader, err := c.GetObject(context.Background(), bucketName, "new-object", minio.GetObjectOptions{ServerSideEncryption: newSSE})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}
	defer reader.Close()

	decBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		logError(testName, function, map[string]interface{}{}, startTime, "", "ReadAll failed", err)
		return
	}
	if !bytes.Equal(decBytes, buf) {
		logError(testName, function, map[string]interface{}{}, startTime, "", "Downloaded object doesn't match the empty encrypted object", err)
		return
	}

	delete(args, "objectName")
	successLogger(testName, function, args, startTime).Info()
}

func testEncryptedCopyObjectWrapper(c *minio.Client, bucketName string, sseSrc, sseDst encrypt.ServerSide) {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncNameLoc(2)
	function := "CopyObject(destination, source)"
	args := map[string]interface{}{}
	var srcEncryption, dstEncryption encrypt.ServerSide

	// Make a new bucket in 'us-east-1' (source bucket).
	err := c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// 1. create an sse-c encrypted object to copy by uploading
	const srcSize = 1024 * 1024
	buf := bytes.Repeat([]byte("abcde"), srcSize) // gives a buffer of 5MiB
	_, err = c.PutObject(context.Background(), bucketName, "srcObject", bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{
		ServerSideEncryption: sseSrc,
	})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject call failed", err)
		return
	}

	if sseSrc != nil && sseSrc.Type() != encrypt.S3 {
		srcEncryption = sseSrc
	}

	// 2. copy object and change encryption key
	src := minio.CopySrcOptions{
		Bucket:     bucketName,
		Object:     "srcObject",
		Encryption: srcEncryption,
	}
	args["source"] = src

	dst := minio.CopyDestOptions{
		Bucket:     bucketName,
		Object:     "dstObject",
		Encryption: sseDst,
	}
	args["destination"] = dst

	_, err = c.CopyObject(context.Background(), dst, src)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObject failed", err)
		return
	}

	if sseDst != nil && sseDst.Type() != encrypt.S3 {
		dstEncryption = sseDst
	}
	// 3. get copied object and check if content is equal
	coreClient := minio.Core{c}
	reader, _, _, err := coreClient.GetObject(context.Background(), bucketName, "dstObject", minio.GetObjectOptions{ServerSideEncryption: dstEncryption})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}

	decBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAll failed", err)
		return
	}
	if !bytes.Equal(decBytes, buf) {
		logError(testName, function, args, startTime, "", "Downloaded object mismatched for encrypted object", err)
		return
	}
	reader.Close()

	// Test key rotation for source object in-place.
	var newSSE encrypt.ServerSide
	if sseSrc != nil && sseSrc.Type() == encrypt.SSEC {
		newSSE = encrypt.DefaultPBKDF([]byte("Don't Panic"), []byte(bucketName+"srcObject")) // replace key
	}
	if sseSrc != nil && sseSrc.Type() == encrypt.S3 {
		newSSE = encrypt.NewSSE()
	}
	if newSSE != nil {
		dst = minio.CopyDestOptions{
			Bucket:     bucketName,
			Object:     "srcObject",
			Encryption: newSSE,
		}
		args["destination"] = dst

		_, err = c.CopyObject(context.Background(), dst, src)
		if err != nil {
			logError(testName, function, args, startTime, "", "CopyObject failed", err)
			return
		}

		// Get copied object and check if content is equal
		reader, _, _, err = coreClient.GetObject(context.Background(), bucketName, "srcObject", minio.GetObjectOptions{ServerSideEncryption: newSSE})
		if err != nil {
			logError(testName, function, args, startTime, "", "GetObject failed", err)
			return
		}

		decBytes, err = ioutil.ReadAll(reader)
		if err != nil {
			logError(testName, function, args, startTime, "", "ReadAll failed", err)
			return
		}
		if !bytes.Equal(decBytes, buf) {
			logError(testName, function, args, startTime, "", "Downloaded object mismatched for encrypted object", err)
			return
		}
		reader.Close()

		// Test in-place decryption.
		dst = minio.CopyDestOptions{
			Bucket: bucketName,
			Object: "srcObject",
		}
		args["destination"] = dst

		src = minio.CopySrcOptions{
			Bucket:     bucketName,
			Object:     "srcObject",
			Encryption: newSSE,
		}
		args["source"] = src
		_, err = c.CopyObject(context.Background(), dst, src)
		if err != nil {
			logError(testName, function, args, startTime, "", "CopyObject Key rotation failed", err)
			return
		}
	}

	// Get copied decrypted object and check if content is equal
	reader, _, _, err = coreClient.GetObject(context.Background(), bucketName, "srcObject", minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}
	defer reader.Close()

	decBytes, err = ioutil.ReadAll(reader)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAll failed", err)
		return
	}
	if !bytes.Equal(decBytes, buf) {
		logError(testName, function, args, startTime, "", "Downloaded object mismatched for encrypted object", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Test encrypted copy object
func testUnencryptedToSSECCopyObject() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObject(destination, source)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v2 client object creation failed", err)
		return
	}
	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")

	sseDst := encrypt.DefaultPBKDF([]byte("correct horse battery staple"), []byte(bucketName+"dstObject"))
	// c.TraceOn(os.Stderr)
	testEncryptedCopyObjectWrapper(c, bucketName, nil, sseDst)
}

// Test encrypted copy object
func testUnencryptedToSSES3CopyObject() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObject(destination, source)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v2 client object creation failed", err)
		return
	}
	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")

	var sseSrc encrypt.ServerSide
	sseDst := encrypt.NewSSE()
	// c.TraceOn(os.Stderr)
	testEncryptedCopyObjectWrapper(c, bucketName, sseSrc, sseDst)
}

// Test encrypted copy object
func testUnencryptedToUnencryptedCopyObject() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObject(destination, source)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v2 client object creation failed", err)
		return
	}
	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")

	var sseSrc, sseDst encrypt.ServerSide
	// c.TraceOn(os.Stderr)
	testEncryptedCopyObjectWrapper(c, bucketName, sseSrc, sseDst)
}

// Test encrypted copy object
func testEncryptedSSECToSSECCopyObject() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObject(destination, source)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v2 client object creation failed", err)
		return
	}
	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")

	sseSrc := encrypt.DefaultPBKDF([]byte("correct horse battery staple"), []byte(bucketName+"srcObject"))
	sseDst := encrypt.DefaultPBKDF([]byte("correct horse battery staple"), []byte(bucketName+"dstObject"))
	// c.TraceOn(os.Stderr)
	testEncryptedCopyObjectWrapper(c, bucketName, sseSrc, sseDst)
}

// Test encrypted copy object
func testEncryptedSSECToSSES3CopyObject() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObject(destination, source)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v2 client object creation failed", err)
		return
	}
	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")

	sseSrc := encrypt.DefaultPBKDF([]byte("correct horse battery staple"), []byte(bucketName+"srcObject"))
	sseDst := encrypt.NewSSE()
	// c.TraceOn(os.Stderr)
	testEncryptedCopyObjectWrapper(c, bucketName, sseSrc, sseDst)
}

// Test encrypted copy object
func testEncryptedSSECToUnencryptedCopyObject() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObject(destination, source)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v2 client object creation failed", err)
		return
	}
	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")

	sseSrc := encrypt.DefaultPBKDF([]byte("correct horse battery staple"), []byte(bucketName+"srcObject"))
	var sseDst encrypt.ServerSide
	// c.TraceOn(os.Stderr)
	testEncryptedCopyObjectWrapper(c, bucketName, sseSrc, sseDst)
}

// Test encrypted copy object
func testEncryptedSSES3ToSSECCopyObject() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObject(destination, source)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v2 client object creation failed", err)
		return
	}
	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")

	sseSrc := encrypt.NewSSE()
	sseDst := encrypt.DefaultPBKDF([]byte("correct horse battery staple"), []byte(bucketName+"dstObject"))
	// c.TraceOn(os.Stderr)
	testEncryptedCopyObjectWrapper(c, bucketName, sseSrc, sseDst)
}

// Test encrypted copy object
func testEncryptedSSES3ToSSES3CopyObject() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObject(destination, source)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v2 client object creation failed", err)
		return
	}
	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")

	sseSrc := encrypt.NewSSE()
	sseDst := encrypt.NewSSE()
	// c.TraceOn(os.Stderr)
	testEncryptedCopyObjectWrapper(c, bucketName, sseSrc, sseDst)
}

// Test encrypted copy object
func testEncryptedSSES3ToUnencryptedCopyObject() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObject(destination, source)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v2 client object creation failed", err)
		return
	}
	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")

	sseSrc := encrypt.NewSSE()
	var sseDst encrypt.ServerSide
	// c.TraceOn(os.Stderr)
	testEncryptedCopyObjectWrapper(c, bucketName, sseSrc, sseDst)
}

// Test encrypted copy object
func testEncryptedCopyObjectV2() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObject(destination, source)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV2(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v2 client object creation failed", err)
		return
	}
	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")

	sseSrc := encrypt.DefaultPBKDF([]byte("correct horse battery staple"), []byte(bucketName+"srcObject"))
	sseDst := encrypt.DefaultPBKDF([]byte("correct horse battery staple"), []byte(bucketName+"dstObject"))
	// c.TraceOn(os.Stderr)
	testEncryptedCopyObjectWrapper(c, bucketName, sseSrc, sseDst)
}

func testDecryptedCopyObject() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObject(destination, source)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v2 client object creation failed", err)
		return
	}

	bucketName, objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-"), "object"
	if err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"}); err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	encryption := encrypt.DefaultPBKDF([]byte("correct horse battery staple"), []byte(bucketName+objectName))
	_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(bytes.Repeat([]byte("a"), 1024*1024)), 1024*1024, minio.PutObjectOptions{
		ServerSideEncryption: encryption,
	})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject call failed", err)
		return
	}

	src := minio.CopySrcOptions{
		Bucket:     bucketName,
		Object:     objectName,
		Encryption: encrypt.SSECopy(encryption),
	}
	args["source"] = src

	dst := minio.CopyDestOptions{
		Bucket: bucketName,
		Object: "decrypted-" + objectName,
	}
	args["destination"] = dst

	if _, err = c.CopyObject(context.Background(), dst, src); err != nil {
		logError(testName, function, args, startTime, "", "CopyObject failed", err)
		return
	}
	if _, err = c.GetObject(context.Background(), bucketName, "decrypted-"+objectName, minio.GetObjectOptions{}); err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}
	successLogger(testName, function, args, startTime).Info()
}

func testSSECMultipartEncryptedToSSECCopyObjectPart() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObjectPart(destination, source)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	client, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v4 client object creation failed", err)
		return
	}

	// Instantiate new core client object.
	c := minio.Core{client}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test")

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}
	defer cleanupBucket(bucketName, client)
	// Make a buffer with 6MB of data
	buf := bytes.Repeat([]byte("abcdef"), 1024*1024)

	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	password := "correct horse battery staple"
	srcencryption := encrypt.DefaultPBKDF([]byte(password), []byte(bucketName+objectName))

	// Upload a 6MB object using multipart mechanism
	uploadID, err := c.NewMultipartUpload(context.Background(), bucketName, objectName, minio.PutObjectOptions{ServerSideEncryption: srcencryption})
	if err != nil {
		logError(testName, function, args, startTime, "", "NewMultipartUpload call failed", err)
		return
	}

	var completeParts []minio.CompletePart

	part, err := c.PutObjectPart(context.Background(), bucketName, objectName, uploadID, 1, bytes.NewReader(buf[:5*1024*1024]), 5*1024*1024, "", "", srcencryption)
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObjectPart call failed", err)
		return
	}
	completeParts = append(completeParts, minio.CompletePart{PartNumber: part.PartNumber, ETag: part.ETag})

	part, err = c.PutObjectPart(context.Background(), bucketName, objectName, uploadID, 2, bytes.NewReader(buf[5*1024*1024:]), 1024*1024, "", "", srcencryption)
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObjectPart call failed", err)
		return
	}
	completeParts = append(completeParts, minio.CompletePart{PartNumber: part.PartNumber, ETag: part.ETag})

	// Complete the multipart upload
	_, err = c.CompleteMultipartUpload(context.Background(), bucketName, objectName, uploadID, completeParts, minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "CompleteMultipartUpload call failed", err)
		return
	}

	// Stat the object and check its length matches
	objInfo, err := c.StatObject(context.Background(), bucketName, objectName, minio.StatObjectOptions{ServerSideEncryption: srcencryption})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject call failed", err)
		return
	}

	destBucketName := bucketName
	destObjectName := objectName + "-dest"
	dstencryption := encrypt.DefaultPBKDF([]byte(password), []byte(destBucketName+destObjectName))

	uploadID, err = c.NewMultipartUpload(context.Background(), destBucketName, destObjectName, minio.PutObjectOptions{ServerSideEncryption: dstencryption})
	if err != nil {
		logError(testName, function, args, startTime, "", "NewMultipartUpload call failed", err)
		return
	}

	// Content of the destination object will be two copies of
	// `objectName` concatenated, followed by first byte of
	// `objectName`.
	metadata := make(map[string]string)
	header := make(http.Header)
	encrypt.SSECopy(srcencryption).Marshal(header)
	dstencryption.Marshal(header)
	for k, v := range header {
		metadata[k] = v[0]
	}

	metadata["x-amz-copy-source-if-match"] = objInfo.ETag

	// First of three parts
	fstPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 1, 0, -1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Second of three parts
	sndPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 2, 0, -1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Last of three parts
	lstPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 3, 0, 1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Complete the multipart upload
	_, err = c.CompleteMultipartUpload(context.Background(), destBucketName, destObjectName, uploadID, []minio.CompletePart{fstPart, sndPart, lstPart}, minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "CompleteMultipartUpload call failed", err)
		return
	}

	// Stat the object and check its length matches
	objInfo, err = c.StatObject(context.Background(), destBucketName, destObjectName, minio.StatObjectOptions{ServerSideEncryption: dstencryption})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject call failed", err)
		return
	}

	if objInfo.Size != (6*1024*1024)*2+1 {
		logError(testName, function, args, startTime, "", "Destination object has incorrect size!", err)
		return
	}

	// Now we read the data back
	getOpts := minio.GetObjectOptions{ServerSideEncryption: dstencryption}
	getOpts.SetRange(0, 6*1024*1024-1)
	r, _, _, err := c.GetObject(context.Background(), destBucketName, destObjectName, getOpts)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject call failed", err)
		return
	}
	getBuf := make([]byte, 6*1024*1024)
	_, err = readFull(r, getBuf)
	if err != nil {
		logError(testName, function, args, startTime, "", "Read buffer failed", err)
		return
	}
	if !bytes.Equal(getBuf, buf) {
		logError(testName, function, args, startTime, "", "Got unexpected data in first 6MB", err)
		return
	}

	getOpts.SetRange(6*1024*1024, 0)
	r, _, _, err = c.GetObject(context.Background(), destBucketName, destObjectName, getOpts)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject call failed", err)
		return
	}
	getBuf = make([]byte, 6*1024*1024+1)
	_, err = readFull(r, getBuf)
	if err != nil {
		logError(testName, function, args, startTime, "", "Read buffer failed", err)
		return
	}
	if !bytes.Equal(getBuf[:6*1024*1024], buf) {
		logError(testName, function, args, startTime, "", "Got unexpected data in second 6MB", err)
		return
	}
	if getBuf[6*1024*1024] != buf[0] {
		logError(testName, function, args, startTime, "", "Got unexpected data in last byte of copied object!", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()

	// Do not need to remove destBucketName its same as bucketName.
}

// Test Core CopyObjectPart implementation
func testSSECEncryptedToSSECCopyObjectPart() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObjectPart(destination, source)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	client, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v4 client object creation failed", err)
		return
	}

	// Instantiate new core client object.
	c := minio.Core{client}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test")

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}
	defer cleanupBucket(bucketName, client)
	// Make a buffer with 5MB of data
	buf := bytes.Repeat([]byte("abcde"), 1024*1024)

	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	password := "correct horse battery staple"
	srcencryption := encrypt.DefaultPBKDF([]byte(password), []byte(bucketName+objectName))
	putmetadata := map[string]string{
		"Content-Type": "binary/octet-stream",
	}
	opts := minio.PutObjectOptions{
		UserMetadata:         putmetadata,
		ServerSideEncryption: srcencryption,
	}
	uploadInfo, err := c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), "", "", opts)
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject call failed", err)
		return
	}

	st, err := c.StatObject(context.Background(), bucketName, objectName, minio.StatObjectOptions{ServerSideEncryption: srcencryption})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject call failed", err)
		return
	}

	if st.Size != int64(len(buf)) {
		logError(testName, function, args, startTime, "", fmt.Sprintf("Error: number of bytes does not match, want %v, got %v\n", len(buf), st.Size), err)
		return
	}

	destBucketName := bucketName
	destObjectName := objectName + "-dest"
	dstencryption := encrypt.DefaultPBKDF([]byte(password), []byte(destBucketName+destObjectName))

	uploadID, err := c.NewMultipartUpload(context.Background(), destBucketName, destObjectName, minio.PutObjectOptions{ServerSideEncryption: dstencryption})
	if err != nil {
		logError(testName, function, args, startTime, "", "NewMultipartUpload call failed", err)
		return
	}

	// Content of the destination object will be two copies of
	// `objectName` concatenated, followed by first byte of
	// `objectName`.
	metadata := make(map[string]string)
	header := make(http.Header)
	encrypt.SSECopy(srcencryption).Marshal(header)
	dstencryption.Marshal(header)
	for k, v := range header {
		metadata[k] = v[0]
	}

	metadata["x-amz-copy-source-if-match"] = uploadInfo.ETag

	// First of three parts
	fstPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 1, 0, -1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Second of three parts
	sndPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 2, 0, -1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Last of three parts
	lstPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 3, 0, 1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Complete the multipart upload
	_, err = c.CompleteMultipartUpload(context.Background(), destBucketName, destObjectName, uploadID, []minio.CompletePart{fstPart, sndPart, lstPart}, minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "CompleteMultipartUpload call failed", err)
		return
	}

	// Stat the object and check its length matches
	objInfo, err := c.StatObject(context.Background(), destBucketName, destObjectName, minio.StatObjectOptions{ServerSideEncryption: dstencryption})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject call failed", err)
		return
	}

	if objInfo.Size != (5*1024*1024)*2+1 {
		logError(testName, function, args, startTime, "", "Destination object has incorrect size!", err)
		return
	}

	// Now we read the data back
	getOpts := minio.GetObjectOptions{ServerSideEncryption: dstencryption}
	getOpts.SetRange(0, 5*1024*1024-1)
	r, _, _, err := c.GetObject(context.Background(), destBucketName, destObjectName, getOpts)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject call failed", err)
		return
	}
	getBuf := make([]byte, 5*1024*1024)
	_, err = readFull(r, getBuf)
	if err != nil {
		logError(testName, function, args, startTime, "", "Read buffer failed", err)
		return
	}
	if !bytes.Equal(getBuf, buf) {
		logError(testName, function, args, startTime, "", "Got unexpected data in first 5MB", err)
		return
	}

	getOpts.SetRange(5*1024*1024, 0)
	r, _, _, err = c.GetObject(context.Background(), destBucketName, destObjectName, getOpts)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject call failed", err)
		return
	}
	getBuf = make([]byte, 5*1024*1024+1)
	_, err = readFull(r, getBuf)
	if err != nil {
		logError(testName, function, args, startTime, "", "Read buffer failed", err)
		return
	}
	if !bytes.Equal(getBuf[:5*1024*1024], buf) {
		logError(testName, function, args, startTime, "", "Got unexpected data in second 5MB", err)
		return
	}
	if getBuf[5*1024*1024] != buf[0] {
		logError(testName, function, args, startTime, "", "Got unexpected data in last byte of copied object!", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()

	// Do not need to remove destBucketName its same as bucketName.
}

// Test Core CopyObjectPart implementation for SSEC encrypted to unencrypted copy
func testSSECEncryptedToUnencryptedCopyPart() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObjectPart(destination, source)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	client, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v4 client object creation failed", err)
		return
	}

	// Instantiate new core client object.
	c := minio.Core{client}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test")

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}
	defer cleanupBucket(bucketName, client)
	// Make a buffer with 5MB of data
	buf := bytes.Repeat([]byte("abcde"), 1024*1024)

	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	password := "correct horse battery staple"
	srcencryption := encrypt.DefaultPBKDF([]byte(password), []byte(bucketName+objectName))

	opts := minio.PutObjectOptions{
		UserMetadata: map[string]string{
			"Content-Type": "binary/octet-stream",
		},
		ServerSideEncryption: srcencryption,
	}
	uploadInfo, err := c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), "", "", opts)
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject call failed", err)
		return
	}

	st, err := c.StatObject(context.Background(), bucketName, objectName, minio.StatObjectOptions{ServerSideEncryption: srcencryption})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject call failed", err)
		return
	}

	if st.Size != int64(len(buf)) {
		logError(testName, function, args, startTime, "", fmt.Sprintf("Error: number of bytes does not match, want %v, got %v\n", len(buf), st.Size), err)
		return
	}

	destBucketName := bucketName
	destObjectName := objectName + "-dest"
	var dstencryption encrypt.ServerSide

	uploadID, err := c.NewMultipartUpload(context.Background(), destBucketName, destObjectName, minio.PutObjectOptions{ServerSideEncryption: dstencryption})
	if err != nil {
		logError(testName, function, args, startTime, "", "NewMultipartUpload call failed", err)
		return
	}

	// Content of the destination object will be two copies of
	// `objectName` concatenated, followed by first byte of
	// `objectName`.
	metadata := make(map[string]string)
	header := make(http.Header)
	encrypt.SSECopy(srcencryption).Marshal(header)
	for k, v := range header {
		metadata[k] = v[0]
	}

	metadata["x-amz-copy-source-if-match"] = uploadInfo.ETag

	// First of three parts
	fstPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 1, 0, -1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Second of three parts
	sndPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 2, 0, -1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Last of three parts
	lstPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 3, 0, 1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Complete the multipart upload
	_, err = c.CompleteMultipartUpload(context.Background(), destBucketName, destObjectName, uploadID, []minio.CompletePart{fstPart, sndPart, lstPart}, minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "CompleteMultipartUpload call failed", err)
		return
	}

	// Stat the object and check its length matches
	objInfo, err := c.StatObject(context.Background(), destBucketName, destObjectName, minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject call failed", err)
		return
	}

	if objInfo.Size != (5*1024*1024)*2+1 {
		logError(testName, function, args, startTime, "", "Destination object has incorrect size!", err)
		return
	}

	// Now we read the data back
	getOpts := minio.GetObjectOptions{}
	getOpts.SetRange(0, 5*1024*1024-1)
	r, _, _, err := c.GetObject(context.Background(), destBucketName, destObjectName, getOpts)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject call failed", err)
		return
	}
	getBuf := make([]byte, 5*1024*1024)
	_, err = readFull(r, getBuf)
	if err != nil {
		logError(testName, function, args, startTime, "", "Read buffer failed", err)
		return
	}
	if !bytes.Equal(getBuf, buf) {
		logError(testName, function, args, startTime, "", "Got unexpected data in first 5MB", err)
		return
	}

	getOpts.SetRange(5*1024*1024, 0)
	r, _, _, err = c.GetObject(context.Background(), destBucketName, destObjectName, getOpts)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject call failed", err)
		return
	}
	getBuf = make([]byte, 5*1024*1024+1)
	_, err = readFull(r, getBuf)
	if err != nil {
		logError(testName, function, args, startTime, "", "Read buffer failed", err)
		return
	}
	if !bytes.Equal(getBuf[:5*1024*1024], buf) {
		logError(testName, function, args, startTime, "", "Got unexpected data in second 5MB", err)
		return
	}
	if getBuf[5*1024*1024] != buf[0] {
		logError(testName, function, args, startTime, "", "Got unexpected data in last byte of copied object!", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()

	// Do not need to remove destBucketName its same as bucketName.
}

// Test Core CopyObjectPart implementation for SSEC encrypted to SSE-S3 encrypted copy
func testSSECEncryptedToSSES3CopyObjectPart() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObjectPart(destination, source)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	client, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v4 client object creation failed", err)
		return
	}

	// Instantiate new core client object.
	c := minio.Core{client}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test")

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}
	defer cleanupBucket(bucketName, client)
	// Make a buffer with 5MB of data
	buf := bytes.Repeat([]byte("abcde"), 1024*1024)

	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	password := "correct horse battery staple"
	srcencryption := encrypt.DefaultPBKDF([]byte(password), []byte(bucketName+objectName))
	putmetadata := map[string]string{
		"Content-Type": "binary/octet-stream",
	}
	opts := minio.PutObjectOptions{
		UserMetadata:         putmetadata,
		ServerSideEncryption: srcencryption,
	}

	uploadInfo, err := c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), "", "", opts)
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject call failed", err)
		return
	}

	st, err := c.StatObject(context.Background(), bucketName, objectName, minio.StatObjectOptions{ServerSideEncryption: srcencryption})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject call failed", err)
		return
	}

	if st.Size != int64(len(buf)) {
		logError(testName, function, args, startTime, "", fmt.Sprintf("Error: number of bytes does not match, want %v, got %v\n", len(buf), st.Size), err)
		return
	}

	destBucketName := bucketName
	destObjectName := objectName + "-dest"
	dstencryption := encrypt.NewSSE()

	uploadID, err := c.NewMultipartUpload(context.Background(), destBucketName, destObjectName, minio.PutObjectOptions{ServerSideEncryption: dstencryption})
	if err != nil {
		logError(testName, function, args, startTime, "", "NewMultipartUpload call failed", err)
		return
	}

	// Content of the destination object will be two copies of
	// `objectName` concatenated, followed by first byte of
	// `objectName`.
	metadata := make(map[string]string)
	header := make(http.Header)
	encrypt.SSECopy(srcencryption).Marshal(header)
	dstencryption.Marshal(header)

	for k, v := range header {
		metadata[k] = v[0]
	}

	metadata["x-amz-copy-source-if-match"] = uploadInfo.ETag

	// First of three parts
	fstPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 1, 0, -1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Second of three parts
	sndPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 2, 0, -1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Last of three parts
	lstPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 3, 0, 1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Complete the multipart upload
	_, err = c.CompleteMultipartUpload(context.Background(), destBucketName, destObjectName, uploadID, []minio.CompletePart{fstPart, sndPart, lstPart}, minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "CompleteMultipartUpload call failed", err)
		return
	}

	// Stat the object and check its length matches
	objInfo, err := c.StatObject(context.Background(), destBucketName, destObjectName, minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject call failed", err)
		return
	}

	if objInfo.Size != (5*1024*1024)*2+1 {
		logError(testName, function, args, startTime, "", "Destination object has incorrect size!", err)
		return
	}

	// Now we read the data back
	getOpts := minio.GetObjectOptions{}
	getOpts.SetRange(0, 5*1024*1024-1)
	r, _, _, err := c.GetObject(context.Background(), destBucketName, destObjectName, getOpts)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject call failed", err)
		return
	}
	getBuf := make([]byte, 5*1024*1024)
	_, err = readFull(r, getBuf)
	if err != nil {
		logError(testName, function, args, startTime, "", "Read buffer failed", err)
		return
	}
	if !bytes.Equal(getBuf, buf) {
		logError(testName, function, args, startTime, "", "Got unexpected data in first 5MB", err)
		return
	}

	getOpts.SetRange(5*1024*1024, 0)
	r, _, _, err = c.GetObject(context.Background(), destBucketName, destObjectName, getOpts)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject call failed", err)
		return
	}
	getBuf = make([]byte, 5*1024*1024+1)
	_, err = readFull(r, getBuf)
	if err != nil {
		logError(testName, function, args, startTime, "", "Read buffer failed", err)
		return
	}
	if !bytes.Equal(getBuf[:5*1024*1024], buf) {
		logError(testName, function, args, startTime, "", "Got unexpected data in second 5MB", err)
		return
	}
	if getBuf[5*1024*1024] != buf[0] {
		logError(testName, function, args, startTime, "", "Got unexpected data in last byte of copied object!", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()

	// Do not need to remove destBucketName its same as bucketName.
}

// Test Core CopyObjectPart implementation for unencrypted to SSEC encryption copy part
func testUnencryptedToSSECCopyObjectPart() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObjectPart(destination, source)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	client, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v4 client object creation failed", err)
		return
	}

	// Instantiate new core client object.
	c := minio.Core{client}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test")

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}
	defer cleanupBucket(bucketName, client)
	// Make a buffer with 5MB of data
	buf := bytes.Repeat([]byte("abcde"), 1024*1024)

	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	password := "correct horse battery staple"
	putmetadata := map[string]string{
		"Content-Type": "binary/octet-stream",
	}
	opts := minio.PutObjectOptions{
		UserMetadata: putmetadata,
	}
	uploadInfo, err := c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), "", "", opts)
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject call failed", err)
		return
	}

	st, err := c.StatObject(context.Background(), bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject call failed", err)
		return
	}

	if st.Size != int64(len(buf)) {
		logError(testName, function, args, startTime, "", fmt.Sprintf("Error: number of bytes does not match, want %v, got %v\n", len(buf), st.Size), err)
		return
	}

	destBucketName := bucketName
	destObjectName := objectName + "-dest"
	dstencryption := encrypt.DefaultPBKDF([]byte(password), []byte(destBucketName+destObjectName))

	uploadID, err := c.NewMultipartUpload(context.Background(), destBucketName, destObjectName, minio.PutObjectOptions{ServerSideEncryption: dstencryption})
	if err != nil {
		logError(testName, function, args, startTime, "", "NewMultipartUpload call failed", err)
		return
	}

	// Content of the destination object will be two copies of
	// `objectName` concatenated, followed by first byte of
	// `objectName`.
	metadata := make(map[string]string)
	header := make(http.Header)
	dstencryption.Marshal(header)
	for k, v := range header {
		metadata[k] = v[0]
	}

	metadata["x-amz-copy-source-if-match"] = uploadInfo.ETag

	// First of three parts
	fstPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 1, 0, -1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Second of three parts
	sndPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 2, 0, -1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Last of three parts
	lstPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 3, 0, 1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Complete the multipart upload
	_, err = c.CompleteMultipartUpload(context.Background(), destBucketName, destObjectName, uploadID, []minio.CompletePart{fstPart, sndPart, lstPart}, minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "CompleteMultipartUpload call failed", err)
		return
	}

	// Stat the object and check its length matches
	objInfo, err := c.StatObject(context.Background(), destBucketName, destObjectName, minio.StatObjectOptions{ServerSideEncryption: dstencryption})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject call failed", err)
		return
	}

	if objInfo.Size != (5*1024*1024)*2+1 {
		logError(testName, function, args, startTime, "", "Destination object has incorrect size!", err)
		return
	}

	// Now we read the data back
	getOpts := minio.GetObjectOptions{ServerSideEncryption: dstencryption}
	getOpts.SetRange(0, 5*1024*1024-1)
	r, _, _, err := c.GetObject(context.Background(), destBucketName, destObjectName, getOpts)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject call failed", err)
		return
	}
	getBuf := make([]byte, 5*1024*1024)
	_, err = readFull(r, getBuf)
	if err != nil {
		logError(testName, function, args, startTime, "", "Read buffer failed", err)
		return
	}
	if !bytes.Equal(getBuf, buf) {
		logError(testName, function, args, startTime, "", "Got unexpected data in first 5MB", err)
		return
	}

	getOpts.SetRange(5*1024*1024, 0)
	r, _, _, err = c.GetObject(context.Background(), destBucketName, destObjectName, getOpts)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject call failed", err)
		return
	}
	getBuf = make([]byte, 5*1024*1024+1)
	_, err = readFull(r, getBuf)
	if err != nil {
		logError(testName, function, args, startTime, "", "Read buffer failed", err)
		return
	}
	if !bytes.Equal(getBuf[:5*1024*1024], buf) {
		logError(testName, function, args, startTime, "", "Got unexpected data in second 5MB", err)
		return
	}
	if getBuf[5*1024*1024] != buf[0] {
		logError(testName, function, args, startTime, "", "Got unexpected data in last byte of copied object!", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()

	// Do not need to remove destBucketName its same as bucketName.
}

// Test Core CopyObjectPart implementation for unencrypted to unencrypted copy
func testUnencryptedToUnencryptedCopyPart() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObjectPart(destination, source)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	client, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v4 client object creation failed", err)
		return
	}

	// Instantiate new core client object.
	c := minio.Core{client}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test")

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}
	defer cleanupBucket(bucketName, client)
	// Make a buffer with 5MB of data
	buf := bytes.Repeat([]byte("abcde"), 1024*1024)

	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	putmetadata := map[string]string{
		"Content-Type": "binary/octet-stream",
	}
	opts := minio.PutObjectOptions{
		UserMetadata: putmetadata,
	}
	uploadInfo, err := c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), "", "", opts)
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject call failed", err)
		return
	}
	st, err := c.StatObject(context.Background(), bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject call failed", err)
		return
	}

	if st.Size != int64(len(buf)) {
		logError(testName, function, args, startTime, "", fmt.Sprintf("Error: number of bytes does not match, want %v, got %v\n", len(buf), st.Size), err)
		return
	}

	destBucketName := bucketName
	destObjectName := objectName + "-dest"

	uploadID, err := c.NewMultipartUpload(context.Background(), destBucketName, destObjectName, minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "NewMultipartUpload call failed", err)
		return
	}

	// Content of the destination object will be two copies of
	// `objectName` concatenated, followed by first byte of
	// `objectName`.
	metadata := make(map[string]string)
	header := make(http.Header)
	for k, v := range header {
		metadata[k] = v[0]
	}

	metadata["x-amz-copy-source-if-match"] = uploadInfo.ETag

	// First of three parts
	fstPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 1, 0, -1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Second of three parts
	sndPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 2, 0, -1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Last of three parts
	lstPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 3, 0, 1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Complete the multipart upload
	_, err = c.CompleteMultipartUpload(context.Background(), destBucketName, destObjectName, uploadID, []minio.CompletePart{fstPart, sndPart, lstPart}, minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "CompleteMultipartUpload call failed", err)
		return
	}

	// Stat the object and check its length matches
	objInfo, err := c.StatObject(context.Background(), destBucketName, destObjectName, minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject call failed", err)
		return
	}

	if objInfo.Size != (5*1024*1024)*2+1 {
		logError(testName, function, args, startTime, "", "Destination object has incorrect size!", err)
		return
	}

	// Now we read the data back
	getOpts := minio.GetObjectOptions{}
	getOpts.SetRange(0, 5*1024*1024-1)
	r, _, _, err := c.GetObject(context.Background(), destBucketName, destObjectName, getOpts)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject call failed", err)
		return
	}
	getBuf := make([]byte, 5*1024*1024)
	_, err = readFull(r, getBuf)
	if err != nil {
		logError(testName, function, args, startTime, "", "Read buffer failed", err)
		return
	}
	if !bytes.Equal(getBuf, buf) {
		logError(testName, function, args, startTime, "", "Got unexpected data in first 5MB", err)
		return
	}

	getOpts.SetRange(5*1024*1024, 0)
	r, _, _, err = c.GetObject(context.Background(), destBucketName, destObjectName, getOpts)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject call failed", err)
		return
	}
	getBuf = make([]byte, 5*1024*1024+1)
	_, err = readFull(r, getBuf)
	if err != nil {
		logError(testName, function, args, startTime, "", "Read buffer failed", err)
		return
	}
	if !bytes.Equal(getBuf[:5*1024*1024], buf) {
		logError(testName, function, args, startTime, "", "Got unexpected data in second 5MB", err)
		return
	}
	if getBuf[5*1024*1024] != buf[0] {
		logError(testName, function, args, startTime, "", "Got unexpected data in last byte of copied object!", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()

	// Do not need to remove destBucketName its same as bucketName.
}

// Test Core CopyObjectPart implementation for unencrypted to SSE-S3 encrypted copy
func testUnencryptedToSSES3CopyObjectPart() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObjectPart(destination, source)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	client, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v4 client object creation failed", err)
		return
	}

	// Instantiate new core client object.
	c := minio.Core{client}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test")

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}
	defer cleanupBucket(bucketName, client)
	// Make a buffer with 5MB of data
	buf := bytes.Repeat([]byte("abcde"), 1024*1024)

	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	opts := minio.PutObjectOptions{
		UserMetadata: map[string]string{
			"Content-Type": "binary/octet-stream",
		},
	}
	uploadInfo, err := c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), "", "", opts)
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject call failed", err)
		return
	}
	st, err := c.StatObject(context.Background(), bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject call failed", err)
		return
	}

	if st.Size != int64(len(buf)) {
		logError(testName, function, args, startTime, "", fmt.Sprintf("Error: number of bytes does not match, want %v, got %v\n", len(buf), st.Size), err)
		return
	}

	destBucketName := bucketName
	destObjectName := objectName + "-dest"
	dstencryption := encrypt.NewSSE()

	uploadID, err := c.NewMultipartUpload(context.Background(), destBucketName, destObjectName, minio.PutObjectOptions{ServerSideEncryption: dstencryption})
	if err != nil {
		logError(testName, function, args, startTime, "", "NewMultipartUpload call failed", err)
		return
	}

	// Content of the destination object will be two copies of
	// `objectName` concatenated, followed by first byte of
	// `objectName`.
	metadata := make(map[string]string)
	header := make(http.Header)
	dstencryption.Marshal(header)

	for k, v := range header {
		metadata[k] = v[0]
	}

	metadata["x-amz-copy-source-if-match"] = uploadInfo.ETag

	// First of three parts
	fstPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 1, 0, -1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Second of three parts
	sndPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 2, 0, -1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Last of three parts
	lstPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 3, 0, 1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Complete the multipart upload
	_, err = c.CompleteMultipartUpload(context.Background(), destBucketName, destObjectName, uploadID, []minio.CompletePart{fstPart, sndPart, lstPart}, minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "CompleteMultipartUpload call failed", err)
		return
	}

	// Stat the object and check its length matches
	objInfo, err := c.StatObject(context.Background(), destBucketName, destObjectName, minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject call failed", err)
		return
	}

	if objInfo.Size != (5*1024*1024)*2+1 {
		logError(testName, function, args, startTime, "", "Destination object has incorrect size!", err)
		return
	}

	// Now we read the data back
	getOpts := minio.GetObjectOptions{}
	getOpts.SetRange(0, 5*1024*1024-1)
	r, _, _, err := c.GetObject(context.Background(), destBucketName, destObjectName, getOpts)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject call failed", err)
		return
	}
	getBuf := make([]byte, 5*1024*1024)
	_, err = readFull(r, getBuf)
	if err != nil {
		logError(testName, function, args, startTime, "", "Read buffer failed", err)
		return
	}
	if !bytes.Equal(getBuf, buf) {
		logError(testName, function, args, startTime, "", "Got unexpected data in first 5MB", err)
		return
	}

	getOpts.SetRange(5*1024*1024, 0)
	r, _, _, err = c.GetObject(context.Background(), destBucketName, destObjectName, getOpts)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject call failed", err)
		return
	}
	getBuf = make([]byte, 5*1024*1024+1)
	_, err = readFull(r, getBuf)
	if err != nil {
		logError(testName, function, args, startTime, "", "Read buffer failed", err)
		return
	}
	if !bytes.Equal(getBuf[:5*1024*1024], buf) {
		logError(testName, function, args, startTime, "", "Got unexpected data in second 5MB", err)
		return
	}
	if getBuf[5*1024*1024] != buf[0] {
		logError(testName, function, args, startTime, "", "Got unexpected data in last byte of copied object!", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()

	// Do not need to remove destBucketName its same as bucketName.
}

// Test Core CopyObjectPart implementation for SSE-S3 to SSEC encryption copy part
func testSSES3EncryptedToSSECCopyObjectPart() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObjectPart(destination, source)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	client, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v4 client object creation failed", err)
		return
	}

	// Instantiate new core client object.
	c := minio.Core{client}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test")

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}
	defer cleanupBucket(bucketName, client)
	// Make a buffer with 5MB of data
	buf := bytes.Repeat([]byte("abcde"), 1024*1024)

	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	password := "correct horse battery staple"
	srcEncryption := encrypt.NewSSE()
	opts := minio.PutObjectOptions{
		UserMetadata: map[string]string{
			"Content-Type": "binary/octet-stream",
		},
		ServerSideEncryption: srcEncryption,
	}
	uploadInfo, err := c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), "", "", opts)
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject call failed", err)
		return
	}

	st, err := c.StatObject(context.Background(), bucketName, objectName, minio.StatObjectOptions{ServerSideEncryption: srcEncryption})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject call failed", err)
		return
	}

	if st.Size != int64(len(buf)) {
		logError(testName, function, args, startTime, "", fmt.Sprintf("Error: number of bytes does not match, want %v, got %v\n", len(buf), st.Size), err)
		return
	}

	destBucketName := bucketName
	destObjectName := objectName + "-dest"
	dstencryption := encrypt.DefaultPBKDF([]byte(password), []byte(destBucketName+destObjectName))

	uploadID, err := c.NewMultipartUpload(context.Background(), destBucketName, destObjectName, minio.PutObjectOptions{ServerSideEncryption: dstencryption})
	if err != nil {
		logError(testName, function, args, startTime, "", "NewMultipartUpload call failed", err)
		return
	}

	// Content of the destination object will be two copies of
	// `objectName` concatenated, followed by first byte of
	// `objectName`.
	metadata := make(map[string]string)
	header := make(http.Header)
	dstencryption.Marshal(header)
	for k, v := range header {
		metadata[k] = v[0]
	}

	metadata["x-amz-copy-source-if-match"] = uploadInfo.ETag

	// First of three parts
	fstPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 1, 0, -1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Second of three parts
	sndPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 2, 0, -1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Last of three parts
	lstPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 3, 0, 1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Complete the multipart upload
	_, err = c.CompleteMultipartUpload(context.Background(), destBucketName, destObjectName, uploadID, []minio.CompletePart{fstPart, sndPart, lstPart}, minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "CompleteMultipartUpload call failed", err)
		return
	}

	// Stat the object and check its length matches
	objInfo, err := c.StatObject(context.Background(), destBucketName, destObjectName, minio.StatObjectOptions{ServerSideEncryption: dstencryption})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject call failed", err)
		return
	}

	if objInfo.Size != (5*1024*1024)*2+1 {
		logError(testName, function, args, startTime, "", "Destination object has incorrect size!", err)
		return
	}

	// Now we read the data back
	getOpts := minio.GetObjectOptions{ServerSideEncryption: dstencryption}
	getOpts.SetRange(0, 5*1024*1024-1)
	r, _, _, err := c.GetObject(context.Background(), destBucketName, destObjectName, getOpts)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject call failed", err)
		return
	}
	getBuf := make([]byte, 5*1024*1024)
	_, err = readFull(r, getBuf)
	if err != nil {
		logError(testName, function, args, startTime, "", "Read buffer failed", err)
		return
	}
	if !bytes.Equal(getBuf, buf) {
		logError(testName, function, args, startTime, "", "Got unexpected data in first 5MB", err)
		return
	}

	getOpts.SetRange(5*1024*1024, 0)
	r, _, _, err = c.GetObject(context.Background(), destBucketName, destObjectName, getOpts)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject call failed", err)
		return
	}
	getBuf = make([]byte, 5*1024*1024+1)
	_, err = readFull(r, getBuf)
	if err != nil {
		logError(testName, function, args, startTime, "", "Read buffer failed", err)
		return
	}
	if !bytes.Equal(getBuf[:5*1024*1024], buf) {
		logError(testName, function, args, startTime, "", "Got unexpected data in second 5MB", err)
		return
	}
	if getBuf[5*1024*1024] != buf[0] {
		logError(testName, function, args, startTime, "", "Got unexpected data in last byte of copied object!", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()

	// Do not need to remove destBucketName its same as bucketName.
}

// Test Core CopyObjectPart implementation for unencrypted to unencrypted copy
func testSSES3EncryptedToUnencryptedCopyPart() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObjectPart(destination, source)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	client, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v4 client object creation failed", err)
		return
	}

	// Instantiate new core client object.
	c := minio.Core{client}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test")

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}
	defer cleanupBucket(bucketName, client)
	// Make a buffer with 5MB of data
	buf := bytes.Repeat([]byte("abcde"), 1024*1024)

	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	srcEncryption := encrypt.NewSSE()
	opts := minio.PutObjectOptions{
		UserMetadata: map[string]string{
			"Content-Type": "binary/octet-stream",
		},
		ServerSideEncryption: srcEncryption,
	}
	uploadInfo, err := c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), "", "", opts)
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject call failed", err)
		return
	}
	st, err := c.StatObject(context.Background(), bucketName, objectName, minio.StatObjectOptions{ServerSideEncryption: srcEncryption})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject call failed", err)
		return
	}

	if st.Size != int64(len(buf)) {
		logError(testName, function, args, startTime, "", fmt.Sprintf("Error: number of bytes does not match, want %v, got %v\n", len(buf), st.Size), err)
		return
	}

	destBucketName := bucketName
	destObjectName := objectName + "-dest"

	uploadID, err := c.NewMultipartUpload(context.Background(), destBucketName, destObjectName, minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "NewMultipartUpload call failed", err)
		return
	}

	// Content of the destination object will be two copies of
	// `objectName` concatenated, followed by first byte of
	// `objectName`.
	metadata := make(map[string]string)
	header := make(http.Header)
	for k, v := range header {
		metadata[k] = v[0]
	}

	metadata["x-amz-copy-source-if-match"] = uploadInfo.ETag

	// First of three parts
	fstPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 1, 0, -1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Second of three parts
	sndPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 2, 0, -1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Last of three parts
	lstPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 3, 0, 1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Complete the multipart upload
	_, err = c.CompleteMultipartUpload(context.Background(), destBucketName, destObjectName, uploadID, []minio.CompletePart{fstPart, sndPart, lstPart}, minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "CompleteMultipartUpload call failed", err)
		return
	}

	// Stat the object and check its length matches
	objInfo, err := c.StatObject(context.Background(), destBucketName, destObjectName, minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject call failed", err)
		return
	}

	if objInfo.Size != (5*1024*1024)*2+1 {
		logError(testName, function, args, startTime, "", "Destination object has incorrect size!", err)
		return
	}

	// Now we read the data back
	getOpts := minio.GetObjectOptions{}
	getOpts.SetRange(0, 5*1024*1024-1)
	r, _, _, err := c.GetObject(context.Background(), destBucketName, destObjectName, getOpts)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject call failed", err)
		return
	}
	getBuf := make([]byte, 5*1024*1024)
	_, err = readFull(r, getBuf)
	if err != nil {
		logError(testName, function, args, startTime, "", "Read buffer failed", err)
		return
	}
	if !bytes.Equal(getBuf, buf) {
		logError(testName, function, args, startTime, "", "Got unexpected data in first 5MB", err)
		return
	}

	getOpts.SetRange(5*1024*1024, 0)
	r, _, _, err = c.GetObject(context.Background(), destBucketName, destObjectName, getOpts)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject call failed", err)
		return
	}
	getBuf = make([]byte, 5*1024*1024+1)
	_, err = readFull(r, getBuf)
	if err != nil {
		logError(testName, function, args, startTime, "", "Read buffer failed", err)
		return
	}
	if !bytes.Equal(getBuf[:5*1024*1024], buf) {
		logError(testName, function, args, startTime, "", "Got unexpected data in second 5MB", err)
		return
	}
	if getBuf[5*1024*1024] != buf[0] {
		logError(testName, function, args, startTime, "", "Got unexpected data in last byte of copied object!", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()

	// Do not need to remove destBucketName its same as bucketName.
}

// Test Core CopyObjectPart implementation for unencrypted to SSE-S3 encrypted copy
func testSSES3EncryptedToSSES3CopyObjectPart() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObjectPart(destination, source)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	client, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v4 client object creation failed", err)
		return
	}

	// Instantiate new core client object.
	c := minio.Core{client}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test")

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}
	defer cleanupBucket(bucketName, client)
	// Make a buffer with 5MB of data
	buf := bytes.Repeat([]byte("abcde"), 1024*1024)

	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	srcEncryption := encrypt.NewSSE()
	opts := minio.PutObjectOptions{
		UserMetadata: map[string]string{
			"Content-Type": "binary/octet-stream",
		},
		ServerSideEncryption: srcEncryption,
	}

	uploadInfo, err := c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), "", "", opts)
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject call failed", err)
		return
	}
	st, err := c.StatObject(context.Background(), bucketName, objectName, minio.StatObjectOptions{ServerSideEncryption: srcEncryption})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject call failed", err)
		return
	}
	if st.Size != int64(len(buf)) {
		logError(testName, function, args, startTime, "", fmt.Sprintf("Error: number of bytes does not match, want %v, got %v\n", len(buf), st.Size), err)
		return
	}

	destBucketName := bucketName
	destObjectName := objectName + "-dest"
	dstencryption := encrypt.NewSSE()

	uploadID, err := c.NewMultipartUpload(context.Background(), destBucketName, destObjectName, minio.PutObjectOptions{ServerSideEncryption: dstencryption})
	if err != nil {
		logError(testName, function, args, startTime, "", "NewMultipartUpload call failed", err)
		return
	}

	// Content of the destination object will be two copies of
	// `objectName` concatenated, followed by first byte of
	// `objectName`.
	metadata := make(map[string]string)
	header := make(http.Header)
	dstencryption.Marshal(header)

	for k, v := range header {
		metadata[k] = v[0]
	}

	metadata["x-amz-copy-source-if-match"] = uploadInfo.ETag

	// First of three parts
	fstPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 1, 0, -1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Second of three parts
	sndPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 2, 0, -1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Last of three parts
	lstPart, err := c.CopyObjectPart(context.Background(), bucketName, objectName, destBucketName, destObjectName, uploadID, 3, 0, 1, metadata)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObjectPart call failed", err)
		return
	}

	// Complete the multipart upload
	_, err = c.CompleteMultipartUpload(context.Background(), destBucketName, destObjectName, uploadID, []minio.CompletePart{fstPart, sndPart, lstPart}, minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "CompleteMultipartUpload call failed", err)
		return
	}

	// Stat the object and check its length matches
	objInfo, err := c.StatObject(context.Background(), destBucketName, destObjectName, minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject call failed", err)
		return
	}

	if objInfo.Size != (5*1024*1024)*2+1 {
		logError(testName, function, args, startTime, "", "Destination object has incorrect size!", err)
		return
	}

	// Now we read the data back
	getOpts := minio.GetObjectOptions{}
	getOpts.SetRange(0, 5*1024*1024-1)
	r, _, _, err := c.GetObject(context.Background(), destBucketName, destObjectName, getOpts)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject call failed", err)
		return
	}
	getBuf := make([]byte, 5*1024*1024)
	_, err = readFull(r, getBuf)
	if err != nil {
		logError(testName, function, args, startTime, "", "Read buffer failed", err)
		return
	}
	if !bytes.Equal(getBuf, buf) {
		logError(testName, function, args, startTime, "", "Got unexpected data in first 5MB", err)
		return
	}

	getOpts.SetRange(5*1024*1024, 0)
	r, _, _, err = c.GetObject(context.Background(), destBucketName, destObjectName, getOpts)
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject call failed", err)
		return
	}
	getBuf = make([]byte, 5*1024*1024+1)
	_, err = readFull(r, getBuf)
	if err != nil {
		logError(testName, function, args, startTime, "", "Read buffer failed", err)
		return
	}
	if !bytes.Equal(getBuf[:5*1024*1024], buf) {
		logError(testName, function, args, startTime, "", "Got unexpected data in second 5MB", err)
		return
	}
	if getBuf[5*1024*1024] != buf[0] {
		logError(testName, function, args, startTime, "", "Got unexpected data in last byte of copied object!", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()

	// Do not need to remove destBucketName its same as bucketName.
}
func testUserMetadataCopying() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObject(destination, source)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	// c.TraceOn(os.Stderr)
	testUserMetadataCopyingWrapper(c)
}

func testUserMetadataCopyingWrapper(c *minio.Client) {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObject(destination, source)"
	args := map[string]interface{}{}

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	// Make a new bucket in 'us-east-1' (source bucket).
	err := c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	fetchMeta := func(object string) (h http.Header) {
		objInfo, err := c.StatObject(context.Background(), bucketName, object, minio.StatObjectOptions{})
		if err != nil {
			logError(testName, function, args, startTime, "", "Stat failed", err)
			return
		}
		h = make(http.Header)
		for k, vs := range objInfo.Metadata {
			if strings.HasPrefix(strings.ToLower(k), "x-amz-meta-") {
				h.Add(k, vs[0])
			}
		}
		return h
	}

	// 1. create a client encrypted object to copy by uploading
	const srcSize = 1024 * 1024
	buf := bytes.Repeat([]byte("abcde"), srcSize) // gives a buffer of 5MiB
	metadata := make(http.Header)
	metadata.Set("x-amz-meta-myheader", "myvalue")
	m := make(map[string]string)
	m["x-amz-meta-myheader"] = "myvalue"
	_, err = c.PutObject(context.Background(), bucketName, "srcObject",
		bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{UserMetadata: m})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObjectWithMetadata failed", err)
		return
	}
	if !reflect.DeepEqual(metadata, fetchMeta("srcObject")) {
		logError(testName, function, args, startTime, "", "Metadata match failed", err)
		return
	}

	// 2. create source
	src := minio.CopySrcOptions{
		Bucket: bucketName,
		Object: "srcObject",
	}

	// 2.1 create destination with metadata set
	dst1 := minio.CopyDestOptions{
		Bucket:          bucketName,
		Object:          "dstObject-1",
		UserMetadata:    map[string]string{"notmyheader": "notmyvalue"},
		ReplaceMetadata: true,
	}

	// 3. Check that copying to an object with metadata set resets
	// the headers on the copy.
	args["source"] = src
	args["destination"] = dst1
	_, err = c.CopyObject(context.Background(), dst1, src)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObject failed", err)
		return
	}

	expectedHeaders := make(http.Header)
	expectedHeaders.Set("x-amz-meta-notmyheader", "notmyvalue")
	if !reflect.DeepEqual(expectedHeaders, fetchMeta("dstObject-1")) {
		logError(testName, function, args, startTime, "", "Metadata match failed", err)
		return
	}

	// 4. create destination with no metadata set and same source
	dst2 := minio.CopyDestOptions{
		Bucket: bucketName,
		Object: "dstObject-2",
	}

	// 5. Check that copying to an object with no metadata set,
	// copies metadata.
	args["source"] = src
	args["destination"] = dst2
	_, err = c.CopyObject(context.Background(), dst2, src)
	if err != nil {
		logError(testName, function, args, startTime, "", "CopyObject failed", err)
		return
	}

	expectedHeaders = metadata
	if !reflect.DeepEqual(expectedHeaders, fetchMeta("dstObject-2")) {
		logError(testName, function, args, startTime, "", "Metadata match failed", err)
		return
	}

	// 6. Compose a pair of sources.
	dst3 := minio.CopyDestOptions{
		Bucket:          bucketName,
		Object:          "dstObject-3",
		ReplaceMetadata: true,
	}

	function = "ComposeObject(destination, sources)"
	args["source"] = []minio.CopySrcOptions{src, src}
	args["destination"] = dst3
	_, err = c.ComposeObject(context.Background(), dst3, src, src)
	if err != nil {
		logError(testName, function, args, startTime, "", "ComposeObject failed", err)
		return
	}

	// Check that no headers are copied in this case
	if !reflect.DeepEqual(make(http.Header), fetchMeta("dstObject-3")) {
		logError(testName, function, args, startTime, "", "Metadata match failed", err)
		return
	}

	// 7. Compose a pair of sources with dest user metadata set.
	dst4 := minio.CopyDestOptions{
		Bucket:          bucketName,
		Object:          "dstObject-4",
		UserMetadata:    map[string]string{"notmyheader": "notmyvalue"},
		ReplaceMetadata: true,
	}

	function = "ComposeObject(destination, sources)"
	args["source"] = []minio.CopySrcOptions{src, src}
	args["destination"] = dst4
	_, err = c.ComposeObject(context.Background(), dst4, src, src)
	if err != nil {
		logError(testName, function, args, startTime, "", "ComposeObject failed", err)
		return
	}

	// Check that no headers are copied in this case
	expectedHeaders = make(http.Header)
	expectedHeaders.Set("x-amz-meta-notmyheader", "notmyvalue")
	if !reflect.DeepEqual(expectedHeaders, fetchMeta("dstObject-4")) {
		logError(testName, function, args, startTime, "", "Metadata match failed", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

func testUserMetadataCopyingV2() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "CopyObject(destination, source)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV2(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client v2 object creation failed", err)
		return
	}

	// c.TraceOn(os.Stderr)
	testUserMetadataCopyingWrapper(c)
}

func testStorageClassMetadataPutObject() {
	// initialize logging params
	startTime := time.Now()
	function := "testStorageClassMetadataPutObject()"
	args := map[string]interface{}{}
	testName := getFuncName()

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v4 client object creation failed", err)
		return
	}

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test")
	// Make a new bucket in 'us-east-1' (source bucket).
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	fetchMeta := func(object string) (h http.Header) {
		objInfo, err := c.StatObject(context.Background(), bucketName, object, minio.StatObjectOptions{})
		if err != nil {
			logError(testName, function, args, startTime, "", "Stat failed", err)
			return
		}
		h = make(http.Header)
		for k, vs := range objInfo.Metadata {
			if strings.HasPrefix(strings.ToLower(k), "x-amz-storage-class") {
				for _, v := range vs {
					h.Add(k, v)
				}
			}
		}
		return h
	}

	metadata := make(http.Header)
	metadata.Set("x-amz-storage-class", "REDUCED_REDUNDANCY")

	emptyMetadata := make(http.Header)

	const srcSize = 1024 * 1024
	buf := bytes.Repeat([]byte("abcde"), srcSize) // gives a buffer of 1MiB

	_, err = c.PutObject(context.Background(), bucketName, "srcObjectRRSClass",
		bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{StorageClass: "REDUCED_REDUNDANCY"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	// Get the returned metadata
	returnedMeta := fetchMeta("srcObjectRRSClass")

	// The response metada should either be equal to metadata (with REDUCED_REDUNDANCY) or emptyMetadata (in case of gateways)
	if !reflect.DeepEqual(metadata, returnedMeta) && !reflect.DeepEqual(emptyMetadata, returnedMeta) {
		logError(testName, function, args, startTime, "", "Metadata match failed", err)
		return
	}

	metadata = make(http.Header)
	metadata.Set("x-amz-storage-class", "STANDARD")

	_, err = c.PutObject(context.Background(), bucketName, "srcObjectSSClass",
		bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{StorageClass: "STANDARD"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}
	if reflect.DeepEqual(metadata, fetchMeta("srcObjectSSClass")) {
		logError(testName, function, args, startTime, "", "Metadata verification failed, STANDARD storage class should not be a part of response metadata", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

func testStorageClassInvalidMetadataPutObject() {
	// initialize logging params
	startTime := time.Now()
	function := "testStorageClassInvalidMetadataPutObject()"
	args := map[string]interface{}{}
	testName := getFuncName()

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v4 client object creation failed", err)
		return
	}

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test")
	// Make a new bucket in 'us-east-1' (source bucket).
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	const srcSize = 1024 * 1024
	buf := bytes.Repeat([]byte("abcde"), srcSize) // gives a buffer of 1MiB

	_, err = c.PutObject(context.Background(), bucketName, "srcObjectRRSClass",
		bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{StorageClass: "INVALID_STORAGE_CLASS"})
	if err == nil {
		logError(testName, function, args, startTime, "", "PutObject with invalid storage class passed, was expected to fail", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

func testStorageClassMetadataCopyObject() {
	// initialize logging params
	startTime := time.Now()
	function := "testStorageClassMetadataCopyObject()"
	args := map[string]interface{}{}
	testName := getFuncName()

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO v4 client object creation failed", err)
		return
	}

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test")
	// Make a new bucket in 'us-east-1' (source bucket).
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	fetchMeta := func(object string) (h http.Header) {
		objInfo, err := c.StatObject(context.Background(), bucketName, object, minio.StatObjectOptions{})
		args["bucket"] = bucketName
		args["object"] = object
		if err != nil {
			logError(testName, function, args, startTime, "", "Stat failed", err)
			return
		}
		h = make(http.Header)
		for k, vs := range objInfo.Metadata {
			if strings.HasPrefix(strings.ToLower(k), "x-amz-storage-class") {
				for _, v := range vs {
					h.Add(k, v)
				}
			}
		}
		return h
	}

	metadata := make(http.Header)
	metadata.Set("x-amz-storage-class", "REDUCED_REDUNDANCY")

	emptyMetadata := make(http.Header)

	const srcSize = 1024 * 1024
	buf := bytes.Repeat([]byte("abcde"), srcSize)

	// Put an object with RRS Storage class
	_, err = c.PutObject(context.Background(), bucketName, "srcObjectRRSClass",
		bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{StorageClass: "REDUCED_REDUNDANCY"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	// Make server side copy of object uploaded in previous step
	src := minio.CopySrcOptions{
		Bucket: bucketName,
		Object: "srcObjectRRSClass",
	}
	dst := minio.CopyDestOptions{
		Bucket: bucketName,
		Object: "srcObjectRRSClassCopy",
	}
	if _, err = c.CopyObject(context.Background(), dst, src); err != nil {
		logError(testName, function, args, startTime, "", "CopyObject failed on RRS", err)
		return
	}

	// Get the returned metadata
	returnedMeta := fetchMeta("srcObjectRRSClassCopy")

	// The response metada should either be equal to metadata (with REDUCED_REDUNDANCY) or emptyMetadata (in case of gateways)
	if !reflect.DeepEqual(metadata, returnedMeta) && !reflect.DeepEqual(emptyMetadata, returnedMeta) {
		logError(testName, function, args, startTime, "", "Metadata match failed", err)
		return
	}

	metadata = make(http.Header)
	metadata.Set("x-amz-storage-class", "STANDARD")

	// Put an object with Standard Storage class
	_, err = c.PutObject(context.Background(), bucketName, "srcObjectSSClass",
		bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{StorageClass: "STANDARD"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	// Make server side copy of object uploaded in previous step
	src = minio.CopySrcOptions{
		Bucket: bucketName,
		Object: "srcObjectSSClass",
	}
	dst = minio.CopyDestOptions{
		Bucket: bucketName,
		Object: "srcObjectSSClassCopy",
	}
	if _, err = c.CopyObject(context.Background(), dst, src); err != nil {
		logError(testName, function, args, startTime, "", "CopyObject failed on SS", err)
		return
	}
	// Fetch the meta data of copied object
	if reflect.DeepEqual(metadata, fetchMeta("srcObjectSSClassCopy")) {
		logError(testName, function, args, startTime, "", "Metadata verification failed, STANDARD storage class should not be a part of response metadata", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Test put object with size -1 byte object.
func testPutObjectNoLengthV2() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "PutObject(bucketName, objectName, reader, size, opts)"
	args := map[string]interface{}{
		"bucketName": "",
		"objectName": "",
		"size":       -1,
		"opts":       "",
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV2(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client v2 object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	objectName := bucketName + "unique"
	args["objectName"] = objectName

	bufSize := dataFileMap["datafile-129-MB"]
	var reader = getDataReader("datafile-129-MB")
	defer reader.Close()
	args["size"] = bufSize

	// Upload an object.
	_, err = c.PutObject(context.Background(), bucketName, objectName, reader, -1, minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObjectWithSize failed", err)
		return
	}

	st, err := c.StatObject(context.Background(), bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject failed", err)
		return
	}

	if st.Size != int64(bufSize) {
		logError(testName, function, args, startTime, "", "Expected upload object size "+string(bufSize)+" got "+string(st.Size), err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Test put objects of unknown size.
func testPutObjectsUnknownV2() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "PutObject(bucketName, objectName, reader,size,opts)"
	args := map[string]interface{}{
		"bucketName": "",
		"objectName": "",
		"size":       "",
		"opts":       "",
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV2(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client v2 object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Issues are revealed by trying to upload multiple files of unknown size
	// sequentially (on 4GB machines)
	for i := 1; i <= 4; i++ {
		// Simulate that we could be receiving byte slices of data that we want
		// to upload as a file
		rpipe, wpipe := io.Pipe()
		defer rpipe.Close()
		go func() {
			b := []byte("test")
			wpipe.Write(b)
			wpipe.Close()
		}()

		// Upload the object.
		objectName := fmt.Sprintf("%sunique%d", bucketName, i)
		args["objectName"] = objectName

		ui, err := c.PutObject(context.Background(), bucketName, objectName, rpipe, -1, minio.PutObjectOptions{})
		if err != nil {
			logError(testName, function, args, startTime, "", "PutObjectStreaming failed", err)
			return
		}

		if ui.Size != 4 {
			logError(testName, function, args, startTime, "", "Expected upload object size "+string(4)+" got "+string(ui.Size), nil)
			return
		}

		st, err := c.StatObject(context.Background(), bucketName, objectName, minio.StatObjectOptions{})
		if err != nil {
			logError(testName, function, args, startTime, "", "StatObjectStreaming failed", err)
			return
		}

		if st.Size != int64(4) {
			logError(testName, function, args, startTime, "", "Expected upload object size "+string(4)+" got "+string(st.Size), err)
			return
		}

	}

	successLogger(testName, function, args, startTime).Info()
}

// Test put object with 0 byte object.
func testPutObject0ByteV2() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "PutObject(bucketName, objectName, reader, size, opts)"
	args := map[string]interface{}{
		"bucketName": "",
		"objectName": "",
		"size":       0,
		"opts":       "",
	}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV2(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client v2 object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	objectName := bucketName + "unique"
	args["objectName"] = objectName
	args["opts"] = minio.PutObjectOptions{}

	// Upload an object.
	_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader([]byte("")), 0, minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObjectWithSize failed", err)
		return
	}
	st, err := c.StatObject(context.Background(), bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObjectWithSize failed", err)
		return
	}
	if st.Size != 0 {
		logError(testName, function, args, startTime, "", "Expected upload object size 0 but got "+string(st.Size), err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Test expected error cases
func testComposeObjectErrorCases() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "ComposeObject(destination, sourceList)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	testComposeObjectErrorCasesWrapper(c)
}

// Test concatenating multiple 10K objects V4
func testCompose10KSources() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "ComposeObject(destination, sourceList)"
	args := map[string]interface{}{}

	// Instantiate new minio client object
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client object creation failed", err)
		return
	}

	testComposeMultipleSources(c)
}

// Tests comprehensive list of all methods.
func testFunctionalV2() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "testFunctionalV2()"
	functionAll := ""
	args := map[string]interface{}{}

	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV2(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client v2 object creation failed", err)
		return
	}

	// Enable to debug
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	location := "us-east-1"
	// Make a new bucket.
	function = "MakeBucket(bucketName, location)"
	functionAll = "MakeBucket(bucketName, location)"
	args = map[string]interface{}{
		"bucketName": bucketName,
		"location":   location,
	}
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: location})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	// Generate a random file name.
	fileName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	file, err := os.Create(fileName)
	if err != nil {
		logError(testName, function, args, startTime, "", "file create failed", err)
		return
	}
	for i := 0; i < 3; i++ {
		buf := make([]byte, rand.Intn(1<<19))
		_, err = file.Write(buf)
		if err != nil {
			logError(testName, function, args, startTime, "", "file write failed", err)
			return
		}
	}
	file.Close()

	// Verify if bucket exits and you have access.
	var exists bool
	function = "BucketExists(bucketName)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName": bucketName,
	}
	exists, err = c.BucketExists(context.Background(), bucketName)
	if err != nil {
		logError(testName, function, args, startTime, "", "BucketExists failed", err)
		return
	}
	if !exists {
		logError(testName, function, args, startTime, "", "Could not find existing bucket "+bucketName, err)
		return
	}

	// Make the bucket 'public read/write'.
	function = "SetBucketPolicy(bucketName, bucketPolicy)"
	functionAll += ", " + function

	readWritePolicy := `{"Version": "2012-10-17","Statement": [{"Action": ["s3:ListBucketMultipartUploads", "s3:ListBucket"],"Effect": "Allow","Principal": {"AWS": ["*"]},"Resource": ["arn:aws:s3:::` + bucketName + `"],"Sid": ""}]}`

	args = map[string]interface{}{
		"bucketName":   bucketName,
		"bucketPolicy": readWritePolicy,
	}
	err = c.SetBucketPolicy(context.Background(), bucketName, readWritePolicy)

	if err != nil {
		logError(testName, function, args, startTime, "", "SetBucketPolicy failed", err)
		return
	}

	// List all buckets.
	function = "ListBuckets()"
	functionAll += ", " + function
	args = nil
	buckets, err := c.ListBuckets(context.Background())
	if len(buckets) == 0 {
		logError(testName, function, args, startTime, "", "List buckets cannot be empty", err)
		return
	}
	if err != nil {
		logError(testName, function, args, startTime, "", "ListBuckets failed", err)
		return
	}

	// Verify if previously created bucket is listed in list buckets.
	bucketFound := false
	for _, bucket := range buckets {
		if bucket.Name == bucketName {
			bucketFound = true
		}
	}

	// If bucket not found error out.
	if !bucketFound {
		logError(testName, function, args, startTime, "", "Bucket "+bucketName+"not found", err)
		return
	}

	objectName := bucketName + "unique"

	// Generate data
	buf := bytes.Repeat([]byte("n"), rand.Intn(1<<19))

	args = map[string]interface{}{
		"bucketName":  bucketName,
		"objectName":  objectName,
		"contentType": "",
	}
	_, err = c.PutObject(context.Background(), bucketName, objectName, bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	st, err := c.StatObject(context.Background(), bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject failed", err)
		return
	}
	if st.Size != int64(len(buf)) {
		logError(testName, function, args, startTime, "", "Expected uploaded object length "+string(len(buf))+" got "+string(st.Size), err)
		return
	}

	objectNameNoLength := objectName + "-nolength"
	args["objectName"] = objectNameNoLength
	_, err = c.PutObject(context.Background(), bucketName, objectNameNoLength, bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}
	st, err = c.StatObject(context.Background(), bucketName, objectNameNoLength, minio.StatObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "StatObject failed", err)
		return
	}
	if st.Size != int64(len(buf)) {
		logError(testName, function, args, startTime, "", "Expected uploaded object length "+string(len(buf))+" got "+string(st.Size), err)
		return
	}

	// Instantiate a done channel to close all listing.
	doneCh := make(chan struct{})
	defer close(doneCh)

	objFound := false
	isRecursive := true // Recursive is true.
	function = "ListObjects(bucketName, objectName, isRecursive, doneCh)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName":  bucketName,
		"objectName":  objectName,
		"isRecursive": isRecursive,
	}
	for obj := range c.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{UseV1: true, Prefix: objectName, Recursive: isRecursive}) {
		if obj.Key == objectName {
			objFound = true
			break
		}
	}
	if !objFound {
		logError(testName, function, args, startTime, "", "Could not find existing object "+objectName, err)
		return
	}

	incompObjNotFound := true
	function = "ListIncompleteUploads(bucketName, objectName, isRecursive, doneCh)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName":  bucketName,
		"objectName":  objectName,
		"isRecursive": isRecursive,
	}
	for objIncompl := range c.ListIncompleteUploads(context.Background(), bucketName, objectName, isRecursive) {
		if objIncompl.Key != "" {
			incompObjNotFound = false
			break
		}
	}
	if !incompObjNotFound {
		logError(testName, function, args, startTime, "", "Unexpected dangling incomplete upload found", err)
		return
	}

	function = "GetObject(bucketName, objectName)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName": bucketName,
		"objectName": objectName,
	}
	newReader, err := c.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}

	newReadBytes, err := ioutil.ReadAll(newReader)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAll failed", err)
		return
	}
	newReader.Close()

	if !bytes.Equal(newReadBytes, buf) {
		logError(testName, function, args, startTime, "", "Bytes mismatch", err)
		return
	}

	function = "FGetObject(bucketName, objectName, fileName)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName": bucketName,
		"objectName": objectName,
		"fileName":   fileName + "-f",
	}
	err = c.FGetObject(context.Background(), bucketName, objectName, fileName+"-f", minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "FgetObject failed", err)
		return
	}

	// Generate presigned HEAD object url.
	function = "PresignedHeadObject(bucketName, objectName, expires, reqParams)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName": bucketName,
		"objectName": objectName,
		"expires":    3600 * time.Second,
	}
	presignedHeadURL, err := c.PresignedHeadObject(context.Background(), bucketName, objectName, 3600*time.Second, nil)
	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedHeadObject failed", err)
		return
	}

	transport, err := minio.DefaultTransport(mustParseBool(os.Getenv(enableHTTPS)))
	if err != nil {
		logError(testName, function, args, startTime, "", "DefaultTransport failed", err)
		return
	}

	httpClient := &http.Client{
		// Setting a sensible time out of 30secs to wait for response
		// headers. Request is pro-actively canceled after 30secs
		// with no response.
		Timeout:   30 * time.Second,
		Transport: transport,
	}

	req, err := http.NewRequest(http.MethodHead, presignedHeadURL.String(), nil)
	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedHeadObject URL head request failed", err)
		return
	}

	// Verify if presigned url works.
	resp, err := httpClient.Do(req)
	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedHeadObject URL head request failed", err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		logError(testName, function, args, startTime, "", "PresignedHeadObject URL returns status "+string(resp.StatusCode), err)
		return
	}
	if resp.Header.Get("ETag") == "" {
		logError(testName, function, args, startTime, "", "Got empty ETag", err)
		return
	}
	resp.Body.Close()

	// Generate presigned GET object url.
	function = "PresignedGetObject(bucketName, objectName, expires, reqParams)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName": bucketName,
		"objectName": objectName,
		"expires":    3600 * time.Second,
	}
	presignedGetURL, err := c.PresignedGetObject(context.Background(), bucketName, objectName, 3600*time.Second, nil)
	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedGetObject failed", err)
		return
	}

	// Verify if presigned url works.
	req, err = http.NewRequest(http.MethodGet, presignedGetURL.String(), nil)
	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedGetObject request incorrect", err)
		return
	}

	resp, err = httpClient.Do(req)
	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedGetObject response incorrect", err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		logError(testName, function, args, startTime, "", "PresignedGetObject URL returns status "+string(resp.StatusCode), err)
		return
	}
	newPresignedBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAll failed", err)
		return
	}
	resp.Body.Close()
	if !bytes.Equal(newPresignedBytes, buf) {
		logError(testName, function, args, startTime, "", "Bytes mismatch", err)
		return
	}

	// Set request parameters.
	reqParams := make(url.Values)
	reqParams.Set("response-content-disposition", "attachment; filename=\"test.txt\"")
	// Generate presigned GET object url.
	args["reqParams"] = reqParams
	presignedGetURL, err = c.PresignedGetObject(context.Background(), bucketName, objectName, 3600*time.Second, reqParams)
	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedGetObject failed", err)
		return
	}

	// Verify if presigned url works.
	req, err = http.NewRequest(http.MethodGet, presignedGetURL.String(), nil)
	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedGetObject request incorrect", err)
		return
	}

	resp, err = httpClient.Do(req)
	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedGetObject response incorrect", err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		logError(testName, function, args, startTime, "", "PresignedGetObject URL returns status "+string(resp.StatusCode), err)
		return
	}
	newPresignedBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAll failed", err)
		return
	}
	if !bytes.Equal(newPresignedBytes, buf) {
		logError(testName, function, args, startTime, "", "Bytes mismatch", err)
		return
	}
	// Verify content disposition.
	if resp.Header.Get("Content-Disposition") != "attachment; filename=\"test.txt\"" {
		logError(testName, function, args, startTime, "", "wrong Content-Disposition received ", err)
		return
	}

	function = "PresignedPutObject(bucketName, objectName, expires)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName": bucketName,
		"objectName": objectName + "-presigned",
		"expires":    3600 * time.Second,
	}
	presignedPutURL, err := c.PresignedPutObject(context.Background(), bucketName, objectName+"-presigned", 3600*time.Second)
	if err != nil {
		logError(testName, function, args, startTime, "", "PresignedPutObject failed", err)
		return
	}

	// Generate data more than 32K
	buf = bytes.Repeat([]byte("1"), rand.Intn(1<<10)+32*1024)

	req, err = http.NewRequest(http.MethodPut, presignedPutURL.String(), bytes.NewReader(buf))
	if err != nil {
		logError(testName, function, args, startTime, "", "HTTP request to PresignedPutObject URL failed", err)
		return
	}

	resp, err = httpClient.Do(req)
	if err != nil {
		logError(testName, function, args, startTime, "", "HTTP request to PresignedPutObject URL failed", err)
		return
	}

	function = "GetObject(bucketName, objectName)"
	functionAll += ", " + function
	args = map[string]interface{}{
		"bucketName": bucketName,
		"objectName": objectName + "-presigned",
	}
	newReader, err = c.GetObject(context.Background(), bucketName, objectName+"-presigned", minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}

	newReadBytes, err = ioutil.ReadAll(newReader)
	if err != nil {
		logError(testName, function, args, startTime, "", "ReadAll failed", err)
		return
	}
	newReader.Close()

	if !bytes.Equal(newReadBytes, buf) {
		logError(testName, function, args, startTime, "", "Bytes mismatch", err)
		return
	}

	os.Remove(fileName)
	os.Remove(fileName + "-f")
	successLogger(testName, functionAll, args, startTime).Info()
}

// Test get object with GetObject with context
func testGetObjectContext() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "GetObject(ctx, bucketName, objectName)"
	args := map[string]interface{}{
		"ctx":        "",
		"bucketName": "",
		"objectName": "",
	}
	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client v4 object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	bufSize := dataFileMap["datafile-33-kB"]
	var reader = getDataReader("datafile-33-kB")
	defer reader.Close()
	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	_, err = c.PutObject(context.Background(), bucketName, objectName, reader, int64(bufSize), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	args["ctx"] = ctx
	cancel()

	r, err := c.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed unexpectedly", err)
		return
	}

	if _, err = r.Stat(); err == nil {
		logError(testName, function, args, startTime, "", "GetObject should fail on short timeout", err)
		return
	}
	r.Close()

	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Hour)
	args["ctx"] = ctx
	defer cancel()

	// Read the data back
	r, err = c.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed", err)
		return
	}

	st, err := r.Stat()
	if err != nil {
		logError(testName, function, args, startTime, "", "object Stat call failed", err)
		return
	}
	if st.Size != int64(bufSize) {
		logError(testName, function, args, startTime, "", "Number of bytes in stat does not match: want "+string(bufSize)+", got"+string(st.Size), err)
		return
	}
	if err := r.Close(); err != nil {
		logError(testName, function, args, startTime, "", "object Close() call failed", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()

}

// Test get object with FGetObject with a user provided context
func testFGetObjectContext() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "FGetObject(ctx, bucketName, objectName, fileName)"
	args := map[string]interface{}{
		"ctx":        "",
		"bucketName": "",
		"objectName": "",
		"fileName":   "",
	}
	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client v4 object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	bufSize := dataFileMap["datafile-1-MB"]
	var reader = getDataReader("datafile-1-MB")
	defer reader.Close()
	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	_, err = c.PutObject(context.Background(), bucketName, objectName, reader, int64(bufSize), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	args["ctx"] = ctx
	defer cancel()

	fileName := "tempfile-context"
	args["fileName"] = fileName
	// Read the data back
	err = c.FGetObject(ctx, bucketName, objectName, fileName+"-f", minio.GetObjectOptions{})
	if err == nil {
		logError(testName, function, args, startTime, "", "FGetObject should fail on short timeout", err)
		return
	}
	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()

	// Read the data back
	err = c.FGetObject(ctx, bucketName, objectName, fileName+"-fcontext", minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "FGetObject with long timeout failed", err)
		return
	}
	if err = os.Remove(fileName + "-fcontext"); err != nil {
		logError(testName, function, args, startTime, "", "Remove file failed", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()

}

// Test get object with GetObject with a user provided context
func testGetObjectRanges() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "GetObject(ctx, bucketName, objectName, fileName)"
	args := map[string]interface{}{
		"ctx":        "",
		"bucketName": "",
		"objectName": "",
		"fileName":   "",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	rng := rand.NewSource(time.Now().UnixNano())
	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client v4 object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rng, "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	bufSize := dataFileMap["datafile-129-MB"]
	var reader = getDataReader("datafile-129-MB")
	defer reader.Close()
	// Save the data
	objectName := randString(60, rng, "")
	args["objectName"] = objectName

	_, err = c.PutObject(context.Background(), bucketName, objectName, reader, int64(bufSize), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	// Read the data back
	tests := []struct {
		start int64
		end   int64
	}{
		{
			start: 1024,
			end:   1024 + 1<<20,
		},
		{
			start: 20e6,
			end:   20e6 + 10000,
		},
		{
			start: 40e6,
			end:   40e6 + 10000,
		},
		{
			start: 60e6,
			end:   60e6 + 10000,
		},
		{
			start: 80e6,
			end:   80e6 + 10000,
		},
		{
			start: 120e6,
			end:   int64(bufSize),
		},
	}
	for _, test := range tests {
		wantRC := getDataReader("datafile-129-MB")
		io.CopyN(ioutil.Discard, wantRC, test.start)
		want := mustCrcReader(io.LimitReader(wantRC, test.end-test.start+1))
		opts := minio.GetObjectOptions{}
		opts.SetRange(test.start, test.end)
		args["opts"] = fmt.Sprintf("%+v", test)
		obj, err := c.GetObject(ctx, bucketName, objectName, opts)
		if err != nil {
			logError(testName, function, args, startTime, "", "FGetObject with long timeout failed", err)
			return
		}
		err = crcMatches(obj, want)
		if err != nil {
			logError(testName, function, args, startTime, "", fmt.Sprintf("GetObject offset %d -> %d", test.start, test.end), err)
			return
		}
	}

	successLogger(testName, function, args, startTime).Info()
}

// Test get object ACLs with GetObjectACL with custom provided context
func testGetObjectACLContext() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "GetObjectACL(ctx, bucketName, objectName)"
	args := map[string]interface{}{
		"ctx":        "",
		"bucketName": "",
		"objectName": "",
	}
	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// skipping region functional tests for non s3 runs
	if os.Getenv(serverEndpoint) != "s3.amazonaws.com" {
		ignoredLog(testName, function, args, startTime, "Skipped region functional tests for non s3 runs").Info()
		return
	}

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client v4 object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	bufSize := dataFileMap["datafile-1-MB"]
	var reader = getDataReader("datafile-1-MB")
	defer reader.Close()
	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	// Add meta data to add a canned acl
	metaData := map[string]string{
		"X-Amz-Acl": "public-read-write",
	}

	_, err = c.PutObject(context.Background(), bucketName,
		objectName, reader, int64(bufSize),
		minio.PutObjectOptions{
			ContentType:  "binary/octet-stream",
			UserMetadata: metaData,
		})

	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	args["ctx"] = ctx
	defer cancel()

	// Read the data back
	objectInfo, getObjectACLErr := c.GetObjectACL(ctx, bucketName, objectName)
	if getObjectACLErr != nil {
		logError(testName, function, args, startTime, "", "GetObjectACL failed. ", getObjectACLErr)
		return
	}

	s, ok := objectInfo.Metadata["X-Amz-Acl"]
	if !ok {
		logError(testName, function, args, startTime, "", "GetObjectACL fail unable to find \"X-Amz-Acl\"", nil)
		return
	}

	if len(s) != 1 {
		logError(testName, function, args, startTime, "", "GetObjectACL fail \"X-Amz-Acl\" canned acl expected \"1\" got "+fmt.Sprintf(`"%d"`, len(s)), nil)
		return
	}

	if s[0] != "public-read-write" {
		logError(testName, function, args, startTime, "", "GetObjectACL fail \"X-Amz-Acl\" expected \"public-read-write\" but got"+fmt.Sprintf("%q", s[0]), nil)
		return
	}

	bufSize = dataFileMap["datafile-1-MB"]
	var reader2 = getDataReader("datafile-1-MB")
	defer reader2.Close()
	// Save the data
	objectName = randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	// Add meta data to add a canned acl
	metaData = map[string]string{
		"X-Amz-Grant-Read":  "id=fooread@minio.go",
		"X-Amz-Grant-Write": "id=foowrite@minio.go",
	}

	_, err = c.PutObject(context.Background(), bucketName, objectName, reader2, int64(bufSize), minio.PutObjectOptions{ContentType: "binary/octet-stream", UserMetadata: metaData})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject failed", err)
		return
	}

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	args["ctx"] = ctx
	defer cancel()

	// Read the data back
	objectInfo, getObjectACLErr = c.GetObjectACL(ctx, bucketName, objectName)
	if getObjectACLErr == nil {
		logError(testName, function, args, startTime, "", "GetObjectACL fail", getObjectACLErr)
		return
	}

	if len(objectInfo.Metadata) != 3 {
		logError(testName, function, args, startTime, "", "GetObjectACL fail expected \"3\" ACLs but got "+fmt.Sprintf(`"%d"`, len(objectInfo.Metadata)), nil)
		return
	}

	s, ok = objectInfo.Metadata["X-Amz-Grant-Read"]
	if !ok {
		logError(testName, function, args, startTime, "", "GetObjectACL fail unable to find \"X-Amz-Grant-Read\"", nil)
		return
	}

	if len(s) != 1 {
		logError(testName, function, args, startTime, "", "GetObjectACL fail \"X-Amz-Grant-Read\" acl expected \"1\" got "+fmt.Sprintf(`"%d"`, len(s)), nil)
		return
	}

	if s[0] != "fooread@minio.go" {
		logError(testName, function, args, startTime, "", "GetObjectACL fail \"X-Amz-Grant-Read\" acl expected \"fooread@minio.go\" got "+fmt.Sprintf("%q", s), nil)
		return
	}

	s, ok = objectInfo.Metadata["X-Amz-Grant-Write"]
	if !ok {
		logError(testName, function, args, startTime, "", "GetObjectACL fail unable to find \"X-Amz-Grant-Write\"", nil)
		return
	}

	if len(s) != 1 {
		logError(testName, function, args, startTime, "", "GetObjectACL fail \"X-Amz-Grant-Write\" acl expected \"1\" got "+fmt.Sprintf(`"%d"`, len(s)), nil)
		return
	}

	if s[0] != "foowrite@minio.go" {
		logError(testName, function, args, startTime, "", "GetObjectACL fail \"X-Amz-Grant-Write\" acl expected \"foowrite@minio.go\" got "+fmt.Sprintf("%q", s), nil)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Test validates putObject with context to see if request cancellation is honored for V2.
func testPutObjectContextV2() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "PutObject(ctx, bucketName, objectName, reader, size, opts)"
	args := map[string]interface{}{
		"ctx":        "",
		"bucketName": "",
		"objectName": "",
		"size":       "",
		"opts":       "",
	}
	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV2(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client v2 object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Make a new bucket.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}
	defer cleanupBucket(bucketName, c)
	bufSize := dataFileMap["datatfile-33-kB"]
	var reader = getDataReader("datafile-33-kB")
	defer reader.Close()

	objectName := fmt.Sprintf("test-file-%v", rand.Uint32())
	args["objectName"] = objectName

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	args["ctx"] = ctx
	args["size"] = bufSize
	defer cancel()

	_, err = c.PutObject(ctx, bucketName, objectName, reader, int64(bufSize), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject with short timeout failed", err)
		return
	}

	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Hour)
	args["ctx"] = ctx

	defer cancel()
	reader = getDataReader("datafile-33-kB")
	defer reader.Close()
	_, err = c.PutObject(ctx, bucketName, objectName, reader, int64(bufSize), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject with long timeout failed", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()

}

// Test get object with GetObject with custom context
func testGetObjectContextV2() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "GetObject(ctx, bucketName, objectName)"
	args := map[string]interface{}{
		"ctx":        "",
		"bucketName": "",
		"objectName": "",
	}
	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV2(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client v2 object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	bufSize := dataFileMap["datafile-33-kB"]
	var reader = getDataReader("datafile-33-kB")
	defer reader.Close()
	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	_, err = c.PutObject(context.Background(), bucketName, objectName, reader, int64(bufSize), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject call failed", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	args["ctx"] = ctx
	cancel()

	r, err := c.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject failed unexpectedly", err)
		return
	}
	if _, err = r.Stat(); err == nil {
		logError(testName, function, args, startTime, "", "GetObject should fail on short timeout", err)
		return
	}
	r.Close()

	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()

	// Read the data back
	r, err = c.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "GetObject shouldn't fail on longer timeout", err)
		return
	}

	st, err := r.Stat()
	if err != nil {
		logError(testName, function, args, startTime, "", "object Stat call failed", err)
		return
	}
	if st.Size != int64(bufSize) {
		logError(testName, function, args, startTime, "", "Number of bytes in stat does not match, expected "+string(bufSize)+" got "+string(st.Size), err)
		return
	}
	if err := r.Close(); err != nil {
		logError(testName, function, args, startTime, "", " object Close() call failed", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()

}

// Test get object with FGetObject with custom context
func testFGetObjectContextV2() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "FGetObject(ctx, bucketName, objectName,fileName)"
	args := map[string]interface{}{
		"ctx":        "",
		"bucketName": "",
		"objectName": "",
		"fileName":   "",
	}
	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV2(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client v2 object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket call failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	bufSize := dataFileMap["datatfile-1-MB"]
	var reader = getDataReader("datafile-1-MB")
	defer reader.Close()
	// Save the data
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	_, err = c.PutObject(context.Background(), bucketName, objectName, reader, int64(bufSize), minio.PutObjectOptions{ContentType: "binary/octet-stream"})
	if err != nil {
		logError(testName, function, args, startTime, "", "PutObject call failed", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	args["ctx"] = ctx
	defer cancel()

	fileName := "tempfile-context"
	args["fileName"] = fileName

	// Read the data back
	err = c.FGetObject(ctx, bucketName, objectName, fileName+"-f", minio.GetObjectOptions{})
	if err == nil {
		logError(testName, function, args, startTime, "", "FGetObject should fail on short timeout", err)
		return
	}
	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()

	// Read the data back
	err = c.FGetObject(ctx, bucketName, objectName, fileName+"-fcontext", minio.GetObjectOptions{})
	if err != nil {
		logError(testName, function, args, startTime, "", "FGetObject call shouldn't fail on long timeout", err)
		return
	}

	if err = os.Remove(fileName + "-fcontext"); err != nil {
		logError(testName, function, args, startTime, "", "Remove file failed", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()

}

// Test list object v1 and V2
func testListObjects() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "ListObjects(bucketName, objectPrefix, recursive, doneCh)"
	args := map[string]interface{}{
		"bucketName":   "",
		"objectPrefix": "",
		"recursive":    "true",
	}
	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client v4 object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1"})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	defer cleanupBucket(bucketName, c)

	testObjects := []struct {
		name         string
		storageClass string
	}{
		// Special characters
		{"foo bar", "STANDARD"},
		{"foo-%", "STANDARD"},
		{"random-object-1", "STANDARD"},
		{"random-object-2", "REDUCED_REDUNDANCY"},
	}

	for i, object := range testObjects {
		bufSize := dataFileMap["datafile-33-kB"]
		var reader = getDataReader("datafile-33-kB")
		defer reader.Close()
		_, err = c.PutObject(context.Background(), bucketName, object.name, reader, int64(bufSize),
			minio.PutObjectOptions{ContentType: "binary/octet-stream", StorageClass: object.storageClass})
		if err != nil {
			logError(testName, function, args, startTime, "", fmt.Sprintf("PutObject %d call failed", i+1), err)
			return
		}
	}

	testList := func(listFn func(context.Context, string, minio.ListObjectsOptions) <-chan minio.ObjectInfo, bucket string, opts minio.ListObjectsOptions) {
		var objCursor int

		// check for object name and storage-class from listing object result
		for objInfo := range listFn(context.Background(), bucket, opts) {
			if objInfo.Err != nil {
				logError(testName, function, args, startTime, "", "ListObjects failed unexpectedly", err)
				return
			}
			if objInfo.Key != testObjects[objCursor].name {
				logError(testName, function, args, startTime, "", "ListObjects does not return expected object name", err)
				return
			}
			if objInfo.StorageClass != testObjects[objCursor].storageClass {
				// Ignored as Gateways (Azure/GCS etc) wont return storage class
				ignoredLog(testName, function, args, startTime, "ListObjects doesn't return expected storage class").Info()
			}
			objCursor++
		}

		if objCursor != len(testObjects) {
			logError(testName, function, args, startTime, "", "ListObjects returned unexpected number of items", errors.New(""))
			return
		}
	}

	testList(c.ListObjects, bucketName, minio.ListObjectsOptions{Recursive: true, UseV1: true})
	testList(c.ListObjects, bucketName, minio.ListObjectsOptions{Recursive: true})
	testList(c.ListObjects, bucketName, minio.ListObjectsOptions{Recursive: true, WithMetadata: true})

	successLogger(testName, function, args, startTime).Info()
}

// Test deleting multiple objects with object retention set in Governance mode
func testRemoveObjects() {
	// initialize logging params
	startTime := time.Now()
	testName := getFuncName()
	function := "RemoveObjects(bucketName, objectsCh, opts)"
	args := map[string]interface{}{
		"bucketName":   "",
		"objectPrefix": "",
		"recursive":    "true",
	}
	// Seed random based on current time.
	rand.Seed(time.Now().Unix())

	// Instantiate new minio client object.
	c, err := minio.New(os.Getenv(serverEndpoint),
		&minio.Options{
			Creds:  credentials.NewStaticV4(os.Getenv(accessKey), os.Getenv(secretKey), ""),
			Secure: mustParseBool(os.Getenv(enableHTTPS)),
		})
	if err != nil {
		logError(testName, function, args, startTime, "", "MinIO client v4 object creation failed", err)
		return
	}

	// Enable tracing, write to stderr.
	// c.TraceOn(os.Stderr)

	// Set user agent.
	c.SetAppInfo("MinIO-go-FunctionalTest", "0.1.0")

	// Generate a new random bucket name.
	bucketName := randString(60, rand.NewSource(time.Now().UnixNano()), "minio-go-test-")
	args["bucketName"] = bucketName
	objectName := randString(60, rand.NewSource(time.Now().UnixNano()), "")
	args["objectName"] = objectName

	// Make a new bucket.
	err = c.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: "us-east-1", ObjectLocking: true})
	if err != nil {
		logError(testName, function, args, startTime, "", "MakeBucket failed", err)
		return
	}

	bufSize := dataFileMap["datafile-129-MB"]
	var reader = getDataReader("datafile-129-MB")
	defer reader.Close()

	n, err := c.PutObject(context.Background(), bucketName, objectName, reader, int64(bufSize), minio.PutObjectOptions{})
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("Uploaded", objectName, " of size: ", n, "to bucket: ", bucketName, "Successfully.")

	// Replace with smaller...
	bufSize = dataFileMap["datafile-10-kB"]
	reader = getDataReader("datafile-10-kB")
	defer reader.Close()

	n, err = c.PutObject(context.Background(), bucketName, objectName, reader, int64(bufSize), minio.PutObjectOptions{})
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("Uploaded", objectName, " of size: ", n, "to bucket: ", bucketName, "Successfully.")

	t := time.Date(2030, time.April, 25, 14, 0, 0, 0, time.UTC)
	m := minio.RetentionMode(minio.Governance)
	opts := minio.PutObjectRetentionOptions{
		GovernanceBypass: false,
		RetainUntilDate:  &t,
		Mode:             &m,
	}
	err = c.PutObjectRetention(context.Background(), bucketName, objectName, opts)
	if err != nil {
		log.Fatalln(err)
	}

	objectsCh := make(chan minio.ObjectInfo)
	// Send object names that are needed to be removed to objectsCh
	go func() {
		defer close(objectsCh)
		// List all objects from a bucket-name with a matching prefix.
		for object := range c.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{UseV1: true, Recursive: true}) {
			if object.Err != nil {
				log.Fatalln(object.Err)
			}
			objectsCh <- object
		}
	}()

	for rErr := range c.RemoveObjects(context.Background(), bucketName, objectsCh, minio.RemoveObjectsOptions{}) {
		// Error is expected here because Retention is set on the object
		// and RemoveObjects is called without Bypass Governance
		if rErr.Err == nil {
			logError(testName, function, args, startTime, "", "Expected error during deletion", nil)
			return
		}
	}

	objectsCh1 := make(chan minio.ObjectInfo)

	// Send object names that are needed to be removed to objectsCh
	go func() {
		defer close(objectsCh1)
		// List all objects from a bucket-name with a matching prefix.
		for object := range c.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{UseV1: true, Recursive: true}) {
			if object.Err != nil {
				log.Fatalln(object.Err)
			}
			objectsCh1 <- object
		}
	}()

	opts1 := minio.RemoveObjectsOptions{
		GovernanceBypass: true,
	}

	for rErr := range c.RemoveObjects(context.Background(), bucketName, objectsCh1, opts1) {
		// Error is not expected here because Retention is set on the object
		// and RemoveObjects is called with Bypass Governance
		logError(testName, function, args, startTime, "", "Error detected during deletion", rErr.Err)
		return
	}

	// Delete all objects and buckets
	if err = cleanupVersionedBucket(bucketName, c); err != nil {
		logError(testName, function, args, startTime, "", "CleanupBucket failed", err)
		return
	}

	successLogger(testName, function, args, startTime).Info()
}

// Convert string to bool and always return false if any error
func mustParseBool(str string) bool {
	b, err := strconv.ParseBool(str)
	if err != nil {
		return false
	}
	return b
}

func main() {
	// Output to stdout instead of the default stderr
	log.SetOutput(os.Stdout)
	// create custom formatter
	mintFormatter := mintJSONFormatter{}
	// set custom formatter
	log.SetFormatter(&mintFormatter)
	// log Info or above -- success cases are Info level, failures are Fatal level
	log.SetLevel(log.InfoLevel)

	tls := mustParseBool(os.Getenv(enableHTTPS))
	kms := mustParseBool(os.Getenv(enableKMS))
	if os.Getenv(enableKMS) == "" {
		// Default to KMS tests.
		kms = true
	}
	// execute tests
	if isFullMode() {
		testMakeBucketErrorV2()
		testGetObjectClosedTwiceV2()
		testFPutObjectV2()
		testMakeBucketRegionsV2()
		testGetObjectReadSeekFunctionalV2()
		testGetObjectReadAtFunctionalV2()
		testGetObjectRanges()
		testCopyObjectV2()
		testFunctionalV2()
		testComposeObjectErrorCasesV2()
		testCompose10KSourcesV2()
		testUserMetadataCopyingV2()
		testPutObject0ByteV2()
		testPutObjectNoLengthV2()
		testPutObjectsUnknownV2()
		testGetObjectContextV2()
		testFPutObjectContextV2()
		testFGetObjectContextV2()
		testPutObjectContextV2()
		testPutObjectWithVersioning()
		testMakeBucketError()
		testMakeBucketRegions()
		testPutObjectWithMetadata()
		testPutObjectReadAt()
		testPutObjectStreaming()
		testGetObjectSeekEnd()
		testGetObjectClosedTwice()
		testRemoveMultipleObjects()
		testFPutObjectMultipart()
		testFPutObject()
		testGetObjectReadSeekFunctional()
		testGetObjectReadAtFunctional()
		testGetObjectReadAtWhenEOFWasReached()
		testPresignedPostPolicy()
		testCopyObject()
		testComposeObjectErrorCases()
		testCompose10KSources()
		testUserMetadataCopying()
		testBucketNotification()
		testFunctional()
		testGetObjectModified()
		testPutObjectUploadSeekedObject()
		testGetObjectContext()
		testFPutObjectContext()
		testFGetObjectContext()
		testGetObjectACLContext()
		testPutObjectContext()
		testStorageClassMetadataPutObject()
		testStorageClassInvalidMetadataPutObject()
		testStorageClassMetadataCopyObject()
		testPutObjectWithContentLanguage()
		testListObjects()
		testRemoveObjects()
		testListObjectVersions()
		testStatObjectWithVersioning()
		testGetObjectWithVersioning()
		testCopyObjectWithVersioning()
		testConcurrentCopyObjectWithVersioning()
		testComposeObjectWithVersioning()
		testRemoveObjectWithVersioning()
		testRemoveObjectsWithVersioning()
		testObjectTaggingWithVersioning()

		// SSE-C tests will only work over TLS connection.
		if tls {
			testSSECEncryptionPutGet()
			testSSECEncryptionFPut()
			testSSECEncryptedGetObjectReadAtFunctional()
			testSSECEncryptedGetObjectReadSeekFunctional()
			testEncryptedCopyObjectV2()
			testEncryptedSSECToSSECCopyObject()
			testEncryptedSSECToUnencryptedCopyObject()
			testUnencryptedToSSECCopyObject()
			testUnencryptedToUnencryptedCopyObject()
			testEncryptedEmptyObject()
			testDecryptedCopyObject()
			testSSECEncryptedToSSECCopyObjectPart()
			testSSECMultipartEncryptedToSSECCopyObjectPart()
			testSSECEncryptedToUnencryptedCopyPart()
			testUnencryptedToSSECCopyObjectPart()
			testUnencryptedToUnencryptedCopyPart()
			testEncryptedSSECToSSES3CopyObject()
			testEncryptedSSES3ToSSECCopyObject()
			testSSECEncryptedToSSES3CopyObjectPart()
			testSSES3EncryptedToSSECCopyObjectPart()
		}

		// KMS tests
		if kms {
			testSSES3EncryptionPutGet()
			testSSES3EncryptionFPut()
			testSSES3EncryptedGetObjectReadAtFunctional()
			testSSES3EncryptedGetObjectReadSeekFunctional()
			testEncryptedSSES3ToSSES3CopyObject()
			testEncryptedSSES3ToUnencryptedCopyObject()
			testUnencryptedToSSES3CopyObject()
			testUnencryptedToSSES3CopyObjectPart()
			testSSES3EncryptedToUnencryptedCopyPart()
			testSSES3EncryptedToSSES3CopyObjectPart()
		}
	} else {
		testFunctional()
		testFunctionalV2()
	}
}
