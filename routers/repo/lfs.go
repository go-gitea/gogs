// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"bufio"
	"bytes"
	"fmt"
	gotemplate "html/template"
	"io"
	"io/ioutil"
	"path"
	"strconv"
	"strings"
	"sync"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/pipeline"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
)

const (
	tplSettingsLFS         base.TplName = "repo/settings/lfs"
	tplSettingsLFSLocks    base.TplName = "repo/settings/lfs_locks"
	tplSettingsLFSFile     base.TplName = "repo/settings/lfs_file"
	tplSettingsLFSFileFind base.TplName = "repo/settings/lfs_file_find"
	tplSettingsLFSPointers base.TplName = "repo/settings/lfs_pointers"
)

// LFSFiles shows a repository's LFS files
func LFSFiles(ctx *context.Context) {
	if !setting.LFS.StartServer {
		ctx.NotFound("LFSFiles", nil)
		return
	}
	page := ctx.QueryInt("page")
	if page <= 1 {
		page = 1
	}
	total, err := ctx.Repo.Repository.CountLFSMetaObjects()
	if err != nil {
		ctx.ServerError("LFSFiles", err)
		return
	}
	ctx.Data["Total"] = total

	pager := context.NewPagination(int(total), setting.UI.ExplorePagingNum, page, 5)
	ctx.Data["Title"] = ctx.Tr("repo.settings.lfs")
	ctx.Data["PageIsSettingsLFS"] = true
	lfsMetaObjects, err := ctx.Repo.Repository.GetLFSMetaObjects(pager.Paginater.Current(), setting.UI.ExplorePagingNum)
	if err != nil {
		ctx.ServerError("LFSFiles", err)
		return
	}
	ctx.Data["LFSFiles"] = lfsMetaObjects
	ctx.Data["Page"] = pager
	ctx.HTML(200, tplSettingsLFS)
}

// LFSLocks shows a repository's LFS locks
func LFSLocks(ctx *context.Context) {
	if !setting.LFS.StartServer {
		ctx.NotFound("LFSLocks", nil)
		return
	}
	ctx.Data["LFSFilesLink"] = ctx.Repo.RepoLink + "/settings/lfs"

	page := ctx.QueryInt("page")
	if page <= 1 {
		page = 1
	}
	total, err := models.CountLFSLockByRepoID(ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("LFSLocks", err)
		return
	}
	ctx.Data["Total"] = total

	pager := context.NewPagination(int(total), setting.UI.ExplorePagingNum, page, 5)
	ctx.Data["Title"] = ctx.Tr("repo.settings.lfs_locks")
	ctx.Data["PageIsSettingsLFS"] = true
	lfsLocks, err := models.GetLFSLockByRepoID(ctx.Repo.Repository.ID, pager.Paginater.Current(), setting.UI.ExplorePagingNum)
	if err != nil {
		ctx.ServerError("LFSLocks", err)
		return
	}
	ctx.Data["LFSLocks"] = lfsLocks

	if len(lfsLocks) == 0 {
		ctx.Data["Page"] = pager
		ctx.HTML(200, tplSettingsLFSLocks)
		return
	}

	// Clone base repo.
	tmpBasePath, err := models.CreateTemporaryPath("locks")
	if err != nil {
		log.Error("Failed to create temporary path: %v", err)
		ctx.ServerError("LFSLocks", err)
		return
	}
	defer func() {
		if err := models.RemoveTemporaryPath(tmpBasePath); err != nil {
			log.Error("LFSLocks: RemoveTemporaryPath: %v", err)
		}
	}()

	if err := git.Clone(ctx.Repo.Repository.RepoPath(), tmpBasePath, git.CloneRepoOptions{
		Bare:   true,
		Shared: true,
	}); err != nil {
		log.Error("Failed to clone repository: %s (%v)", ctx.Repo.Repository.FullName(), err)
		ctx.ServerError("LFSLocks", fmt.Errorf("Failed to clone repository: %s (%v)", ctx.Repo.Repository.FullName(), err))
		return
	}

	gitRepo, err := git.OpenRepository(tmpBasePath)
	if err != nil {
		log.Error("Unable to open temporary repository: %s (%v)", tmpBasePath, err)
		ctx.ServerError("LFSLocks", fmt.Errorf("Failed to open new temporary repository in: %s %v", tmpBasePath, err))
		return
	}
	defer gitRepo.Close()

	filenames := make([]string, len(lfsLocks))

	for i, lock := range lfsLocks {
		filenames[i] = lock.Path
	}

	if err := gitRepo.ReadTreeToIndex(ctx.Repo.Repository.DefaultBranch); err != nil {
		log.Error("Unable to read the default branch to the index: %s (%v)", ctx.Repo.Repository.DefaultBranch, err)
		ctx.ServerError("LFSLocks", fmt.Errorf("Unable to read the default branch to the index: %s (%v)", ctx.Repo.Repository.DefaultBranch, err))
		return
	}

	name2attribute2info, err := gitRepo.CheckAttribute(git.CheckAttributeOpts{
		Attributes: []string{"lockable"},
		Filenames:  filenames,
		CachedOnly: true,
	})
	if err != nil {
		log.Error("Unable to check attributes in %s (%v)", tmpBasePath, err)
		ctx.ServerError("LFSLocks", err)
		return
	}

	lockables := make([]bool, len(lfsLocks))
	for i, lock := range lfsLocks {
		attribute2info, has := name2attribute2info[lock.Path]
		if !has {
			continue
		}
		if attribute2info["lockable"] != "set" {
			continue
		}
		lockables[i] = true
	}
	ctx.Data["Lockables"] = lockables

	filelist, err := gitRepo.LsFiles(filenames...)
	if err != nil {
		log.Error("Unable to lsfiles in %s (%v)", tmpBasePath, err)
		ctx.ServerError("LFSLocks", err)
		return
	}

	filemap := make(map[string]bool, len(filelist))
	for _, name := range filelist {
		filemap[name] = true
	}

	linkable := make([]bool, len(lfsLocks))
	for i, lock := range lfsLocks {
		linkable[i] = filemap[lock.Path]
	}
	ctx.Data["Linkable"] = linkable

	ctx.Data["Page"] = pager
	ctx.HTML(200, tplSettingsLFSLocks)
}

// LFSLockFile locks a file
func LFSLockFile(ctx *context.Context) {
	if !setting.LFS.StartServer {
		ctx.NotFound("LFSLocks", nil)
		return
	}
	originalPath := ctx.Query("path")
	lockPath := originalPath
	if len(lockPath) == 0 {
		ctx.Flash.Error(ctx.Tr("repo.settings.lfs_invalid_locking_path", originalPath))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/lfs/locks")
		return
	}
	if lockPath[len(lockPath)-1] == '/' {
		ctx.Flash.Error(ctx.Tr("repo.settings.lfs_invalid_lock_directory", originalPath))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/lfs/locks")
		return
	}
	lockPath = path.Clean("/" + lockPath)[1:]
	if len(lockPath) == 0 {
		ctx.Flash.Error(ctx.Tr("repo.settings.lfs_invalid_locking_path", originalPath))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/lfs/locks")
		return
	}

	_, err := models.CreateLFSLock(&models.LFSLock{
		Repo:  ctx.Repo.Repository,
		Path:  lockPath,
		Owner: ctx.User,
	})
	if err != nil {
		if models.IsErrLFSLockAlreadyExist(err) {
			ctx.Flash.Error(ctx.Tr("repo.settings.lfs_lock_already_exists", originalPath))
			ctx.Redirect(ctx.Repo.RepoLink + "/settings/lfs/locks")
			return
		}
		ctx.ServerError("LFSLockFile", err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/lfs/locks")
}

// LFSUnlock forcibly unlocks an LFS lock
func LFSUnlock(ctx *context.Context) {
	if !setting.LFS.StartServer {
		ctx.NotFound("LFSUnlock", nil)
		return
	}
	_, err := models.DeleteLFSLockByID(ctx.ParamsInt64("lid"), ctx.User, true)
	if err != nil {
		ctx.ServerError("LFSUnlock", err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/lfs/locks")
}

// LFSFileGet serves a single LFS file
func LFSFileGet(ctx *context.Context) {
	if !setting.LFS.StartServer {
		ctx.NotFound("LFSFileGet", nil)
		return
	}
	ctx.Data["LFSFilesLink"] = ctx.Repo.RepoLink + "/settings/lfs"
	oid := ctx.Params("oid")
	ctx.Data["Title"] = oid
	ctx.Data["PageIsSettingsLFS"] = true
	meta, err := ctx.Repo.Repository.GetLFSMetaObjectByOid(oid)
	if err != nil {
		if err == models.ErrLFSObjectNotExist {
			ctx.NotFound("LFSFileGet", nil)
			return
		}
		ctx.ServerError("LFSFileGet", err)
		return
	}
	ctx.Data["LFSFile"] = meta
	dataRc, err := lfs.ReadMetaObject(meta)
	if err != nil {
		ctx.ServerError("LFSFileGet", err)
		return
	}
	defer dataRc.Close()
	buf := make([]byte, 1024)
	n, err := dataRc.Read(buf)
	if err != nil {
		ctx.ServerError("Data", err)
		return
	}
	buf = buf[:n]

	ctx.Data["IsTextFile"] = base.IsTextFile(buf)
	isRepresentableAsText := base.IsRepresentableAsText(buf)

	fileSize := meta.Size
	ctx.Data["FileSize"] = meta.Size
	ctx.Data["RawFileLink"] = fmt.Sprintf("%s%s.git/info/lfs/objects/%s/%s", setting.AppURL, ctx.Repo.Repository.FullName(), meta.Oid, "direct")
	switch {
	case isRepresentableAsText:
		// This will be true for SVGs.
		if base.IsImageFile(buf) {
			ctx.Data["IsImageFile"] = true
		}

		if fileSize >= setting.UI.MaxDisplayFileSize {
			ctx.Data["IsFileTooLarge"] = true
			break
		}

		d, _ := ioutil.ReadAll(dataRc)
		buf = charset.ToUTF8WithFallback(append(buf, d...))

		// Building code view blocks with line number on server side.
		var fileContent string
		if content, err := charset.ToUTF8WithErr(buf); err != nil {
			log.Error("ToUTF8WithErr: %v", err)
			fileContent = string(buf)
		} else {
			fileContent = content
		}

		var output bytes.Buffer
		lines := strings.Split(fileContent, "\n")
		// Remove blank line at the end of file
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		for index, line := range lines {
			line = gotemplate.HTMLEscapeString(line)
			if index != len(lines)-1 {
				line += "\n"
			}
			output.WriteString(fmt.Sprintf(`<li class="L%d" rel="L%d">%s</li>`, index+1, index+1, line))
		}
		ctx.Data["FileContent"] = gotemplate.HTML(output.String())

		output.Reset()
		for i := 0; i < len(lines); i++ {
			output.WriteString(fmt.Sprintf(`<span id="L%d">%d</span>`, i+1, i+1))
		}
		ctx.Data["LineNums"] = gotemplate.HTML(output.String())

	case base.IsPDFFile(buf):
		ctx.Data["IsPDFFile"] = true
	case base.IsVideoFile(buf):
		ctx.Data["IsVideoFile"] = true
	case base.IsAudioFile(buf):
		ctx.Data["IsAudioFile"] = true
	case base.IsImageFile(buf):
		ctx.Data["IsImageFile"] = true
	}
	ctx.HTML(200, tplSettingsLFSFile)
}

// LFSDelete disassociates the provided oid from the repository and if the lfs file is no longer associated with any repositories - deletes it
func LFSDelete(ctx *context.Context) {
	if !setting.LFS.StartServer {
		ctx.NotFound("LFSDelete", nil)
		return
	}
	oid := ctx.Params("oid")
	count, err := ctx.Repo.Repository.RemoveLFSMetaObjectByOid(oid)
	if err != nil {
		ctx.ServerError("LFSDelete", err)
		return
	}
	// FIXME: Warning: the LFS store is not locked - and can't be locked - there could be a race condition here
	// Please note a similar condition happens in models/repo.go DeleteRepository
	if count == 0 {
		oidPath := path.Join(oid[0:2], oid[2:4], oid[4:])
		err = storage.LFS.Delete(oidPath)
		if err != nil {
			ctx.ServerError("LFSDelete", err)
			return
		}
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/lfs")
}

// LFSFileFind guesses a sha for the provided oid (or uses the provided sha) and then finds the commits that contain this sha
func LFSFileFind(ctx *context.Context) {
	if !setting.LFS.StartServer {
		ctx.NotFound("LFSFind", nil)
		return
	}
	oid := ctx.Query("oid")
	size := ctx.QueryInt64("size")
	if len(oid) == 0 || size == 0 {
		ctx.NotFound("LFSFind", nil)
		return
	}
	sha := ctx.Query("sha")
	ctx.Data["Title"] = oid
	ctx.Data["PageIsSettingsLFS"] = true
	var hash git.SHA1
	if len(sha) == 0 {
		meta := models.LFSMetaObject{Oid: oid, Size: size}
		pointer := meta.Pointer()
		hash = git.ComputeBlobHash([]byte(pointer))
		sha = hash.String()
	} else {
		hash = git.MustIDFromString(sha)
	}
	ctx.Data["LFSFilesLink"] = ctx.Repo.RepoLink + "/settings/lfs"
	ctx.Data["Oid"] = oid
	ctx.Data["Size"] = size
	ctx.Data["SHA"] = sha

	results, err := pipeline.FindLFSFile(ctx.Repo.GitRepo, hash)
	if err != nil && err != io.EOF {
		log.Error("Failure in FindLFSFile: %v", err)
		ctx.ServerError("LFSFind: FindLFSFile.", err)
		return
	}

	ctx.Data["Results"] = results
	ctx.HTML(200, tplSettingsLFSFileFind)
}

// LFSPointerFiles will search the repository for pointer files and report which are missing LFS files in the content store
func LFSPointerFiles(ctx *context.Context) {
	if !setting.LFS.StartServer {
		ctx.NotFound("LFSFileGet", nil)
		return
	}
	ctx.Data["PageIsSettingsLFS"] = true
	err := git.LoadGitVersion()
	if err != nil {
		log.Fatal("Error retrieving git version: %v", err)
	}
	ctx.Data["LFSFilesLink"] = ctx.Repo.RepoLink + "/settings/lfs"

	basePath := ctx.Repo.Repository.RepoPath()

	pointerChan := make(chan pointerResult)

	catFileCheckReader, catFileCheckWriter := io.Pipe()
	shasToBatchReader, shasToBatchWriter := io.Pipe()
	catFileBatchReader, catFileBatchWriter := io.Pipe()
	errChan := make(chan error, 1)
	wg := sync.WaitGroup{}
	wg.Add(5)

	var numPointers, numAssociated, numNoExist, numAssociatable int

	go func() {
		defer wg.Done()
		pointers := make([]pointerResult, 0, 50)
		for pointer := range pointerChan {
			pointers = append(pointers, pointer)
			if pointer.InRepo {
				numAssociated++
			}
			if !pointer.Exists {
				numNoExist++
			}
			if !pointer.InRepo && pointer.Accessible {
				numAssociatable++
			}
		}
		numPointers = len(pointers)
		ctx.Data["Pointers"] = pointers
		ctx.Data["NumPointers"] = numPointers
		ctx.Data["NumAssociated"] = numAssociated
		ctx.Data["NumAssociatable"] = numAssociatable
		ctx.Data["NumNoExist"] = numNoExist
		ctx.Data["NumNotAssociated"] = numPointers - numAssociated
	}()
	go createPointerResultsFromCatFileBatch(catFileBatchReader, &wg, pointerChan, ctx.Repo.Repository, ctx.User)
	go pipeline.CatFileBatch(shasToBatchReader, catFileBatchWriter, &wg, basePath)
	go pipeline.BlobsLessThan1024FromCatFileBatchCheck(catFileCheckReader, shasToBatchWriter, &wg)
	if git.CheckGitVersionAtLeast("2.6.0") != nil {
		revListReader, revListWriter := io.Pipe()
		shasToCheckReader, shasToCheckWriter := io.Pipe()
		wg.Add(2)
		go pipeline.CatFileBatchCheck(shasToCheckReader, catFileCheckWriter, &wg, basePath)
		go pipeline.BlobsFromRevListObjects(revListReader, shasToCheckWriter, &wg)
		go pipeline.RevListAllObjects(revListWriter, &wg, basePath, errChan)
	} else {
		go pipeline.CatFileBatchCheckAllObjects(catFileCheckWriter, &wg, basePath, errChan)
	}
	wg.Wait()

	select {
	case err, has := <-errChan:
		if has {
			ctx.ServerError("LFSPointerFiles", err)
		}
	default:
	}
	ctx.HTML(200, tplSettingsLFSPointers)
}

type pointerResult struct {
	SHA        string
	Oid        string
	Size       int64
	InRepo     bool
	Exists     bool
	Accessible bool
}

func createPointerResultsFromCatFileBatch(catFileBatchReader *io.PipeReader, wg *sync.WaitGroup, pointerChan chan<- pointerResult, repo *models.Repository, user *models.User) {
	defer wg.Done()
	defer catFileBatchReader.Close()
	contentStore := lfs.ContentStore{ObjectStorage: storage.LFS}

	bufferedReader := bufio.NewReader(catFileBatchReader)
	buf := make([]byte, 1025)
	for {
		// File descriptor line: sha
		sha, err := bufferedReader.ReadString(' ')
		if err != nil {
			_ = catFileBatchReader.CloseWithError(err)
			break
		}
		// Throw away the blob
		if _, err := bufferedReader.ReadString(' '); err != nil {
			_ = catFileBatchReader.CloseWithError(err)
			break
		}
		sizeStr, err := bufferedReader.ReadString('\n')
		if err != nil {
			_ = catFileBatchReader.CloseWithError(err)
			break
		}
		size, err := strconv.Atoi(sizeStr[:len(sizeStr)-1])
		if err != nil {
			_ = catFileBatchReader.CloseWithError(err)
			break
		}
		pointerBuf := buf[:size+1]
		if _, err := io.ReadFull(bufferedReader, pointerBuf); err != nil {
			_ = catFileBatchReader.CloseWithError(err)
			break
		}
		pointerBuf = pointerBuf[:size]
		// Now we need to check if the pointerBuf is an LFS pointer
		pointer := lfs.IsPointerFile(&pointerBuf)
		if pointer == nil {
			continue
		}

		result := pointerResult{
			SHA:  strings.TrimSpace(sha),
			Oid:  pointer.Oid,
			Size: pointer.Size,
		}

		// Then we need to check that this pointer is in the db
		if _, err := repo.GetLFSMetaObjectByOid(pointer.Oid); err != nil {
			if err != models.ErrLFSObjectNotExist {
				_ = catFileBatchReader.CloseWithError(err)
				break
			}
		} else {
			result.InRepo = true
		}

		result.Exists, err = contentStore.Exists(pointer)
		if err != nil {
			_ = catFileBatchReader.CloseWithError(err)
			break
		}

		if result.Exists {
			if !result.InRepo {
				// Can we fix?
				// OK well that's "simple"
				// - we need to check whether current user has access to a repo that has access to the file
				result.Accessible, err = models.LFSObjectAccessible(user, result.Oid)
				if err != nil {
					_ = catFileBatchReader.CloseWithError(err)
					break
				}
			} else {
				result.Accessible = true
			}
		}
		pointerChan <- result
	}
	close(pointerChan)
}

// LFSAutoAssociate auto associates accessible lfs files
func LFSAutoAssociate(ctx *context.Context) {
	if !setting.LFS.StartServer {
		ctx.NotFound("LFSAutoAssociate", nil)
		return
	}
	oids := ctx.QueryStrings("oid")
	metas := make([]*models.LFSMetaObject, len(oids))
	for i, oid := range oids {
		idx := strings.IndexRune(oid, ' ')
		if idx < 0 || idx+1 > len(oid) {
			ctx.ServerError("LFSAutoAssociate", fmt.Errorf("Illegal oid input: %s", oid))
			return
		}
		var err error
		metas[i] = &models.LFSMetaObject{}
		metas[i].Size, err = strconv.ParseInt(oid[idx+1:], 10, 64)
		if err != nil {
			ctx.ServerError("LFSAutoAssociate", fmt.Errorf("Illegal oid input: %s %v", oid, err))
			return
		}
		metas[i].Oid = oid[:idx]
		// metas[i].RepositoryID = ctx.Repo.Repository.ID
	}
	if err := models.LFSAutoAssociate(metas, ctx.User, ctx.Repo.Repository.ID); err != nil {
		ctx.ServerError("LFSAutoAssociate", err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/lfs")
}
