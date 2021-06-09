// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package recaptcha

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/goccy/go-json"
)

// Response is the structure of JSON returned from API
type Response struct {
	Success     bool        `json:"success"`
	ChallengeTS string      `json:"challenge_ts"`
	Hostname    string      `json:"hostname"`
	ErrorCodes  []ErrorCode `json:"error-codes"`
}

const apiURL = "api/siteverify"

// Verify calls Google Recaptcha API to verify token
func Verify(ctx context.Context, response string) (bool, error) {
	post := url.Values{
		"secret":   {setting.Service.RecaptchaSecret},
		"response": {response},
	}
	// Basically a copy of http.PostForm, but with a context
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		util.URLJoin(setting.Service.RecaptchaURL, apiURL), strings.NewReader(post.Encode()))
	if err != nil {
		return false, fmt.Errorf("Failed to create CAPTCHA request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("Failed to send CAPTCHA response: %s", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("Failed to read CAPTCHA response: %s", err)
	}
	var jsonResponse Response
	err = json.Unmarshal(body, &jsonResponse)
	if err != nil {
		return false, fmt.Errorf("Failed to parse CAPTCHA response: %s", err)
	}
	var respErr error
	if len(jsonResponse.ErrorCodes) > 0 {
		respErr = jsonResponse.ErrorCodes[0]
	}
	return jsonResponse.Success, respErr
}

// ErrorCode is a reCaptcha error
type ErrorCode string

// String fulfills the Stringer interface
func (e ErrorCode) String() string {
	switch e {
	case "missing-input-secret":
		return "The secret parameter is missing."
	case "invalid-input-secret":
		return "The secret parameter is invalid or malformed."
	case "missing-input-response":
		return "The response parameter is missing."
	case "invalid-input-response":
		return "The response parameter is invalid or malformed."
	case "bad-request":
		return "The request is invalid or malformed."
	case "timeout-or-duplicate":
		return "The response is no longer valid: either is too old or has been used previously."
	}
	return string(e)
}

// Error fulfills the error interface
func (e ErrorCode) Error() string {
	return e.String()
}
