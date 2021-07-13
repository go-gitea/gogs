// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"archive/zip"
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/web"

	jsoniter "github.com/json-iterator/go"
	gzipp "github.com/klauspost/compress/gzip"
	"github.com/stretchr/testify/assert"
)

func storeObjectInRepo(t *testing.T, repositoryID int64, content *[]byte) string {
	pointer, err := lfs.GeneratePointer(bytes.NewReader(*content))
	assert.NoError(t, err)

	_, err = models.NewLFSMetaObject(&models.LFSMetaObject{Pointer: pointer, RepositoryID: repositoryID})
	assert.NoError(t, err)
	contentStore := lfs.NewContentStore()
	exist, err := contentStore.Exists(pointer)
	assert.NoError(t, err)
	if !exist {
		err := contentStore.Put(pointer, bytes.NewReader(*content))
		assert.NoError(t, err)
	}
	return pointer.Oid
}

func storeAndGetLfs(t *testing.T, content *[]byte, extraHeader *http.Header, expectedStatus int) *httptest.ResponseRecorder {
	repo, err := models.GetRepositoryByOwnerAndName("user2", "repo1")
	assert.NoError(t, err)
	oid := storeObjectInRepo(t, repo.ID, content)
	defer repo.RemoveLFSMetaObjectByOid(oid)

	session := loginUser(t, "user2")

	// Request OID
	req := NewRequest(t, "GET", "/user2/repo1.git/info/lfs/objects/"+oid+"/test")
	req.Header.Set("Accept-Encoding", "gzip")
	if extraHeader != nil {
		for key, values := range *extraHeader {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}

	resp := session.MakeRequest(t, req, expectedStatus)

	return resp
}

func checkResponseTestContentEncoding(t *testing.T, content *[]byte, resp *httptest.ResponseRecorder, expectGzip bool) {
	contentEncoding := resp.Header().Get("Content-Encoding")
	if !expectGzip || !setting.EnableGzip {
		assert.NotContains(t, contentEncoding, "gzip")

		result := resp.Body.Bytes()
		assert.Equal(t, *content, result)
	} else {
		assert.Contains(t, contentEncoding, "gzip")
		gzippReader, err := gzipp.NewReader(resp.Body)
		assert.NoError(t, err)
		result, err := ioutil.ReadAll(gzippReader)
		assert.NoError(t, err)
		assert.Equal(t, *content, result)
	}
}

func TestGetLFSSmall(t *testing.T) {
	defer prepareTestEnv(t)()
	git.CheckLFSVersion()
	if !setting.LFS.StartServer {
		t.Skip()
		return
	}
	content := []byte("A very small file\n")

	resp := storeAndGetLfs(t, &content, nil, http.StatusOK)
	checkResponseTestContentEncoding(t, &content, resp, false)
}

func TestGetLFSLarge(t *testing.T) {
	defer prepareTestEnv(t)()
	git.CheckLFSVersion()
	if !setting.LFS.StartServer {
		t.Skip()
		return
	}
	content := make([]byte, web.GzipMinSize*10)
	for i := range content {
		content[i] = byte(i % 256)
	}

	resp := storeAndGetLfs(t, &content, nil, http.StatusOK)
	checkResponseTestContentEncoding(t, &content, resp, true)
}

func TestGetLFSGzip(t *testing.T) {
	defer prepareTestEnv(t)()
	git.CheckLFSVersion()
	if !setting.LFS.StartServer {
		t.Skip()
		return
	}
	b := make([]byte, web.GzipMinSize*10)
	for i := range b {
		b[i] = byte(i % 256)
	}
	outputBuffer := bytes.NewBuffer([]byte{})
	gzippWriter := gzipp.NewWriter(outputBuffer)
	gzippWriter.Write(b)
	gzippWriter.Close()
	content := outputBuffer.Bytes()

	resp := storeAndGetLfs(t, &content, nil, http.StatusOK)
	checkResponseTestContentEncoding(t, &content, resp, false)
}

func TestGetLFSZip(t *testing.T) {
	defer prepareTestEnv(t)()
	git.CheckLFSVersion()
	if !setting.LFS.StartServer {
		t.Skip()
		return
	}
	b := make([]byte, web.GzipMinSize*10)
	for i := range b {
		b[i] = byte(i % 256)
	}
	outputBuffer := bytes.NewBuffer([]byte{})
	zipWriter := zip.NewWriter(outputBuffer)
	fileWriter, err := zipWriter.Create("default")
	assert.NoError(t, err)
	fileWriter.Write(b)
	zipWriter.Close()
	content := outputBuffer.Bytes()

	resp := storeAndGetLfs(t, &content, nil, http.StatusOK)
	checkResponseTestContentEncoding(t, &content, resp, false)
}

func TestGetLFSRangeNo(t *testing.T) {
	defer prepareTestEnv(t)()
	git.CheckLFSVersion()
	if !setting.LFS.StartServer {
		t.Skip()
		return
	}
	content := []byte("123456789\n")

	resp := storeAndGetLfs(t, &content, nil, http.StatusOK)
	assert.Equal(t, content, resp.Body.Bytes())
}

func TestGetLFSRange(t *testing.T) {
	defer prepareTestEnv(t)()
	git.CheckLFSVersion()
	if !setting.LFS.StartServer {
		t.Skip()
		return
	}
	content := []byte("123456789\n")

	tests := []struct {
		in     string
		out    string
		status int
	}{
		{"bytes=0-0", "1", http.StatusPartialContent},
		{"bytes=0-1", "12", http.StatusPartialContent},
		{"bytes=1-1", "2", http.StatusPartialContent},
		{"bytes=1-3", "234", http.StatusPartialContent},
		{"bytes=1-", "23456789\n", http.StatusPartialContent},
		// end-range smaller than start-range is ignored
		{"bytes=1-0", "23456789\n", http.StatusPartialContent},
		{"bytes=0-10", "123456789\n", http.StatusPartialContent},
		// end-range bigger than length-1 is ignored
		{"bytes=0-11", "123456789\n", http.StatusPartialContent},
		{"bytes=11-", "Requested Range Not Satisfiable", http.StatusRequestedRangeNotSatisfiable},
		// incorrect header value cause whole header to be ignored
		{"bytes=-", "123456789\n", http.StatusOK},
		{"foobar", "123456789\n", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			h := http.Header{
				"Range": []string{tt.in},
			}
			resp := storeAndGetLfs(t, &content, &h, tt.status)
			if tt.status == http.StatusPartialContent || tt.status == http.StatusOK {
				assert.Equal(t, tt.out, resp.Body.String())
			} else {
				var er lfs.ErrorResponse
				err := jsoniter.Unmarshal(resp.Body.Bytes(), &er)
				assert.NoError(t, err)
				assert.Equal(t, tt.out, er.Message)
			}
		})
	}
}
