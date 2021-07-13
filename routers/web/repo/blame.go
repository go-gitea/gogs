// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"container/list"
	"fmt"
	"html"
	gotemplate "html/template"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/timeutil"
)

const (
	tplBlame base.TplName = "repo/home"
)

type blameRow struct {
	RowNumber      int
	Avatar         gotemplate.HTML
	RepoLink       string
	PartSha        string
	PreviousSha    string
	PreviousShaURL string
	IsFirstCommit  bool
	CommitURL      string
	CommitMessage  string
	CommitSince    gotemplate.HTML
	Code           gotemplate.HTML
}

// RefBlame render blame page
func RefBlame(ctx *context.Context) {
	fileName := ctx.Repo.TreePath
	if len(fileName) == 0 {
		ctx.NotFound("Blame FileName", nil)
		return
	}

	userName := ctx.Repo.Owner.Name
	repoName := ctx.Repo.Repository.Name
	commitID := ctx.Repo.CommitID

	branchLink := ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()
	treeLink := branchLink
	rawLink := ctx.Repo.RepoLink + "/raw/" + ctx.Repo.BranchNameSubURL()

	if len(ctx.Repo.TreePath) > 0 {
		treeLink += "/" + ctx.Repo.TreePath
	}

	var treeNames []string
	paths := make([]string, 0, 5)
	if len(ctx.Repo.TreePath) > 0 {
		treeNames = strings.Split(ctx.Repo.TreePath, "/")
		for i := range treeNames {
			paths = append(paths, strings.Join(treeNames[:i+1], "/"))
		}

		ctx.Data["HasParentPath"] = true
		if len(paths)-2 >= 0 {
			ctx.Data["ParentPath"] = "/" + paths[len(paths)-1]
		}
	}

	// Get current entry user currently looking at.
	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(ctx.Repo.TreePath)
	if err != nil {
		ctx.NotFoundOrServerError("Repo.Commit.GetTreeEntryByPath", git.IsErrNotExist, err)
		return
	}

	blob := entry.Blob()

	ctx.Data["Paths"] = paths
	ctx.Data["TreeLink"] = treeLink
	ctx.Data["TreeNames"] = treeNames
	ctx.Data["BranchLink"] = branchLink

	ctx.Data["RawFileLink"] = rawLink + "/" + ctx.Repo.TreePath
	ctx.Data["PageIsViewCode"] = true

	ctx.Data["IsBlame"] = true

	ctx.Data["FileSize"] = blob.Size()
	ctx.Data["FileName"] = blob.Name()

	ctx.Data["NumLines"], err = blob.GetBlobLineCount()
	if err != nil {
		ctx.NotFound("GetBlobLineCount", err)
		return
	}

	blameReader, err := git.CreateBlameReader(ctx, models.RepoPath(userName, repoName), commitID, fileName)
	if err != nil {
		ctx.NotFound("CreateBlameReader", err)
		return
	}
	defer blameReader.Close()

	blameParts := make([]git.BlamePart, 0)

	for {
		blamePart, err := blameReader.NextPart()
		if err != nil {
			ctx.NotFound("NextPart", err)
			return
		}
		if blamePart == nil {
			break
		}
		blameParts = append(blameParts, *blamePart)
	}

	// Get Topics of this repo
	renderRepoTopics(ctx)
	if ctx.Written() {
		return
	}

	commitNames, previousCommits := processBlameParts(ctx, blameParts)
	if ctx.Written() {
		return
	}

	renderBlame(ctx, blameParts, commitNames, previousCommits)

	ctx.HTML(http.StatusOK, tplBlame)
}

func processBlameParts(ctx *context.Context, blameParts []git.BlamePart) (map[string]models.UserCommit, map[string]string) {
	// store commit data by SHA to look up avatar info etc
	commitNames := make(map[string]models.UserCommit)
	// previousCommits contains links from SHA to parent SHA,
	// if parent also contains the current TreePath.
	previousCommits := make(map[string]string)
	// and as blameParts can reference the same commits multiple
	// times, we cache the lookup work locally
	commits := list.New()
	commitCache := map[string]*git.Commit{}
	commitCache[ctx.Repo.Commit.ID.String()] = ctx.Repo.Commit

	for _, part := range blameParts {
		sha := part.Sha
		if _, ok := commitNames[sha]; ok {
			continue
		}

		// find the blamePart commit, to look up parent & email address for avatars
		commit, ok := commitCache[sha]
		var err error
		if !ok {
			commit, err = ctx.Repo.GitRepo.GetCommit(sha)
			if err != nil {
				if git.IsErrNotExist(err) {
					ctx.NotFound("Repo.GitRepo.GetCommit", err)
				} else {
					ctx.ServerError("Repo.GitRepo.GetCommit", err)
				}
				return nil, nil
			}
			commitCache[sha] = commit
		}

		// find parent commit
		if commit.ParentCount() > 0 {
			psha := commit.Parents[0]
			previousCommit, ok := commitCache[psha.String()]
			if !ok {
				previousCommit, _ = commit.Parent(0)
				if previousCommit != nil {
					commitCache[psha.String()] = previousCommit
				}
			}
			// only store parent commit ONCE, if it has the file
			if previousCommit != nil {
				if haz1, _ := previousCommit.HasFile(ctx.Repo.TreePath); haz1 {
					previousCommits[commit.ID.String()] = previousCommit.ID.String()
				}
			}
		}

		commits.PushBack(commit)

		commitNames[commit.ID.String()] = models.UserCommit{}
	}

	// populate commit email addresses to later look up avatars.
	commits = models.ValidateCommitsWithEmails(commits)
	for e := commits.Front(); e != nil; e = e.Next() {
		c := e.Value.(models.UserCommit)
		commitNames[c.ID.String()] = c
	}

	return commitNames, previousCommits
}

func renderBlame(ctx *context.Context, blameParts []git.BlamePart, commitNames map[string]models.UserCommit, previousCommits map[string]string) {
	repoLink := ctx.Repo.RepoLink

	var lines = make([]string, 0)
	rows := make([]*blameRow, 0)

	var i = 0
	var commitCnt = 0
	for _, part := range blameParts {
		for index, line := range part.Lines {
			i++
			lines = append(lines, line)

			br := &blameRow{
				RowNumber: i,
			}

			commit := commitNames[part.Sha]
			previousSha := previousCommits[part.Sha]
			if index == 0 {
				// Count commit number
				commitCnt++

				// User avatar image
				commitSince := timeutil.TimeSinceUnix(timeutil.TimeStamp(commit.Author.When.Unix()), ctx.Data["Lang"].(string))

				var avatar string
				if commit.User != nil {
					avatar = string(templates.Avatar(commit.User, 18, "mr-3"))
				} else {
					avatar = string(templates.AvatarByEmail(commit.Author.Email, commit.Author.Name, 18, "mr-3"))
				}

				br.Avatar = gotemplate.HTML(avatar)
				br.RepoLink = repoLink
				br.PartSha = part.Sha
				br.PreviousSha = previousSha
				br.PreviousShaURL = fmt.Sprintf("%s/blame/commit/%s/%s", repoLink, previousSha, ctx.Repo.TreePath)
				br.CommitURL = fmt.Sprintf("%s/commit/%s", repoLink, part.Sha)
				br.CommitMessage = html.EscapeString(commit.CommitMessage)
				br.CommitSince = commitSince
			}

			if i != len(lines)-1 {
				line += "\n"
			}
			fileName := fmt.Sprintf("%v", ctx.Data["FileName"])
			line = highlight.Code(fileName, line)

			br.Code = gotemplate.HTML(line)
			rows = append(rows, br)
		}
	}

	ctx.Data["BlameRows"] = rows
	ctx.Data["CommitCnt"] = commitCnt
}
