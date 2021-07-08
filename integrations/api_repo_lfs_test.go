// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"bytes"
	"net/http"
	"path"
	"strconv"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/setting"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
)

func TestAPILFSNotStarted(t *testing.T) {
	defer prepareTestEnv(t)()

	setting.LFS.StartServer = false

	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)

	req := NewRequestf(t, "POST", "/%s/%s.git/info/lfs/objects/batch", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequestf(t, "PUT", "/%s/%s.git/info/lfs/objects/oid/10", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequestf(t, "GET", "/%s/%s.git/info/lfs/objects/oid/name", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequestf(t, "GET", "/%s/%s.git/info/lfs/objects/oid", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequestf(t, "POST", "/%s/%s.git/info/lfs/verify", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPILFSMediaType(t *testing.T) {
	defer prepareTestEnv(t)()

	setting.LFS.StartServer = true

	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)

	req := NewRequestf(t, "POST", "/%s/%s.git/info/lfs/objects/batch", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusUnsupportedMediaType)
	req = NewRequestf(t, "POST", "/%s/%s.git/info/lfs/verify", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusUnsupportedMediaType)
}

func createLFSTestRepository(t *testing.T, name string) *models.Repository {
	ctx := NewAPITestContext(t, "user2", "lfs-"+name+"-repo")
	t.Run("CreateRepo", doAPICreateRepository(ctx, false))

	repo, err := models.GetRepositoryByOwnerAndName("user2", "lfs-"+name+"-repo")
	assert.NoError(t, err)

	return repo
}

func TestAPILFSBatch(t *testing.T) {
	defer prepareTestEnv(t)()

	setting.LFS.StartServer = true

	repo := createLFSTestRepository(t, "batch")

	content := []byte("dummy1")
	oid := storeObjectInRepo(t, repo.ID, &content)
	defer repo.RemoveLFSMetaObjectByOid(oid)

	session := loginUser(t, "user2")

	newRequest := func(t testing.TB, br *lfs.BatchRequest) *http.Request {
		req := NewRequestWithJSON(t, "POST", "/user2/lfs-batch-repo.git/info/lfs/objects/batch", br)
		req.Header.Set("Accept", lfs.MediaType)
		req.Header.Set("Content-Type", lfs.MediaType)
		return req
	}
	decodeResponse := func(t *testing.T, b *bytes.Buffer) *lfs.BatchResponse {
		var br lfs.BatchResponse

		json := jsoniter.ConfigCompatibleWithStandardLibrary
		assert.NoError(t, json.Unmarshal(b.Bytes(), &br))
		return &br
	}

	t.Run("InvalidJsonRequest", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := newRequest(t, nil)

		session.MakeRequest(t, req, http.StatusBadRequest)
	})

	t.Run("InvalidOperation", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := newRequest(t, &lfs.BatchRequest{
			Operation: "dummy",
		})

		session.MakeRequest(t, req, http.StatusBadRequest)
	})

	t.Run("InvalidPointer", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := newRequest(t, &lfs.BatchRequest{
			Operation: "download",
			Objects: []lfs.Pointer{
				{Oid: "dummy"},
				{Oid: oid, Size: -1},
			},
		})

		resp := session.MakeRequest(t, req, http.StatusOK)
		br := decodeResponse(t, resp.Body)
		assert.Len(t, br.Objects, 2)
		assert.Equal(t, "dummy", br.Objects[0].Oid)
		assert.Equal(t, oid, br.Objects[1].Oid)
		assert.Equal(t, int64(0), br.Objects[0].Size)
		assert.Equal(t, int64(-1), br.Objects[1].Size)
		assert.NotNil(t, br.Objects[0].Error)
		assert.NotNil(t, br.Objects[1].Error)
		assert.Equal(t, http.StatusUnprocessableEntity, br.Objects[0].Error.Code)
		assert.Equal(t, http.StatusUnprocessableEntity, br.Objects[1].Error.Code)
		assert.Equal(t, "Oid or size are invalid", br.Objects[0].Error.Message)
		assert.Equal(t, "Oid or size are invalid", br.Objects[1].Error.Message)
	})

	t.Run("PointerSizeMismatch", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := newRequest(t, &lfs.BatchRequest{
			Operation: "download",
			Objects: []lfs.Pointer{
				{Oid: oid, Size: 1},
			},
		})

		resp := session.MakeRequest(t, req, http.StatusOK)
		br := decodeResponse(t, resp.Body)
		assert.Len(t, br.Objects, 1)
		assert.NotNil(t, br.Objects[0].Error)
		assert.Equal(t, http.StatusUnprocessableEntity, br.Objects[0].Error.Code)
		assert.Equal(t, "Object "+oid+" is not 1 bytes", br.Objects[0].Error.Message)
	})

	t.Run("Download", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		t.Run("PointerNotInStore", func(t *testing.T) {
			defer PrintCurrentTest(t)()

			req := newRequest(t, &lfs.BatchRequest{
				Operation: "download",
				Objects: []lfs.Pointer{
					{Oid: "fb8f7d8435968c4f82a726a92395be4d16f2f63116caf36c8ad35c60831ab042", Size: 6},
				},
			})

			resp := session.MakeRequest(t, req, http.StatusOK)
			br := decodeResponse(t, resp.Body)
			assert.Len(t, br.Objects, 1)
			assert.NotNil(t, br.Objects[0].Error)
			assert.Equal(t, http.StatusNotFound, br.Objects[0].Error.Code)
		})

		t.Run("MetaNotFound", func(t *testing.T) {
			defer PrintCurrentTest(t)()

			p := lfs.Pointer{Oid: "05eeb4eb5be71f2dd291ca39157d6d9effd7d1ea19cbdc8a99411fe2a8f26a00", Size: 6}

			contentStore := lfs.NewContentStore()
			exist, err := contentStore.Exists(p)
			assert.NoError(t, err)
			assert.False(t, exist)
			err = contentStore.Put(p, bytes.NewReader([]byte("dummy0")))
			assert.NoError(t, err)

			req := newRequest(t, &lfs.BatchRequest{
				Operation: "download",
				Objects:   []lfs.Pointer{p},
			})

			resp := session.MakeRequest(t, req, http.StatusOK)
			br := decodeResponse(t, resp.Body)
			assert.Len(t, br.Objects, 1)
			assert.NotNil(t, br.Objects[0].Error)
			assert.Equal(t, http.StatusNotFound, br.Objects[0].Error.Code)
		})

		t.Run("Success", func(t *testing.T) {
			defer PrintCurrentTest(t)()

			req := newRequest(t, &lfs.BatchRequest{
				Operation: "download",
				Objects: []lfs.Pointer{
					{Oid: oid, Size: 6},
				},
			})

			resp := session.MakeRequest(t, req, http.StatusOK)
			br := decodeResponse(t, resp.Body)
			assert.Len(t, br.Objects, 1)
			assert.Nil(t, br.Objects[0].Error)
			assert.Contains(t, br.Objects[0].Actions, "download")
			l := br.Objects[0].Actions["download"]
			assert.NotNil(t, l)
			assert.NotEmpty(t, l.Href)
		})
	})

	t.Run("Upload", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		t.Run("FileTooBig", func(t *testing.T) {
			defer PrintCurrentTest(t)()

			oldMaxFileSize := setting.LFS.MaxFileSize
			setting.LFS.MaxFileSize = 2

			req := newRequest(t, &lfs.BatchRequest{
				Operation: "upload",
				Objects: []lfs.Pointer{
					{Oid: "fb8f7d8435968c4f82a726a92395be4d16f2f63116caf36c8ad35c60831ab042", Size: 6},
				},
			})

			resp := session.MakeRequest(t, req, http.StatusOK)
			br := decodeResponse(t, resp.Body)
			assert.Len(t, br.Objects, 1)
			assert.NotNil(t, br.Objects[0].Error)
			assert.Equal(t, http.StatusUnprocessableEntity, br.Objects[0].Error.Code)
			assert.Equal(t, "Size must be less than or equal to 2", br.Objects[0].Error.Message)

			setting.LFS.MaxFileSize = oldMaxFileSize
		})

		t.Run("AddMeta", func(t *testing.T) {
			defer PrintCurrentTest(t)()

			p := lfs.Pointer{Oid: "05eeb4eb5be71f2dd291ca39157d6d9effd7d1ea19cbdc8a99411fe2a8f26a00", Size: 6}

			contentStore := lfs.NewContentStore()
			exist, err := contentStore.Exists(p)
			assert.NoError(t, err)
			assert.True(t, exist)

			meta, err := repo.GetLFSMetaObjectByOid(p.Oid)
			assert.Nil(t, meta)
			assert.Equal(t, models.ErrLFSObjectNotExist, err)

			req := newRequest(t, &lfs.BatchRequest{
				Operation: "upload",
				Objects:   []lfs.Pointer{p},
			})

			resp := session.MakeRequest(t, req, http.StatusOK)
			br := decodeResponse(t, resp.Body)
			assert.Len(t, br.Objects, 1)
			assert.Nil(t, br.Objects[0].Error)
			assert.Empty(t, br.Objects[0].Actions)

			meta, err = repo.GetLFSMetaObjectByOid(p.Oid)
			assert.NoError(t, err)
			assert.NotNil(t, meta)
		})

		t.Run("AlreadyExists", func(t *testing.T) {
			defer PrintCurrentTest(t)()

			req := newRequest(t, &lfs.BatchRequest{
				Operation: "upload",
				Objects: []lfs.Pointer{
					{Oid: oid, Size: 6},
				},
			})

			resp := session.MakeRequest(t, req, http.StatusOK)
			br := decodeResponse(t, resp.Body)
			assert.Len(t, br.Objects, 1)
			assert.Nil(t, br.Objects[0].Error)
			assert.Empty(t, br.Objects[0].Actions)
		})

		t.Run("NewFile", func(t *testing.T) {
			defer PrintCurrentTest(t)()

			req := newRequest(t, &lfs.BatchRequest{
				Operation: "upload",
				Objects: []lfs.Pointer{
					{Oid: "d6f175817f886ec6fbbc1515326465fa96c3bfd54a4ea06cfd6dbbd8340e0153", Size: 1},
				},
			})

			resp := session.MakeRequest(t, req, http.StatusOK)
			br := decodeResponse(t, resp.Body)
			assert.Len(t, br.Objects, 1)
			assert.Nil(t, br.Objects[0].Error)
			assert.Contains(t, br.Objects[0].Actions, "upload")
			ul := br.Objects[0].Actions["upload"]
			assert.NotNil(t, ul)
			assert.NotEmpty(t, ul.Href)
			assert.Contains(t, br.Objects[0].Actions, "verify")
			vl := br.Objects[0].Actions["verify"]
			assert.NotNil(t, vl)
			assert.NotEmpty(t, vl.Href)
		})
	})
}

func TestAPILFSUpload(t *testing.T) {
	defer prepareTestEnv(t)()

	setting.LFS.StartServer = true

	repo := createLFSTestRepository(t, "upload")

	content := []byte("dummy3")
	oid := storeObjectInRepo(t, repo.ID, &content)
	defer repo.RemoveLFSMetaObjectByOid(oid)

	session := loginUser(t, "user2")

	newRequest := func(t testing.TB, p lfs.Pointer, content string) *http.Request {
		req := NewRequestWithBody(t, "PUT", path.Join("/user2/lfs-upload-repo.git/info/lfs/objects/", p.Oid, strconv.FormatInt(p.Size, 10)), strings.NewReader(content))
		return req
	}

	t.Run("InvalidPointer", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := newRequest(t, lfs.Pointer{Oid: "dummy"}, "")

		session.MakeRequest(t, req, http.StatusUnprocessableEntity)
	})

	t.Run("AlreadyExistsInStore", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		p := lfs.Pointer{Oid: "83de2e488b89a0aa1c97496b888120a28b0c1e15463a4adb8405578c540f36d4", Size: 6}

		contentStore := lfs.NewContentStore()
		exist, err := contentStore.Exists(p)
		assert.NoError(t, err)
		assert.False(t, exist)
		err = contentStore.Put(p, bytes.NewReader([]byte("dummy5")))
		assert.NoError(t, err)

		meta, err := repo.GetLFSMetaObjectByOid(p.Oid)
		assert.Nil(t, meta)
		assert.Equal(t, models.ErrLFSObjectNotExist, err)

		req := newRequest(t, p, "")

		session.MakeRequest(t, req, http.StatusOK)

		meta, err = repo.GetLFSMetaObjectByOid(p.Oid)
		assert.NoError(t, err)
		assert.NotNil(t, meta)
	})

	t.Run("MetaAlreadyExists", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := newRequest(t, lfs.Pointer{Oid: oid, Size: 6}, "")

		session.MakeRequest(t, req, http.StatusOK)
	})

	t.Run("HashMismatch", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := newRequest(t, lfs.Pointer{Oid: "2581dd7bbc1fe44726de4b7dd806a087a978b9c5aec0a60481259e34be09b06a", Size: 1}, "a")

		session.MakeRequest(t, req, http.StatusUnprocessableEntity)
	})

	t.Run("SizeMismatch", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := newRequest(t, lfs.Pointer{Oid: "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb", Size: 2}, "a")

		session.MakeRequest(t, req, http.StatusUnprocessableEntity)
	})

	t.Run("Success", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		p := lfs.Pointer{Oid: "6ccce4863b70f258d691f59609d31b4502e1ba5199942d3bc5d35d17a4ce771d", Size: 5}

		req := newRequest(t, p, "gitea")

		session.MakeRequest(t, req, http.StatusOK)

		contentStore := lfs.NewContentStore()
		exist, err := contentStore.Exists(p)
		assert.NoError(t, err)
		assert.True(t, exist)

		meta, err := repo.GetLFSMetaObjectByOid(p.Oid)
		assert.NoError(t, err)
		assert.NotNil(t, meta)
	})
}

func TestAPILFSVerify(t *testing.T) {
	defer prepareTestEnv(t)()

	setting.LFS.StartServer = true

	repo := createLFSTestRepository(t, "verify")

	content := []byte("dummy3")
	oid := storeObjectInRepo(t, repo.ID, &content)
	defer repo.RemoveLFSMetaObjectByOid(oid)

	session := loginUser(t, "user2")

	newRequest := func(t testing.TB, p *lfs.Pointer) *http.Request {
		req := NewRequestWithJSON(t, "POST", "/user2/lfs-verify-repo.git/info/lfs/verify", p)
		req.Header.Set("Accept", lfs.MediaType)
		req.Header.Set("Content-Type", lfs.MediaType)
		return req
	}

	t.Run("InvalidJsonRequest", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := newRequest(t, nil)

		session.MakeRequest(t, req, http.StatusUnprocessableEntity)
	})

	t.Run("InvalidPointer", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := newRequest(t, &lfs.Pointer{})

		session.MakeRequest(t, req, http.StatusUnprocessableEntity)
	})

	t.Run("PointerNotExisting", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := newRequest(t, &lfs.Pointer{Oid: "fb8f7d8435968c4f82a726a92395be4d16f2f63116caf36c8ad35c60831ab042", Size: 6})

		session.MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("Success", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := newRequest(t, &lfs.Pointer{Oid: oid, Size: 6})

		session.MakeRequest(t, req, http.StatusOK)
	})
}
