// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package wiki

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/sync"
	"code.gitea.io/gitea/modules/util"
)

var (
	reservedWikiNames = []string{"_pages", "_new", "_edit", "raw"}
	wikiWorkingPool   = sync.NewExclusivePool()
)

func nameAllowed(name string) error {
	if util.IsStringInSlice(name, reservedWikiNames) {
		return models.ErrWikiReservedName{
			Title: name,
		}
	}
	return nil
}

// NameToSubURL converts a wiki name to its corresponding sub-URL.
func NameToSubURL(name string) string {
	return url.QueryEscape(strings.ReplaceAll(name, " ", "-"))
}

// NormalizeWikiName normalizes a wiki name
func NormalizeWikiName(name string) string {
	return strings.ReplaceAll(name, "-", " ")
}

// NameToFilename converts a wiki name to its corresponding filename.
func NameToFilename(name string) string {
	name = strings.ReplaceAll(name, " ", "-")
	return url.QueryEscape(name) + ".md"
}

// FilenameToName converts a wiki filename to its corresponding page name.
func FilenameToName(filename string) (string, error) {
	if !strings.HasSuffix(filename, ".md") {
		return "", models.ErrWikiInvalidFileName{
			FileName: filename,
		}
	}
	basename := filename[:len(filename)-3]
	unescaped, err := url.QueryUnescape(basename)
	if err != nil {
		return "", err
	}
	return NormalizeWikiName(unescaped), nil
}

// InitWiki initializes a wiki for repository,
// it does nothing when repository already has wiki.
func InitWiki(repo *models.Repository) error {
	if repo.HasWiki() {
		return nil
	}

	if err := git.InitRepository(repo.WikiPath(), true); err != nil {
		return fmt.Errorf("InitRepository: %v", err)
	} else if err = repo_module.CreateDelegateHooks(repo.WikiPath()); err != nil {
		return fmt.Errorf("createDelegateHooks: %v", err)
	} else if _, err = git.NewCommand("symbolic-ref", "HEAD", git.BranchPrefix+"master").RunInDir(repo.WikiPath()); err != nil {
		return fmt.Errorf("unable to set default wiki branch to master: %v", err)
	}
	return nil
}

// prepareWikiFileName try to find a suitable file path with file name by the given raw wiki name.
// return: existence, prepared file path with name, error
func prepareWikiFileName(gitRepo *git.Repository, wikiName string) (bool, string, error) {
	unescaped := wikiName + ".md"
	escaped := NameToFilename(wikiName)

	// Look for both files
	filesInIndex, err := gitRepo.LsFiles(unescaped, escaped)
	if err != nil {
		log.Error("%v", err)
		return false, escaped, err
	}

	foundEscaped := false
	for _, filename := range filesInIndex {
		switch filename {
		case unescaped:
			// if we find the unescaped file return it
			return true, unescaped, nil
		case escaped:
			foundEscaped = true
		}
	}

	// If not return whether the escaped file exists, and the escaped filename to keep backwards compatibility.
	return foundEscaped, escaped, nil
}

// updateWikiPage adds a new page to the repository wiki.
func updateWikiPage(doer *models.User, repo *models.Repository, oldWikiName, newWikiName, content, message string, isNew bool) (err error) {
	if err = nameAllowed(newWikiName); err != nil {
		return err
	}
	wikiWorkingPool.CheckIn(fmt.Sprint(repo.ID))
	defer wikiWorkingPool.CheckOut(fmt.Sprint(repo.ID))

	if err = InitWiki(repo); err != nil {
		return fmt.Errorf("InitWiki: %v", err)
	}

	hasMasterBranch := git.IsBranchExist(repo.WikiPath(), "master")

	basePath, err := models.CreateTemporaryPath("update-wiki")
	if err != nil {
		return err
	}
	defer func() {
		if err := models.RemoveTemporaryPath(basePath); err != nil {
			log.Error("Merge: RemoveTemporaryPath: %s", err)
		}
	}()

	cloneOpts := git.CloneRepoOptions{
		Bare:   true,
		Shared: true,
	}

	if hasMasterBranch {
		cloneOpts.Branch = "master"
	}

	if err := git.Clone(repo.WikiPath(), basePath, cloneOpts); err != nil {
		log.Error("Failed to clone repository: %s (%v)", repo.FullName(), err)
		return fmt.Errorf("Failed to clone repository: %s (%v)", repo.FullName(), err)
	}

	gitRepo, err := git.OpenRepository(basePath)
	if err != nil {
		log.Error("Unable to open temporary repository: %s (%v)", basePath, err)
		return fmt.Errorf("Failed to open new temporary repository in: %s %v", basePath, err)
	}
	defer gitRepo.Close()

	if hasMasterBranch {
		if err := gitRepo.ReadTreeToIndex("HEAD"); err != nil {
			log.Error("Unable to read HEAD tree to index in: %s %v", basePath, err)
			return fmt.Errorf("Unable to read HEAD tree to index in: %s %v", basePath, err)
		}
	}

	isWikiExist, newWikiPath, err := prepareWikiFileName(gitRepo, newWikiName)
	if err != nil {
		return err
	}

	if isNew {
		if isWikiExist {
			return models.ErrWikiAlreadyExist{
				Title: newWikiPath,
			}
		}
	} else {
		// avoid check existence again if wiki name is not changed since gitRepo.LsFiles(...) is not free.
		isOldWikiExist := true
		oldWikiPath := newWikiPath
		if oldWikiName != newWikiName {
			isOldWikiExist, oldWikiPath, err = prepareWikiFileName(gitRepo, oldWikiName)
			if err != nil {
				return err
			}
		}

		if isOldWikiExist {
			err := gitRepo.RemoveFilesFromIndex(oldWikiPath)
			if err != nil {
				log.Error("%v", err)
				return err
			}
		}
	}

	// FIXME: The wiki doesn't have lfs support at present - if this changes need to check attributes here

	objectHash, err := gitRepo.HashObject(strings.NewReader(content))
	if err != nil {
		log.Error("%v", err)
		return err
	}

	if err := gitRepo.AddObjectToIndex("100644", objectHash, newWikiPath); err != nil {
		log.Error("%v", err)
		return err
	}

	tree, err := gitRepo.WriteTree()
	if err != nil {
		log.Error("%v", err)
		return err
	}

	commitTreeOpts := git.CommitTreeOpts{
		Message: message,
	}

	committer := doer.NewGitSig()

	sign, signingKey, signer, _ := repo.SignWikiCommit(doer)
	if sign {
		commitTreeOpts.KeyID = signingKey
		if repo.GetTrustModel() == models.CommitterTrustModel || repo.GetTrustModel() == models.CollaboratorCommitterTrustModel {
			committer = signer
		}
	} else {
		commitTreeOpts.NoGPGSign = true
	}
	if hasMasterBranch {
		commitTreeOpts.Parents = []string{"HEAD"}
	}

	commitHash, err := gitRepo.CommitTree(doer.NewGitSig(), committer, tree, commitTreeOpts)
	if err != nil {
		log.Error("%v", err)
		return err
	}

	if err := git.Push(basePath, git.PushOptions{
		Remote: "origin",
		Branch: fmt.Sprintf("%s:%s%s", commitHash.String(), git.BranchPrefix, "master"),
		Env: models.FullPushingEnvironment(
			doer,
			doer,
			repo,
			repo.Name+".wiki",
			0,
		),
	}); err != nil {
		log.Error("%v", err)
		if git.IsErrPushOutOfDate(err) || git.IsErrPushRejected(err) {
			return err
		}
		return fmt.Errorf("Push: %v", err)
	}

	return nil
}

// AddWikiPage adds a new wiki page with a given wikiPath.
func AddWikiPage(doer *models.User, repo *models.Repository, wikiName, content, message string) error {
	return updateWikiPage(doer, repo, "", wikiName, content, message, true)
}

// EditWikiPage updates a wiki page identified by its wikiPath,
// optionally also changing wikiPath.
func EditWikiPage(doer *models.User, repo *models.Repository, oldWikiName, newWikiName, content, message string) error {
	return updateWikiPage(doer, repo, oldWikiName, newWikiName, content, message, false)
}

// DeleteWikiPage deletes a wiki page identified by its path.
func DeleteWikiPage(doer *models.User, repo *models.Repository, wikiName string) (err error) {
	wikiWorkingPool.CheckIn(fmt.Sprint(repo.ID))
	defer wikiWorkingPool.CheckOut(fmt.Sprint(repo.ID))

	if err = InitWiki(repo); err != nil {
		return fmt.Errorf("InitWiki: %v", err)
	}

	basePath, err := models.CreateTemporaryPath("update-wiki")
	if err != nil {
		return err
	}
	defer func() {
		if err := models.RemoveTemporaryPath(basePath); err != nil {
			log.Error("Merge: RemoveTemporaryPath: %s", err)
		}
	}()

	if err := git.Clone(repo.WikiPath(), basePath, git.CloneRepoOptions{
		Bare:   true,
		Shared: true,
		Branch: "master",
	}); err != nil {
		log.Error("Failed to clone repository: %s (%v)", repo.FullName(), err)
		return fmt.Errorf("Failed to clone repository: %s (%v)", repo.FullName(), err)
	}

	gitRepo, err := git.OpenRepository(basePath)
	if err != nil {
		log.Error("Unable to open temporary repository: %s (%v)", basePath, err)
		return fmt.Errorf("Failed to open new temporary repository in: %s %v", basePath, err)
	}
	defer gitRepo.Close()

	if err := gitRepo.ReadTreeToIndex("HEAD"); err != nil {
		log.Error("Unable to read HEAD tree to index in: %s %v", basePath, err)
		return fmt.Errorf("Unable to read HEAD tree to index in: %s %v", basePath, err)
	}

	wikiPath := NameToFilename(wikiName)
	filesInIndex, err := gitRepo.LsFiles(wikiPath)
	found := false
	for _, file := range filesInIndex {
		if file == wikiPath {
			found = true
			break
		}
	}
	if found {
		err := gitRepo.RemoveFilesFromIndex(wikiPath)
		if err != nil {
			return err
		}
	} else {
		return os.ErrNotExist
	}

	// FIXME: The wiki doesn't have lfs support at present - if this changes need to check attributes here

	tree, err := gitRepo.WriteTree()
	if err != nil {
		return err
	}
	message := "Delete page '" + wikiName + "'"
	commitTreeOpts := git.CommitTreeOpts{
		Message: message,
		Parents: []string{"HEAD"},
	}

	committer := doer.NewGitSig()

	sign, signingKey, signer, _ := repo.SignWikiCommit(doer)
	if sign {
		commitTreeOpts.KeyID = signingKey
		if repo.GetTrustModel() == models.CommitterTrustModel || repo.GetTrustModel() == models.CollaboratorCommitterTrustModel {
			committer = signer
		}
	} else {
		commitTreeOpts.NoGPGSign = true
	}

	commitHash, err := gitRepo.CommitTree(doer.NewGitSig(), committer, tree, commitTreeOpts)
	if err != nil {
		return err
	}

	if err := git.Push(basePath, git.PushOptions{
		Remote: "origin",
		Branch: fmt.Sprintf("%s:%s%s", commitHash.String(), git.BranchPrefix, "master"),
		Env:    models.PushingEnvironment(doer, repo),
	}); err != nil {
		if git.IsErrPushOutOfDate(err) || git.IsErrPushRejected(err) {
			return err
		}
		return fmt.Errorf("Push: %v", err)
	}

	return nil
}
