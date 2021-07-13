// Copyright 2020 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package archiver

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
)

// ArchiveRequest defines the parameters of an archive request, which notably
// includes the specific repository being archived as well as the commit, the
// name by which it was requested, and the kind of archive being requested.
// This is entirely opaque to external entities, though, and mostly used as a
// handle elsewhere.
type ArchiveRequest struct {
	RepoID   int64
	refName  string
	Type     git.ArchiveType
	CommitID string
}

// SHA1 hashes will only go up to 40 characters, but SHA256 hashes will go all
// the way to 64.
var shaRegex = regexp.MustCompile(`^[0-9a-f]{4,64}$`)

// NewRequest creates an archival request, based on the URI.  The
// resulting ArchiveRequest is suitable for being passed to ArchiveRepository()
// if it's determined that the request still needs to be satisfied.
func NewRequest(repoID int64, repo *git.Repository, uri string) (*ArchiveRequest, error) {
	r := &ArchiveRequest{
		RepoID: repoID,
	}

	var ext string
	switch {
	case strings.HasSuffix(uri, ".zip"):
		ext = ".zip"
		r.Type = git.ZIP
	case strings.HasSuffix(uri, ".tar.gz"):
		ext = ".tar.gz"
		r.Type = git.TARGZ
	default:
		return nil, fmt.Errorf("Unknown format: %s", uri)
	}

	r.refName = strings.TrimSuffix(uri, ext)

	var err error
	// Get corresponding commit.
	if repo.IsBranchExist(r.refName) {
		r.CommitID, err = repo.GetBranchCommitID(r.refName)
		if err != nil {
			return nil, err
		}
	} else if repo.IsTagExist(r.refName) {
		r.CommitID, err = repo.GetTagCommitID(r.refName)
		if err != nil {
			return nil, err
		}
	} else if shaRegex.MatchString(r.refName) {
		if repo.IsCommitExist(r.refName) {
			r.CommitID = r.refName
		} else {
			return nil, git.ErrNotExist{
				ID: r.refName,
			}
		}
	} else {
		return nil, fmt.Errorf("Unknow ref %s type", r.refName)
	}

	return r, nil
}

// GetArchiveName returns the name of the caller, based on the ref used by the
// caller to create this request.
func (aReq *ArchiveRequest) GetArchiveName() string {
	return strings.ReplaceAll(aReq.refName, "/", "-") + "." + aReq.Type.String()
}

func doArchive(r *ArchiveRequest) (*models.RepoArchiver, error) {
	ctx, commiter, err := models.TxDBContext()
	if err != nil {
		return nil, err
	}
	defer commiter.Close()

	archiver, err := models.GetRepoArchiver(ctx, r.RepoID, r.Type, r.CommitID)
	if err != nil {
		return nil, err
	}

	if archiver != nil {
		// FIXME: If another process are generating it, we think it's not ready and just return
		// Or we should wait until the archive generated.
		if archiver.Status == models.RepoArchiverGenerating {
			return nil, nil
		}
	} else {
		archiver = &models.RepoArchiver{
			RepoID:   r.RepoID,
			Type:     r.Type,
			CommitID: r.CommitID,
			Status:   models.RepoArchiverGenerating,
		}
		if err := models.AddRepoArchiver(ctx, archiver); err != nil {
			return nil, err
		}
	}

	rPath, err := archiver.RelativePath()
	if err != nil {
		return nil, err
	}

	_, err = storage.RepoArchives.Stat(rPath)
	if err == nil {
		if archiver.Status == models.RepoArchiverGenerating {
			archiver.Status = models.RepoArchiverReady
			return archiver, models.UpdateRepoArchiverStatus(ctx, archiver)
		}
		return archiver, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("unable to stat archive: %v", err)
	}

	rd, w := io.Pipe()
	defer func() {
		w.Close()
		rd.Close()
	}()
	var done = make(chan error)
	repo, err := archiver.LoadRepo()
	if err != nil {
		return nil, fmt.Errorf("archiver.LoadRepo failed: %v", err)
	}

	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	go func(done chan error, w *io.PipeWriter, archiver *models.RepoArchiver, gitRepo *git.Repository) {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("%v", r)
			}
		}()

		err = gitRepo.CreateArchive(
			graceful.GetManager().ShutdownContext(),
			archiver.Type,
			w,
			setting.Repository.PrefixArchiveFiles,
			archiver.CommitID,
		)
		_ = w.CloseWithError(err)
		done <- err
	}(done, w, archiver, gitRepo)

	// TODO: add lfs data to zip
	// TODO: add submodule data to zip

	if _, err := storage.RepoArchives.Save(rPath, rd, -1); err != nil {
		return nil, fmt.Errorf("unable to write archive: %v", err)
	}

	err = <-done
	if err != nil {
		return nil, err
	}

	if archiver.Status == models.RepoArchiverGenerating {
		archiver.Status = models.RepoArchiverReady
		if err = models.UpdateRepoArchiverStatus(ctx, archiver); err != nil {
			return nil, err
		}
	}

	return archiver, commiter.Commit()
}

// ArchiveRepository satisfies the ArchiveRequest being passed in.  Processing
// will occur in a separate goroutine, as this phase may take a while to
// complete.  If the archive already exists, ArchiveRepository will not do
// anything.  In all cases, the caller should be examining the *ArchiveRequest
// being returned for completion, as it may be different than the one they passed
// in.
func ArchiveRepository(request *ArchiveRequest) (*models.RepoArchiver, error) {
	return doArchive(request)
}

var archiverQueue queue.UniqueQueue

// Init initlize archive
func Init() error {
	handler := func(data ...queue.Data) {
		for _, datum := range data {
			archiveReq, ok := datum.(*ArchiveRequest)
			if !ok {
				log.Error("Unable to process provided datum: %v - not possible to cast to IndexerData", datum)
				continue
			}
			log.Trace("ArchiverData Process: %#v", archiveReq)
			if _, err := doArchive(archiveReq); err != nil {
				log.Error("Archive %v faild: %v", datum, err)
			}
		}
	}

	archiverQueue = queue.CreateUniqueQueue("repo-archive", handler, new(ArchiveRequest))
	if archiverQueue == nil {
		return errors.New("unable to create codes indexer queue")
	}

	go graceful.GetManager().RunWithShutdownFns(archiverQueue.Run)

	return nil
}

// StartArchive push the archive request to the queue
func StartArchive(request *ArchiveRequest) error {
	has, err := archiverQueue.Has(request)
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	return archiverQueue.Push(request)
}
