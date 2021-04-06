// Copyright 2020 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package archiver

import (
	"io"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// ArchiveRequest defines the parameters of an archive request, which notably
// includes the specific repository being archived as well as the commit, the
// name by which it was requested, and the kind of archive being requested.
// This is entirely opaque to external entities, though, and mostly used as a
// handle elsewhere.
type ArchiveRequest struct {
	uri             string
	repo            *git.Repository
	refName         string
	ext             string
	archivePath     string
	archiveType     git.ArchiveType
	archiveComplete bool
	commit          *git.Commit
	cchan           chan struct{}
}

var archiveInProgress []*ArchiveRequest
var archiveMutex sync.Mutex

// SHA1 hashes will only go up to 40 characters, but SHA256 hashes will go all
// the way to 64.
var shaRegex = regexp.MustCompile(`^[0-9a-f]{4,64}$`)

// These facilitate testing, by allowing the unit tests to control (to some extent)
// the goroutine used for processing the queue.
var archiveQueueMutex *sync.Mutex
var archiveQueueStartCond *sync.Cond
var archiveQueueReleaseCond *sync.Cond

// ArchiveType exposes the underlying archive type without allowing it to be modified
func (aReq *ArchiveRequest) ArchiveType() git.ArchiveType {
	return aReq.archiveType
}

// GetArchivePath returns the path from which we can serve this archive.
func (aReq *ArchiveRequest) GetArchivePath() string {
	return aReq.archivePath
}

// GetArchiveName returns the name of the caller, based on the ref used by the
// caller to create this request.
func (aReq *ArchiveRequest) GetArchiveName() string {
	return aReq.refName + aReq.ext
}

// IsComplete returns the completion status of this request.
func (aReq *ArchiveRequest) IsComplete() bool {
	return aReq.archiveComplete
}

// WaitForCompletion will wait for this request to complete, with no timeout.
// It returns whether the archive was actually completed, as the channel could
// have also been closed due to an error.
func (aReq *ArchiveRequest) WaitForCompletion(ctx *context.Context) bool {
	select {
	case <-aReq.cchan:
	case <-ctx.Req.Context().Done():
	}

	return aReq.IsComplete()
}

// TimedWaitForCompletion will wait for this request to complete, with timeout
// happening after the specified Duration.  It returns whether the archive is
// now complete and whether we hit the timeout or not.  The latter may not be
// useful if the request is complete or we started to shutdown.
func (aReq *ArchiveRequest) TimedWaitForCompletion(ctx *context.Context, dur time.Duration) (bool, bool) {
	timeout := false
	select {
	case <-time.After(dur):
		timeout = true
	case <-aReq.cchan:
	case <-ctx.Req.Context().Done():
	}

	return aReq.IsComplete(), timeout
}

// The caller must hold the archiveMutex across calls to getArchiveRequest.
func getArchiveRequest(repo *git.Repository, commit *git.Commit, archiveType git.ArchiveType) *ArchiveRequest {
	for _, r := range archiveInProgress {
		// Need to be referring to the same repository.
		if r.repo.Path == repo.Path && r.commit.ID == commit.ID && r.archiveType == archiveType {
			return r
		}
	}
	return nil
}

// DeriveRequestFrom creates an archival request, based on the URI.  The
// resulting ArchiveRequest is suitable for being passed to ArchiveRepository()
// if it's determined that the request still needs to be satisfied.
func DeriveRequestFrom(ctx *context.Context, uri string) *ArchiveRequest {
	if ctx.Repo == nil || ctx.Repo.GitRepo == nil {
		log.Trace("Repo not initialized")
		return nil
	}
	r := &ArchiveRequest{
		uri:  uri,
		repo: ctx.Repo.GitRepo,
	}

	switch {
	case strings.HasSuffix(uri, ".zip"):
		r.ext = ".zip"
		r.archivePath = path.Join(r.repo.Path, "archives/zip")
		r.archiveType = git.ZIP
	case strings.HasSuffix(uri, ".tar.gz"):
		r.ext = ".tar.gz"
		r.archivePath = path.Join(r.repo.Path, "archives/targz")
		r.archiveType = git.TARGZ
	case strings.HasSuffix(uri, ".bundle"):
		r.ext = ".bundle"
		r.archivePath = path.Join(r.repo.Path, "archives/bundle")
		r.archiveType = git.BUNDLE
	default:
		log.Trace("Unknown format: %s", uri)
		return nil
	}

	r.refName = strings.TrimSuffix(r.uri, r.ext)
	isDir, err := util.IsDir(r.archivePath)
	if err != nil {
		ctx.ServerError("Download -> util.IsDir(archivePath)", err)
		return nil
	}
	if !isDir {
		if err := os.MkdirAll(r.archivePath, os.ModePerm); err != nil {
			ctx.ServerError("Download -> os.MkdirAll(archivePath)", err)
			return nil
		}
	}

	// Get corresponding commit.
	if r.repo.IsBranchExist(r.refName) {
		r.commit, err = r.repo.GetBranchCommit(r.refName)
		if err != nil {
			ctx.ServerError("GetBranchCommit", err)
			return nil
		}
	} else if r.repo.IsTagExist(r.refName) {
		r.commit, err = r.repo.GetTagCommit(r.refName)
		if err != nil {
			ctx.ServerError("GetTagCommit", err)
			return nil
		}
	} else if shaRegex.MatchString(r.refName) {
		r.commit, err = r.repo.GetCommit(r.refName)
		if err != nil {
			ctx.NotFound("GetCommit", nil)
			return nil
		}
	} else {
		ctx.NotFound("DeriveRequestFrom", nil)
		return nil
	}

	archiveMutex.Lock()
	defer archiveMutex.Unlock()
	if rExisting := getArchiveRequest(r.repo, r.commit, r.archiveType); rExisting != nil {
		return rExisting
	}

	r.archivePath = path.Join(r.archivePath, base.ShortSha(r.commit.ID.String())+r.ext)
	r.archiveComplete, err = util.IsFile(r.archivePath)
	if err != nil {
		ctx.ServerError("util.IsFile", err)
		return nil
	}
	return r
}

func doArchive(r *ArchiveRequest) {
	var (
		err         error
		tmpArchive  *os.File
		destArchive *os.File
	)

	// Close the channel to indicate to potential waiters that this request
	// has finished.
	defer close(r.cchan)

	// It could have happened that we enqueued two archival requests, due to
	// race conditions and difficulties in locking.  Do one last check that
	// the archive we're referring to doesn't already exist.  If it does exist,
	// then just mark the request as complete and move on.
	isFile, err := util.IsFile(r.archivePath)
	if err != nil {
		log.Error("Unable to check if %s util.IsFile: %v. Will ignore and recreate.", r.archivePath, err)
	}
	if isFile {
		r.archiveComplete = true
		return
	}

	// Create a temporary file to use while the archive is being built.  We
	// will then copy it into place (r.archivePath) once it's fully
	// constructed.
	tmpArchive, err = ioutil.TempFile("", "archive")
	if err != nil {
		log.Error("Unable to create a temporary archive file! Error: %v", err)
		return
	}
	defer func() {
		tmpArchive.Close()
		os.Remove(tmpArchive.Name())
	}()

	if r.archiveType == git.BUNDLE {
		err = r.repo.CreateBundle(graceful.GetManager().ShutdownContext(), tmpArchive.Name())
		if err != nil {
			log.Error("Download -> CreateBundle "+tmpArchive.Name(), err)
			return
		}
		// Bundle needed to be closed to finish writing, Sync did not seem to work...
		if err := tmpArchive.Close(); err != nil {
			log.Error("Download -> CreateBundle -> Close "+tmpArchive.Name(), err)
			return
		}
		if tmpArchive, err = os.Open(tmpArchive.Name()); err != nil {
			log.Error("Download -> CreateBundle -> Open ", err)
			return
		}
	} else {
		err = r.commit.CreateArchive(graceful.GetManager().ShutdownContext(), tmpArchive.Name(), git.CreateArchiveOpts{
			Format: r.archiveType,
			Prefix: setting.Repository.PrefixArchiveFiles,
		})
		if err != nil {
			log.Error("Download -> CreateArchive "+tmpArchive.Name(), err)
			return
		}
	}

	// Now we copy it into place
	if destArchive, err = os.Create(r.archivePath); err != nil {
		log.Error("Unable to open archive " + r.archivePath)
		return
	}
	_, err = io.Copy(destArchive, tmpArchive)
	destArchive.Close()
	if err != nil {
		log.Error("Unable to write archive " + r.archivePath)
		return
	}

	// Block any attempt to finalize creating a new request if we're marking
	r.archiveComplete = true
}

// ArchiveRepository satisfies the ArchiveRequest being passed in.  Processing
// will occur in a separate goroutine, as this phase may take a while to
// complete.  If the archive already exists, ArchiveRepository will not do
// anything.  In all cases, the caller should be examining the *ArchiveRequest
// being returned for completion, as it may be different than the one they passed
// in.
func ArchiveRepository(request *ArchiveRequest) *ArchiveRequest {
	// We'll return the request that's already been enqueued if it has been
	// enqueued, or we'll immediately enqueue it if it has not been enqueued
	// and it is not marked complete.
	archiveMutex.Lock()
	defer archiveMutex.Unlock()
	if rExisting := getArchiveRequest(request.repo, request.commit, request.archiveType); rExisting != nil {
		return rExisting
	}
	if request.archiveComplete {
		return request
	}

	request.cchan = make(chan struct{})
	archiveInProgress = append(archiveInProgress, request)
	go func() {
		// Wait to start, if we have the Cond for it.  This is currently only
		// useful for testing, so that the start and release of queued entries
		// can be controlled to examine the queue.
		if archiveQueueStartCond != nil {
			archiveQueueMutex.Lock()
			archiveQueueStartCond.Wait()
			archiveQueueMutex.Unlock()
		}

		// Drop the mutex while we process the request.  This may take a long
		// time, and it's not necessary now that we've added the reequest to
		// archiveInProgress.
		doArchive(request)

		if archiveQueueReleaseCond != nil {
			archiveQueueMutex.Lock()
			archiveQueueReleaseCond.Wait()
			archiveQueueMutex.Unlock()
		}

		// Purge this request from the list.  To do so, we'll just take the
		// index at which we ended up at and swap the final element into that
		// position, then chop off the now-redundant final element.  The slice
		// may have change in between these two segments and we may have moved,
		// so we search for it here.  We could perhaps avoid this search
		// entirely if len(archiveInProgress) == 1, but we should verify
		// correctness.
		archiveMutex.Lock()
		defer archiveMutex.Unlock()

		idx := -1
		for _idx, req := range archiveInProgress {
			if req == request {
				idx = _idx
				break
			}
		}
		if idx == -1 {
			log.Error("ArchiveRepository: Failed to find request for removal.")
			return
		}
		archiveInProgress = append(archiveInProgress[:idx], archiveInProgress[idx+1:]...)
	}()

	return request
}
