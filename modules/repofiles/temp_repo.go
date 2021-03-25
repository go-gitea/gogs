// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/gitdiff"
)

// TemporaryUploadRepository is a type to wrap our upload repositories as a shallow clone
type TemporaryUploadRepository struct {
	repo     *models.Repository
	gitRepo  *git.Repository
	basePath string
}

// NewTemporaryUploadRepository creates a new temporary upload repository
func NewTemporaryUploadRepository(repo *models.Repository) (*TemporaryUploadRepository, error) {
	basePath, err := models.CreateTemporaryPath("upload")
	if err != nil {
		return nil, err
	}
	t := &TemporaryUploadRepository{repo: repo, basePath: basePath}
	return t, nil
}

// Close the repository cleaning up all files
func (t *TemporaryUploadRepository) Close() {
	defer t.gitRepo.Close()
	if err := models.RemoveTemporaryPath(t.basePath); err != nil {
		log.Error("Failed to remove temporary path %s: %v", t.basePath, err)
	}
}

// Clone the base repository to our path and set branch as the HEAD
func (t *TemporaryUploadRepository) Clone(branch string) error {
	if _, err := git.NewCommand("clone", "-s", "--bare", "-b", branch, t.repo.RepoPath(), t.basePath).Run(); err != nil {
		stderr := err.Error()
		if matched, _ := regexp.MatchString(".*Remote branch .* not found in upstream origin.*", stderr); matched {
			return git.ErrBranchNotExist{
				Name: branch,
			}
		} else if matched, _ := regexp.MatchString(".* repository .* does not exist.*", stderr); matched {
			return models.ErrRepoNotExist{
				ID:        t.repo.ID,
				UID:       t.repo.OwnerID,
				OwnerName: t.repo.OwnerName,
				Name:      t.repo.Name,
			}
		} else {
			return fmt.Errorf("Clone: %v %s", err, stderr)
		}
	}
	gitRepo, err := git.OpenRepository(t.basePath)
	if err != nil {
		return err
	}
	t.gitRepo = gitRepo
	return nil
}

// SetDefaultIndex sets the git index to our HEAD
func (t *TemporaryUploadRepository) SetDefaultIndex() error {
	if _, err := git.NewCommand("read-tree", "HEAD").RunInDir(t.basePath); err != nil {
		return fmt.Errorf("SetDefaultIndex: %v", err)
	}
	return nil
}

// LsFiles checks if the given filename arguments are in the index
func (t *TemporaryUploadRepository) LsFiles(filenames ...string) ([]string, error) {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)

	cmdArgs := []string{"ls-files", "-z", "--"}
	for _, arg := range filenames {
		if arg != "" {
			cmdArgs = append(cmdArgs, arg)
		}
	}

	if err := git.NewCommand(cmdArgs...).RunInDirPipeline(t.basePath, stdOut, stdErr); err != nil {
		log.Error("Unable to run git ls-files for temporary repo: %s (%s) Error: %v\nstdout: %s\nstderr: %s", t.repo.FullName(), t.basePath, err, stdOut.String(), stdErr.String())
		err = fmt.Errorf("Unable to run git ls-files for temporary repo of: %s Error: %v\nstdout: %s\nstderr: %s", t.repo.FullName(), err, stdOut.String(), stdErr.String())
		return nil, err
	}

	filelist := make([]string, len(filenames))
	for _, line := range bytes.Split(stdOut.Bytes(), []byte{'\000'}) {
		filelist = append(filelist, string(line))
	}

	return filelist, nil
}

// RemoveFilesFromIndex removes the given files from the index
func (t *TemporaryUploadRepository) RemoveFilesFromIndex(filenames ...string) error {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)
	stdIn := new(bytes.Buffer)
	for _, file := range filenames {
		if file != "" {
			stdIn.WriteString("0 0000000000000000000000000000000000000000\t")
			stdIn.WriteString(file)
			stdIn.WriteByte('\000')
		}
	}

	if err := git.NewCommand("update-index", "--remove", "-z", "--index-info").RunInDirFullPipeline(t.basePath, stdOut, stdErr, stdIn); err != nil {
		log.Error("Unable to update-index for temporary repo: %s (%s) Error: %v\nstdout: %s\nstderr: %s", t.repo.FullName(), t.basePath, err, stdOut.String(), stdErr.String())
		return fmt.Errorf("Unable to update-index for temporary repo: %s Error: %v\nstdout: %s\nstderr: %s", t.repo.FullName(), err, stdOut.String(), stdErr.String())
	}
	return nil
}

// HashObject writes the provided content to the object db and returns its hash
func (t *TemporaryUploadRepository) HashObject(content io.Reader) (string, error) {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)

	if err := git.NewCommand("hash-object", "-w", "--stdin").RunInDirFullPipeline(t.basePath, stdOut, stdErr, content); err != nil {
		log.Error("Unable to hash-object to temporary repo: %s (%s) Error: %v\nstdout: %s\nstderr: %s", t.repo.FullName(), t.basePath, err, stdOut.String(), stdErr.String())
		return "", fmt.Errorf("Unable to hash-object to temporary repo: %s Error: %v\nstdout: %s\nstderr: %s", t.repo.FullName(), err, stdOut.String(), stdErr.String())
	}

	return strings.TrimSpace(stdOut.String()), nil
}

// AddObjectToIndex adds the provided object hash to the index with the provided mode and path
func (t *TemporaryUploadRepository) AddObjectToIndex(mode, objectHash, objectPath string) error {
	if _, err := git.NewCommand("update-index", "--add", "--replace", "--cacheinfo", mode, objectHash, objectPath).RunInDir(t.basePath); err != nil {
		stderr := err.Error()
		if matched, _ := regexp.MatchString(".*Invalid path '.*", stderr); matched {
			return models.ErrFilePathInvalid{
				Message: objectPath,
				Path:    objectPath,
			}
		}
		log.Error("Unable to add object to index: %s %s %s in temporary repo %s(%s) Error: %v", mode, objectHash, objectPath, t.repo.FullName(), t.basePath, err)
		return fmt.Errorf("Unable to add object to index at %s in temporary repo %s Error: %v", objectPath, t.repo.FullName(), err)
	}
	return nil
}

// WriteTree writes the current index as a tree to the object db and returns its hash
func (t *TemporaryUploadRepository) WriteTree() (string, error) {
	stdout, err := git.NewCommand("write-tree").RunInDir(t.basePath)
	if err != nil {
		log.Error("Unable to write tree in temporary repo: %s(%s): Error: %v", t.repo.FullName(), t.basePath, err)
		return "", fmt.Errorf("Unable to write-tree in temporary repo for: %s Error: %v", t.repo.FullName(), err)
	}
	return strings.TrimSpace(stdout), nil
}

// GetLastCommit gets the last commit ID SHA of the repo
func (t *TemporaryUploadRepository) GetLastCommit() (string, error) {
	return t.GetLastCommitByRef("HEAD")
}

// GetLastCommitByRef gets the last commit ID SHA of the repo by ref
func (t *TemporaryUploadRepository) GetLastCommitByRef(ref string) (string, error) {
	if ref == "" {
		ref = "HEAD"
	}
	stdout, err := git.NewCommand("rev-parse", ref).RunInDir(t.basePath)
	if err != nil {
		log.Error("Unable to get last ref for %s in temporary repo: %s(%s): Error: %v", ref, t.repo.FullName(), t.basePath, err)
		return "", fmt.Errorf("Unable to rev-parse %s in temporary repo for: %s Error: %v", ref, t.repo.FullName(), err)
	}
	return strings.TrimSpace(stdout), nil
}

// CommitTree creates a commit from a given tree for the user with provided message
func (t *TemporaryUploadRepository) CommitTree(author, committer *models.User, treeHash, message string, signoff bool) (string, error) {
	return t.CommitTreeWithDate(author, committer, treeHash, message, signoff, time.Now(), time.Now())
}

// CommitTreeWithDate creates a commit from a given tree for the user with provided message
func (t *TemporaryUploadRepository) CommitTreeWithDate(author, committer *models.User, treeHash, message string, signoff bool, authorDate, committerDate time.Time) (string, error) {
	authorSig := author.NewGitSig()
	committerSig := committer.NewGitSig()

	err := git.LoadGitVersion()
	if err != nil {
		return "", fmt.Errorf("Unable to get git version: %v", err)
	}

	// Because this may call hooks we should pass in the environment
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME="+authorSig.Name,
		"GIT_AUTHOR_EMAIL="+authorSig.Email,
		"GIT_AUTHOR_DATE="+authorDate.Format(time.RFC3339),
		"GIT_COMMITTER_DATE="+committerDate.Format(time.RFC3339),
	)

	messageBytes := new(bytes.Buffer)
	_, _ = messageBytes.WriteString(message)
	_, _ = messageBytes.WriteString("\n")

	args := []string{"commit-tree", treeHash, "-p", "HEAD"}

	// Determine if we should sign
	if git.CheckGitVersionAtLeast("1.7.9") == nil {
		sign, keyID, signer, _ := t.repo.SignCRUDAction(author, t.basePath, "HEAD")
		if sign {
			args = append(args, "-S"+keyID)
			if t.repo.GetTrustModel() == models.CommitterTrustModel || t.repo.GetTrustModel() == models.CollaboratorCommitterTrustModel {
				if committerSig.Name != authorSig.Name || committerSig.Email != authorSig.Email {
					// Add trailers
					_, _ = messageBytes.WriteString("\n")
					_, _ = messageBytes.WriteString("Co-authored-by: ")
					_, _ = messageBytes.WriteString(committerSig.String())
					_, _ = messageBytes.WriteString("\n")
					_, _ = messageBytes.WriteString("Co-committed-by: ")
					_, _ = messageBytes.WriteString(committerSig.String())
					_, _ = messageBytes.WriteString("\n")
				}
				committerSig = signer
			}
		} else if git.CheckGitVersionAtLeast("2.0.0") == nil {
			args = append(args, "--no-gpg-sign")
		}
	}

	if signoff {
		// Signed-off-by
		_, _ = messageBytes.WriteString("\n")
		_, _ = messageBytes.WriteString("Signed-off-by: ")
		_, _ = messageBytes.WriteString(committerSig.String())
	}

	env = append(env,
		"GIT_COMMITTER_NAME="+committerSig.Name,
		"GIT_COMMITTER_EMAIL="+committerSig.Email,
	)

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	if err := git.NewCommand(args...).RunInDirTimeoutEnvFullPipeline(env, -1, t.basePath, stdout, stderr, messageBytes); err != nil {
		log.Error("Unable to commit-tree in temporary repo: %s (%s) Error: %v\nStdout: %s\nStderr: %s",
			t.repo.FullName(), t.basePath, err, stdout, stderr)
		return "", fmt.Errorf("Unable to commit-tree in temporary repo: %s Error: %v\nStdout: %s\nStderr: %s",
			t.repo.FullName(), err, stdout, stderr)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Push the provided commitHash to the repository branch by the provided user
func (t *TemporaryUploadRepository) Push(doer *models.User, commitHash, branch string) error {
	// Because calls hooks we need to pass in the environment
	env := models.PushingEnvironment(doer, t.repo)
	if err := git.Push(t.basePath, git.PushOptions{
		Remote: t.repo.RepoPath(),
		Branch: strings.TrimSpace(commitHash) + ":refs/heads/" + strings.TrimSpace(branch),
		Env:    env,
	}); err != nil {
		if git.IsErrPushOutOfDate(err) {
			return err
		} else if git.IsErrPushRejected(err) {
			rejectErr := err.(*git.ErrPushRejected)
			log.Info("Unable to push back to repo from temporary repo due to rejection: %s (%s)\nStdout: %s\nStderr: %s\nError: %v",
				t.repo.FullName(), t.basePath, rejectErr.StdOut, rejectErr.StdErr, rejectErr.Err)
			return err
		}
		log.Error("Unable to push back to repo from temporary repo: %s (%s)\nError: %v",
			t.repo.FullName(), t.basePath, err)
		return fmt.Errorf("Unable to push back to repo from temporary repo: %s (%s) Error: %v",
			t.repo.FullName(), t.basePath, err)
	}
	return nil
}

// DiffIndex returns a Diff of the current index to the head
func (t *TemporaryUploadRepository) DiffIndex() (*gitdiff.Diff, error) {
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		log.Error("Unable to open stdout pipe: %v", err)
		return nil, fmt.Errorf("Unable to open stdout pipe: %v", err)
	}
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()
	stderr := new(bytes.Buffer)
	var diff *gitdiff.Diff
	var finalErr error

	if err := git.NewCommand("diff-index", "--src-prefix=\\a/", "--dst-prefix=\\b/", "--cached", "-p", "HEAD").
		RunInDirTimeoutEnvFullPipelineFunc(nil, 30*time.Second, t.basePath, stdoutWriter, stderr, nil, func(ctx context.Context, cancel context.CancelFunc) error {
			_ = stdoutWriter.Close()
			diff, finalErr = gitdiff.ParsePatch(setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, stdoutReader)
			if finalErr != nil {
				log.Error("ParsePatch: %v", finalErr)
				cancel()
			}
			_ = stdoutReader.Close()
			return finalErr
		}); err != nil {
		if finalErr != nil {
			log.Error("Unable to ParsePatch in temporary repo %s (%s). Error: %v", t.repo.FullName(), t.basePath, finalErr)
			return nil, finalErr
		}
		log.Error("Unable to run diff-index pipeline in temporary repo %s (%s). Error: %v\nStderr: %s",
			t.repo.FullName(), t.basePath, err, stderr)
		return nil, fmt.Errorf("Unable to run diff-index pipeline in temporary repo %s. Error: %v\nStderr: %s",
			t.repo.FullName(), err, stderr)
	}

	diff.NumFiles, diff.TotalAddition, diff.TotalDeletion, err = git.GetDiffShortStat(t.basePath, "--cached", "HEAD")
	if err != nil {
		return nil, err
	}

	return diff, nil
}

// GetBranchCommit Gets the commit object of the given branch
func (t *TemporaryUploadRepository) GetBranchCommit(branch string) (*git.Commit, error) {
	if t.gitRepo == nil {
		return nil, fmt.Errorf("repository has not been cloned")
	}
	return t.gitRepo.GetBranchCommit(branch)
}

// GetCommit Gets the commit object of the given commit ID
func (t *TemporaryUploadRepository) GetCommit(commitID string) (*git.Commit, error) {
	if t.gitRepo == nil {
		return nil, fmt.Errorf("repository has not been cloned")
	}
	return t.gitRepo.GetCommit(commitID)
}
