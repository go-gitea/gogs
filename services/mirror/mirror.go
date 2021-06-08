// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mirror

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/services"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/sync"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
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
	err := git.LoadGitVersion()
	if err != nil {
		return "", err
	}
	if git.CheckGitVersionAtLeast("2.7") == nil {
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
	u.User = nil
	return u.String()
}

// UpdateAddress writes new address to Git repository and database
func UpdateAddress(m *models.Mirror, addr string) error {
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

	m.Repo.OriginalURL = addr
	return models.UpdateRepositoryCols(m.Repo, "original_url")
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
			if strings.HasPrefix(lines[i], " * [new tag]") {
				refName = git.TagPrefix + refName
			} else if strings.HasPrefix(lines[i], " * [new branch]") {
				refName = git.BranchPrefix + refName
			}
			results = append(results, &mirrorSyncResult{
				refName:     refName,
				oldCommitID: gitShortEmptySha,
			})
		case strings.HasPrefix(lines[i], " - "): // Delete reference
			results = append(results, &mirrorSyncResult{
				refName:     refName,
				newCommitID: gitShortEmptySha,
			})
		case strings.HasPrefix(lines[i], " + "): // Force update
			if idx := strings.Index(refName, " "); idx > -1 {
				refName = refName[:idx]
			}
			delimIdx := strings.Index(lines[i][3:], " ")
			if delimIdx == -1 {
				log.Error("SHA delimiter not found: %q", lines[i])
				continue
			}
			shas := strings.Split(lines[i][3:delimIdx+3], "...")
			if len(shas) != 2 {
				log.Error("Expect two SHAs but not what found: %q", lines[i])
				continue
			}
			results = append(results, &mirrorSyncResult{
				refName:     refName,
				oldCommitID: shas[0],
				newCommitID: shas[1],
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
func runSync(ctx context.Context, m *models.Mirror) ([]*mirrorSyncResult, bool) {
	repoPath := m.Repo.RepoPath()
	wikiPath := m.Repo.WikiPath()
	timeout := time.Duration(setting.Git.Timeout.Mirror) * time.Second

	log.Trace("SyncMirrors [repo: %-v]: running git remote update...", m.Repo)
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
	defer gitRepo.Close()

	log.Trace("SyncMirrors [repo: %-v]: syncing releases with tags...", m.Repo)
	if err = repo_module.SyncReleasesWithTags(m.Repo, gitRepo); err != nil {
		log.Error("Failed to synchronize tags to releases for repository: %v", err)
	}

	if m.LFS && setting.LFS.StartServer {
		log.Trace("SyncMirrors [repo: %-v]: syncing LFS objects...", m.Repo)
		readAddress(m)
		ep := lfs.DetermineEndpoint(m.Address, m.LFSEndpoint)
		if err = repo_module.StoreMissingLfsObjectsInRepository(ctx, m.Repo, gitRepo, ep); err != nil {
			log.Error("Failed to synchronize LFS objects for repository: %v", err)
		}
	}

	log.Trace("SyncMirrors [repo: %-v]: updating size of repository", m.Repo)
	if err := m.Repo.UpdateSize(models.DefaultDBContext()); err != nil {
		log.Error("Failed to update size for mirror repository: %v", err)
	}

	if m.Repo.HasWiki() {
		log.Trace("SyncMirrors [repo: %-v Wiki]: running git remote update...", m.Repo)
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
		log.Trace("SyncMirrors [repo: %-v Wiki]: git remote update complete", m.Repo)
	}

	log.Trace("SyncMirrors [repo: %-v]: invalidating mirror branch caches...", m.Repo)
	branches, _, err := repo_module.GetBranches(m.Repo, 0, 0)
	if err != nil {
		log.Error("GetBranches: %v", err)
		return nil, false
	}

	for _, branch := range branches {
		cache.Remove(m.Repo.GetCommitsCountCacheKey(branch.Name, true))
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
			syncMirror(ctx, repoID)
		}
	}
}

func syncMirror(ctx context.Context, repoID string) {
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

	id, _ := strconv.ParseInt(repoID, 10, 64)
	m, err := models.GetMirrorByRepoID(id)
	if err != nil {
		log.Error("GetMirrorByRepoID [%s]: %v", repoID, err)
		return
	}

	log.Trace("SyncMirrors [repo: %-v]: Running Sync", m.Repo)
	results, ok := runSync(ctx, m)
	if !ok {
		return
	}

	log.Trace("SyncMirrors [repo: %-v]: Scheduling next update", m.Repo)
	m.ScheduleNextUpdate()
	if err = models.UpdateMirror(m); err != nil {
		log.Error("UpdateMirror [%s]: %v", repoID, err)
		return
	}

	var gitRepo *git.Repository
	if len(results) == 0 {
		log.Trace("SyncMirrors [repo: %-v]: no branches updated", m.Repo)
	} else {
		log.Trace("SyncMirrors [repo: %-v]: %d branches updated", m.Repo, len(results))
		gitRepo, err = git.OpenRepository(m.Repo.RepoPath())
		if err != nil {
			log.Error("OpenRepository [%d]: %v", m.RepoID, err)
			return
		}
		defer gitRepo.Close()

		if ok := checkAndUpdateEmptyRepository(m, gitRepo, results); !ok {
			return
		}
	}

	for _, result := range results {
		// Discard GitHub pull requests, i.e. refs/pull/*
		if strings.HasPrefix(result.refName, "refs/pull/") {
			continue
		}

		tp, _ := git.SplitRefName(result.refName)

		// Create reference
		if result.oldCommitID == gitShortEmptySha {
			if tp == git.TagPrefix {
				tp = "tag"
			} else if tp == git.BranchPrefix {
				tp = "branch"
			}
			commitID, err := gitRepo.GetRefCommitID(result.refName)
			if err != nil {
				log.Error("gitRepo.GetRefCommitID [repo_id: %s, ref_name: %s]: %v", m.RepoID, result.refName, err)
				continue
			}
			notification.NotifySyncPushCommits(m.Repo.MustOwner(), m.Repo, &repo_module.PushUpdateOptions{
				RefFullName: result.refName,
				OldCommitID: git.EmptySHA,
				NewCommitID: commitID,
			}, repo_module.NewPushCommits())
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

		notification.NotifySyncPushCommits(m.Repo.MustOwner(), m.Repo, &repo_module.PushUpdateOptions{
			RefFullName: result.refName,
			OldCommitID: oldCommitID,
			NewCommitID: newCommitID,
		}, theCommits)
	}
	log.Trace("SyncMirrors [repo: %-v]: done notifying updated branches/tags - now updating last commit time", m.Repo)

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

	log.Trace("SyncMirrors [repo: %-v]: Successfully updated", m.Repo)
}

func checkAndUpdateEmptyRepository(m *models.Mirror, gitRepo *git.Repository, results []*mirrorSyncResult) bool {
	if !m.Repo.IsEmpty {
		return true
	}

	hasDefault := false
	hasMaster := false
	hasMain := false
	defaultBranchName := m.Repo.DefaultBranch
	if len(defaultBranchName) == 0 {
		defaultBranchName = setting.Repository.DefaultBranch
	}
	firstName := ""
	for _, result := range results {
		if strings.HasPrefix(result.refName, "refs/pull/") {
			continue
		}
		tp, name := git.SplitRefName(result.refName)
		if len(tp) > 0 && tp != git.BranchPrefix {
			continue
		}
		if len(firstName) == 0 {
			firstName = name
		}

		hasDefault = hasDefault || name == defaultBranchName
		hasMaster = hasMaster || name == "master"
		hasMain = hasMain || name == "main"
	}

	if len(firstName) > 0 {
		if hasDefault {
			m.Repo.DefaultBranch = defaultBranchName
		} else if hasMaster {
			m.Repo.DefaultBranch = "master"
		} else if hasMain {
			m.Repo.DefaultBranch = "main"
		} else {
			m.Repo.DefaultBranch = firstName
		}
		// Update the git repository default branch
		if err := gitRepo.SetDefaultBranch(m.Repo.DefaultBranch); err != nil {
			if !git.IsErrUnsupportedVersion(err) {
				log.Error("Failed to update default branch of underlying git repository %-v. Error: %v", m.Repo, err)
				desc := fmt.Sprintf("Failed to uupdate default branch of underlying git repository '%s': %v", m.Repo.RepoPath(), err)
				if err = models.CreateRepositoryNotice(desc); err != nil {
					log.Error("CreateRepositoryNotice: %v", err)
				}
				return false
			}
		}
		m.Repo.IsEmpty = false
		// Update the is empty and default_branch columns
		if err := models.UpdateRepositoryCols(m.Repo, "default_branch", "is_empty"); err != nil {
			log.Error("Failed to update default branch of repository %-v. Error: %v", m.Repo, err)
			desc := fmt.Sprintf("Failed to uupdate default branch of repository '%s': %v", m.Repo.RepoPath(), err)
			if err = models.CreateRepositoryNotice(desc); err != nil {
				log.Error("CreateRepositoryNotice: %v", err)
			}
			return false
		}
	}
	return true
}

// InitSyncMirrors initializes a go routine to sync the mirrors
func InitSyncMirrors() error {
	go graceful.GetManager().RunWithShutdownContext(SyncMirrors)
	return nil
}

func init() {
	services.RegisterService("mirror", InitSyncMirrors, "setting")
}

// StartToMirror adds repoID to mirror queue
func StartToMirror(repoID int64) {
	go mirrorQueue.Add(repoID)
}
