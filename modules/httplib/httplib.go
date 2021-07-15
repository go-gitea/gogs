// Copyright 2013 The Beego Authors. All rights reserved.
// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package httplib

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/xml"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
)

var defaultSetting = Settings{false, "GiteaServer", 60 * time.Second, 60 * time.Second, nil, nil, nil, false}
var defaultCookieJar http.CookieJar
var settingMutex sync.Mutex

// createDefaultCookie creates a global cookiejar to store cookies.
func createDefaultCookie() {
	settingMutex.Lock()
	defer settingMutex.Unlock()
	defaultCookieJar, _ = cookiejar.New(nil)
}

// SetDefaultSetting overwrites default settings
func SetDefaultSetting(setting Settings) {
	settingMutex.Lock()
	defer settingMutex.Unlock()
	defaultSetting = setting
	if defaultSetting.ConnectTimeout == 0 {
		defaultSetting.ConnectTimeout = 60 * time.Second
	}
	if defaultSetting.ReadWriteTimeout == 0 {
		defaultSetting.ReadWriteTimeout = 60 * time.Second
	}
}

// newRequest returns *Request with specific method
func newRequest(url, method string) *Request {
	var resp http.Response
	req := http.Request{
		Method:     method,
		Header:     make(http.Header),
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
	}
	return &Request{url, &req, map[string]string{}, map[string]string{}, defaultSetting, &resp, nil}
}

// NewRequest returns *Request with specific method
func NewRequest(url, method string) *Request {
	return newRequest(url, method)
}

// Get returns *Request with GET method.
func Get(url string) *Request {
	return newRequest(url, "GET")
}

// Post returns *Request with POST method.
func Post(url string) *Request {
	return newRequest(url, "POST")
}

// Put returns *Request with PUT method.
func Put(url string) *Request {
	return newRequest(url, "PUT")
}

// Delete returns *Request DELETE method.
func Delete(url string) *Request {
	return newRequest(url, "DELETE")
}

// Head returns *Request with HEAD method.
func Head(url string) *Request {
	return newRequest(url, "HEAD")
}

// Settings is the default settings for http client
type Settings struct {
	ShowDebug        bool
	UserAgent        string
	ConnectTimeout   time.Duration
	ReadWriteTimeout time.Duration
	TLSClientConfig  *tls.Config
	Proxy            func(*http.Request) (*url.URL, error)
	Transport        http.RoundTripper
	EnableCookie     bool
}

// Request provides more useful methods for requesting one url than http.Request.
type Request struct {
	url     string
	req     *http.Request
	params  map[string]string
	files   map[string]string
	setting Settings
	resp    *http.Response
	body    []byte
}

// Setting changes request settings
func (r *Request) Setting(setting Settings) *Request {
	r.setting = setting
	return r
}

// SetContext sets the request's Context
func (r *Request) SetContext(ctx context.Context) *Request {
	r.req = r.req.WithContext(ctx)
	return r
}

// SetBasicAuth sets the request's Authorization header to use HTTP Basic Authentication with the provided username and password.
func (r *Request) SetBasicAuth(username, password string) *Request {
	r.req.SetBasicAuth(username, password)
	return r
}

// SetEnableCookie sets enable/disable cookiejar
func (r *Request) SetEnableCookie(enable bool) *Request {
	r.setting.EnableCookie = enable
	return r
}

// SetUserAgent sets User-Agent header field
func (r *Request) SetUserAgent(useragent string) *Request {
	r.setting.UserAgent = useragent
	return r
}

// Debug sets show debug or not when executing request.
func (r *Request) Debug(isdebug bool) *Request {
	r.setting.ShowDebug = isdebug
	return r
}

// SetTimeout sets connect time out and read-write time out for BeegoRequest.
func (r *Request) SetTimeout(connectTimeout, readWriteTimeout time.Duration) *Request {
	r.setting.ConnectTimeout = connectTimeout
	r.setting.ReadWriteTimeout = readWriteTimeout
	return r
}

// SetTLSClientConfig sets tls connection configurations if visiting https url.
func (r *Request) SetTLSClientConfig(config *tls.Config) *Request {
	r.setting.TLSClientConfig = config
	return r
}

// Header add header item string in request.
func (r *Request) Header(key, value string) *Request {
	r.req.Header.Set(key, value)
	return r
}

// HeaderWithSensitiveCase add header item in request and keep the case of the header key.
func (r *Request) HeaderWithSensitiveCase(key, value string) *Request {
	r.req.Header[key] = []string{value}
	return r
}

// Headers returns headers in request.
func (r *Request) Headers() http.Header {
	return r.req.Header
}

// SetProtocolVersion sets the protocol version for incoming requests.
// Client requests always use HTTP/1.1.
func (r *Request) SetProtocolVersion(vers string) *Request {
	if len(vers) == 0 {
		vers = "HTTP/1.1"
	}

	major, minor, ok := http.ParseHTTPVersion(vers)
	if ok {
		r.req.Proto = vers
		r.req.ProtoMajor = major
		r.req.ProtoMinor = minor
	}

	return r
}

// SetCookie add cookie into request.
func (r *Request) SetCookie(cookie *http.Cookie) *Request {
	r.req.Header.Add("Cookie", cookie.String())
	return r
}

// SetTransport sets transport to
func (r *Request) SetTransport(transport http.RoundTripper) *Request {
	r.setting.Transport = transport
	return r
}

// SetProxy sets http proxy
// example:
//
//	func(req *http.Request) (*url.URL, error) {
// 		u, _ := url.ParseRequestURI("http://127.0.0.1:8118")
// 		return u, nil
// 	}
func (r *Request) SetProxy(proxy func(*http.Request) (*url.URL, error)) *Request {
	r.setting.Proxy = proxy
	return r
}

// Param adds query param in to request.
// params build query string as ?key1=value1&key2=value2...
func (r *Request) Param(key, value string) *Request {
	r.params[key] = value
	return r
}

// PostFile uploads file via http
func (r *Request) PostFile(formname, filename string) *Request {
	r.files[formname] = filename
	return r
}

// Body adds request raw body.
// it supports string and []byte.
func (r *Request) Body(data interface{}) *Request {
	switch t := data.(type) {
	case string:
		bf := bytes.NewBufferString(t)
		r.req.Body = ioutil.NopCloser(bf)
		r.req.ContentLength = int64(len(t))
	case []byte:
		bf := bytes.NewBuffer(t)
		r.req.Body = ioutil.NopCloser(bf)
		r.req.ContentLength = int64(len(t))
	}
	return r
}

func (r *Request) getResponse() (*http.Response, error) {
	if r.resp.StatusCode != 0 {
		return r.resp, nil
	}
	var paramBody string
	if len(r.params) > 0 {
		var buf bytes.Buffer
		for k, v := range r.params {
			buf.WriteString(url.QueryEscape(k))
			buf.WriteByte('=')
			buf.WriteString(url.QueryEscape(v))
			buf.WriteByte('&')
		}
		paramBody = buf.String()
		paramBody = paramBody[0 : len(paramBody)-1]
	}

	if r.req.Method == "GET" && len(paramBody) > 0 {
		if strings.Contains(r.url, "?") {
			r.url += "&" + paramBody
		} else {
			r.url = r.url + "?" + paramBody
		}
	} else if r.req.Method == "POST" && r.req.Body == nil {
		if len(r.files) > 0 {
			pr, pw := io.Pipe()
			bodyWriter := multipart.NewWriter(pw)
			go func() {
				for formname, filename := range r.files {
					fileWriter, err := bodyWriter.CreateFormFile(formname, filename)
					if err != nil {
						log.Fatal(err)
					}
					fh, err := os.Open(filename)
					if err != nil {
						log.Fatal(err)
					}
					//iocopy
					_, err = io.Copy(fileWriter, fh)
					fh.Close()
					if err != nil {
						log.Fatal(err)
					}
				}
				for k, v := range r.params {
					err := bodyWriter.WriteField(k, v)
					if err != nil {
						log.Fatal(err)
					}
				}
				_ = bodyWriter.Close()
				_ = pw.Close()
			}()
			r.Header("Content-Type", bodyWriter.FormDataContentType())
			r.req.Body = ioutil.NopCloser(pr)
		} else if len(paramBody) > 0 {
			r.Header("Content-Type", "application/x-www-form-urlencoded")
			r.Body(paramBody)
		}
	}

	url, err := url.Parse(r.url)
	if err != nil {
		return nil, err
	}

	r.req.URL = url

	trans := r.setting.Transport

	if trans == nil {
		// create default transport
		proxy := r.setting.Proxy
		if proxy == nil {
			proxy = http.ProxyFromEnvironment
		}
		trans = &http.Transport{
			TLSClientConfig: r.setting.TLSClientConfig,
			Proxy:           proxy,
			DialContext:     TimeoutDialer(r.setting.ConnectTimeout),
		}
	} else if t, ok := trans.(*http.Transport); ok {
		if t.TLSClientConfig == nil {
			t.TLSClientConfig = r.setting.TLSClientConfig
		}
		if t.Proxy == nil {
			t.Proxy = r.setting.Proxy
		}
		if t.DialContext == nil {
			t.DialContext = TimeoutDialer(r.setting.ConnectTimeout)
		}
	}

	var jar http.CookieJar
	if r.setting.EnableCookie {
		if defaultCookieJar == nil {
			createDefaultCookie()
		}
		jar = defaultCookieJar
	} else {
		jar = nil
	}

	client := &http.Client{
		Transport: trans,
		Jar:       jar,
		Timeout:   r.setting.ReadWriteTimeout,
	}

	if len(r.setting.UserAgent) > 0 && len(r.req.Header.Get("User-Agent")) == 0 {
		r.req.Header.Set("User-Agent", r.setting.UserAgent)
	}

	if r.setting.ShowDebug {
		dump, err := httputil.DumpRequest(r.req, true)
		if err != nil {
			println(err.Error())
		}
		println(string(dump))
	}

	resp, err := client.Do(r.req)
	if err != nil {
		return nil, err
	}
	r.resp = resp
	return resp, nil
}

// String returns the body string in response.
// it calls Response inner.
func (r *Request) String() (string, error) {
	data, err := r.Bytes()
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// Bytes returns the body []byte in response.
// it calls Response inner.
func (r *Request) Bytes() ([]byte, error) {
	if r.body != nil {
		return r.body, nil
	}
	resp, err := r.getResponse()
	if err != nil {
		return nil, err
	}
	if resp.Body == nil {
		return nil, nil
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	r.body = data
	return data, nil
}

// ToFile saves the body data in response to one file.
// it calls Response inner.
func (r *Request) ToFile(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	resp, err := r.getResponse()
	if err != nil {
		return err
	}
	if resp.Body == nil {
		return nil
	}
	defer resp.Body.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// ToJSON returns the map that marshals from the body bytes as json in response .
// it calls Response inner.
func (r *Request) ToJSON(v interface{}) error {
	data, err := r.Bytes()
	if err != nil {
		return err
	}
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	err = json.Unmarshal(data, v)
	return err
}

// ToXML returns the map that marshals from the body bytes as xml in response .
// it calls Response inner.
func (r *Request) ToXML(v interface{}) error {
	data, err := r.Bytes()
	if err != nil {
		return err
	}
	err = xml.Unmarshal(data, v)
	return err
}

// Response executes request client gets response manually.
func (r *Request) Response() (*http.Response, error) {
	return r.getResponse()
}

// TimeoutDialer returns functions of connection dialer with timeout settings for http.Transport Dial field.
func TimeoutDialer(cTimeout time.Duration) func(ctx context.Context, net, addr string) (c net.Conn, err error) {
	return func(ctx context.Context, netw, addr string) (net.Conn, error) {
		d := net.Dialer{Timeout: cTimeout}
		conn, err := d.DialContext(ctx, netw, addr)
		if err != nil {
			return nil, err
		}
		return conn, nil
	}
}
