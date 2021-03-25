// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"bytes"
	"container/list"
	"fmt"
	"html"
	gotemplate "html/template"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/timeutil"
)

const (
	tplBlame base.TplName = "repo/home"
)

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

	commit, err := ctx.Repo.GitRepo.GetCommit(commitID)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound("Repo.GitRepo.GetCommit", err)
		} else {
			ctx.ServerError("Repo.GitRepo.GetCommit", err)
		}
		return
	}
	if len(commitID) != 40 {
		commitID = commit.ID.String()
	}

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

	// Show latest commit info of repository in table header,
	// or of directory if not in root directory.
	latestCommit := ctx.Repo.Commit
	if len(ctx.Repo.TreePath) > 0 {
		latestCommit, err = ctx.Repo.Commit.GetCommitByPath(ctx.Repo.TreePath)
		if err != nil {
			ctx.ServerError("GetCommitByPath", err)
			return
		}
	}
	ctx.Data["LatestCommit"] = latestCommit
	ctx.Data["LatestCommitVerification"] = models.ParseCommitWithSignature(latestCommit)
	ctx.Data["LatestCommitUser"] = models.ValidateCommitWithEmail(latestCommit)

	statuses, err := models.GetLatestCommitStatus(ctx.Repo.Repository.ID, ctx.Repo.Commit.ID.String(), models.ListOptions{})
	if err != nil {
		log.Error("GetLatestCommitStatus: %v", err)
	}

	// Get current entry user currently looking at.
	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(ctx.Repo.TreePath)
	if err != nil {
		ctx.NotFoundOrServerError("Repo.Commit.GetTreeEntryByPath", git.IsErrNotExist, err)
		return
	}

	blob := entry.Blob()

	ctx.Data["LatestCommitStatus"] = models.CalcCommitStatus(statuses)
	ctx.Data["LatestCommitStatuses"] = statuses

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

	blameReader, err := git.CreateBlameReader(ctx.Req.Context(), models.RepoPath(userName, repoName), commitID, fileName)
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

	commitNames := make(map[string]models.UserCommit)
	commits := list.New()

	for _, part := range blameParts {
		sha := part.Sha
		if _, ok := commitNames[sha]; ok {
			continue
		}

		commit, err := ctx.Repo.GitRepo.GetCommit(sha)
		if err != nil {
			if git.IsErrNotExist(err) {
				ctx.NotFound("Repo.GitRepo.GetCommit", err)
			} else {
				ctx.ServerError("Repo.GitRepo.GetCommit", err)
			}
			return
		}

		commits.PushBack(commit)

		commitNames[commit.ID.String()] = models.UserCommit{}
	}

	commits = models.ValidateCommitsWithEmails(commits)

	for e := commits.Front(); e != nil; e = e.Next() {
		c := e.Value.(models.UserCommit)

		commitNames[c.ID.String()] = c
	}

	// Get Topics of this repo
	renderRepoTopics(ctx)
	if ctx.Written() {
		return
	}

	renderBlame(ctx, blameParts, commitNames)

	ctx.HTML(200, tplBlame)
}

func renderBlame(ctx *context.Context, blameParts []git.BlamePart, commitNames map[string]models.UserCommit) {
	repoLink := ctx.Repo.RepoLink

	lines := make([]string, 0)

	var commitInfo bytes.Buffer
	var lineNumbers bytes.Buffer
	var codeLines bytes.Buffer

	i := 0
	for pi, part := range blameParts {
		for index, line := range part.Lines {
			i++
			lines = append(lines, line)

			attr := ""
			if len(part.Lines)-1 == index && len(blameParts)-1 != pi {
				attr = " bottom-line"
			}
			commit := commitNames[part.Sha]
			if index == 0 {
				// User avatar image
				commitSince := timeutil.TimeSinceUnix(timeutil.TimeStamp(commit.Author.When.Unix()), ctx.Data["Lang"].(string))

				var avatar string
				if commit.User != nil {
					avatar = string(templates.Avatar(commit.User, 18, "mr-3"))
				} else {
					avatar = string(templates.AvatarByEmail(commit.Author.Email, commit.Author.Name, 18, "mr-3"))
				}

				commitInfo.WriteString(fmt.Sprintf(`<div class="blame-info%s"><div class="blame-data"><div class="blame-avatar">%s</div><div class="blame-message"><a href="%s/commit/%s" title="%[5]s">%[5]s</a></div><div class="blame-time">%s</div></div></div>`, attr, avatar, repoLink, part.Sha, html.EscapeString(commit.CommitMessage), commitSince))
			} else {
				commitInfo.WriteString(fmt.Sprintf(`<div class="blame-info%s">&#8203;</div>`, attr))
			}

			// Line number
			if len(part.Lines)-1 == index && len(blameParts)-1 != pi {
				lineNumbers.WriteString(fmt.Sprintf(`<span id="L%d" data-line-number="%d" class="bottom-line"></span>`, i, i))
			} else {
				lineNumbers.WriteString(fmt.Sprintf(`<span id="L%d" data-line-number="%d"></span>`, i, i))
			}

			if i != len(lines)-1 {
				line += "\n"
			}
			fileName := fmt.Sprintf("%v", ctx.Data["FileName"])
			line = highlight.Code(fileName, line)
			line = `<code class="code-inner">` + line + `</code>`
			if len(part.Lines)-1 == index && len(blameParts)-1 != pi {
				codeLines.WriteString(fmt.Sprintf(`<li class="L%d bottom-line" rel="L%d">%s</li>`, i, i, line))
			} else {
				codeLines.WriteString(fmt.Sprintf(`<li class="L%d" rel="L%d">%s</li>`, i, i, line))
			}
		}
	}

	ctx.Data["BlameContent"] = gotemplate.HTML(codeLines.String())
	ctx.Data["BlameCommitInfo"] = gotemplate.HTML(commitInfo.String())
	ctx.Data["BlameLineNums"] = gotemplate.HTML(lineNumbers.String())
}
