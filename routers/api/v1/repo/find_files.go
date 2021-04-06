// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"

	"github.com/go-enry/go-enry/v2"
)

var excludedDirs = []string{
	"vendor",
	"build",
	"log",
	"tmp",
}

// GetRepoFiles get all files' entries of a repository
func GetRepoFiles(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/find/{branchname} repository repoFindFiles
	// ---
	// summary: Get all files' entries of a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: branchname
	//   in: path
	//   description: branch name of the repo
	//   type: string
	//   required: true
	// responses:
	//   200:
	//     description: success

	tree, err := ctx.Repo.Commit.SubTree(ctx.Repo.TreePath)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Repo.Commit.SubTree", err)
		return
	}

	entries, err := tree.ListEntriesRecursive()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ListEntriesRecursive", err)
		return
	}
	entries.CustomSort(base.NaturalSortLess)

	rx := generateMatcher()
	var files []string
	for _, entry := range entries {
		if !isExcludedEntry(entry, rx) {
			files = append(files, entry.Name())
		}
	}
	ctx.JSON(http.StatusOK, files)
}

func isExcludedEntry(entry *git.TreeEntry, rx *regexp.Regexp) bool {
	if entry.IsDir() {
		return true
	}

	if entry.IsSubModule() {
		return true
	}

	if enry.IsVendor(entry.Name()) {
		return true
	}

	// Exclude hardcoded dirs
	if rx.MatchString(entry.Name()) {
		return true
	}
	return false
}

func generateMatcher() *regexp.Regexp {
	dirRegex := ""

	for i, dir := range excludedDirs {
		// Matched vendor or Vendor or VENDOR directories.
		dirRegex += fmt.Sprintf("(^%s\\/.*$)|(^%s\\/.*$)|(^%s\\/.*$)",
			dir,
			strings.Title(strings.ToLower(dir)),
			strings.ToUpper(dir),
		)
		if i < len(excludedDirs)-1 {
			dirRegex += "|"
		}
	}

	rx, _ := regexp.Compile(dirRegex)
	return rx
}
