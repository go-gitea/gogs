// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
)

// ToNotificationThread convert a Notification to api.NotificationThread
func ToNotificationThread(n *models.Notification) *api.NotificationThread {
	result := &api.NotificationThread{
		ID:        n.ID,
		Unread:    !(n.Status == models.NotificationStatusRead || n.Status == models.NotificationStatusPinned),
		Pinned:    n.Status == models.NotificationStatusPinned,
		UpdatedAt: n.UpdatedUnix.AsTime(),
		URL:       n.APIURL(),
	}

	//since user only get notifications when he has access to use minimal access mode
	if n.Repository != nil {
		result.Repository = ToRepo(n.Repository, models.AccessModeRead)
	}

	//handle Subject
	switch n.Source {
	case models.NotificationSourceIssue:
		result.Subject = &api.NotificationSubject{Type: api.NotifySubjectIssue}
		if n.Issue != nil {
			result.Subject.Title = n.Issue.Title
			result.Subject.URL = n.Issue.APIURL()
			result.Subject.State = n.Issue.State()
			comment, err := n.Issue.GetLastComment()
			if err == nil && comment != nil {
				result.Subject.LatestCommentURL = comment.APIURL()
			}
		}
	case models.NotificationSourcePullRequest:
		result.Subject = &api.NotificationSubject{Type: api.NotifySubjectPull}
		if n.Issue != nil {
			result.Subject.Title = n.Issue.Title
			result.Subject.URL = n.Issue.APIURL()
			result.Subject.State = n.Issue.State()
			comment, err := n.Issue.GetLastComment()
			if err == nil && comment != nil {
				result.Subject.LatestCommentURL = comment.APIURL()
			}

			pr, _ := n.Issue.GetPullRequest()
			if pr != nil && pr.HasMerged {
				result.Subject.State = "merged"
			}
		}
	case models.NotificationSourceCommit:
		result.Subject = &api.NotificationSubject{
			Type:  api.NotifySubjectCommit,
			Title: n.CommitID,
			URL:   n.Repository.HTMLURL() + "/commit/" + n.CommitID,
		}
	case models.NotificationSourceRepository:
		result.Subject = &api.NotificationSubject{
			Type:  api.NotifySubjectRepository,
			Title: n.Repository.FullName(),
			URL:   n.Repository.Link(),
		}
	}

	return result
}

// ToNotifications convert list of Notification to api.NotificationThread list
func ToNotifications(nl models.NotificationList) []*api.NotificationThread {
	var result = make([]*api.NotificationThread, 0, len(nl))
	for _, n := range nl {
		result = append(result, ToNotificationThread(n))
	}
	return result
}
