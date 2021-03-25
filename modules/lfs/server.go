// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"

	"github.com/dgrijalva/jwt-go"
	jsoniter "github.com/json-iterator/go"
)

const (
	metaMediaType = "application/vnd.git-lfs+json"
)

// RequestVars contain variables from the HTTP request. Variables from routing, json body decoding, and
// some headers are stored.
type RequestVars struct {
	Oid           string
	Size          int64
	User          string
	Password      string
	Repo          string
	Authorization string
}

// BatchVars contains multiple RequestVars processed in one batch operation.
// https://github.com/git-lfs/git-lfs/blob/master/docs/api/batch.md
type BatchVars struct {
	Transfers []string       `json:"transfers,omitempty"`
	Operation string         `json:"operation"`
	Objects   []*RequestVars `json:"objects"`
}

// BatchResponse contains multiple object metadata Representation structures
// for use with the batch API.
type BatchResponse struct {
	Transfer string            `json:"transfer,omitempty"`
	Objects  []*Representation `json:"objects"`
}

// Representation is object metadata as seen by clients of the lfs server.
type Representation struct {
	Oid     string           `json:"oid"`
	Size    int64            `json:"size"`
	Actions map[string]*link `json:"actions"`
	Error   *ObjectError     `json:"error,omitempty"`
}

// ObjectError defines the JSON structure returned to the client in case of an error
type ObjectError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Claims is a JWT Token Claims
type Claims struct {
	RepoID int64
	Op     string
	UserID int64
	jwt.StandardClaims
}

// ObjectLink builds a URL linking to the object.
func (v *RequestVars) ObjectLink() string {
	return setting.AppURL + path.Join(v.User, v.Repo+".git", "info/lfs/objects", v.Oid)
}

// VerifyLink builds a URL for verifying the object.
func (v *RequestVars) VerifyLink() string {
	return setting.AppURL + path.Join(v.User, v.Repo+".git", "info/lfs/verify")
}

// link provides a structure used to build a hypermedia representation of an HTTP link.
type link struct {
	Href      string            `json:"href"`
	Header    map[string]string `json:"header,omitempty"`
	ExpiresAt time.Time         `json:"expires_at,omitempty"`
}

var oidRegExp = regexp.MustCompile(`^[A-Fa-f0-9]+$`)

func isOidValid(oid string) bool {
	return oidRegExp.MatchString(oid)
}

// ObjectOidHandler is the main request routing entry point into LFS server functions
func ObjectOidHandler(ctx *context.Context) {
	if !setting.LFS.StartServer {
		log.Debug("Attempt to access LFS server but LFS server is disabled")
		writeStatus(ctx, 404)
		return
	}

	if ctx.Req.Method == "GET" || ctx.Req.Method == "HEAD" {
		if MetaMatcher(ctx.Req) {
			getMetaHandler(ctx)
			return
		}

		getContentHandler(ctx)
		return
	} else if ctx.Req.Method == "PUT" {
		PutHandler(ctx)
		return
	}

	log.Warn("Unhandled LFS method: %s for %s/%s OID[%s]", ctx.Req.Method, ctx.Params("username"), ctx.Params("reponame"), ctx.Params("oid"))
	writeStatus(ctx, 404)
}

func getAuthenticatedRepoAndMeta(ctx *context.Context, rv *RequestVars, requireWrite bool) (*models.LFSMetaObject, *models.Repository) {
	if !isOidValid(rv.Oid) {
		log.Info("Attempt to access invalid LFS OID[%s] in %s/%s", rv.Oid, rv.User, rv.Repo)
		writeStatus(ctx, 404)
		return nil, nil
	}

	repository, err := models.GetRepositoryByOwnerAndName(rv.User, rv.Repo)
	if err != nil {
		log.Error("Unable to get repository: %s/%s Error: %v", rv.User, rv.Repo, err)
		writeStatus(ctx, 404)
		return nil, nil
	}

	if !authenticate(ctx, repository, rv.Authorization, requireWrite) {
		requireAuth(ctx)
		return nil, nil
	}

	meta, err := repository.GetLFSMetaObjectByOid(rv.Oid)
	if err != nil {
		log.Error("Unable to get LFS OID[%s] Error: %v", rv.Oid, err)
		writeStatus(ctx, 404)
		return nil, nil
	}

	return meta, repository
}

// getContentHandler gets the content from the content store
func getContentHandler(ctx *context.Context) {
	rv := unpack(ctx)

	meta, _ := getAuthenticatedRepoAndMeta(ctx, rv, false)
	if meta == nil {
		// Status already written in getAuthenticatedRepoAndMeta
		return
	}

	// Support resume download using Range header
	var fromByte, toByte int64
	toByte = meta.Size - 1
	statusCode := 200
	if rangeHdr := ctx.Req.Header.Get("Range"); rangeHdr != "" {
		regex := regexp.MustCompile(`bytes=(\d+)\-(\d*).*`)
		match := regex.FindStringSubmatch(rangeHdr)
		if len(match) > 1 {
			statusCode = 206
			fromByte, _ = strconv.ParseInt(match[1], 10, 32)

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

	contentStore := &ContentStore{ObjectStorage: storage.LFS}
	content, err := contentStore.Get(meta, fromByte)
	if err != nil {
		if IsErrRangeNotSatisfiable(err) {
			writeStatus(ctx, http.StatusRequestedRangeNotSatisfiable)
		} else {
			// Errors are logged in contentStore.Get
			writeStatus(ctx, 404)
		}
		return
	}
	defer content.Close()

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
	logRequest(ctx.Req, statusCode)
}

// getMetaHandler retrieves metadata about the object
func getMetaHandler(ctx *context.Context) {
	rv := unpack(ctx)

	meta, _ := getAuthenticatedRepoAndMeta(ctx, rv, false)
	if meta == nil {
		// Status already written in getAuthenticatedRepoAndMeta
		return
	}

	ctx.Resp.Header().Set("Content-Type", metaMediaType)

	if ctx.Req.Method == "GET" {
		json := jsoniter.ConfigCompatibleWithStandardLibrary
		enc := json.NewEncoder(ctx.Resp)
		if err := enc.Encode(Represent(rv, meta, true, false)); err != nil {
			log.Error("Failed to encode representation as json. Error: %v", err)
		}
	}

	logRequest(ctx.Req, 200)
}

// PostHandler instructs the client how to upload data
func PostHandler(ctx *context.Context) {
	if !setting.LFS.StartServer {
		log.Debug("Attempt to access LFS server but LFS server is disabled")
		writeStatus(ctx, 404)
		return
	}

	if !MetaMatcher(ctx.Req) {
		log.Info("Attempt to POST without accepting the correct media type: %s", metaMediaType)
		writeStatus(ctx, 400)
		return
	}

	rv := unpack(ctx)

	repository, err := models.GetRepositoryByOwnerAndName(rv.User, rv.Repo)
	if err != nil {
		log.Error("Unable to get repository: %s/%s Error: %v", rv.User, rv.Repo, err)
		writeStatus(ctx, 404)
		return
	}

	if !authenticate(ctx, repository, rv.Authorization, true) {
		requireAuth(ctx)
		return
	}

	if !isOidValid(rv.Oid) {
		log.Info("Invalid LFS OID[%s] attempt to POST in %s/%s", rv.Oid, rv.User, rv.Repo)
		writeStatus(ctx, 404)
		return
	}

	if setting.LFS.MaxFileSize > 0 && rv.Size > setting.LFS.MaxFileSize {
		log.Info("Denied LFS OID[%s] upload of size %d to %s/%s because of LFS_MAX_FILE_SIZE=%d", rv.Oid, rv.Size, rv.User, rv.Repo, setting.LFS.MaxFileSize)
		writeStatus(ctx, 413)
		return
	}

	meta, err := models.NewLFSMetaObject(&models.LFSMetaObject{Oid: rv.Oid, Size: rv.Size, RepositoryID: repository.ID})
	if err != nil {
		log.Error("Unable to write LFS OID[%s] size %d meta object in %v/%v to database. Error: %v", rv.Oid, rv.Size, rv.User, rv.Repo, err)
		writeStatus(ctx, 404)
		return
	}

	ctx.Resp.Header().Set("Content-Type", metaMediaType)

	sentStatus := 202
	contentStore := &ContentStore{ObjectStorage: storage.LFS}
	exist, err := contentStore.Exists(meta)
	if err != nil {
		log.Error("Unable to check if LFS OID[%s] exist on %s / %s. Error: %v", rv.Oid, rv.User, rv.Repo, err)
		writeStatus(ctx, 500)
		return
	}
	if meta.Existing && exist {
		sentStatus = 200
	}
	ctx.Resp.WriteHeader(sentStatus)

	json := jsoniter.ConfigCompatibleWithStandardLibrary
	enc := json.NewEncoder(ctx.Resp)
	if err := enc.Encode(Represent(rv, meta, meta.Existing, true)); err != nil {
		log.Error("Failed to encode representation as json. Error: %v", err)
	}
	logRequest(ctx.Req, sentStatus)
}

// BatchHandler provides the batch api
func BatchHandler(ctx *context.Context) {
	if !setting.LFS.StartServer {
		log.Debug("Attempt to access LFS server but LFS server is disabled")
		writeStatus(ctx, 404)
		return
	}

	if !MetaMatcher(ctx.Req) {
		log.Info("Attempt to BATCH without accepting the correct media type: %s", metaMediaType)
		writeStatus(ctx, 400)
		return
	}

	bv := unpackbatch(ctx)

	var responseObjects []*Representation

	// Create a response object
	for _, object := range bv.Objects {
		if !isOidValid(object.Oid) {
			log.Info("Invalid LFS OID[%s] attempt to BATCH in %s/%s", object.Oid, object.User, object.Repo)
			continue
		}

		repository, err := models.GetRepositoryByOwnerAndName(object.User, object.Repo)
		if err != nil {
			log.Error("Unable to get repository: %s/%s Error: %v", object.User, object.Repo, err)
			writeStatus(ctx, 404)
			return
		}

		requireWrite := false
		if bv.Operation == "upload" {
			requireWrite = true
		}

		if !authenticate(ctx, repository, object.Authorization, requireWrite) {
			requireAuth(ctx)
			return
		}

		contentStore := &ContentStore{ObjectStorage: storage.LFS}

		meta, err := repository.GetLFSMetaObjectByOid(object.Oid)
		if err == nil { // Object is found and exists
			exist, err := contentStore.Exists(meta)
			if err != nil {
				log.Error("Unable to check if LFS OID[%s] exist on %s / %s. Error: %v", object.Oid, object.User, object.Repo, err)
				writeStatus(ctx, 500)
				return
			}
			if exist {
				responseObjects = append(responseObjects, Represent(object, meta, true, false))
				continue
			}
		}

		if requireWrite && setting.LFS.MaxFileSize > 0 && object.Size > setting.LFS.MaxFileSize {
			log.Info("Denied LFS OID[%s] upload of size %d to %s/%s because of LFS_MAX_FILE_SIZE=%d", object.Oid, object.Size, object.User, object.Repo, setting.LFS.MaxFileSize)
			writeStatus(ctx, 413)
			return
		}

		// Object is not found
		meta, err = models.NewLFSMetaObject(&models.LFSMetaObject{Oid: object.Oid, Size: object.Size, RepositoryID: repository.ID})
		if err == nil {
			exist, err := contentStore.Exists(meta)
			if err != nil {
				log.Error("Unable to check if LFS OID[%s] exist on %s / %s. Error: %v", object.Oid, object.User, object.Repo, err)
				writeStatus(ctx, 500)
				return
			}
			responseObjects = append(responseObjects, Represent(object, meta, meta.Existing, !exist))
		} else {
			log.Error("Unable to write LFS OID[%s] size %d meta object in %v/%v to database. Error: %v", object.Oid, object.Size, object.User, object.Repo, err)
		}
	}

	ctx.Resp.Header().Set("Content-Type", metaMediaType)

	respobj := &BatchResponse{Objects: responseObjects}

	json := jsoniter.ConfigCompatibleWithStandardLibrary
	enc := json.NewEncoder(ctx.Resp)
	if err := enc.Encode(respobj); err != nil {
		log.Error("Failed to encode representation as json. Error: %v", err)
	}
	logRequest(ctx.Req, 200)
}

// PutHandler receives data from the client and puts it into the content store
func PutHandler(ctx *context.Context) {
	rv := unpack(ctx)

	meta, repository := getAuthenticatedRepoAndMeta(ctx, rv, true)
	if meta == nil {
		// Status already written in getAuthenticatedRepoAndMeta
		return
	}

	contentStore := &ContentStore{ObjectStorage: storage.LFS}
	defer ctx.Req.Body.Close()
	if err := contentStore.Put(meta, ctx.Req.Body); err != nil {
		// Put will log the error itself
		ctx.Resp.WriteHeader(500)
		if err == errSizeMismatch || err == errHashMismatch {
			fmt.Fprintf(ctx.Resp, `{"message":"%s"}`, err)
		} else {
			fmt.Fprintf(ctx.Resp, `{"message":"Internal Server Error"}`)
		}
		if _, err = repository.RemoveLFSMetaObjectByOid(rv.Oid); err != nil {
			log.Error("Whilst removing metaobject for LFS OID[%s] due to preceding error there was another Error: %v", rv.Oid, err)
		}
		return
	}

	logRequest(ctx.Req, 200)
}

// VerifyHandler verify oid and its size from the content store
func VerifyHandler(ctx *context.Context) {
	if !setting.LFS.StartServer {
		log.Debug("Attempt to access LFS server but LFS server is disabled")
		writeStatus(ctx, 404)
		return
	}

	if !MetaMatcher(ctx.Req) {
		log.Info("Attempt to VERIFY without accepting the correct media type: %s", metaMediaType)
		writeStatus(ctx, 400)
		return
	}

	rv := unpack(ctx)

	meta, _ := getAuthenticatedRepoAndMeta(ctx, rv, true)
	if meta == nil {
		// Status already written in getAuthenticatedRepoAndMeta
		return
	}

	contentStore := &ContentStore{ObjectStorage: storage.LFS}
	ok, err := contentStore.Verify(meta)
	if err != nil {
		// Error will be logged in Verify
		ctx.Resp.WriteHeader(500)
		fmt.Fprintf(ctx.Resp, `{"message":"Internal Server Error"}`)
		return
	}
	if !ok {
		writeStatus(ctx, 422)
		return
	}

	logRequest(ctx.Req, 200)
}

// Represent takes a RequestVars and Meta and turns it into a Representation suitable
// for json encoding
func Represent(rv *RequestVars, meta *models.LFSMetaObject, download, upload bool) *Representation {
	rep := &Representation{
		Oid:     meta.Oid,
		Size:    meta.Size,
		Actions: make(map[string]*link),
	}

	header := make(map[string]string)

	if rv.Authorization == "" {
		// https://github.com/github/git-lfs/issues/1088
		header["Authorization"] = "Authorization: Basic dummy"
	} else {
		header["Authorization"] = rv.Authorization
	}

	if download {
		rep.Actions["download"] = &link{Href: rv.ObjectLink(), Header: header}
	}

	if upload {
		rep.Actions["upload"] = &link{Href: rv.ObjectLink(), Header: header}
	}

	if upload && !download {
		// Force client side verify action while gitea lacks proper server side verification
		verifyHeader := make(map[string]string)
		for k, v := range header {
			verifyHeader[k] = v
		}

		// This is only needed to workaround https://github.com/git-lfs/git-lfs/issues/3662
		verifyHeader["Accept"] = metaMediaType

		rep.Actions["verify"] = &link{Href: rv.VerifyLink(), Header: verifyHeader}
	}

	return rep
}

// MetaMatcher provides a mux.MatcherFunc that only allows requests that contain
// an Accept header with the metaMediaType
func MetaMatcher(r *http.Request) bool {
	mediaParts := strings.Split(r.Header.Get("Accept"), ";")
	mt := mediaParts[0]
	return mt == metaMediaType
}

func unpack(ctx *context.Context) *RequestVars {
	r := ctx.Req
	rv := &RequestVars{
		User:          ctx.Params("username"),
		Repo:          strings.TrimSuffix(ctx.Params("reponame"), ".git"),
		Oid:           ctx.Params("oid"),
		Authorization: r.Header.Get("Authorization"),
	}

	if r.Method == "POST" { // Maybe also check if +json
		var p RequestVars
		bodyReader := r.Body
		defer bodyReader.Close()
		json := jsoniter.ConfigCompatibleWithStandardLibrary
		dec := json.NewDecoder(bodyReader)
		err := dec.Decode(&p)
		if err != nil {
			// The error is logged as a WARN here because this may represent misbehaviour rather than a true error
			log.Warn("Unable to decode POST request vars for LFS OID[%s] in %s/%s: Error: %v", rv.Oid, rv.User, rv.Repo, err)
			return rv
		}

		rv.Oid = p.Oid
		rv.Size = p.Size
	}

	return rv
}

// TODO cheap hack, unify with unpack
func unpackbatch(ctx *context.Context) *BatchVars {
	r := ctx.Req
	var bv BatchVars

	bodyReader := r.Body
	defer bodyReader.Close()
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	dec := json.NewDecoder(bodyReader)
	err := dec.Decode(&bv)
	if err != nil {
		// The error is logged as a WARN here because this may represent misbehaviour rather than a true error
		log.Warn("Unable to decode BATCH request vars in %s/%s: Error: %v", ctx.Params("username"), strings.TrimSuffix(ctx.Params("reponame"), ".git"), err)
		return &bv
	}

	for i := 0; i < len(bv.Objects); i++ {
		bv.Objects[i].User = ctx.Params("username")
		bv.Objects[i].Repo = strings.TrimSuffix(ctx.Params("reponame"), ".git")
		bv.Objects[i].Authorization = r.Header.Get("Authorization")
	}

	return &bv
}

func writeStatus(ctx *context.Context, status int) {
	message := http.StatusText(status)

	mediaParts := strings.Split(ctx.Req.Header.Get("Accept"), ";")
	mt := mediaParts[0]
	if strings.HasSuffix(mt, "+json") {
		message = `{"message":"` + message + `"}`
	}

	ctx.Resp.WriteHeader(status)
	fmt.Fprint(ctx.Resp, message)
	logRequest(ctx.Req, status)
}

func logRequest(r *http.Request, status int) {
	log.Debug("LFS request - Method: %s, URL: %s, Status %d", r.Method, r.URL, status)
}

// authenticate uses the authorization string to determine whether
// or not to proceed. This server assumes an HTTP Basic auth format.
func authenticate(ctx *context.Context, repository *models.Repository, authorization string, requireWrite bool) bool {
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
	if canRead {
		return true
	}

	user, repo, opStr, err := parseToken(authorization)
	if err != nil {
		// Most of these are Warn level - the true internal server errors are logged in parseToken already
		log.Warn("Authentication failure for provided token with Error: %v", err)
		return false
	}
	ctx.User = user
	if opStr == "basic" {
		perm, err = models.GetUserRepoPermission(repository, ctx.User)
		if err != nil {
			log.Error("Unable to GetUserRepoPermission for user %-v in repo %-v Error: %v", ctx.User, repository)
			return false
		}
		return perm.CanAccess(accessMode, models.UnitTypeCode)
	}
	if repository.ID == repo.ID {
		if requireWrite && opStr != "upload" {
			return false
		}
		return true
	}
	return false
}

func parseToken(authorization string) (*models.User, *models.Repository, string, error) {
	if authorization == "" {
		return nil, nil, "unknown", fmt.Errorf("No token")
	}
	if strings.HasPrefix(authorization, "Bearer ") {
		token, err := jwt.ParseWithClaims(authorization[7:], &Claims{}, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return setting.LFS.JWTSecretBytes, nil
		})
		if err != nil {
			// The error here is WARN level because it is caused by bad authorization rather than an internal server error
			return nil, nil, "unknown", err
		}
		claims, claimsOk := token.Claims.(*Claims)
		if !token.Valid || !claimsOk {
			return nil, nil, "unknown", fmt.Errorf("Token claim invalid")
		}
		r, err := models.GetRepositoryByID(claims.RepoID)
		if err != nil {
			log.Error("Unable to GetRepositoryById[%d]: Error: %v", claims.RepoID, err)
			return nil, nil, claims.Op, err
		}
		u, err := models.GetUserByID(claims.UserID)
		if err != nil {
			log.Error("Unable to GetUserById[%d]: Error: %v", claims.UserID, err)
			return nil, r, claims.Op, err
		}
		return u, r, claims.Op, nil
	}

	if strings.HasPrefix(authorization, "Basic ") {
		c, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(authorization, "Basic "))
		if err != nil {
			return nil, nil, "basic", err
		}
		cs := string(c)
		i := strings.IndexByte(cs, ':')
		if i < 0 {
			return nil, nil, "basic", fmt.Errorf("Basic auth invalid")
		}
		user, password := cs[:i], cs[i+1:]
		u, err := models.GetUserByName(user)
		if err != nil {
			log.Error("Unable to GetUserByName[%d]: Error: %v", user, err)
			return nil, nil, "basic", err
		}
		if !u.IsPasswordSet() || !u.ValidatePassword(password) {
			return nil, nil, "basic", fmt.Errorf("Basic auth failed")
		}
		return u, nil, "basic", nil
	}

	return nil, nil, "unknown", fmt.Errorf("Token not found")
}

func requireAuth(ctx *context.Context) {
	ctx.Resp.Header().Set("WWW-Authenticate", "Basic realm=gitea-lfs")
	writeStatus(ctx, 401)
}
