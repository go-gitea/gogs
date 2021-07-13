/*
 * MinIO Go Library for Amazon S3 Compatible Cloud Storage
 * Copyright 2015-2017 MinIO, Inc.
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
	"io"
	"net/http"

	"github.com/minio/minio-go/v7/pkg/encrypt"
)

// Core - Inherits Client and adds new methods to expose the low level S3 APIs.
type Core struct {
	*Client
}

// NewCore - Returns new initialized a Core client, this CoreClient should be
// only used under special conditions such as need to access lower primitives
// and being able to use them to write your own wrappers.
func NewCore(endpoint string, opts *Options) (*Core, error) {
	var s3Client Core
	client, err := New(endpoint, opts)
	if err != nil {
		return nil, err
	}
	s3Client.Client = client
	return &s3Client, nil
}

// ListObjects - List all the objects at a prefix, optionally with marker and delimiter
// you can further filter the results.
func (c Core) ListObjects(bucket, prefix, marker, delimiter string, maxKeys int) (result ListBucketResult, err error) {
	return c.listObjectsQuery(context.Background(), bucket, prefix, marker, delimiter, maxKeys, nil)
}

// ListObjectsV2 - Lists all the objects at a prefix, similar to ListObjects() but uses
// continuationToken instead of marker to support iteration over the results.
func (c Core) ListObjectsV2(bucketName, objectPrefix, continuationToken string, fetchOwner bool, delimiter string, maxkeys int) (ListBucketV2Result, error) {
	return c.listObjectsV2Query(context.Background(), bucketName, objectPrefix, continuationToken, fetchOwner, false, delimiter, maxkeys, nil)
}

// CopyObject - copies an object from source object to destination object on server side.
func (c Core) CopyObject(ctx context.Context, sourceBucket, sourceObject, destBucket, destObject string, metadata map[string]string, srcOpts CopySrcOptions, dstOpts PutObjectOptions) (ObjectInfo, error) {
	return c.copyObjectDo(ctx, sourceBucket, sourceObject, destBucket, destObject, metadata, srcOpts, dstOpts)
}

// CopyObjectPart - creates a part in a multipart upload by copying (a
// part of) an existing object.
func (c Core) CopyObjectPart(ctx context.Context, srcBucket, srcObject, destBucket, destObject string, uploadID string,
	partID int, startOffset, length int64, metadata map[string]string) (p CompletePart, err error) {

	return c.copyObjectPartDo(ctx, srcBucket, srcObject, destBucket, destObject, uploadID,
		partID, startOffset, length, metadata)
}

// PutObject - Upload object. Uploads using single PUT call.
func (c Core) PutObject(ctx context.Context, bucket, object string, data io.Reader, size int64, md5Base64, sha256Hex string, opts PutObjectOptions) (UploadInfo, error) {
	hookReader := newHook(data, opts.Progress)
	return c.putObjectDo(ctx, bucket, object, hookReader, md5Base64, sha256Hex, size, opts)
}

// NewMultipartUpload - Initiates new multipart upload and returns the new uploadID.
func (c Core) NewMultipartUpload(ctx context.Context, bucket, object string, opts PutObjectOptions) (uploadID string, err error) {
	result, err := c.initiateMultipartUpload(ctx, bucket, object, opts)
	return result.UploadID, err
}

// ListMultipartUploads - List incomplete uploads.
func (c Core) ListMultipartUploads(ctx context.Context, bucket, prefix, keyMarker, uploadIDMarker, delimiter string, maxUploads int) (result ListMultipartUploadsResult, err error) {
	return c.listMultipartUploadsQuery(ctx, bucket, keyMarker, uploadIDMarker, prefix, delimiter, maxUploads)
}

// PutObjectPart - Upload an object part.
func (c Core) PutObjectPart(ctx context.Context, bucket, object, uploadID string, partID int, data io.Reader, size int64, md5Base64, sha256Hex string, sse encrypt.ServerSide) (ObjectPart, error) {
	return c.uploadPart(ctx, bucket, object, uploadID, data, partID, md5Base64, sha256Hex, size, sse)
}

// ListObjectParts - List uploaded parts of an incomplete upload.x
func (c Core) ListObjectParts(ctx context.Context, bucket, object, uploadID string, partNumberMarker int, maxParts int) (result ListObjectPartsResult, err error) {
	return c.listObjectPartsQuery(ctx, bucket, object, uploadID, partNumberMarker, maxParts)
}

// CompleteMultipartUpload - Concatenate uploaded parts and commit to an object.
func (c Core) CompleteMultipartUpload(ctx context.Context, bucket, object, uploadID string, parts []CompletePart, opts PutObjectOptions) (string, error) {
	res, err := c.completeMultipartUpload(ctx, bucket, object, uploadID, completeMultipartUpload{
		Parts: parts,
	}, opts)
	return res.ETag, err
}

// AbortMultipartUpload - Abort an incomplete upload.
func (c Core) AbortMultipartUpload(ctx context.Context, bucket, object, uploadID string) error {
	return c.abortMultipartUpload(ctx, bucket, object, uploadID)
}

// GetBucketPolicy - fetches bucket access policy for a given bucket.
func (c Core) GetBucketPolicy(ctx context.Context, bucket string) (string, error) {
	return c.getBucketPolicy(ctx, bucket)
}

// PutBucketPolicy - applies a new bucket access policy for a given bucket.
func (c Core) PutBucketPolicy(ctx context.Context, bucket, bucketPolicy string) error {
	return c.putBucketPolicy(ctx, bucket, bucketPolicy)
}

// GetObject is a lower level API implemented to support reading
// partial objects and also downloading objects with special conditions
// matching etag, modtime etc.
func (c Core) GetObject(ctx context.Context, bucketName, objectName string, opts GetObjectOptions) (io.ReadCloser, ObjectInfo, http.Header, error) {
	return c.getObject(ctx, bucketName, objectName, opts)
}

// StatObject is a lower level API implemented to support special
// conditions matching etag, modtime on a request.
func (c Core) StatObject(ctx context.Context, bucketName, objectName string, opts StatObjectOptions) (ObjectInfo, error) {
	return c.statObject(ctx, bucketName, objectName, opts)
}
