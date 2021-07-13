// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"github.com/gobwas/glob"
)

// Deliver deliver hook task
func Deliver(t *models.HookTask) error {
	w, err := models.GetWebhookByID(t.HookID)
	if err != nil {
		return err
	}

	defer func() {
		err := recover()
		if err == nil {
			return
		}
		// There was a panic whilst delivering a hook...
		log.Error("PANIC whilst trying to deliver webhook[%d] for repo[%d] to %s Panic: %v\nStacktrace: %s", t.ID, t.RepoID, w.URL, err, log.Stack(2))
	}()

	t.IsDelivered = true

	var req *http.Request

	switch w.HTTPMethod {
	case "":
		log.Info("HTTP Method for webhook %d empty, setting to POST as default", t.ID)
		fallthrough
	case http.MethodPost:
		switch w.ContentType {
		case models.ContentTypeJSON:
			req, err = http.NewRequest("POST", w.URL, strings.NewReader(t.PayloadContent))
			if err != nil {
				return err
			}

			req.Header.Set("Content-Type", "application/json")
		case models.ContentTypeForm:
			var forms = url.Values{
				"payload": []string{t.PayloadContent},
			}

			req, err = http.NewRequest("POST", w.URL, strings.NewReader(forms.Encode()))
			if err != nil {
				return err
			}

			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	case http.MethodGet:
		u, err := url.Parse(w.URL)
		if err != nil {
			return err
		}
		vals := u.Query()
		vals["payload"] = []string{t.PayloadContent}
		u.RawQuery = vals.Encode()
		req, err = http.NewRequest("GET", u.String(), nil)
		if err != nil {
			return err
		}
	case http.MethodPut:
		switch w.Type {
		case models.MATRIX:
			req, err = getMatrixHookRequest(w, t)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("Invalid http method for webhook: [%d] %v", t.ID, w.HTTPMethod)
		}
	default:
		return fmt.Errorf("Invalid http method for webhook: [%d] %v", t.ID, w.HTTPMethod)
	}

	var signatureSHA1 string
	var signatureSHA256 string
	if len(w.Secret) > 0 {
		sig1 := hmac.New(sha1.New, []byte(w.Secret))
		sig256 := hmac.New(sha256.New, []byte(w.Secret))
		_, err = io.MultiWriter(sig1, sig256).Write([]byte(t.PayloadContent))
		if err != nil {
			log.Error("prepareWebhooks.sigWrite: %v", err)
		}
		signatureSHA1 = hex.EncodeToString(sig1.Sum(nil))
		signatureSHA256 = hex.EncodeToString(sig256.Sum(nil))
	}

	req.Header.Add("X-Gitea-Delivery", t.UUID)
	req.Header.Add("X-Gitea-Event", t.EventType.Event())
	req.Header.Add("X-Gitea-Signature", signatureSHA256)
	req.Header.Add("X-Gogs-Delivery", t.UUID)
	req.Header.Add("X-Gogs-Event", t.EventType.Event())
	req.Header.Add("X-Gogs-Signature", signatureSHA256)
	req.Header.Add("X-Hub-Signature", "sha1="+signatureSHA1)
	req.Header.Add("X-Hub-Signature-256", "sha256="+signatureSHA256)
	req.Header["X-GitHub-Delivery"] = []string{t.UUID}
	req.Header["X-GitHub-Event"] = []string{t.EventType.Event()}

	// Record delivery information.
	t.RequestInfo = &models.HookRequest{
		URL:        req.URL.String(),
		HTTPMethod: req.Method,
		Headers:    map[string]string{},
	}
	for k, vals := range req.Header {
		t.RequestInfo.Headers[k] = strings.Join(vals, ",")
	}

	t.ResponseInfo = &models.HookResponse{
		Headers: map[string]string{},
	}

	defer func() {
		t.Delivered = time.Now().UnixNano()
		if t.IsSucceed {
			log.Trace("Hook delivered: %s", t.UUID)
		} else {
			log.Trace("Hook delivery failed: %s", t.UUID)
		}

		if err := models.UpdateHookTask(t); err != nil {
			log.Error("UpdateHookTask [%d]: %v", t.ID, err)
		}

		// Update webhook last delivery status.
		if t.IsSucceed {
			w.LastStatus = models.HookStatusSucceed
		} else {
			w.LastStatus = models.HookStatusFail
		}
		if err = models.UpdateWebhookLastStatus(w); err != nil {
			log.Error("UpdateWebhookLastStatus: %v", err)
			return
		}
	}()

	if setting.DisableWebhooks {
		return fmt.Errorf("Webhook task skipped (webhooks disabled): [%d]", t.ID)
	}

	resp, err := webhookHTTPClient.Do(req)
	if err != nil {
		t.ResponseInfo.Body = fmt.Sprintf("Delivery: %v", err)
		return err
	}
	defer resp.Body.Close()

	// Status code is 20x can be seen as succeed.
	t.IsSucceed = resp.StatusCode/100 == 2
	t.ResponseInfo.Status = resp.StatusCode
	for k, vals := range resp.Header {
		t.ResponseInfo.Headers[k] = strings.Join(vals, ",")
	}

	p, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.ResponseInfo.Body = fmt.Sprintf("read body: %s", err)
		return err
	}
	t.ResponseInfo.Body = string(p)
	return nil
}

// DeliverHooks checks and delivers undelivered hooks.
// FIXME: graceful: This would likely benefit from either a worker pool with dummy queue
// or a full queue. Then more hooks could be sent at same time.
func DeliverHooks(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	default:
	}
	tasks, err := models.FindUndeliveredHookTasks()
	if err != nil {
		log.Error("DeliverHooks: %v", err)
		return
	}

	// Update hook task status.
	for _, t := range tasks {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err = Deliver(t); err != nil {
			log.Error("deliver: %v", err)
		}
	}

	// Start listening on new hook requests.
	for {
		select {
		case <-ctx.Done():
			hookQueue.Close()
			return
		case repoIDStr := <-hookQueue.Queue():
			log.Trace("DeliverHooks [repo_id: %v]", repoIDStr)
			hookQueue.Remove(repoIDStr)

			repoID, err := strconv.ParseInt(repoIDStr, 10, 64)
			if err != nil {
				log.Error("Invalid repo ID: %s", repoIDStr)
				continue
			}

			tasks, err := models.FindRepoUndeliveredHookTasks(repoID)
			if err != nil {
				log.Error("Get repository [%d] hook tasks: %v", repoID, err)
				continue
			}
			for _, t := range tasks {
				select {
				case <-ctx.Done():
					return
				default:
				}
				if err = Deliver(t); err != nil {
					log.Error("deliver: %v", err)
				}
			}
		}
	}

}

var (
	webhookHTTPClient *http.Client
	once              sync.Once
	hostMatchers      []glob.Glob
)

func webhookProxy() func(req *http.Request) (*url.URL, error) {
	if setting.Webhook.ProxyURL == "" {
		return http.ProxyFromEnvironment
	}

	once.Do(func() {
		for _, h := range setting.Webhook.ProxyHosts {
			if g, err := glob.Compile(h); err == nil {
				hostMatchers = append(hostMatchers, g)
			} else {
				log.Error("glob.Compile %s failed: %v", h, err)
			}
		}
	})

	return func(req *http.Request) (*url.URL, error) {
		for _, v := range hostMatchers {
			if v.Match(req.URL.Host) {
				return http.ProxyURL(setting.Webhook.ProxyURLFixed)(req)
			}
		}
		return http.ProxyFromEnvironment(req)
	}
}

// InitDeliverHooks starts the hooks delivery thread
func InitDeliverHooks() {
	timeout := time.Duration(setting.Webhook.DeliverTimeout) * time.Second

	webhookHTTPClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: setting.Webhook.SkipTLSVerify},
			Proxy:           webhookProxy(),
			Dial: func(netw, addr string) (net.Conn, error) {
				return net.DialTimeout(netw, addr, timeout) // dial timeout
			},
		},
		Timeout: timeout, // request timeout
	}

	go graceful.GetManager().RunWithShutdownContext(DeliverHooks)
}
