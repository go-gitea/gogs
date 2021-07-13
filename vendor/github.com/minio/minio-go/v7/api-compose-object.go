/*
 * MinIO Go Library for Amazon S3 Compatible Cloud Storage
 * Copyright 2017, 2018 MinIO, Inc.
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

package minio

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7/pkg/encrypt"
	"github.com/minio/minio-go/v7/pkg/s3utils"
)

// CopyDestOptions represents options specified by user for CopyObject/ComposeObject APIs
type CopyDestOptions struct {
	Bucket string // points to destination bucket
	Object string // points to destination object

	// `Encryption` is the key info for server-side-encryption with customer
	// provided key. If it is nil, no encryption is performed.
	Encryption encrypt.ServerSide

	// `userMeta` is the user-metadata key-value pairs to be set on the
	// destination. The keys are automatically prefixed with `x-amz-meta-`
	// if needed. If nil is passed, and if only a single source (of any
	// size) is provided in the ComposeObject call, then metadata from the
	// source is copied to the destination.
	// if no user-metadata is provided, it is copied from source
	// (when there is only once source object in the compose
	// request)
	UserMetadata map[string]string
	// UserMetadata is only set to destination if ReplaceMetadata is true
	// other value is UserMetadata is ignored and we preserve src.UserMetadata
	// NOTE: if you set this value to true and now metadata is present
	// in UserMetadata your destination object will not have any metadata
	// set.
	ReplaceMetadata bool

	// `userTags` is the user defined object tags to be set on destination.
	// This will be set only if the `replaceTags` field is set to true.
	// Otherwise this field is ignored
	UserTags    map[string]string
	ReplaceTags bool

	// Specifies whether you want to apply a Legal Hold to the copied object.
	LegalHold LegalHoldStatus

	// Object Retention related fields
	Mode            RetentionMode
	RetainUntilDate time.Time

	Size int64 // Needs to be specified if progress bar is specified.
	// Progress of the entire copy operation will be sent here.
	Progress io.Reader
}

// Process custom-metadata to remove a `x-amz-meta-` prefix if
// present and validate that keys are distinct (after this
// prefix removal).
func filterCustomMeta(userMeta map[string]string) map[string]string {
	m := make(map[string]string)
	for k, v := range userMeta {
		if strings.HasPrefix(strings.ToLower(k), "x-amz-meta-") {
			k = k[len("x-amz-meta-"):]
		}
		if _, ok := m[k]; ok {
			continue
		}
		m[k] = v
	}
	return m
}

// Marshal converts all the CopyDestOptions into their
// equivalent HTTP header representation
func (opts CopyDestOptions) Marshal(header http.Header) {
	const replaceDirective = "REPLACE"
	if opts.ReplaceTags {
		header.Set(amzTaggingHeaderDirective, replaceDirective)
		if tags := s3utils.TagEncode(opts.UserTags); tags != "" {
			header.Set(amzTaggingHeader, tags)
		}
	}

	if opts.LegalHold != LegalHoldStatus("") {
		header.Set(amzLegalHoldHeader, opts.LegalHold.String())
	}

	if opts.Mode != RetentionMode("") && !opts.RetainUntilDate.IsZero() {
		header.Set(amzLockMode, opts.Mode.String())
		header.Set(amzLockRetainUntil, opts.RetainUntilDate.Format(time.RFC3339))
	}

	if opts.Encryption != nil {
		opts.Encryption.Marshal(header)
	}

	if opts.ReplaceMetadata {
		header.Set("x-amz-metadata-directive", replaceDirective)
		for k, v := range filterCustomMeta(opts.UserMetadata) {
			if isAmzHeader(k) || isStandardHeader(k) || isStorageClassHeader(k) {
				header.Set(k, v)
			} else {
				header.Set("x-amz-meta-"+k, v)
			}
		}
	}
}

// toDestinationInfo returns a validated copyOptions object.
func (opts CopyDestOptions) validate() (err error) {
	// Input validation.
	if err = s3utils.CheckValidBucketName(opts.Bucket); err != nil {
		return err
	}
	if err = s3utils.CheckValidObjectName(opts.Object); err != nil {
		return err
	}
	if opts.Progress != nil && opts.Size < 0 {
		return errInvalidArgument("For progress bar effective size needs to be specified")
	}
	return nil
}

// CopySrcOptions represents a source object to be copied, using
// server-side copying APIs.
type CopySrcOptions struct {
	Bucket, Object       string
	VersionID            string
	MatchETag            string
	NoMatchETag          string
	MatchModifiedSince   time.Time
	MatchUnmodifiedSince time.Time
	MatchRange           bool
	Start, End           int64
	Encryption           encrypt.ServerSide
}

// Marshal converts all the CopySrcOptions into their
// equivalent HTTP header representation
func (opts CopySrcOptions) Marshal(header http.Header) {
	// Set the source header
	header.Set("x-amz-copy-source", s3utils.EncodePath(opts.Bucket+"/"+opts.Object))
	if opts.VersionID != "" {
		header.Set("x-amz-copy-source", s3utils.EncodePath(opts.Bucket+"/"+opts.Object)+"?versionId="+opts.VersionID)
	}

	if opts.MatchETag != "" {
		header.Set("x-amz-copy-source-if-match", opts.MatchETag)
	}
	if opts.NoMatchETag != "" {
		header.Set("x-amz-copy-source-if-none-match", opts.NoMatchETag)
	}

	if !opts.MatchModifiedSince.IsZero() {
		header.Set("x-amz-copy-source-if-modified-since", opts.MatchModifiedSince.Format(http.TimeFormat))
	}
	if !opts.MatchUnmodifiedSince.IsZero() {
		header.Set("x-amz-copy-source-if-unmodified-since", opts.MatchUnmodifiedSince.Format(http.TimeFormat))
	}

	if opts.Encryption != nil {
		encrypt.SSECopy(opts.Encryption).Marshal(header)
	}
}

func (opts CopySrcOptions) validate() (err error) {
	// Input validation.
	if err = s3utils.CheckValidBucketName(opts.Bucket); err != nil {
		return err
	}
	if err = s3utils.CheckValidObjectName(opts.Object); err != nil {
		return err
	}
	if opts.Start > opts.End || opts.Start < 0 {
		return errInvalidArgument("start must be non-negative, and start must be at most end.")
	}
	return nil
}

// Low level implementation of CopyObject API, supports only upto 5GiB worth of copy.
func (c Client) copyObjectDo(ctx context.Context, srcBucket, srcObject, destBucket, destObject string,
	metadata map[string]string, srcOpts CopySrcOptions, dstOpts PutObjectOptions) (ObjectInfo, error) {

	// Build headers.
	headers := make(http.Header)

	// Set all the metadata headers.
	for k, v := range metadata {
		headers.Set(k, v)
	}
	if !dstOpts.Internal.ReplicationStatus.Empty() {
		headers.Set(amzBucketReplicationStatus, string(dstOpts.Internal.ReplicationStatus))
	}
	if !dstOpts.Internal.SourceMTime.IsZero() {
		headers.Set(minIOBucketSourceMTime, dstOpts.Internal.SourceMTime.Format(time.RFC3339Nano))
	}
	if dstOpts.Internal.SourceETag != "" {
		headers.Set(minIOBucketSourceETag, dstOpts.Internal.SourceETag)
	}
	if dstOpts.Internal.ReplicationRequest {
		headers.Set(minIOBucketReplicationRequest, "")
	}
	if len(dstOpts.UserTags) != 0 {
		headers.Set(amzTaggingHeader, s3utils.TagEncode(dstOpts.UserTags))
	}

	reqMetadata := requestMetadata{
		bucketName:   destBucket,
		objectName:   destObject,
		customHeader: headers,
	}
	if dstOpts.Internal.SourceVersionID != "" {
		if _, err := uuid.Parse(dstOpts.Internal.SourceVersionID); err != nil {
			return ObjectInfo{}, errInvalidArgument(err.Error())
		}
		urlValues := make(url.Values)
		urlValues.Set("versionId", dstOpts.Internal.SourceVersionID)
		reqMetadata.queryValues = urlValues
	}

	// Set the source header
	headers.Set("x-amz-copy-source", s3utils.EncodePath(srcBucket+"/"+srcObject))
	if srcOpts.VersionID != "" {
		headers.Set("x-amz-copy-source", s3utils.EncodePath(srcBucket+"/"+srcObject)+"?versionId="+srcOpts.VersionID)
	}
	// Send upload-part-copy request
	resp, err := c.executeMethod(ctx, http.MethodPut, reqMetadata)
	defer closeResponse(resp)
	if err != nil {
		return ObjectInfo{}, err
	}

	// Check if we got an error response.
	if resp.StatusCode != http.StatusOK {
		return ObjectInfo{}, httpRespToErrorResponse(resp, srcBucket, srcObject)
	}

	cpObjRes := copyObjectResult{}
	err = xmlDecoder(resp.Body, &cpObjRes)
	if err != nil {
		return ObjectInfo{}, err
	}

	objInfo := ObjectInfo{
		Key:          destObject,
		ETag:         strings.Trim(cpObjRes.ETag, "\""),
		LastModified: cpObjRes.LastModified,
	}
	return objInfo, nil
}

func (c Client) copyObjectPartDo(ctx context.Context, srcBucket, srcObject, destBucket, destObject string, uploadID string,
	partID int, startOffset int64, length int64, metadata map[string]string) (p CompletePart, err error) {

	headers := make(http.Header)

	// Set source
	headers.Set("x-amz-copy-source", s3utils.EncodePath(srcBucket+"/"+srcObject))

	if startOffset < 0 {
		return p, errInvalidArgument("startOffset must be non-negative")
	}

	if length >= 0 {
		headers.Set("x-amz-copy-source-range", fmt.Sprintf("bytes=%d-%d", startOffset, startOffset+length-1))
	}

	for k, v := range metadata {
		headers.Set(k, v)
	}

	queryValues := make(url.Values)
	queryValues.Set("partNumber", strconv.Itoa(partID))
	queryValues.Set("uploadId", uploadID)

	resp, err := c.executeMethod(ctx, http.MethodPut, requestMetadata{
		bucketName:   destBucket,
		objectName:   destObject,
		customHeader: headers,
		queryValues:  queryValues,
	})
	defer closeResponse(resp)
	if err != nil {
		return
	}

	// Check if we got an error response.
	if resp.StatusCode != http.StatusOK {
		return p, httpRespToErrorResponse(resp, destBucket, destObject)
	}

	// Decode copy-part response on success.
	cpObjRes := copyObjectResult{}
	err = xmlDecoder(resp.Body, &cpObjRes)
	if err != nil {
		return p, err
	}
	p.PartNumber, p.ETag = partID, cpObjRes.ETag
	return p, nil
}

// uploadPartCopy - helper function to create a part in a multipart
// upload via an upload-part-copy request
// https://docs.aws.amazon.com/AmazonS3/latest/API/mpUploadUploadPartCopy.html
func (c Client) uploadPartCopy(ctx context.Context, bucket, object, uploadID string, partNumber int,
	headers http.Header) (p CompletePart, err error) {

	// Build query parameters
	urlValues := make(url.Values)
	urlValues.Set("partNumber", strconv.Itoa(partNumber))
	urlValues.Set("uploadId", uploadID)

	// Send upload-part-copy request
	resp, err := c.executeMethod(ctx, http.MethodPut, requestMetadata{
		bucketName:   bucket,
		objectName:   object,
		customHeader: headers,
		queryValues:  urlValues,
	})
	defer closeResponse(resp)
	if err != nil {
		return p, err
	}

	// Check if we got an error response.
	if resp.StatusCode != http.StatusOK {
		return p, httpRespToErrorResponse(resp, bucket, object)
	}

	// Decode copy-part response on success.
	cpObjRes := copyObjectResult{}
	err = xmlDecoder(resp.Body, &cpObjRes)
	if err != nil {
		return p, err
	}
	p.PartNumber, p.ETag = partNumber, cpObjRes.ETag
	return p, nil
}

// ComposeObject - creates an object using server-side copying
// of existing objects. It takes a list of source objects (with optional offsets)
// and concatenates them into a new object using only server-side copying
// operations. Optionally takes progress reader hook for applications to
// look at current progress.
func (c Client) ComposeObject(ctx context.Context, dst CopyDestOptions, srcs ...CopySrcOptions) (UploadInfo, error) {
	if len(srcs) < 1 || len(srcs) > maxPartsCount {
		return UploadInfo{}, errInvalidArgument("There must be as least one and up to 10000 source objects.")
	}

	for _, src := range srcs {
		if err := src.validate(); err != nil {
			return UploadInfo{}, err
		}
	}

	if err := dst.validate(); err != nil {
		return UploadInfo{}, err
	}

	srcObjectInfos := make([]ObjectInfo, len(srcs))
	srcObjectSizes := make([]int64, len(srcs))
	var totalSize, totalParts int64
	var err error
	for i, src := range srcs {
		opts := StatObjectOptions{ServerSideEncryption: encrypt.SSE(src.Encryption), VersionID: src.VersionID}
		srcObjectInfos[i], err = c.statObject(context.Background(), src.Bucket, src.Object, opts)
		if err != nil {
			return UploadInfo{}, err
		}

		srcCopySize := srcObjectInfos[i].Size
		// Check if a segment is specified, and if so, is the
		// segment within object bounds?
		if src.MatchRange {
			// Since range is specified,
			//    0 <= src.start <= src.end
			// so only invalid case to check is:
			if src.End >= srcCopySize || src.Start < 0 {
				return UploadInfo{}, errInvalidArgument(
					fmt.Sprintf("CopySrcOptions %d has invalid segment-to-copy [%d, %d] (size is %d)",
						i, src.Start, src.End, srcCopySize))
			}
			srcCopySize = src.End - src.Start + 1
		}

		// Only the last source may be less than `absMinPartSize`
		if srcCopySize < absMinPartSize && i < len(srcs)-1 {
			return UploadInfo{}, errInvalidArgument(
				fmt.Sprintf("CopySrcOptions %d is too small (%d) and it is not the last part", i, srcCopySize))
		}

		// Is data to copy too large?
		totalSize += srcCopySize
		if totalSize > maxMultipartPutObjectSize {
			return UploadInfo{}, errInvalidArgument(fmt.Sprintf("Cannot compose an object of size %d (> 5TiB)", totalSize))
		}

		// record source size
		srcObjectSizes[i] = srcCopySize

		// calculate parts needed for current source
		totalParts += partsRequired(srcCopySize)
		// Do we need more parts than we are allowed?
		if totalParts > maxPartsCount {
			return UploadInfo{}, errInvalidArgument(fmt.Sprintf(
				"Your proposed compose object requires more than %d parts", maxPartsCount))
		}
	}

	// Single source object case (i.e. when only one source is
	// involved, it is being copied wholly and at most 5GiB in
	// size, emptyfiles are also supported).
	if (totalParts == 1 && srcs[0].Start == -1 && totalSize <= maxPartSize) || (totalSize == 0) {
		return c.CopyObject(ctx, dst, srcs[0])
	}

	// Now, handle multipart-copy cases.

	// 1. Ensure that the object has not been changed while
	//    we are copying data.
	for i, src := range srcs {
		src.MatchETag = srcObjectInfos[i].ETag
	}

	// 2. Initiate a new multipart upload.

	// Set user-metadata on the destination object. If no
	// user-metadata is specified, and there is only one source,
	// (only) then metadata from source is copied.
	var userMeta map[string]string
	if dst.ReplaceMetadata {
		userMeta = dst.UserMetadata
	} else {
		userMeta = srcObjectInfos[0].UserMetadata
	}

	var userTags map[string]string
	if dst.ReplaceTags {
		userTags = dst.UserTags
	} else {
		userTags = srcObjectInfos[0].UserTags
	}

	uploadID, err := c.newUploadID(ctx, dst.Bucket, dst.Object, PutObjectOptions{
		ServerSideEncryption: dst.Encryption,
		UserMetadata:         userMeta,
		UserTags:             userTags,
		Mode:                 dst.Mode,
		RetainUntilDate:      dst.RetainUntilDate,
		LegalHold:            dst.LegalHold,
	})
	if err != nil {
		return UploadInfo{}, err
	}

	// 3. Perform copy part uploads
	objParts := []CompletePart{}
	partIndex := 1
	for i, src := range srcs {
		var h = make(http.Header)
		src.Marshal(h)
		if dst.Encryption != nil && dst.Encryption.Type() == encrypt.SSEC {
			dst.Encryption.Marshal(h)
		}

		// calculate start/end indices of parts after
		// splitting.
		startIdx, endIdx := calculateEvenSplits(srcObjectSizes[i], src)
		for j, start := range startIdx {
			end := endIdx[j]

			// Add (or reset) source range header for
			// upload part copy request.
			h.Set("x-amz-copy-source-range",
				fmt.Sprintf("bytes=%d-%d", start, end))

			// make upload-part-copy request
			complPart, err := c.uploadPartCopy(ctx, dst.Bucket,
				dst.Object, uploadID, partIndex, h)
			if err != nil {
				return UploadInfo{}, err
			}
			if dst.Progress != nil {
				io.CopyN(ioutil.Discard, dst.Progress, end-start+1)
			}
			objParts = append(objParts, complPart)
			partIndex++
		}
	}

	// 4. Make final complete-multipart request.
	uploadInfo, err := c.completeMultipartUpload(ctx, dst.Bucket, dst.Object, uploadID,
		completeMultipartUpload{Parts: objParts}, PutObjectOptions{})
	if err != nil {
		return UploadInfo{}, err
	}

	uploadInfo.Size = totalSize
	return uploadInfo, nil
}

// partsRequired is maximum parts possible with
// max part size of ceiling(maxMultipartPutObjectSize / (maxPartsCount - 1))
func partsRequired(size int64) int64 {
	maxPartSize := maxMultipartPutObjectSize / (maxPartsCount - 1)
	r := size / int64(maxPartSize)
	if size%int64(maxPartSize) > 0 {
		r++
	}
	return r
}

// calculateEvenSplits - computes splits for a source and returns
// start and end index slices. Splits happen evenly to be sure that no
// part is less than 5MiB, as that could fail the multipart request if
// it is not the last part.
func calculateEvenSplits(size int64, src CopySrcOptions) (startIndex, endIndex []int64) {
	if size == 0 {
		return
	}

	reqParts := partsRequired(size)
	startIndex = make([]int64, reqParts)
	endIndex = make([]int64, reqParts)
	// Compute number of required parts `k`, as:
	//
	// k = ceiling(size / copyPartSize)
	//
	// Now, distribute the `size` bytes in the source into
	// k parts as evenly as possible:
	//
	// r parts sized (q+1) bytes, and
	// (k - r) parts sized q bytes, where
	//
	// size = q * k + r (by simple division of size by k,
	// so that 0 <= r < k)
	//
	start := src.Start
	if start == -1 {
		start = 0
	}
	quot, rem := size/reqParts, size%reqParts
	nextStart := start
	for j := int64(0); j < reqParts; j++ {
		curPartSize := quot
		if j < rem {
			curPartSize++
		}

		cStart := nextStart
		cEnd := cStart + curPartSize - 1
		nextStart = cEnd + 1

		startIndex[j], endIndex[j] = cStart, cEnd
	}
	return
}
