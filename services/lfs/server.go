// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	lfs_module "code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/dgrijalva/jwt-go"
	"github.com/goccy/go-json"
)

// requestContext contain variables from the HTTP request.
type requestContext struct {
	User          string
	Repo          string
	Authorization string
}

// Claims is a JWT Token Claims
type Claims struct {
	RepoID int64
	Op     string
	UserID int64
	jwt.StandardClaims
}

// DownloadLink builds a URL to download the object.
func (rc *requestContext) DownloadLink(p lfs_module.Pointer) string {
	return setting.AppURL + path.Join(rc.User, rc.Repo+".git", "info/lfs/objects", p.Oid)
}

// UploadLink builds a URL to upload the object.
func (rc *requestContext) UploadLink(p lfs_module.Pointer) string {
	return setting.AppURL + path.Join(rc.User, rc.Repo+".git", "info/lfs/objects", p.Oid, strconv.FormatInt(p.Size, 10))
}

// VerifyLink builds a URL for verifying the object.
func (rc *requestContext) VerifyLink(p lfs_module.Pointer) string {
	return setting.AppURL + path.Join(rc.User, rc.Repo+".git", "info/lfs/verify")
}

// CheckAcceptMediaType checks if the client accepts the LFS media type.
func CheckAcceptMediaType(ctx *context.Context) {
	mediaParts := strings.Split(ctx.Req.Header.Get("Accept"), ";")

	if mediaParts[0] != lfs_module.MediaType {
		log.Trace("Calling a LFS method without accepting the correct media type: %s", lfs_module.MediaType)
		writeStatus(ctx, http.StatusUnsupportedMediaType)
		return
	}
}

// DownloadHandler gets the content from the content store
func DownloadHandler(ctx *context.Context) {
	rc := getRequestContext(ctx)
	p := lfs_module.Pointer{Oid: ctx.Params("oid")}

	meta := getAuthenticatedMeta(ctx, rc, p, false)
	if meta == nil {
		return
	}

	// Support resume download using Range header
	var fromByte, toByte int64
	toByte = meta.Size - 1
	statusCode := http.StatusOK
	if rangeHdr := ctx.Req.Header.Get("Range"); rangeHdr != "" {
		regex := regexp.MustCompile(`bytes=(\d+)\-(\d*).*`)
		match := regex.FindStringSubmatch(rangeHdr)
		if len(match) > 1 {
			statusCode = http.StatusPartialContent
			fromByte, _ = strconv.ParseInt(match[1], 10, 32)

			if fromByte >= meta.Size {
				writeStatus(ctx, http.StatusRequestedRangeNotSatisfiable)
				return
			}

			if match[2] != "" {
				_toByte, _ := strconv.ParseInt(match[2], 10, 32)
				if _toByte >= fromByte && _toByte < toByte {
					toByte = _toByte
				}
			}

			ctx.Resp.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", fromByte, toByte, meta.Size-fromByte))
			ctx.Resp.Header().Set("Access-Control-Expose-Headers", "Content-Range")
		}
	}

	contentStore := lfs_module.NewContentStore()
	content, err := contentStore.Get(meta.Pointer)
	if err != nil {
		writeStatus(ctx, http.StatusNotFound)
		return
	}
	defer content.Close()

	if fromByte > 0 {
		_, err = content.Seek(fromByte, io.SeekStart)
		if err != nil {
			log.Error("Whilst trying to read LFS OID[%s]: Unable to seek to %d Error: %v", meta.Oid, fromByte, err)

			writeStatus(ctx, http.StatusInternalServerError)
			return
		}
	}

	contentLength := toByte + 1 - fromByte
	ctx.Resp.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	ctx.Resp.Header().Set("Content-Type", "application/octet-stream")

	filename := ctx.Params("filename")
	if len(filename) > 0 {
		decodedFilename, err := base64.RawURLEncoding.DecodeString(filename)
		if err == nil {
			ctx.Resp.Header().Set("Content-Disposition", "attachment; filename=\""+string(decodedFilename)+"\"")
			ctx.Resp.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
		}
	}

	ctx.Resp.WriteHeader(statusCode)
	if written, err := io.CopyN(ctx.Resp, content, contentLength); err != nil {
		log.Error("Error whilst copying LFS OID[%s] to the response after %d bytes. Error: %v", meta.Oid, written, err)
	}
}

// BatchHandler provides the batch api
func BatchHandler(ctx *context.Context) {
	var br lfs_module.BatchRequest
	if err := decodeJSON(ctx.Req, &br); err != nil {
		log.Trace("Unable to decode BATCH request vars: Error: %v", err)
		writeStatus(ctx, http.StatusBadRequest)
		return
	}

	var isUpload bool
	if br.Operation == "upload" {
		isUpload = true
	} else if br.Operation == "download" {
		isUpload = false
	} else {
		log.Trace("Attempt to BATCH with invalid operation: %s", br.Operation)
		writeStatus(ctx, http.StatusBadRequest)
		return
	}

	rc := getRequestContext(ctx)

	repository := getAuthenticatedRepository(ctx, rc, isUpload)
	if repository == nil {
		return
	}

	contentStore := lfs_module.NewContentStore()

	var responseObjects []*lfs_module.ObjectResponse

	for _, p := range br.Objects {
		if !p.IsValid() {
			responseObjects = append(responseObjects, buildObjectResponse(rc, p, false, false, &lfs_module.ObjectError{
				Code:    http.StatusUnprocessableEntity,
				Message: "Oid or size are invalid",
			}))
			continue
		}

		exists, err := contentStore.Exists(p)
		if err != nil {
			log.Error("Unable to check if LFS OID[%s] exist. Error: %v", p.Oid, rc.User, rc.Repo, err)
			writeStatus(ctx, http.StatusInternalServerError)
			return
		}

		meta, err := repository.GetLFSMetaObjectByOid(p.Oid)
		if err != nil && err != models.ErrLFSObjectNotExist {
			log.Error("Unable to get LFS MetaObject [%s] for %s/%s. Error: %v", p.Oid, rc.User, rc.Repo, err)
			writeStatus(ctx, http.StatusInternalServerError)
			return
		}

		if meta != nil && p.Size != meta.Size {
			responseObjects = append(responseObjects, buildObjectResponse(rc, p, false, false, &lfs_module.ObjectError{
				Code:    http.StatusUnprocessableEntity,
				Message: fmt.Sprintf("Object %s is not %d bytes", p.Oid, p.Size),
			}))
			continue
		}

		var responseObject *lfs_module.ObjectResponse
		if isUpload {
			var err *lfs_module.ObjectError
			if !exists && setting.LFS.MaxFileSize > 0 && p.Size > setting.LFS.MaxFileSize {
				err = &lfs_module.ObjectError{
					Code:    http.StatusUnprocessableEntity,
					Message: fmt.Sprintf("Size must be less than or equal to %d", setting.LFS.MaxFileSize),
				}
			}

			if exists {
				if meta == nil {
					_, err := models.NewLFSMetaObject(&models.LFSMetaObject{Pointer: p, RepositoryID: repository.ID})
					if err != nil {
						log.Error("Unable to create LFS MetaObject [%s] for %s/%s. Error: %v", p.Oid, rc.User, rc.Repo, err)
						writeStatus(ctx, http.StatusInternalServerError)
						return
					}
				}
			}

			responseObject = buildObjectResponse(rc, p, false, !exists, err)
		} else {
			var err *lfs_module.ObjectError
			if !exists || meta == nil {
				err = &lfs_module.ObjectError{
					Code:    http.StatusNotFound,
					Message: http.StatusText(http.StatusNotFound),
				}
			}

			responseObject = buildObjectResponse(rc, p, true, false, err)
		}
		responseObjects = append(responseObjects, responseObject)
	}

	respobj := &lfs_module.BatchResponse{Objects: responseObjects}

	ctx.Resp.Header().Set("Content-Type", lfs_module.MediaType)

	enc := json.NewEncoder(ctx.Resp)
	if err := enc.Encode(respobj); err != nil {
		log.Error("Failed to encode representation as json. Error: %v", err)
	}
}

// UploadHandler receives data from the client and puts it into the content store
func UploadHandler(ctx *context.Context) {
	rc := getRequestContext(ctx)

	p := lfs_module.Pointer{Oid: ctx.Params("oid")}
	var err error
	if p.Size, err = strconv.ParseInt(ctx.Params("size"), 10, 64); err != nil {
		writeStatusMessage(ctx, http.StatusUnprocessableEntity, err.Error())
	}

	if !p.IsValid() {
		log.Trace("Attempt to access invalid LFS OID[%s] in %s/%s", p.Oid, rc.User, rc.Repo)
		writeStatus(ctx, http.StatusUnprocessableEntity)
		return
	}

	repository := getAuthenticatedRepository(ctx, rc, true)
	if repository == nil {
		return
	}

	meta, err := models.NewLFSMetaObject(&models.LFSMetaObject{Pointer: p, RepositoryID: repository.ID})
	if err != nil {
		log.Error("Unable to create LFS MetaObject [%s] for %s/%s. Error: %v", p.Oid, rc.User, rc.Repo, err)
		writeStatus(ctx, http.StatusInternalServerError)
		return
	}

	contentStore := lfs_module.NewContentStore()

	exists, err := contentStore.Exists(p)
	if err != nil {
		log.Error("Unable to check if LFS OID[%s] exist. Error: %v", p.Oid, err)
		writeStatus(ctx, http.StatusInternalServerError)
		return
	}
	if meta.Existing || exists {
		ctx.Resp.WriteHeader(http.StatusOK)
		return
	}

	defer ctx.Req.Body.Close()
	if err := contentStore.Put(meta.Pointer, ctx.Req.Body); err != nil {
		if errors.Is(err, lfs_module.ErrSizeMismatch) || errors.Is(err, lfs_module.ErrHashMismatch) {
			writeStatusMessage(ctx, http.StatusUnprocessableEntity, err.Error())
		} else {
			writeStatus(ctx, http.StatusInternalServerError)
		}
		if _, err = repository.RemoveLFSMetaObjectByOid(p.Oid); err != nil {
			log.Error("Error whilst removing metaobject for LFS OID[%s]: %v", p.Oid, err)
		}
		return
	}

	writeStatus(ctx, http.StatusOK)
}

// VerifyHandler verify oid and its size from the content store
func VerifyHandler(ctx *context.Context) {
	var p lfs_module.Pointer
	if err := decodeJSON(ctx.Req, &p); err != nil {
		writeStatus(ctx, http.StatusUnprocessableEntity)
		return
	}

	rc := getRequestContext(ctx)

	meta := getAuthenticatedMeta(ctx, rc, p, true)
	if meta == nil {
		return
	}

	contentStore := lfs_module.NewContentStore()
	ok, err := contentStore.Verify(meta.Pointer)

	status := http.StatusOK
	if err != nil {
		status = http.StatusInternalServerError
	} else if !ok {
		status = http.StatusNotFound
	}
	writeStatus(ctx, status)
}

func decodeJSON(req *http.Request, v interface{}) error {
	defer req.Body.Close()

	dec := json.NewDecoder(req.Body)
	return dec.Decode(v)
}

func getRequestContext(ctx *context.Context) *requestContext {
	return &requestContext{
		User:          ctx.Params("username"),
		Repo:          strings.TrimSuffix(ctx.Params("reponame"), ".git"),
		Authorization: ctx.Req.Header.Get("Authorization"),
	}
}

func getAuthenticatedMeta(ctx *context.Context, rc *requestContext, p lfs_module.Pointer, requireWrite bool) *models.LFSMetaObject {
	if !p.IsValid() {
		log.Info("Attempt to access invalid LFS OID[%s] in %s/%s", p.Oid, rc.User, rc.Repo)
		writeStatusMessage(ctx, http.StatusUnprocessableEntity, "Oid or size are invalid")
		return nil
	}

	repository := getAuthenticatedRepository(ctx, rc, requireWrite)
	if repository == nil {
		return nil
	}

	meta, err := repository.GetLFSMetaObjectByOid(p.Oid)
	if err != nil {
		log.Error("Unable to get LFS OID[%s] Error: %v", p.Oid, err)
		writeStatus(ctx, http.StatusNotFound)
		return nil
	}

	return meta
}

func getAuthenticatedRepository(ctx *context.Context, rc *requestContext, requireWrite bool) *models.Repository {
	repository, err := models.GetRepositoryByOwnerAndName(rc.User, rc.Repo)
	if err != nil {
		log.Error("Unable to get repository: %s/%s Error: %v", rc.User, rc.Repo, err)
		writeStatus(ctx, http.StatusNotFound)
		return nil
	}

	if !authenticate(ctx, repository, rc.Authorization, false, requireWrite) {
		requireAuth(ctx)
		return nil
	}

	return repository
}

func buildObjectResponse(rc *requestContext, pointer lfs_module.Pointer, download, upload bool, err *lfs_module.ObjectError) *lfs_module.ObjectResponse {
	rep := &lfs_module.ObjectResponse{Pointer: pointer}
	if err != nil {
		rep.Error = err
	} else {
		rep.Actions = make(map[string]*lfs_module.Link)

		header := make(map[string]string)

		if len(rc.Authorization) > 0 {
			header["Authorization"] = rc.Authorization
		}

		if download {
			rep.Actions["download"] = &lfs_module.Link{Href: rc.DownloadLink(pointer), Header: header}
		}
		if upload {
			rep.Actions["upload"] = &lfs_module.Link{Href: rc.UploadLink(pointer), Header: header}

			verifyHeader := make(map[string]string)
			for key, value := range header {
				verifyHeader[key] = value
			}

			// This is only needed to workaround https://github.com/git-lfs/git-lfs/issues/3662
			verifyHeader["Accept"] = lfs_module.MediaType

			rep.Actions["verify"] = &lfs_module.Link{Href: rc.VerifyLink(pointer), Header: verifyHeader}
		}
	}
	return rep
}

func writeStatus(ctx *context.Context, status int) {
	writeStatusMessage(ctx, status, http.StatusText(status))
}

func writeStatusMessage(ctx *context.Context, status int, message string) {
	ctx.Resp.Header().Set("Content-Type", lfs_module.MediaType)
	ctx.Resp.WriteHeader(status)

	er := lfs_module.ErrorResponse{Message: message}

	enc := json.NewEncoder(ctx.Resp)
	if err := enc.Encode(er); err != nil {
		log.Error("Failed to encode error response as json. Error: %v", err)
	}
}

// authenticate uses the authorization string to determine whether
// or not to proceed. This server assumes an HTTP Basic auth format.
func authenticate(ctx *context.Context, repository *models.Repository, authorization string, requireSigned, requireWrite bool) bool {
	accessMode := models.AccessModeRead
	if requireWrite {
		accessMode = models.AccessModeWrite
	}

	// ctx.IsSigned is unnecessary here, this will be checked in perm.CanAccess
	perm, err := models.GetUserRepoPermission(repository, ctx.User)
	if err != nil {
		log.Error("Unable to GetUserRepoPermission for user %-v in repo %-v Error: %v", ctx.User, repository)
		return false
	}

	canRead := perm.CanAccess(accessMode, models.UnitTypeCode)
	if canRead && (!requireSigned || ctx.IsSigned) {
		return true
	}

	user, err := parseToken(authorization, repository, accessMode)
	if err != nil {
		// Most of these are Warn level - the true internal server errors are logged in parseToken already
		log.Warn("Authentication failure for provided token with Error: %v", err)
		return false
	}
	ctx.User = user
	return true
}

func handleLFSToken(tokenSHA string, target *models.Repository, mode models.AccessMode) (*models.User, error) {
	if !strings.Contains(tokenSHA, ".") {
		return nil, nil
	}
	token, err := jwt.ParseWithClaims(tokenSHA, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return setting.LFS.JWTSecretBytes, nil
	})
	if err != nil {
		return nil, nil
	}

	claims, claimsOk := token.Claims.(*Claims)
	if !token.Valid || !claimsOk {
		return nil, fmt.Errorf("invalid token claim")
	}

	if claims.RepoID != target.ID {
		return nil, fmt.Errorf("invalid token claim")
	}

	if mode == models.AccessModeWrite && claims.Op != "upload" {
		return nil, fmt.Errorf("invalid token claim")
	}

	u, err := models.GetUserByID(claims.UserID)
	if err != nil {
		log.Error("Unable to GetUserById[%d]: Error: %v", claims.UserID, err)
		return nil, err
	}
	return u, nil
}

func parseToken(authorization string, target *models.Repository, mode models.AccessMode) (*models.User, error) {
	if authorization == "" {
		return nil, fmt.Errorf("no token")
	}

	parts := strings.SplitN(authorization, " ", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("no token")
	}
	tokenSHA := parts[1]
	switch strings.ToLower(parts[0]) {
	case "bearer":
		fallthrough
	case "token":
		return handleLFSToken(tokenSHA, target, mode)
	}
	return nil, fmt.Errorf("token not found")
}

func requireAuth(ctx *context.Context) {
	ctx.Resp.Header().Set("WWW-Authenticate", "Basic realm=gitea-lfs")
	writeStatus(ctx, http.StatusUnauthorized)
}
