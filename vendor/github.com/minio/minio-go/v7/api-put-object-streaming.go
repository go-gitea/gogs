/*
 * MinIO Go Library for Amazon S3 Compatible Cloud Storage
 * Copyright 2017 MinIO, Inc.
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
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7/pkg/s3utils"
)

// putObjectMultipartStream - upload a large object using
// multipart upload and streaming signature for signing payload.
// Comprehensive put object operation involving multipart uploads.
//
// Following code handles these types of readers.
//
//  - *minio.Object
//  - Any reader which has a method 'ReadAt()'
//
func (c Client) putObjectMultipartStream(ctx context.Context, bucketName, objectName string,
	reader io.Reader, size int64, opts PutObjectOptions) (info UploadInfo, err error) {

	if !isObject(reader) && isReadAt(reader) && !opts.SendContentMd5 {
		// Verify if the reader implements ReadAt and it is not a *minio.Object then we will use parallel uploader.
		info, err = c.putObjectMultipartStreamFromReadAt(ctx, bucketName, objectName, reader.(io.ReaderAt), size, opts)
	} else {
		info, err = c.putObjectMultipartStreamOptionalChecksum(ctx, bucketName, objectName, reader, size, opts)
	}
	if err != nil {
		errResp := ToErrorResponse(err)
		// Verify if multipart functionality is not available, if not
		// fall back to single PutObject operation.
		if errResp.Code == "AccessDenied" && strings.Contains(errResp.Message, "Access Denied") {
			// Verify if size of reader is greater than '5GiB'.
			if size > maxSinglePutObjectSize {
				return UploadInfo{}, errEntityTooLarge(size, maxSinglePutObjectSize, bucketName, objectName)
			}
			// Fall back to uploading as single PutObject operation.
			return c.putObject(ctx, bucketName, objectName, reader, size, opts)
		}
	}
	return info, err
}

// uploadedPartRes - the response received from a part upload.
type uploadedPartRes struct {
	Error   error // Any error encountered while uploading the part.
	PartNum int   // Number of the part uploaded.
	Size    int64 // Size of the part uploaded.
	Part    ObjectPart
}

type uploadPartReq struct {
	PartNum int        // Number of the part uploaded.
	Part    ObjectPart // Size of the part uploaded.
}

// putObjectMultipartFromReadAt - Uploads files bigger than 128MiB.
// Supports all readers which implements io.ReaderAt interface
// (ReadAt method).
//
// NOTE: This function is meant to be used for all readers which
// implement io.ReaderAt which allows us for resuming multipart
// uploads but reading at an offset, which would avoid re-read the
// data which was already uploaded. Internally this function uses
// temporary files for staging all the data, these temporary files are
// cleaned automatically when the caller i.e http client closes the
// stream after uploading all the contents successfully.
func (c Client) putObjectMultipartStreamFromReadAt(ctx context.Context, bucketName, objectName string,
	reader io.ReaderAt, size int64, opts PutObjectOptions) (info UploadInfo, err error) {
	// Input validation.
	if err = s3utils.CheckValidBucketName(bucketName); err != nil {
		return UploadInfo{}, err
	}
	if err = s3utils.CheckValidObjectName(objectName); err != nil {
		return UploadInfo{}, err
	}

	// Calculate the optimal parts info for a given size.
	totalPartsCount, partSize, lastPartSize, err := OptimalPartInfo(size, opts.PartSize)
	if err != nil {
		return UploadInfo{}, err
	}

	// Initiate a new multipart upload.
	uploadID, err := c.newUploadID(ctx, bucketName, objectName, opts)
	if err != nil {
		return UploadInfo{}, err
	}

	// Aborts the multipart upload in progress, if the
	// function returns any error, since we do not resume
	// we should purge the parts which have been uploaded
	// to relinquish storage space.
	defer func() {
		if err != nil {
			c.abortMultipartUpload(ctx, bucketName, objectName, uploadID)
		}
	}()

	// Total data read and written to server. should be equal to 'size' at the end of the call.
	var totalUploadedSize int64

	// Complete multipart upload.
	var complMultipartUpload completeMultipartUpload

	// Declare a channel that sends the next part number to be uploaded.
	// Buffered to 10000 because thats the maximum number of parts allowed
	// by S3.
	uploadPartsCh := make(chan uploadPartReq, 10000)

	// Declare a channel that sends back the response of a part upload.
	// Buffered to 10000 because thats the maximum number of parts allowed
	// by S3.
	uploadedPartsCh := make(chan uploadedPartRes, 10000)

	// Used for readability, lastPartNumber is always totalPartsCount.
	lastPartNumber := totalPartsCount

	// Send each part number to the channel to be processed.
	for p := 1; p <= totalPartsCount; p++ {
		uploadPartsCh <- uploadPartReq{PartNum: p}
	}
	close(uploadPartsCh)

	var partsBuf = make([][]byte, opts.getNumThreads())
	for i := range partsBuf {
		partsBuf[i] = make([]byte, 0, partSize)
	}

	// Receive each part number from the channel allowing three parallel uploads.
	for w := 1; w <= opts.getNumThreads(); w++ {
		go func(w int, partSize int64) {
			// Each worker will draw from the part channel and upload in parallel.
			for uploadReq := range uploadPartsCh {

				// If partNumber was not uploaded we calculate the missing
				// part offset and size. For all other part numbers we
				// calculate offset based on multiples of partSize.
				readOffset := int64(uploadReq.PartNum-1) * partSize

				// As a special case if partNumber is lastPartNumber, we
				// calculate the offset based on the last part size.
				if uploadReq.PartNum == lastPartNumber {
					readOffset = (size - lastPartSize)
					partSize = lastPartSize
				}

				n, rerr := readFull(io.NewSectionReader(reader, readOffset, partSize), partsBuf[w-1][:partSize])
				if rerr != nil && rerr != io.ErrUnexpectedEOF && err != io.EOF {
					uploadedPartsCh <- uploadedPartRes{
						Error: rerr,
					}
					// Exit the goroutine.
					return
				}

				// Get a section reader on a particular offset.
				hookReader := newHook(bytes.NewReader(partsBuf[w-1][:n]), opts.Progress)

				// Proceed to upload the part.
				objPart, err := c.uploadPart(ctx, bucketName, objectName,
					uploadID, hookReader, uploadReq.PartNum,
					"", "", partSize, opts.ServerSideEncryption)
				if err != nil {
					uploadedPartsCh <- uploadedPartRes{
						Error: err,
					}
					// Exit the goroutine.
					return
				}

				// Save successfully uploaded part metadata.
				uploadReq.Part = objPart

				// Send successful part info through the channel.
				uploadedPartsCh <- uploadedPartRes{
					Size:    objPart.Size,
					PartNum: uploadReq.PartNum,
					Part:    uploadReq.Part,
				}
			}
		}(w, partSize)
	}

	// Gather the responses as they occur and update any
	// progress bar.
	for u := 1; u <= totalPartsCount; u++ {
		uploadRes := <-uploadedPartsCh
		if uploadRes.Error != nil {
			return UploadInfo{}, uploadRes.Error
		}
		// Update the totalUploadedSize.
		totalUploadedSize += uploadRes.Size
		// Store the parts to be completed in order.
		complMultipartUpload.Parts = append(complMultipartUpload.Parts, CompletePart{
			ETag:       uploadRes.Part.ETag,
			PartNumber: uploadRes.Part.PartNumber,
		})
	}

	// Verify if we uploaded all the data.
	if totalUploadedSize != size {
		return UploadInfo{}, errUnexpectedEOF(totalUploadedSize, size, bucketName, objectName)
	}

	// Sort all completed parts.
	sort.Sort(completedParts(complMultipartUpload.Parts))

	uploadInfo, err := c.completeMultipartUpload(ctx, bucketName, objectName, uploadID, complMultipartUpload, PutObjectOptions{})
	if err != nil {
		return UploadInfo{}, err
	}

	uploadInfo.Size = totalUploadedSize
	return uploadInfo, nil
}

func (c Client) putObjectMultipartStreamOptionalChecksum(ctx context.Context, bucketName, objectName string,
	reader io.Reader, size int64, opts PutObjectOptions) (info UploadInfo, err error) {
	// Input validation.
	if err = s3utils.CheckValidBucketName(bucketName); err != nil {
		return UploadInfo{}, err
	}
	if err = s3utils.CheckValidObjectName(objectName); err != nil {
		return UploadInfo{}, err
	}

	// Calculate the optimal parts info for a given size.
	totalPartsCount, partSize, lastPartSize, err := OptimalPartInfo(size, opts.PartSize)
	if err != nil {
		return UploadInfo{}, err
	}
	// Initiates a new multipart request
	uploadID, err := c.newUploadID(ctx, bucketName, objectName, opts)
	if err != nil {
		return UploadInfo{}, err
	}

	// Aborts the multipart upload if the function returns
	// any error, since we do not resume we should purge
	// the parts which have been uploaded to relinquish
	// storage space.
	defer func() {
		if err != nil {
			c.abortMultipartUpload(ctx, bucketName, objectName, uploadID)
		}
	}()

	// Total data read and written to server. should be equal to 'size' at the end of the call.
	var totalUploadedSize int64

	// Initialize parts uploaded map.
	partsInfo := make(map[int]ObjectPart)

	// Create a buffer.
	buf := make([]byte, partSize)

	// Avoid declaring variables in the for loop
	var md5Base64 string
	var hookReader io.Reader

	// Part number always starts with '1'.
	var partNumber int
	for partNumber = 1; partNumber <= totalPartsCount; partNumber++ {

		// Proceed to upload the part.
		if partNumber == totalPartsCount {
			partSize = lastPartSize
		}

		if opts.SendContentMd5 {
			length, rerr := readFull(reader, buf)
			if rerr == io.EOF && partNumber > 1 {
				break
			}

			if rerr != nil && rerr != io.ErrUnexpectedEOF && err != io.EOF {
				return UploadInfo{}, rerr
			}

			// Calculate md5sum.
			hash := c.md5Hasher()
			hash.Write(buf[:length])
			md5Base64 = base64.StdEncoding.EncodeToString(hash.Sum(nil))
			hash.Close()

			// Update progress reader appropriately to the latest offset
			// as we read from the source.
			hookReader = newHook(bytes.NewReader(buf[:length]), opts.Progress)
		} else {
			// Update progress reader appropriately to the latest offset
			// as we read from the source.
			hookReader = newHook(reader, opts.Progress)
		}

		objPart, uerr := c.uploadPart(ctx, bucketName, objectName, uploadID,
			io.LimitReader(hookReader, partSize),
			partNumber, md5Base64, "", partSize, opts.ServerSideEncryption)
		if uerr != nil {
			return UploadInfo{}, uerr
		}

		// Save successfully uploaded part metadata.
		partsInfo[partNumber] = objPart

		// Save successfully uploaded size.
		totalUploadedSize += partSize
	}

	// Verify if we uploaded all the data.
	if size > 0 {
		if totalUploadedSize != size {
			return UploadInfo{}, errUnexpectedEOF(totalUploadedSize, size, bucketName, objectName)
		}
	}

	// Complete multipart upload.
	var complMultipartUpload completeMultipartUpload

	// Loop over total uploaded parts to save them in
	// Parts array before completing the multipart request.
	for i := 1; i < partNumber; i++ {
		part, ok := partsInfo[i]
		if !ok {
			return UploadInfo{}, errInvalidArgument(fmt.Sprintf("Missing part number %d", i))
		}
		complMultipartUpload.Parts = append(complMultipartUpload.Parts, CompletePart{
			ETag:       part.ETag,
			PartNumber: part.PartNumber,
		})
	}

	// Sort all completed parts.
	sort.Sort(completedParts(complMultipartUpload.Parts))

	uploadInfo, err := c.completeMultipartUpload(ctx, bucketName, objectName, uploadID, complMultipartUpload, PutObjectOptions{})
	if err != nil {
		return UploadInfo{}, err
	}

	uploadInfo.Size = totalUploadedSize
	return uploadInfo, nil
}

// putObject special function used Google Cloud Storage. This special function
// is used for Google Cloud Storage since Google's multipart API is not S3 compatible.
func (c Client) putObject(ctx context.Context, bucketName, objectName string, reader io.Reader, size int64, opts PutObjectOptions) (info UploadInfo, err error) {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return UploadInfo{}, err
	}
	if err := s3utils.CheckValidObjectName(objectName); err != nil {
		return UploadInfo{}, err
	}

	// Size -1 is only supported on Google Cloud Storage, we error
	// out in all other situations.
	if size < 0 && !s3utils.IsGoogleEndpoint(*c.endpointURL) {
		return UploadInfo{}, errEntityTooSmall(size, bucketName, objectName)
	}

	if opts.SendContentMd5 && s3utils.IsGoogleEndpoint(*c.endpointURL) && size < 0 {
		return UploadInfo{}, errInvalidArgument("MD5Sum cannot be calculated with size '-1'")
	}

	if size > 0 {
		if isReadAt(reader) && !isObject(reader) {
			seeker, ok := reader.(io.Seeker)
			if ok {
				offset, err := seeker.Seek(0, io.SeekCurrent)
				if err != nil {
					return UploadInfo{}, errInvalidArgument(err.Error())
				}
				reader = io.NewSectionReader(reader.(io.ReaderAt), offset, size)
			}
		}
	}

	var md5Base64 string
	if opts.SendContentMd5 {
		// Create a buffer.
		buf := make([]byte, size)

		length, rErr := readFull(reader, buf)
		if rErr != nil && rErr != io.ErrUnexpectedEOF && rErr != io.EOF {
			return UploadInfo{}, rErr
		}

		// Calculate md5sum.
		hash := c.md5Hasher()
		hash.Write(buf[:length])
		md5Base64 = base64.StdEncoding.EncodeToString(hash.Sum(nil))
		reader = bytes.NewReader(buf[:length])
		hash.Close()
	}

	// Update progress reader appropriately to the latest offset as we
	// read from the source.
	readSeeker := newHook(reader, opts.Progress)

	// This function does not calculate sha256 and md5sum for payload.
	// Execute put object.
	return c.putObjectDo(ctx, bucketName, objectName, readSeeker, md5Base64, "", size, opts)
}

// putObjectDo - executes the put object http operation.
// NOTE: You must have WRITE permissions on a bucket to add an object to it.
func (c Client) putObjectDo(ctx context.Context, bucketName, objectName string, reader io.Reader, md5Base64, sha256Hex string, size int64, opts PutObjectOptions) (UploadInfo, error) {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return UploadInfo{}, err
	}
	if err := s3utils.CheckValidObjectName(objectName); err != nil {
		return UploadInfo{}, err
	}
	// Set headers.
	customHeader := opts.Header()

	// Populate request metadata.
	reqMetadata := requestMetadata{
		bucketName:       bucketName,
		objectName:       objectName,
		customHeader:     customHeader,
		contentBody:      reader,
		contentLength:    size,
		contentMD5Base64: md5Base64,
		contentSHA256Hex: sha256Hex,
	}
	if opts.Internal.SourceVersionID != "" {
		if _, err := uuid.Parse(opts.Internal.SourceVersionID); err != nil {
			return UploadInfo{}, errInvalidArgument(err.Error())
		}
		urlValues := make(url.Values)
		urlValues.Set("versionId", opts.Internal.SourceVersionID)
		reqMetadata.queryValues = urlValues
	}

	// Execute PUT an objectName.
	resp, err := c.executeMethod(ctx, http.MethodPut, reqMetadata)
	defer closeResponse(resp)
	if err != nil {
		return UploadInfo{}, err
	}
	if resp != nil {
		if resp.StatusCode != http.StatusOK {
			return UploadInfo{}, httpRespToErrorResponse(resp, bucketName, objectName)
		}
	}

	// extract lifecycle expiry date and rule ID
	expTime, ruleID := amzExpirationToExpiryDateRuleID(resp.Header.Get(amzExpiration))

	return UploadInfo{
		Bucket:           bucketName,
		Key:              objectName,
		ETag:             trimEtag(resp.Header.Get("ETag")),
		VersionID:        resp.Header.Get(amzVersionID),
		Size:             size,
		Expiration:       expTime,
		ExpirationRuleID: ruleID,
	}, nil
}
