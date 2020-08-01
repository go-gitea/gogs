// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mirror

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/sync"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"github.com/mcuadros/go-version"
	"github.com/unknwon/com"
)

// mirrorQueue holds an UniqueQueue object of the mirror
var mirrorQueue = sync.NewUniqueQueue(setting.Repository.MirrorQueueLength)

func readAddress(m *models.Mirror) {
	if len(m.Address) > 0 {
		return
	}
	var err error
	m.Address, err = remoteAddress(m.Repo.RepoPath())
	if err != nil {
		log.Error("remoteAddress: %v", err)
	}
}

func remoteAddress(repoPath string) (string, error) {
	var cmd *git.Command
	binVersion, err := git.BinVersion()
	if err != nil {
		return "", err
	}
	if version.Compare(binVersion, "2.7", ">=") {
		cmd = git.NewCommand("remote", "get-url", "origin")
	} else {
		cmd = git.NewCommand("config", "--get", "remote.origin.url")
	}

	result, err := cmd.RunInDir(repoPath)
	if err != nil {
		if strings.HasPrefix(err.Error(), "exit status 128 - fatal: No such remote ") {
			return "", nil
		}
		return "", err
	}
	if len(result) > 0 {
		return result[:len(result)-1], nil
	}
	return "", nil
}

// sanitizeOutput sanitizes output of a command, replacing occurrences of the
// repository's remote address with a sanitized version.
func sanitizeOutput(output, repoPath string) (string, error) {
	remoteAddr, err := remoteAddress(repoPath)
	if err != nil {
		// if we're unable to load the remote address, then we're unable to
		// sanitize.
		return "", err
	}
	return util.SanitizeMessage(output, remoteAddr), nil
}

// AddressNoCredentials returns mirror address from Git repository config without credentials.
func AddressNoCredentials(m *models.Mirror) string {
	readAddress(m)
	u, err := url.Parse(m.Address)
	if err != nil {
		// this shouldn't happen but just return it unsanitised
		return m.Address
	}
	if u.Scheme == "ssh" {
		return u.String()
	}
	u.User = nil
	return u.String()
}

// SaveAddress writes new address to Git repository config.
func SaveAddress(m *models.Mirror, addr string) error {
	repoPath := m.Repo.RepoPath()
	// Remove old origin
	_, err := git.NewCommand("remote", "rm", "origin").RunInDir(repoPath)
	if err != nil && !strings.HasPrefix(err.Error(), "exit status 128 - fatal: No such remote ") {
		return err
	}

	_, err = git.NewCommand("remote", "add", "origin", "--mirror=fetch", addr).RunInDir(repoPath)
	if err != nil && !strings.HasPrefix(err.Error(), "exit status 128 - fatal: No such remote ") {
		return err
	}

	if m.Repo.HasWiki() {
		wikiPath := m.Repo.WikiPath()
		wikiRemotePath := repo_module.WikiRemoteURL(addr)
		// Remove old origin of wiki
		_, err := git.NewCommand("remote", "rm", "origin").RunInDir(wikiPath)
		if err != nil && !strings.HasPrefix(err.Error(), "exit status 128 - fatal: No such remote ") {
			return err
		}

		_, err = git.NewCommand("remote", "add", "origin", "--mirror=fetch", wikiRemotePath).RunInDir(wikiPath)
		if err != nil && !strings.HasPrefix(err.Error(), "exit status 128 - fatal: No such remote ") {
			return err
		}
	}
	return nil
}

// gitShortEmptySha Git short empty SHA
const gitShortEmptySha = "0000000"

// mirrorSyncResult contains information of a updated reference.
// If the oldCommitID is "0000000", it means a new reference, the value of newCommitID is empty.
// If the newCommitID is "0000000", it means the reference is deleted, the value of oldCommitID is empty.
type mirrorSyncResult struct {
	refName     string
	oldCommitID string
	newCommitID string
}

// parseRemoteUpdateOutput detects create, update and delete operations of references from upstream.
func parseRemoteUpdateOutput(output string) []*mirrorSyncResult {
	results := make([]*mirrorSyncResult, 0, 3)
	lines := strings.Split(output, "\n")
	for i := range lines {
		// Make sure reference name is presented before continue
		idx := strings.Index(lines[i], "-> ")
		if idx == -1 {
			continue
		}

		refName := lines[i][idx+3:]

		switch {
		case strings.HasPrefix(lines[i], " * "): // New reference
			results = append(results, &mirrorSyncResult{
				refName:     refName,
				oldCommitID: gitShortEmptySha,
			})
		case strings.HasPrefix(lines[i], " - "): // Delete reference
			results = append(results, &mirrorSyncResult{
				refName:     refName,
				newCommitID: gitShortEmptySha,
			})
		case strings.HasPrefix(lines[i], "   "): // New commits of a reference
			delimIdx := strings.Index(lines[i][3:], " ")
			if delimIdx == -1 {
				log.Error("SHA delimiter not found: %q", lines[i])
				continue
			}
			shas := strings.Split(lines[i][3:delimIdx+3], "..")
			if len(shas) != 2 {
				log.Error("Expect two SHAs but not what found: %q", lines[i])
				continue
			}
			results = append(results, &mirrorSyncResult{
				refName:     refName,
				oldCommitID: shas[0],
				newCommitID: shas[1],
			})

		default:
			log.Warn("parseRemoteUpdateOutput: unexpected update line %q", lines[i])
		}
	}
	return results
}

// runSync returns true if sync finished without error.
func runSync(m *models.Mirror) ([]*mirrorSyncResult, bool) {
	repoPath := m.Repo.RepoPath()
	wikiPath := m.Repo.WikiPath()
	timeout := time.Duration(setting.Git.Timeout.Mirror) * time.Second

	gitArgs := []string{"remote", "update"}
	if m.EnablePrune {
		gitArgs = append(gitArgs, "--prune")
	}

	stdoutBuilder := strings.Builder{}
	stderrBuilder := strings.Builder{}
	if err := git.NewCommand(gitArgs...).
		SetDescription(fmt.Sprintf("Mirror.runSync: %s", m.Repo.FullName())).
		RunInDirTimeoutPipeline(timeout, repoPath, &stdoutBuilder, &stderrBuilder); err != nil {
		stdout := stdoutBuilder.String()
		stderr := stderrBuilder.String()
		// sanitize the output, since it may contain the remote address, which may
		// contain a password
		stderrMessage, sanitizeErr := sanitizeOutput(stderr, repoPath)
		if sanitizeErr != nil {
			log.Error("sanitizeOutput failed on stderr: %v", sanitizeErr)
			log.Error("Failed to update mirror repository %v:\nStdout: %s\nStderr: %s\nErr: %v", m.Repo, stdout, stderr, err)
			return nil, false
		}
		stdoutMessage, err := sanitizeOutput(stdout, repoPath)
		if err != nil {
			log.Error("sanitizeOutput failed: %v", sanitizeErr)
			log.Error("Failed to update mirror repository %v:\nStdout: %s\nStderr: %s\nErr: %v", m.Repo, stdout, stderrMessage, err)
			return nil, false
		}

		log.Error("Failed to update mirror repository %v:\nStdout: %s\nStderr: %s\nErr: %v", m.Repo, stdoutMessage, stderrMessage, err)
		desc := fmt.Sprintf("Failed to update mirror repository '%s': %s", repoPath, stderrMessage)
		if err = models.CreateRepositoryNotice(desc); err != nil {
			log.Error("CreateRepositoryNotice: %v", err)
		}
		return nil, false
	}
	output := stderrBuilder.String()

	gitRepo, err := git.OpenRepository(repoPath)
	if err != nil {
		log.Error("OpenRepository: %v", err)
		return nil, false
	}
	if err = repo_module.SyncReleasesWithTags(m.Repo, gitRepo); err != nil {
		gitRepo.Close()
		log.Error("Failed to synchronize tags to releases for repository: %v", err)
	}
	gitRepo.Close()

	if err := m.Repo.UpdateSize(models.DefaultDBContext()); err != nil {
		log.Error("Failed to update size for mirror repository: %v", err)
	}

	if m.Repo.HasWiki() {
		stderrBuilder.Reset()
		stdoutBuilder.Reset()
		if err := git.NewCommand("remote", "update", "--prune").
			SetDescription(fmt.Sprintf("Mirror.runSync Wiki: %s ", m.Repo.FullName())).
			RunInDirTimeoutPipeline(timeout, wikiPath, &stdoutBuilder, &stderrBuilder); err != nil {
			stdout := stdoutBuilder.String()
			stderr := stderrBuilder.String()
			// sanitize the output, since it may contain the remote address, which may
			// contain a password
			stderrMessage, sanitizeErr := sanitizeOutput(stderr, repoPath)
			if sanitizeErr != nil {
				log.Error("sanitizeOutput failed on stderr: %v", sanitizeErr)
				log.Error("Failed to update mirror repository wiki %v:\nStdout: %s\nStderr: %s\nErr: %v", m.Repo, stdout, stderr, err)
				return nil, false
			}
			stdoutMessage, err := sanitizeOutput(stdout, repoPath)
			if err != nil {
				log.Error("sanitizeOutput failed: %v", sanitizeErr)
				log.Error("Failed to update mirror repository wiki %v:\nStdout: %s\nStderr: %s\nErr: %v", m.Repo, stdout, stderrMessage, err)
				return nil, false
			}

			log.Error("Failed to update mirror repository wiki %v:\nStdout: %s\nStderr: %s\nErr: %v", m.Repo, stdoutMessage, stderrMessage, err)
			desc := fmt.Sprintf("Failed to update mirror repository wiki '%s': %s", wikiPath, stderrMessage)
			if err = models.CreateRepositoryNotice(desc); err != nil {
				log.Error("CreateRepositoryNotice: %v", err)
			}
			return nil, false
		}
	}

	branches, err := repo_module.GetBranches(m.Repo)
	if err != nil {
		log.Error("GetBranches: %v", err)
		return nil, false
	}

	for i := range branches {
		cache.Remove(m.Repo.GetCommitsCountCacheKey(branches[i].Name, true))
	}

	m.UpdatedUnix = timeutil.TimeStampNow()
	return parseRemoteUpdateOutput(output), true
}

// Address returns mirror address from Git repository config without credentials.
func Address(m *models.Mirror) string {
	readAddress(m)
	return util.SanitizeURLCredentials(m.Address, false)
}

// Username returns the mirror address username
func Username(m *models.Mirror) string {
	readAddress(m)
	u, err := url.Parse(m.Address)
	if err != nil {
		// this shouldn't happen but if it does return ""
		return ""
	}
	if u.Scheme == "ssh" {
		return ""
	}
	return u.User.Username()
}

// Password returns the mirror address password
func Password(m *models.Mirror) string {
	readAddress(m)
	u, err := url.Parse(m.Address)
	if err != nil {
		// this shouldn't happen but if it does return ""
		return ""
	}
	if u.Scheme == "ssh" {
		return ""
	}
	password, _ := u.User.Password()
	return password
}

// Update checks and updates mirror repositories.
func Update(ctx context.Context) error {
	log.Trace("Doing: Update")
	if err := models.MirrorsIterate(func(idx int, bean interface{}) error {
		m := bean.(*models.Mirror)
		if m.Repo == nil {
			log.Error("Disconnected mirror repository found: %d", m.ID)
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("Aborted")
		default:
			mirrorQueue.Add(m.RepoID)
			return nil
		}
	}); err != nil {
		log.Trace("Update: %v", err)
		return err
	}
	log.Trace("Finished: Update")
	return nil
}

// SyncMirrors checks and syncs mirrors.
// FIXME: graceful: this should be a persistable queue
func SyncMirrors(ctx context.Context) {
	// Start listening on new sync requests.
	for {
		select {
		case <-ctx.Done():
			mirrorQueue.Close()
			return
		case repoID := <-mirrorQueue.Queue():
			syncMirror(repoID)
		}
	}
}

func syncMirror(repoID string) {
	log.Trace("SyncMirrors [repo_id: %v]", repoID)
	defer func() {
		err := recover()
		if err == nil {
			return
		}
		// There was a panic whilst syncMirrors...
		log.Error("PANIC whilst syncMirrors[%s] Panic: %v\nStacktrace: %s", repoID, err, log.Stack(2))
	}()
	mirrorQueue.Remove(repoID)

	m, err := models.GetMirrorByRepoID(com.StrTo(repoID).MustInt64())
	if err != nil {
		log.Error("GetMirrorByRepoID [%s]: %v", repoID, err)
		return

	}

	results, ok := runSync(m)
	if !ok {
		return
	}

	m.ScheduleNextUpdate()
	if err = models.UpdateMirror(m); err != nil {
		log.Error("UpdateMirror [%s]: %v", repoID, err)
		return
	}

	var gitRepo *git.Repository
	if len(results) == 0 {
		log.Trace("SyncMirrors [repo_id: %d]: no commits fetched", m.RepoID)
	} else {
		gitRepo, err = git.OpenRepository(m.Repo.RepoPath())
		if err != nil {
			log.Error("OpenRepository [%d]: %v", m.RepoID, err)
			return
		}
		defer gitRepo.Close()
	}

	for _, result := range results {
		// Discard GitHub pull requests, i.e. refs/pull/*
		if strings.HasPrefix(result.refName, "refs/pull/") {
			continue
		}

		tp, _ := git.SplitRefName(result.refName)

		// Create reference
		if result.oldCommitID == gitShortEmptySha {
			notification.NotifySyncCreateRef(m.Repo.MustOwner(), m.Repo, tp, result.refName)
			continue
		}

		// Delete reference
		if result.newCommitID == gitShortEmptySha {
			notification.NotifySyncDeleteRef(m.Repo.MustOwner(), m.Repo, tp, result.refName)
			continue
		}

		// Push commits
		oldCommitID, err := git.GetFullCommitID(gitRepo.Path, result.oldCommitID)
		if err != nil {
			log.Error("GetFullCommitID [%d]: %v", m.RepoID, err)
			continue
		}
		newCommitID, err := git.GetFullCommitID(gitRepo.Path, result.newCommitID)
		if err != nil {
			log.Error("GetFullCommitID [%d]: %v", m.RepoID, err)
			continue
		}
		commits, err := gitRepo.CommitsBetweenIDs(newCommitID, oldCommitID)
		if err != nil {
			log.Error("CommitsBetweenIDs [repo_id: %d, new_commit_id: %s, old_commit_id: %s]: %v", m.RepoID, newCommitID, oldCommitID, err)
			continue
		}

		theCommits := repo_module.ListToPushCommits(commits)
		if len(theCommits.Commits) > setting.UI.FeedMaxCommitNum {
			theCommits.Commits = theCommits.Commits[:setting.UI.FeedMaxCommitNum]
		}

		theCommits.CompareURL = m.Repo.ComposeCompareURL(oldCommitID, newCommitID)

		notification.NotifySyncPushCommits(m.Repo.MustOwner(), m.Repo, result.refName, oldCommitID, newCommitID, theCommits)
	}

	// Get latest commit date and update to current repository updated time
	commitDate, err := git.GetLatestCommitTime(m.Repo.RepoPath())
	if err != nil {
		log.Error("GetLatestCommitDate [%d]: %v", m.RepoID, err)
		return
	}

	if err = models.UpdateRepositoryUpdatedTime(m.RepoID, commitDate); err != nil {
		log.Error("Update repository 'updated_unix' [%d]: %v", m.RepoID, err)
		return
	}
}

// InitSyncMirrors initializes a go routine to sync the mirrors
func InitSyncMirrors() {
	go graceful.GetManager().RunWithShutdownContext(SyncMirrors)
}

// StartToMirror adds repoID to mirror queue
func StartToMirror(repoID int64) {
	go mirrorQueue.Add(repoID)
}
