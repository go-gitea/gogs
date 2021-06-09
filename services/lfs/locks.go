// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"net/http"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	lfs_module "code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/goccy/go-json"
)

func handleLockListOut(ctx *context.Context, repo *models.Repository, lock *models.LFSLock, err error) {
	if err != nil {
		if models.IsErrLFSLockNotExist(err) {
			ctx.JSON(http.StatusOK, api.LFSLockList{
				Locks: []*api.LFSLock{},
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, api.LFSLockError{
			Message: "unable to list locks : Internal Server Error",
		})
		return
	}
	if repo.ID != lock.RepoID {
		ctx.JSON(http.StatusOK, api.LFSLockList{
			Locks: []*api.LFSLock{},
		})
		return
	}
	ctx.JSON(http.StatusOK, api.LFSLockList{
		Locks: []*api.LFSLock{convert.ToLFSLock(lock)},
	})
}

// GetListLockHandler list locks
func GetListLockHandler(ctx *context.Context) {
	rv := getRequestContext(ctx)

	repository, err := models.GetRepositoryByOwnerAndName(rv.User, rv.Repo)
	if err != nil {
		log.Debug("Could not find repository: %s/%s - %s", rv.User, rv.Repo, err)
		ctx.Resp.Header().Set("WWW-Authenticate", "Basic realm=gitea-lfs")
		ctx.JSON(401, api.LFSLockError{
			Message: "You must have pull access to list locks",
		})
		return
	}
	repository.MustOwner()

	authenticated := authenticate(ctx, repository, rv.Authorization, true, false)
	if !authenticated {
		ctx.Resp.Header().Set("WWW-Authenticate", "Basic realm=gitea-lfs")
		ctx.JSON(http.StatusUnauthorized, api.LFSLockError{
			Message: "You must have pull access to list locks",
		})
		return
	}
	ctx.Resp.Header().Set("Content-Type", lfs_module.MediaType)

	cursor := ctx.QueryInt("cursor")
	if cursor < 0 {
		cursor = 0
	}
	limit := ctx.QueryInt("limit")
	if limit > setting.LFS.LocksPagingNum && setting.LFS.LocksPagingNum > 0 {
		limit = setting.LFS.LocksPagingNum
	} else if limit < 0 {
		limit = 0
	}
	id := ctx.Query("id")
	if id != "" { //Case where we request a specific id
		v, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, api.LFSLockError{
				Message: "bad request : " + err.Error(),
			})
			return
		}
		lock, err := models.GetLFSLockByID(v)
		if err != nil && !models.IsErrLFSLockNotExist(err) {
			log.Error("Unable to get lock with ID[%s]: Error: %v", v, err)
		}
		handleLockListOut(ctx, repository, lock, err)
		return
	}

	path := ctx.Query("path")
	if path != "" { //Case where we request a specific id
		lock, err := models.GetLFSLock(repository, path)
		if err != nil && !models.IsErrLFSLockNotExist(err) {
			log.Error("Unable to get lock for repository %-v with path %s: Error: %v", repository, path, err)
		}
		handleLockListOut(ctx, repository, lock, err)
		return
	}

	//If no query params path or id
	lockList, err := models.GetLFSLockByRepoID(repository.ID, cursor, limit)
	if err != nil {
		log.Error("Unable to list locks for repository ID[%d]: Error: %v", repository.ID, err)
		ctx.JSON(http.StatusInternalServerError, api.LFSLockError{
			Message: "unable to list locks : Internal Server Error",
		})
		return
	}
	lockListAPI := make([]*api.LFSLock, len(lockList))
	next := ""
	for i, l := range lockList {
		lockListAPI[i] = convert.ToLFSLock(l)
	}
	if limit > 0 && len(lockList) == limit {
		next = strconv.Itoa(cursor + 1)
	}
	ctx.JSON(http.StatusOK, api.LFSLockList{
		Locks: lockListAPI,
		Next:  next,
	})
}

// PostLockHandler create lock
func PostLockHandler(ctx *context.Context) {
	userName := ctx.Params("username")
	repoName := strings.TrimSuffix(ctx.Params("reponame"), ".git")
	authorization := ctx.Req.Header.Get("Authorization")

	repository, err := models.GetRepositoryByOwnerAndName(userName, repoName)
	if err != nil {
		log.Error("Unable to get repository: %s/%s Error: %v", userName, repoName, err)
		ctx.Resp.Header().Set("WWW-Authenticate", "Basic realm=gitea-lfs")
		ctx.JSON(401, api.LFSLockError{
			Message: "You must have push access to create locks",
		})
		return
	}
	repository.MustOwner()

	authenticated := authenticate(ctx, repository, authorization, true, true)
	if !authenticated {
		ctx.Resp.Header().Set("WWW-Authenticate", "Basic realm=gitea-lfs")
		ctx.JSON(http.StatusUnauthorized, api.LFSLockError{
			Message: "You must have push access to create locks",
		})
		return
	}

	ctx.Resp.Header().Set("Content-Type", lfs_module.MediaType)

	var req api.LFSLockRequest
	bodyReader := ctx.Req.Body
	defer bodyReader.Close()
	dec := json.NewDecoder(bodyReader)
	if err := dec.Decode(&req); err != nil {
		log.Warn("Failed to decode lock request as json. Error: %v", err)
		writeStatus(ctx, 400)
		return
	}

	lock, err := models.CreateLFSLock(&models.LFSLock{
		Repo:  repository,
		Path:  req.Path,
		Owner: ctx.User,
	})
	if err != nil {
		if models.IsErrLFSLockAlreadyExist(err) {
			ctx.JSON(http.StatusConflict, api.LFSLockError{
				Lock:    convert.ToLFSLock(lock),
				Message: "already created lock",
			})
			return
		}
		if models.IsErrLFSUnauthorizedAction(err) {
			ctx.Resp.Header().Set("WWW-Authenticate", "Basic realm=gitea-lfs")
			ctx.JSON(http.StatusUnauthorized, api.LFSLockError{
				Message: "You must have push access to create locks : " + err.Error(),
			})
			return
		}
		log.Error("Unable to CreateLFSLock in repository %-v at %s for user %-v: Error: %v", repository, req.Path, ctx.User, err)
		ctx.JSON(http.StatusInternalServerError, api.LFSLockError{
			Message: "internal server error : Internal Server Error",
		})
		return
	}
	ctx.JSON(http.StatusCreated, api.LFSLockResponse{Lock: convert.ToLFSLock(lock)})
}

// VerifyLockHandler list locks for verification
func VerifyLockHandler(ctx *context.Context) {
	userName := ctx.Params("username")
	repoName := strings.TrimSuffix(ctx.Params("reponame"), ".git")
	authorization := ctx.Req.Header.Get("Authorization")

	repository, err := models.GetRepositoryByOwnerAndName(userName, repoName)
	if err != nil {
		log.Error("Unable to get repository: %s/%s Error: %v", userName, repoName, err)
		ctx.Resp.Header().Set("WWW-Authenticate", "Basic realm=gitea-lfs")
		ctx.JSON(401, api.LFSLockError{
			Message: "You must have push access to verify locks",
		})
		return
	}
	repository.MustOwner()

	authenticated := authenticate(ctx, repository, authorization, true, true)
	if !authenticated {
		ctx.Resp.Header().Set("WWW-Authenticate", "Basic realm=gitea-lfs")
		ctx.JSON(http.StatusUnauthorized, api.LFSLockError{
			Message: "You must have push access to verify locks",
		})
		return
	}

	ctx.Resp.Header().Set("Content-Type", lfs_module.MediaType)

	cursor := ctx.QueryInt("cursor")
	if cursor < 0 {
		cursor = 0
	}
	limit := ctx.QueryInt("limit")
	if limit > setting.LFS.LocksPagingNum && setting.LFS.LocksPagingNum > 0 {
		limit = setting.LFS.LocksPagingNum
	} else if limit < 0 {
		limit = 0
	}
	lockList, err := models.GetLFSLockByRepoID(repository.ID, cursor, limit)
	if err != nil {
		log.Error("Unable to list locks for repository ID[%d]: Error: %v", repository.ID, err)
		ctx.JSON(http.StatusInternalServerError, api.LFSLockError{
			Message: "unable to list locks : Internal Server Error",
		})
		return
	}
	next := ""
	if limit > 0 && len(lockList) == limit {
		next = strconv.Itoa(cursor + 1)
	}
	lockOursListAPI := make([]*api.LFSLock, 0, len(lockList))
	lockTheirsListAPI := make([]*api.LFSLock, 0, len(lockList))
	for _, l := range lockList {
		if l.Owner.ID == ctx.User.ID {
			lockOursListAPI = append(lockOursListAPI, convert.ToLFSLock(l))
		} else {
			lockTheirsListAPI = append(lockTheirsListAPI, convert.ToLFSLock(l))
		}
	}
	ctx.JSON(http.StatusOK, api.LFSLockListVerify{
		Ours:   lockOursListAPI,
		Theirs: lockTheirsListAPI,
		Next:   next,
	})
}

// UnLockHandler delete locks
func UnLockHandler(ctx *context.Context) {
	userName := ctx.Params("username")
	repoName := strings.TrimSuffix(ctx.Params("reponame"), ".git")
	authorization := ctx.Req.Header.Get("Authorization")

	repository, err := models.GetRepositoryByOwnerAndName(userName, repoName)
	if err != nil {
		log.Error("Unable to get repository: %s/%s Error: %v", userName, repoName, err)
		ctx.Resp.Header().Set("WWW-Authenticate", "Basic realm=gitea-lfs")
		ctx.JSON(401, api.LFSLockError{
			Message: "You must have push access to delete locks",
		})
		return
	}
	repository.MustOwner()

	authenticated := authenticate(ctx, repository, authorization, true, true)
	if !authenticated {
		ctx.Resp.Header().Set("WWW-Authenticate", "Basic realm=gitea-lfs")
		ctx.JSON(http.StatusUnauthorized, api.LFSLockError{
			Message: "You must have push access to delete locks",
		})
		return
	}

	ctx.Resp.Header().Set("Content-Type", lfs_module.MediaType)

	var req api.LFSLockDeleteRequest
	bodyReader := ctx.Req.Body
	defer bodyReader.Close()
	dec := json.NewDecoder(bodyReader)
	if err := dec.Decode(&req); err != nil {
		log.Warn("Failed to decode lock request as json. Error: %v", err)
		writeStatus(ctx, 400)
		return
	}

	lock, err := models.DeleteLFSLockByID(ctx.ParamsInt64("lid"), ctx.User, req.Force)
	if err != nil {
		if models.IsErrLFSUnauthorizedAction(err) {
			ctx.Resp.Header().Set("WWW-Authenticate", "Basic realm=gitea-lfs")
			ctx.JSON(http.StatusUnauthorized, api.LFSLockError{
				Message: "You must have push access to delete locks : " + err.Error(),
			})
			return
		}
		log.Error("Unable to DeleteLFSLockByID[%d] by user %-v with force %t: Error: %v", ctx.ParamsInt64("lid"), ctx.User, req.Force, err)
		ctx.JSON(http.StatusInternalServerError, api.LFSLockError{
			Message: "unable to delete lock : Internal Server Error",
		})
		return
	}
	ctx.JSON(http.StatusOK, api.LFSLockResponse{Lock: convert.ToLFSLock(lock)})
}
