// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package stats

import (
	"io/ioutil"
	"os"
	"sync"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

// DBIndexer implements Indexer interface to use database's like search
type DBIndexer struct {
}

// Index repository status function
func (db *DBIndexer) Index(id int64) error {
	repo, err := models.GetRepositoryByID(id)
	if err != nil {
		return err
	}
	if repo.IsEmpty {
		return nil
	}

	status, err := repo.GetIndexerStatus(models.RepoIndexerTypeStats)
	if err != nil {
		return err
	}

	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		return err
	}
	defer gitRepo.Close()

	// Get latest commit for default branch
	commitID, err := gitRepo.GetBranchCommitID(repo.DefaultBranch)
	if err != nil {
		if git.IsErrBranchNotExist(err) || git.IsErrNotExist((err)) {
			log.Debug("Unable to get commit ID for defaultbranch %s in %s ... skipping this repository", repo.DefaultBranch, repo.RepoPath())
			return nil
		}
		log.Error("Unable to get commit ID for defaultbranch %s in %s. Error: %v", repo.DefaultBranch, repo.RepoPath(), err)
		return err
	}

	// Do not recalculate stats if already calculated for this commit
	if status.CommitSha == commitID {
		return nil
	}

	var tmpIndex *os.File
	if git.CheckGitVersionAtLeast("1.7.8") == nil {
		tmpIndex, err = ioutil.TempFile("", "index")
		if err != nil {
			return err
		}
		defer func() {
			err := util.Remove(tmpIndex.Name())
			if err != nil {
				log.Error("failed to remove tmp index file: %v", err)
			}
		}()

		_, err = git.NewCommand("read-tree", commitID).
			RunInDirWithEnv(gitRepo.Path, []string{"GIT_INDEX_FILE=" + tmpIndex.Name()})
		if err != nil {
			return err
		}

		checker := &git.AttrChecker{
			RequestAttrs: []string{"linguist-vendored", "linguist-language"},
			Repo:         gitRepo,
			IndexFile:    tmpIndex.Name(),
		}

		checker.Init()

		wg := new(sync.WaitGroup)
		wg.Add(2)

		errCh := make(chan error)

		// run cmd
		go func() {
			if err := checker.Run(); err != nil {
				errCh <- err
			}
			wg.Done()
		}()

		stats := make(map[string]int64)

		go func() {
			var err error
			stats, err = gitRepo.GetLanguageStats(commitID, func(path string) (string, bool) {
				// get language follow linguist rulers
				// linguist-language=<lang> attribute to an language
				// linguist-vendored attribute to vendor or un-vendor paths
				rs, err := checker.CheckAttrs(path)
				if err != nil {
					log.Error("git.CheckAttrs: %v", err)
					return "", false
				}

				if rs["linguist-vendored"] == "set" {
					return "", true
				}

				if lang, has := rs["linguist-language"]; has {
					if lang == "unspecified" {
						return "", false
					}
					return lang, false
				}

				return "", false
			})
			if err != nil {
				errCh <- err
			}
			checker.Close()
			wg.Done()
		}()

		wg.Wait()

		select {
		case err, has := <-errCh:
			if has {
				log.Error("Unable to get language stats for ID %s for defaultbranch %s in %s. Error: %v", commitID, repo.DefaultBranch, repo.RepoPath(), err)
				return err
			}
		default:
			return repo.UpdateLanguageStats(commitID, stats)
		}
	}

	// Calculate and save language statistics to database
	stats, err := gitRepo.GetLanguageStats(commitID, nil)
	if err != nil {
		log.Error("Unable to get language stats for ID %s for defaultbranch %s in %s. Error: %v", commitID, repo.DefaultBranch, repo.RepoPath(), err)
		return err
	}
	return repo.UpdateLanguageStats(commitID, stats)
}

// Close dummy function
func (db *DBIndexer) Close() {
}
