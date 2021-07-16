// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
)

// ContentType repo content type
type ContentType string

// The string representations of different content types
const (
	// ContentTypeRegular regular content type (file)
	ContentTypeRegular ContentType = "file"
	// ContentTypeDir dir content type (dir)
	ContentTypeDir ContentType = "dir"
	// ContentLink link content type (symlink)
	ContentTypeLink ContentType = "symlink"
	// ContentTag submodule content type (submodule)
	ContentTypeSubmodule ContentType = "submodule"
)

// String gets the string of ContentType
func (ct *ContentType) String() string {
	return string(*ct)
}

// GetContentsOrList gets the meta data of a file's contents (*ContentsResponse) if treePath not a tree
// directory, otherwise a listing of file contents ([]*ContentsResponse). Ref can be a branch, commit or tag
func GetContentsOrList(repo *models.Repository, treePath, ref string) (interface{}, error) {
	if repo.IsEmpty {
		return make([]interface{}, 0), nil
	}
	if ref == "" {
		ref = repo.DefaultBranch
	}
	origRef := ref

	// Check that the path given in opts.treePath is valid (not a git path)
	cleanTreePath := CleanUploadFileName(treePath)
	if cleanTreePath == "" && treePath != "" {
		return nil, models.ErrFilenameInvalid{
			Path: treePath,
		}
	}
	treePath = cleanTreePath

	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	// Get the commit object for the ref
	commit, err := gitRepo.GetCommit(ref)
	if err != nil {
		return nil, err
	}

	entry, err := commit.GetTreeEntryByPath(treePath)
	if err != nil {
		return nil, err
	}

	if entry.Type() != "tree" {
		return GetContents(repo, treePath, origRef, false)
	}

	// We are in a directory, so we return a list of FileContentResponse objects
	var fileList []*api.ContentsResponse

	gitTree, err := commit.SubTree(treePath)
	if err != nil {
		return nil, err
	}
	entries, err := gitTree.ListEntries()
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		subTreePath := path.Join(treePath, e.Name())
		fileContentResponse, err := GetContents(repo, subTreePath, origRef, true)
		if err != nil {
			return nil, err
		}
		fileList = append(fileList, fileContentResponse)
	}
	return fileList, nil
}

// GetContents gets the meta data on a file's contents. Ref can be a branch, commit or tag
func GetContents(repo *models.Repository, treePath, ref string, forList bool) (*api.ContentsResponse, error) {
	if ref == "" {
		ref = repo.DefaultBranch
	}
	origRef := ref

	// Check that the path given in opts.treePath is valid (not a git path)
	cleanTreePath := CleanUploadFileName(treePath)
	if cleanTreePath == "" && treePath != "" {
		return nil, models.ErrFilenameInvalid{
			Path: treePath,
		}
	}
	treePath = cleanTreePath

	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	// Get the commit object for the ref
	commit, err := gitRepo.GetCommit(ref)
	if err != nil {
		return nil, err
	}
	commitID := commit.ID.String()
	if len(ref) >= 4 && strings.HasPrefix(commitID, ref) {
		ref = commit.ID.String()
	}

	entry, err := commit.GetTreeEntryByPath(treePath)
	if err != nil {
		return nil, err
	}

	refType := gitRepo.GetRefType(ref)
	if refType == "invalid" {
		return nil, fmt.Errorf("no commit found for the ref [ref: %s]", ref)
	}

	selfURL, err := url.Parse(fmt.Sprintf("%s/contents/%s?ref=%s", repo.APIURL(), treePath, origRef))
	if err != nil {
		return nil, err
	}
	selfURLString := selfURL.String()

	lastCommit, err := commit.GetCommitByPath(treePath)
	if err != nil {
		return nil, err
	}

	// All content types have these fields in populated
	contentsResponse := &api.ContentsResponse{
		Name:   entry.Name(),
		Path:   treePath,
		SHA:    entry.ID.String(),
		Size:   entry.Size(),
		URL:    &selfURLString,
		Commit: lastCommit.ID.String(),
		Links: &api.FileLinksResponse{
			Self: &selfURLString,
		},
	}

	// Now populate the rest of the ContentsResponse based on entry type
	if entry.IsRegular() || entry.IsExecutable() {
		contentsResponse.Type = string(ContentTypeRegular)
		if blobResponse, err := GetBlobBySHA(repo, entry.ID.String()); err != nil {
			return nil, err
		} else if !forList {
			// We don't show the content if we are getting a list of FileContentResponses
			contentsResponse.Encoding = &blobResponse.Encoding
			contentsResponse.Content = &blobResponse.Content
		}
	} else if entry.IsDir() {
		contentsResponse.Type = string(ContentTypeDir)
	} else if entry.IsLink() {
		contentsResponse.Type = string(ContentTypeLink)
		// The target of a symlink file is the content of the file
		targetFromContent, err := entry.Blob().GetBlobContent()
		if err != nil {
			return nil, err
		}
		contentsResponse.Target = &targetFromContent
	} else if entry.IsSubModule() {
		contentsResponse.Type = string(ContentTypeSubmodule)
		submodule, err := commit.GetSubModule(treePath)
		if err != nil {
			return nil, err
		}
		contentsResponse.SubmoduleGitURL = &submodule.URL
	}
	// Handle links
	if entry.IsRegular() || entry.IsLink() {
		downloadURL, err := url.Parse(fmt.Sprintf("%s/raw/%s/%s/%s", repo.HTMLURL(), refType, ref, treePath))
		if err != nil {
			return nil, err
		}
		downloadURLString := downloadURL.String()
		contentsResponse.DownloadURL = &downloadURLString
	}
	if !entry.IsSubModule() {
		htmlURL, err := url.Parse(fmt.Sprintf("%s/src/%s/%s/%s", repo.HTMLURL(), refType, ref, treePath))
		if err != nil {
			return nil, err
		}
		htmlURLString := htmlURL.String()
		contentsResponse.HTMLURL = &htmlURLString
		contentsResponse.Links.HTMLURL = &htmlURLString

		gitURL, err := url.Parse(fmt.Sprintf("%s/git/blobs/%s", repo.APIURL(), entry.ID.String()))
		if err != nil {
			return nil, err
		}
		gitURLString := gitURL.String()
		contentsResponse.GitURL = &gitURLString
		contentsResponse.Links.GitURL = &gitURLString
	}

	return contentsResponse, nil
}
