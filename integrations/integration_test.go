// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers"
	"code.gitea.io/gitea/routers/routes"

	"gitea.com/macaron/macaron"
	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/unknwon/com"
	"gopkg.in/testfixtures.v2"
)

var mac *macaron.Macaron

type NilResponseRecorder struct {
	httptest.ResponseRecorder
	Length int
}

func (n *NilResponseRecorder) Write(b []byte) (int, error) {
	n.Length += len(b)
	return len(b), nil
}

// NewRecorder returns an initialized ResponseRecorder.
func NewNilResponseRecorder() *NilResponseRecorder {
	return &NilResponseRecorder{
		ResponseRecorder: *httptest.NewRecorder(),
	}
}

func TestMain(m *testing.M) {
	managerCtx, cancel := context.WithCancel(context.Background())
	graceful.InitManager(managerCtx)
	defer cancel()

	initIntegrationTest()
	mac = routes.NewMacaron()
	routes.RegisterRoutes(mac)

	var helper testfixtures.Helper
	if setting.Database.UseMySQL {
		helper = &testfixtures.MySQL{}
	} else if setting.Database.UsePostgreSQL {
		helper = &testfixtures.PostgreSQL{}
	} else if setting.Database.UseSQLite3 {
		helper = &testfixtures.SQLite{}
	} else if setting.Database.UseMSSQL {
		helper = &testfixtures.SQLServer{}
	} else {
		fmt.Println("Unsupported RDBMS for integration tests")
		os.Exit(1)
	}

	err := models.InitFixtures(
		helper,
		path.Join(filepath.Dir(setting.AppPath), "models/fixtures/"),
	)
	if err != nil {
		fmt.Printf("Error initializing test database: %v\n", err)
		os.Exit(1)
	}
	exitCode := m.Run()

	writerCloser.t = nil

	if err = os.RemoveAll(setting.Indexer.IssuePath); err != nil {
		fmt.Printf("os.RemoveAll: %v\n", err)
		os.Exit(1)
	}
	if err = os.RemoveAll(setting.Indexer.RepoPath); err != nil {
		fmt.Printf("Unable to remove repo indexer: %v\n", err)
		os.Exit(1)
	}

	os.Exit(exitCode)
}

func initIntegrationTest() {
	giteaRoot := base.SetupGiteaRoot()
	if giteaRoot == "" {
		fmt.Println("Environment variable $GITEA_ROOT not set")
		os.Exit(1)
	}
	giteaBinary := "gitea"
	if runtime.GOOS == "windows" {
		giteaBinary += ".exe"
	}
	setting.AppPath = path.Join(giteaRoot, giteaBinary)
	if _, err := os.Stat(setting.AppPath); err != nil {
		fmt.Printf("Could not find gitea binary at %s\n", setting.AppPath)
		os.Exit(1)
	}

	giteaConf := os.Getenv("GITEA_CONF")
	if giteaConf == "" {
		fmt.Println("Environment variable $GITEA_CONF not set")
		os.Exit(1)
	} else if !path.IsAbs(giteaConf) {
		setting.CustomConf = path.Join(giteaRoot, giteaConf)
	} else {
		setting.CustomConf = giteaConf
	}

	setting.SetCustomPathAndConf("", "", "")
	setting.NewContext()
	os.RemoveAll(models.LocalCopyPath())
	setting.CheckLFSVersion()
	setting.InitDBConfig()

	switch {
	case setting.Database.UseMySQL:
		db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/",
			setting.Database.User, setting.Database.Passwd, setting.Database.Host))
		defer db.Close()
		if err != nil {
			log.Fatalf("sql.Open: %v", err)
		}
		if _, err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", setting.Database.Name)); err != nil {
			log.Fatalf("db.Exec: %v", err)
		}
	case setting.Database.UsePostgreSQL:
		db, err := sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/?sslmode=%s",
			setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.SSLMode))
		defer db.Close()
		if err != nil {
			log.Fatalf("sql.Open: %v", err)
		}
		dbrows, err := db.Query(fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname = '%s'", setting.Database.Name))
		if err != nil {
			log.Fatalf("db.Query: %v", err)
		}
		defer dbrows.Close()

		if !dbrows.Next() {
			if _, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", setting.Database.Name)); err != nil {
				log.Fatalf("db.Exec: CREATE DATABASE: %v", err)
			}
		}
		// Check if we need to setup a specific schema
		if len(setting.Database.Schema) == 0 {
			break
		}
		db.Close()

		db, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s",
			setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.Name, setting.Database.SSLMode))
		// This is a different db object; requires a different Close()
		defer db.Close()
		if err != nil {
			log.Fatalf("sql.Open: %v", err)
		}
		schrows, err := db.Query(fmt.Sprintf("SELECT 1 FROM information_schema.schemata WHERE schema_name = '%s'", setting.Database.Schema))
		if err != nil {
			log.Fatalf("db.Query: %v", err)
		}
		defer schrows.Close()

		if !schrows.Next() {
			// Create and setup a DB schema
			if _, err = db.Exec(fmt.Sprintf("CREATE SCHEMA %s", setting.Database.Schema)); err != nil {
				log.Fatalf("db.Exec: CREATE SCHEMA: %v", err)
			}
		}

		// Make the user's default search path the created schema; this will affect new connections
		if _, err = db.Exec(fmt.Sprintf(`ALTER USER "%s" SET search_path = %s`, setting.Database.User, setting.Database.Schema)); err != nil {
			log.Fatalf("db.Exec: ALTER USER SET search_path: %v", err)
		}

		// Make the current connection's search the created schema
		if _, err = db.Exec(fmt.Sprintf(`SET search_path = %s`, setting.Database.Schema)); err != nil {
			log.Fatalf("db.Exec: ALTER USER SET search_path: %v", err)
		}

	case setting.Database.UseMSSQL:
		host, port := setting.ParseMSSQLHostPort(setting.Database.Host)
		db, err := sql.Open("mssql", fmt.Sprintf("server=%s; port=%s; database=%s; user id=%s; password=%s;",
			host, port, "master", setting.Database.User, setting.Database.Passwd))
		if err != nil {
			log.Fatalf("sql.Open: %v", err)
		}
		if _, err := db.Exec(fmt.Sprintf("If(db_id(N'%s') IS NULL) BEGIN CREATE DATABASE %s; END;", setting.Database.Name, setting.Database.Name)); err != nil {
			log.Fatalf("db.Exec: %v", err)
		}
		defer db.Close()
	}
	routers.GlobalInit(graceful.GetManager().HammerContext())
	NotifierListenerInit()
}

func prepareTestEnv(t testing.TB, skip ...int) func() {
	t.Helper()
	ourSkip := 2
	if len(skip) > 0 {
		ourSkip += skip[0]
	}
	deferFn := PrintCurrentTest(t, ourSkip)
	assert.NoError(t, models.LoadFixtures())
	assert.NoError(t, os.RemoveAll(setting.RepoRootPath))

	assert.NoError(t, com.CopyDir(path.Join(filepath.Dir(setting.AppPath), "integrations/gitea-repositories-meta"),
		setting.RepoRootPath))
	return deferFn
}

type TestSession struct {
	jar http.CookieJar
}

func (s *TestSession) GetCookie(name string) *http.Cookie {
	baseURL, err := url.Parse(setting.AppURL)
	if err != nil {
		return nil
	}

	for _, c := range s.jar.Cookies(baseURL) {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func (s *TestSession) MakeRequest(t testing.TB, req *http.Request, expectedStatus int) *httptest.ResponseRecorder {
	t.Helper()
	baseURL, err := url.Parse(setting.AppURL)
	assert.NoError(t, err)
	for _, c := range s.jar.Cookies(baseURL) {
		req.AddCookie(c)
	}
	resp := MakeRequest(t, req, expectedStatus)

	ch := http.Header{}
	ch.Add("Cookie", strings.Join(resp.Header()["Set-Cookie"], ";"))
	cr := http.Request{Header: ch}
	s.jar.SetCookies(baseURL, cr.Cookies())

	return resp
}

func (s *TestSession) MakeRequestNilResponseRecorder(t testing.TB, req *http.Request, expectedStatus int) *NilResponseRecorder {
	t.Helper()
	baseURL, err := url.Parse(setting.AppURL)
	assert.NoError(t, err)
	for _, c := range s.jar.Cookies(baseURL) {
		req.AddCookie(c)
	}
	resp := MakeRequestNilResponseRecorder(t, req, expectedStatus)

	ch := http.Header{}
	ch.Add("Cookie", strings.Join(resp.Header()["Set-Cookie"], ";"))
	cr := http.Request{Header: ch}
	s.jar.SetCookies(baseURL, cr.Cookies())

	return resp
}

const userPassword = "password"

var loginSessionCache = make(map[string]*TestSession, 10)

func emptyTestSession(t testing.TB) *TestSession {
	t.Helper()
	jar, err := cookiejar.New(nil)
	assert.NoError(t, err)

	return &TestSession{jar: jar}
}

func loginUser(t testing.TB, userName string) *TestSession {
	t.Helper()
	if session, ok := loginSessionCache[userName]; ok {
		return session
	}
	session := loginUserWithPassword(t, userName, userPassword)
	loginSessionCache[userName] = session
	return session
}

func loginUserWithPassword(t testing.TB, userName, password string) *TestSession {
	t.Helper()
	req := NewRequest(t, "GET", "/user/login")
	resp := MakeRequest(t, req, http.StatusOK)

	doc := NewHTMLParser(t, resp.Body)
	req = NewRequestWithValues(t, "POST", "/user/login", map[string]string{
		"_csrf":     doc.GetCSRF(),
		"user_name": userName,
		"password":  password,
	})
	resp = MakeRequest(t, req, http.StatusFound)

	ch := http.Header{}
	ch.Add("Cookie", strings.Join(resp.Header()["Set-Cookie"], ";"))
	cr := http.Request{Header: ch}

	session := emptyTestSession(t)

	baseURL, err := url.Parse(setting.AppURL)
	assert.NoError(t, err)
	session.jar.SetCookies(baseURL, cr.Cookies())

	return session
}

//token has to be unique this counter take care of
var tokenCounter int64

func getTokenForLoggedInUser(t testing.TB, session *TestSession) string {
	t.Helper()
	tokenCounter++
	req := NewRequest(t, "GET", "/user/settings/applications")
	resp := session.MakeRequest(t, req, http.StatusOK)
	doc := NewHTMLParser(t, resp.Body)
	req = NewRequestWithValues(t, "POST", "/user/settings/applications", map[string]string{
		"_csrf": doc.GetCSRF(),
		"name":  fmt.Sprintf("api-testing-token-%d", tokenCounter),
	})
	resp = session.MakeRequest(t, req, http.StatusFound)
	req = NewRequest(t, "GET", "/user/settings/applications")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	token := htmlDoc.doc.Find(".ui.info p").Text()
	return token
}

func NewRequest(t testing.TB, method, urlStr string) *http.Request {
	t.Helper()
	return NewRequestWithBody(t, method, urlStr, nil)
}

func NewRequestf(t testing.TB, method, urlFormat string, args ...interface{}) *http.Request {
	t.Helper()
	return NewRequest(t, method, fmt.Sprintf(urlFormat, args...))
}

func NewRequestWithValues(t testing.TB, method, urlStr string, values map[string]string) *http.Request {
	t.Helper()
	urlValues := url.Values{}
	for key, value := range values {
		urlValues[key] = []string{value}
	}
	req := NewRequestWithBody(t, method, urlStr, bytes.NewBufferString(urlValues.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func NewRequestWithJSON(t testing.TB, method, urlStr string, v interface{}) *http.Request {
	t.Helper()
	jsonBytes, err := json.Marshal(v)
	assert.NoError(t, err)
	req := NewRequestWithBody(t, method, urlStr, bytes.NewBuffer(jsonBytes))
	req.Header.Add("Content-Type", "application/json")
	return req
}

func NewRequestWithBody(t testing.TB, method, urlStr string, body io.Reader) *http.Request {
	t.Helper()
	request, err := http.NewRequest(method, urlStr, body)
	assert.NoError(t, err)
	request.RequestURI = urlStr
	return request
}

func AddBasicAuthHeader(request *http.Request, username string) *http.Request {
	request.SetBasicAuth(username, userPassword)
	return request
}

const NoExpectedStatus = -1

func MakeRequest(t testing.TB, req *http.Request, expectedStatus int) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	mac.ServeHTTP(recorder, req)
	if expectedStatus != NoExpectedStatus {
		if !assert.EqualValues(t, expectedStatus, recorder.Code,
			"Request: %s %s", req.Method, req.URL.String()) {
			logUnexpectedResponse(t, recorder)
		}
	}
	return recorder
}

func MakeRequestNilResponseRecorder(t testing.TB, req *http.Request, expectedStatus int) *NilResponseRecorder {
	t.Helper()
	recorder := NewNilResponseRecorder()
	mac.ServeHTTP(recorder, req)
	if expectedStatus != NoExpectedStatus {
		if !assert.EqualValues(t, expectedStatus, recorder.Code,
			"Request: %s %s", req.Method, req.URL.String()) {
			logUnexpectedResponse(t, &recorder.ResponseRecorder)
		}
	}
	return recorder
}

// logUnexpectedResponse logs the contents of an unexpected response.
func logUnexpectedResponse(t testing.TB, recorder *httptest.ResponseRecorder) {
	t.Helper()
	respBytes := recorder.Body.Bytes()
	if len(respBytes) == 0 {
		return
	} else if len(respBytes) < 500 {
		// if body is short, just log the whole thing
		t.Log("Response:", string(respBytes))
		return
	}

	// log the "flash" error message, if one exists
	// we must create a new buffer, so that we don't "use up" resp.Body
	htmlDoc, err := goquery.NewDocumentFromReader(bytes.NewBuffer(respBytes))
	if err != nil {
		return // probably a non-HTML response
	}
	errMsg := htmlDoc.Find(".ui.negative.message").Text()
	if len(errMsg) > 0 {
		t.Log("A flash error message was found:", errMsg)
	}
}

func DecodeJSON(t testing.TB, resp *httptest.ResponseRecorder, v interface{}) {
	t.Helper()
	decoder := json.NewDecoder(resp.Body)
	assert.NoError(t, decoder.Decode(v))
}

func GetCSRF(t testing.TB, session *TestSession, urlStr string) string {
	t.Helper()
	req := NewRequest(t, "GET", urlStr)
	resp := session.MakeRequest(t, req, http.StatusOK)
	doc := NewHTMLParser(t, resp.Body)
	return doc.GetCSRF()
}
