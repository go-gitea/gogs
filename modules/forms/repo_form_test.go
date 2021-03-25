// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package forms

import (
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestSubmitReviewForm_IsEmpty(t *testing.T) {
	cases := []struct {
		form     SubmitReviewForm
		expected bool
	}{
		// Approved PR with a comment shouldn't count as empty
		{SubmitReviewForm{Type: "approve", Content: "Awesome"}, false},

		// Approved PR without a comment shouldn't count as empty
		{SubmitReviewForm{Type: "approve", Content: ""}, false},

		// Rejected PR without a comment should count as empty
		{SubmitReviewForm{Type: "reject", Content: ""}, true},

		// Rejected PR with a comment shouldn't count as empty
		{SubmitReviewForm{Type: "reject", Content: "Awesome"}, false},

		// Comment review on a PR with a comment shouldn't count as empty
		{SubmitReviewForm{Type: "comment", Content: "Awesome"}, false},

		// Comment review on a PR without a comment should count as empty
		{SubmitReviewForm{Type: "comment", Content: ""}, true},
	}

	for _, v := range cases {
		assert.Equal(t, v.expected, v.form.HasEmptyContent())
	}
}

func TestIssueLock_HasValidReason(t *testing.T) {
	// Init settings
	_ = setting.Repository

	cases := []struct {
		form     IssueLockForm
		expected bool
	}{
		{IssueLockForm{""}, true}, // an empty reason is accepted
		{IssueLockForm{"Off-topic"}, true},
		{IssueLockForm{"Too heated"}, true},
		{IssueLockForm{"Spam"}, true},
		{IssueLockForm{"Resolved"}, true},

		{IssueLockForm{"ZZZZ"}, false},
		{IssueLockForm{"I want to lock this issue"}, false},
	}

	for _, v := range cases {
		assert.Equal(t, v.expected, v.form.HasValidReason())
	}
}
