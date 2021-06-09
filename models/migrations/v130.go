// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/setting"

	"github.com/goccy/go-json"
	"xorm.io/xorm"
)

func expandWebhooks(x *xorm.Engine) error {
	type HookEvents struct {
		Create               bool `json:"create"`
		Delete               bool `json:"delete"`
		Fork                 bool `json:"fork"`
		Issues               bool `json:"issues"`
		IssueAssign          bool `json:"issue_assign"`
		IssueLabel           bool `json:"issue_label"`
		IssueMilestone       bool `json:"issue_milestone"`
		IssueComment         bool `json:"issue_comment"`
		Push                 bool `json:"push"`
		PullRequest          bool `json:"pull_request"`
		PullRequestAssign    bool `json:"pull_request_assign"`
		PullRequestLabel     bool `json:"pull_request_label"`
		PullRequestMilestone bool `json:"pull_request_milestone"`
		PullRequestComment   bool `json:"pull_request_comment"`
		PullRequestReview    bool `json:"pull_request_review"`
		PullRequestSync      bool `json:"pull_request_sync"`
		Repository           bool `json:"repository"`
		Release              bool `json:"release"`
	}

	type HookEvent struct {
		PushOnly       bool   `json:"push_only"`
		SendEverything bool   `json:"send_everything"`
		ChooseEvents   bool   `json:"choose_events"`
		BranchFilter   string `json:"branch_filter"`

		HookEvents `json:"events"`
	}

	type Webhook struct {
		ID     int64
		Events string
	}

	var bytes []byte
	var last int
	batchSize := setting.Database.IterateBufferSize
	sess := x.NewSession()
	defer sess.Close()
	for {
		if err := sess.Begin(); err != nil {
			return err
		}
		results := make([]Webhook, 0, batchSize)
		err := x.OrderBy("id").
			Limit(batchSize, last).
			Find(&results)
		if err != nil {
			return err
		}
		if len(results) == 0 {
			break
		}
		last += len(results)

		for _, res := range results {
			var events HookEvent
			if err = json.Unmarshal([]byte(res.Events), &events); err != nil {
				return err
			}

			if !events.ChooseEvents {
				continue
			}

			if events.Issues {
				events.IssueAssign = true
				events.IssueLabel = true
				events.IssueMilestone = true
				events.IssueComment = true
			}

			if events.PullRequest {
				events.PullRequestAssign = true
				events.PullRequestLabel = true
				events.PullRequestMilestone = true
				events.PullRequestComment = true
				events.PullRequestReview = true
				events.PullRequestSync = true
			}

			if bytes, err = json.Marshal(&events); err != nil {
				return err
			}

			_, err = sess.Exec("UPDATE webhook SET events = ? WHERE id = ?", string(bytes), res.ID)
			if err != nil {
				return err
			}
		}

		if err := sess.Commit(); err != nil {
			return err
		}
	}
	return nil
}
