// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
)

// ToComment converts a models.Comment to the api.Comment format
func ToComment(c *models.Comment) *api.Comment {
	assets := make([]*api.Attachment, 0, len(c.Attachments))
	for _, att := range c.Attachments {
		assets = append(assets, ToCommentAttachment(att))
	}
	return &api.Comment{
		ID:          c.ID,
		Poster:      ToUser(c.Poster, nil),
		HTMLURL:     c.HTMLURL(),
		IssueURL:    c.IssueURL(),
		PRURL:       c.PRURL(),
		Body:        c.Content,
		Attachments: assets,
		Created:     c.CreatedUnix.AsTime(),
		Updated:     c.UpdatedUnix.AsTime(),
	}
}

// ToCommentAttachment converts models.Attachment to api.Attachment
func ToCommentAttachment(a *models.Attachment) *api.Attachment {
	return &api.Attachment{
		ID:            a.ID,
		Name:          a.Name,
		Created:       a.CreatedUnix.AsTime(),
		DownloadCount: a.DownloadCount,
		Size:          a.Size,
		UUID:          a.UUID,
		DownloadURL:   a.DownloadURL(),
	}
}
