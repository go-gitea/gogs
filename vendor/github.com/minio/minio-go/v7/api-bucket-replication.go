/*
 * MinIO Go Library for Amazon S3 Compatible Cloud Storage
 * Copyright 2020 MinIO, Inc.
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
	"encoding/json"
	"encoding/xml"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7/pkg/replication"
	"github.com/minio/minio-go/v7/pkg/s3utils"
)

// RemoveBucketReplication removes a replication config on an existing bucket.
func (c Client) RemoveBucketReplication(ctx context.Context, bucketName string) error {
	return c.removeBucketReplication(ctx, bucketName)
}

// SetBucketReplication sets a replication config on an existing bucket.
func (c Client) SetBucketReplication(ctx context.Context, bucketName string, cfg replication.Config) error {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return err
	}

	// If replication is empty then delete it.
	if cfg.Empty() {
		return c.removeBucketReplication(ctx, bucketName)
	}
	// Save the updated replication.
	return c.putBucketReplication(ctx, bucketName, cfg)
}

// Saves a new bucket replication.
func (c Client) putBucketReplication(ctx context.Context, bucketName string, cfg replication.Config) error {
	// Get resources properly escaped and lined up before
	// using them in http request.
	urlValues := make(url.Values)
	urlValues.Set("replication", "")
	replication, err := xml.Marshal(cfg)
	if err != nil {
		return err
	}

	reqMetadata := requestMetadata{
		bucketName:       bucketName,
		queryValues:      urlValues,
		contentBody:      bytes.NewReader(replication),
		contentLength:    int64(len(replication)),
		contentMD5Base64: sumMD5Base64(replication),
	}

	// Execute PUT to upload a new bucket replication config.
	resp, err := c.executeMethod(ctx, http.MethodPut, reqMetadata)
	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp, bucketName, "")
	}

	return nil
}

// Remove replication from a bucket.
func (c Client) removeBucketReplication(ctx context.Context, bucketName string) error {
	// Get resources properly escaped and lined up before
	// using them in http request.
	urlValues := make(url.Values)
	urlValues.Set("replication", "")

	// Execute DELETE on objectName.
	resp, err := c.executeMethod(ctx, http.MethodDelete, requestMetadata{
		bucketName:       bucketName,
		queryValues:      urlValues,
		contentSHA256Hex: emptySHA256Hex,
	})
	defer closeResponse(resp)
	if err != nil {
		return err
	}
	return nil
}

// GetBucketReplication fetches bucket replication configuration.If config is not
// found, returns empty config with nil error.
func (c Client) GetBucketReplication(ctx context.Context, bucketName string) (cfg replication.Config, err error) {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return cfg, err
	}
	bucketReplicationCfg, err := c.getBucketReplication(ctx, bucketName)
	if err != nil {
		errResponse := ToErrorResponse(err)
		if errResponse.Code == "ReplicationConfigurationNotFoundError" {
			return cfg, nil
		}
		return cfg, err
	}
	return bucketReplicationCfg, nil
}

// Request server for current bucket replication config.
func (c Client) getBucketReplication(ctx context.Context, bucketName string) (cfg replication.Config, err error) {
	// Get resources properly escaped and lined up before
	// using them in http request.
	urlValues := make(url.Values)
	urlValues.Set("replication", "")

	// Execute GET on bucket to get replication config.
	resp, err := c.executeMethod(ctx, http.MethodGet, requestMetadata{
		bucketName:  bucketName,
		queryValues: urlValues,
	})

	defer closeResponse(resp)
	if err != nil {
		return cfg, err
	}

	if resp.StatusCode != http.StatusOK {
		return cfg, httpRespToErrorResponse(resp, bucketName, "")
	}

	if err = xmlDecoder(resp.Body, &cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// GetBucketReplicationMetrics fetches bucket replication status metrics
func (c Client) GetBucketReplicationMetrics(ctx context.Context, bucketName string) (s replication.Metrics, err error) {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return s, err
	}
	// Get resources properly escaped and lined up before
	// using them in http request.
	urlValues := make(url.Values)
	urlValues.Set("replication-metrics", "")

	// Execute GET on bucket to get replication config.
	resp, err := c.executeMethod(ctx, http.MethodGet, requestMetadata{
		bucketName:  bucketName,
		queryValues: urlValues,
	})

	defer closeResponse(resp)
	if err != nil {
		return s, err
	}

	if resp.StatusCode != http.StatusOK {
		return s, httpRespToErrorResponse(resp, bucketName, "")
	}
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return s, err
	}

	if err := json.Unmarshal(respBytes, &s); err != nil {
		return s, err
	}
	return s, nil
}

// ResetBucketReplication kicks off replication of previously replicated objects if ExistingObjectReplication
// is enabled in the replication config
func (c Client) ResetBucketReplication(ctx context.Context, bucketName string, olderThan time.Duration) (resetID string, err error) {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return "", err
	}
	// Get resources properly escaped and lined up before
	// using them in http request.
	urlValues := make(url.Values)
	urlValues.Set("replication-reset", "")
	if olderThan > 0 {
		urlValues.Set("older-than", olderThan.String())
	}

	// Execute GET on bucket to get replication config.
	resp, err := c.executeMethod(ctx, http.MethodPut, requestMetadata{
		bucketName:  bucketName,
		queryValues: urlValues,
	})

	defer closeResponse(resp)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", httpRespToErrorResponse(resp, bucketName, "")
	}
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if err := json.Unmarshal(respBytes, &resetID); err != nil {
		return "", err
	}
	return resetID, nil
}
