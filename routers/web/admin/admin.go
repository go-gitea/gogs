// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/cron"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/mailer"

	"gitea.com/go-chi/session"
	"github.com/goccy/go-json"
)

const (
	tplDashboard base.TplName = "admin/dashboard"
	tplConfig    base.TplName = "admin/config"
	tplMonitor   base.TplName = "admin/monitor"
	tplQueue     base.TplName = "admin/queue"
)

var sysStatus struct {
	Uptime       string
	NumGoroutine int

	// General statistics.
	MemAllocated string // bytes allocated and still in use
	MemTotal     string // bytes allocated (even if freed)
	MemSys       string // bytes obtained from system (sum of XxxSys below)
	Lookups      uint64 // number of pointer lookups
	MemMallocs   uint64 // number of mallocs
	MemFrees     uint64 // number of frees

	// Main allocation heap statistics.
	HeapAlloc    string // bytes allocated and still in use
	HeapSys      string // bytes obtained from system
	HeapIdle     string // bytes in idle spans
	HeapInuse    string // bytes in non-idle span
	HeapReleased string // bytes released to the OS
	HeapObjects  uint64 // total number of allocated objects

	// Low-level fixed-size structure allocator statistics.
	//	Inuse is bytes used now.
	//	Sys is bytes obtained from system.
	StackInuse  string // bootstrap stacks
	StackSys    string
	MSpanInuse  string // mspan structures
	MSpanSys    string
	MCacheInuse string // mcache structures
	MCacheSys   string
	BuckHashSys string // profiling bucket hash table
	GCSys       string // GC metadata
	OtherSys    string // other system allocations

	// Garbage collector statistics.
	NextGC       string // next run in HeapAlloc time (bytes)
	LastGC       string // last run in absolute time (ns)
	PauseTotalNs string
	PauseNs      string // circular buffer of recent GC pause times, most recent at [(NumGC+255)%256]
	NumGC        uint32
}

func updateSystemStatus() {
	sysStatus.Uptime = timeutil.TimeSincePro(setting.AppStartTime, "en")

	m := new(runtime.MemStats)
	runtime.ReadMemStats(m)
	sysStatus.NumGoroutine = runtime.NumGoroutine()

	sysStatus.MemAllocated = base.FileSize(int64(m.Alloc))
	sysStatus.MemTotal = base.FileSize(int64(m.TotalAlloc))
	sysStatus.MemSys = base.FileSize(int64(m.Sys))
	sysStatus.Lookups = m.Lookups
	sysStatus.MemMallocs = m.Mallocs
	sysStatus.MemFrees = m.Frees

	sysStatus.HeapAlloc = base.FileSize(int64(m.HeapAlloc))
	sysStatus.HeapSys = base.FileSize(int64(m.HeapSys))
	sysStatus.HeapIdle = base.FileSize(int64(m.HeapIdle))
	sysStatus.HeapInuse = base.FileSize(int64(m.HeapInuse))
	sysStatus.HeapReleased = base.FileSize(int64(m.HeapReleased))
	sysStatus.HeapObjects = m.HeapObjects

	sysStatus.StackInuse = base.FileSize(int64(m.StackInuse))
	sysStatus.StackSys = base.FileSize(int64(m.StackSys))
	sysStatus.MSpanInuse = base.FileSize(int64(m.MSpanInuse))
	sysStatus.MSpanSys = base.FileSize(int64(m.MSpanSys))
	sysStatus.MCacheInuse = base.FileSize(int64(m.MCacheInuse))
	sysStatus.MCacheSys = base.FileSize(int64(m.MCacheSys))
	sysStatus.BuckHashSys = base.FileSize(int64(m.BuckHashSys))
	sysStatus.GCSys = base.FileSize(int64(m.GCSys))
	sysStatus.OtherSys = base.FileSize(int64(m.OtherSys))

	sysStatus.NextGC = base.FileSize(int64(m.NextGC))
	sysStatus.LastGC = fmt.Sprintf("%.1fs", float64(time.Now().UnixNano()-int64(m.LastGC))/1000/1000/1000)
	sysStatus.PauseTotalNs = fmt.Sprintf("%.1fs", float64(m.PauseTotalNs)/1000/1000/1000)
	sysStatus.PauseNs = fmt.Sprintf("%.3fs", float64(m.PauseNs[(m.NumGC+255)%256])/1000/1000/1000)
	sysStatus.NumGC = m.NumGC
}

// Dashboard show admin panel dashboard
func Dashboard(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.dashboard")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminDashboard"] = true
	ctx.Data["Stats"] = models.GetStatistic()
	// FIXME: update periodically
	updateSystemStatus()
	ctx.Data["SysStatus"] = sysStatus
	ctx.Data["SSH"] = setting.SSH
	ctx.HTML(http.StatusOK, tplDashboard)
}

// DashboardPost run an admin operation
func DashboardPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AdminDashboardForm)
	ctx.Data["Title"] = ctx.Tr("admin.dashboard")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminDashboard"] = true
	ctx.Data["Stats"] = models.GetStatistic()
	updateSystemStatus()
	ctx.Data["SysStatus"] = sysStatus

	// Run operation.
	if form.Op != "" {
		task := cron.GetTask(form.Op)
		if task != nil {
			go task.RunWithUser(ctx.User, nil)
			ctx.Flash.Success(ctx.Tr("admin.dashboard.task.started", ctx.Tr("admin.dashboard."+form.Op)))
		} else {
			ctx.Flash.Error(ctx.Tr("admin.dashboard.task.unknown", form.Op))
		}
	}
	if form.From == "monitor" {
		ctx.Redirect(setting.AppSubURL + "/admin/monitor")
	} else {
		ctx.Redirect(setting.AppSubURL + "/admin")
	}
}

// SendTestMail send test mail to confirm mail service is OK
func SendTestMail(ctx *context.Context) {
	email := ctx.Query("email")
	// Send a test email to the user's email address and redirect back to Config
	if err := mailer.SendTestMail(email); err != nil {
		ctx.Flash.Error(ctx.Tr("admin.config.test_mail_failed", email, err))
	} else {
		ctx.Flash.Info(ctx.Tr("admin.config.test_mail_sent", email))
	}

	ctx.Redirect(setting.AppSubURL + "/admin/config")
}

func shadowPasswordKV(cfgItem, splitter string) string {
	fields := strings.Split(cfgItem, splitter)
	for i := 0; i < len(fields); i++ {
		if strings.HasPrefix(fields[i], "password=") {
			fields[i] = "password=******"
			break
		}
	}
	return strings.Join(fields, splitter)
}

func shadowURL(provider, cfgItem string) string {
	u, err := url.Parse(cfgItem)
	if err != nil {
		log.Error("Shadowing Password for %v failed: %v", provider, err)
		return cfgItem
	}
	if u.User != nil {
		atIdx := strings.Index(cfgItem, "@")
		if atIdx > 0 {
			colonIdx := strings.LastIndex(cfgItem[:atIdx], ":")
			if colonIdx > 0 {
				return cfgItem[:colonIdx+1] + "******" + cfgItem[atIdx:]
			}
		}
	}
	return cfgItem
}

func shadowPassword(provider, cfgItem string) string {
	switch provider {
	case "redis":
		return shadowPasswordKV(cfgItem, ",")
	case "mysql":
		//root:@tcp(localhost:3306)/macaron?charset=utf8
		atIdx := strings.Index(cfgItem, "@")
		if atIdx > 0 {
			colonIdx := strings.Index(cfgItem[:atIdx], ":")
			if colonIdx > 0 {
				return cfgItem[:colonIdx+1] + "******" + cfgItem[atIdx:]
			}
		}
		return cfgItem
	case "postgres":
		// user=jiahuachen dbname=macaron port=5432 sslmode=disable
		if !strings.HasPrefix(cfgItem, "postgres://") {
			return shadowPasswordKV(cfgItem, " ")
		}
		fallthrough
	case "couchbase":
		return shadowURL(provider, cfgItem)
		// postgres://pqgotest:password@localhost/pqgotest?sslmode=verify-full
		// Notice: use shadowURL
	}
	return cfgItem
}

// Config show admin config page
func Config(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.config")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminConfig"] = true

	ctx.Data["CustomConf"] = setting.CustomConf
	ctx.Data["AppUrl"] = setting.AppURL
	ctx.Data["Domain"] = setting.Domain
	ctx.Data["OfflineMode"] = setting.OfflineMode
	ctx.Data["DisableRouterLog"] = setting.DisableRouterLog
	ctx.Data["RunUser"] = setting.RunUser
	ctx.Data["RunMode"] = strings.Title(setting.RunMode)
	if version, err := git.LocalVersion(); err == nil {
		ctx.Data["GitVersion"] = version.Original()
	}
	ctx.Data["RepoRootPath"] = setting.RepoRootPath
	ctx.Data["CustomRootPath"] = setting.CustomPath
	ctx.Data["StaticRootPath"] = setting.StaticRootPath
	ctx.Data["LogRootPath"] = setting.LogRootPath
	ctx.Data["ScriptType"] = setting.ScriptType
	ctx.Data["ReverseProxyAuthUser"] = setting.ReverseProxyAuthUser
	ctx.Data["ReverseProxyAuthEmail"] = setting.ReverseProxyAuthEmail

	ctx.Data["SSH"] = setting.SSH
	ctx.Data["LFS"] = setting.LFS

	ctx.Data["Service"] = setting.Service
	ctx.Data["DbCfg"] = setting.Database
	ctx.Data["Webhook"] = setting.Webhook

	ctx.Data["MailerEnabled"] = false
	if setting.MailService != nil {
		ctx.Data["MailerEnabled"] = true
		ctx.Data["Mailer"] = setting.MailService
	}

	ctx.Data["CacheAdapter"] = setting.CacheService.Adapter
	ctx.Data["CacheInterval"] = setting.CacheService.Interval

	ctx.Data["CacheConn"] = shadowPassword(setting.CacheService.Adapter, setting.CacheService.Conn)
	ctx.Data["CacheItemTTL"] = setting.CacheService.TTL

	sessionCfg := setting.SessionConfig
	if sessionCfg.Provider == "VirtualSession" {
		var realSession session.Options
		if err := json.Unmarshal([]byte(sessionCfg.ProviderConfig), &realSession); err != nil {
			log.Error("Unable to unmarshall session config for virtualed provider config: %s\nError: %v", sessionCfg.ProviderConfig, err)
		}
		sessionCfg.Provider = realSession.Provider
		sessionCfg.ProviderConfig = realSession.ProviderConfig
		sessionCfg.CookieName = realSession.CookieName
		sessionCfg.CookiePath = realSession.CookiePath
		sessionCfg.Gclifetime = realSession.Gclifetime
		sessionCfg.Maxlifetime = realSession.Maxlifetime
		sessionCfg.Secure = realSession.Secure
		sessionCfg.Domain = realSession.Domain
	}
	sessionCfg.ProviderConfig = shadowPassword(sessionCfg.Provider, sessionCfg.ProviderConfig)
	ctx.Data["SessionConfig"] = sessionCfg

	ctx.Data["DisableGravatar"] = setting.DisableGravatar
	ctx.Data["EnableFederatedAvatar"] = setting.EnableFederatedAvatar

	ctx.Data["Git"] = setting.Git

	type envVar struct {
		Name, Value string
	}

	envVars := map[string]*envVar{}
	if len(os.Getenv("GITEA_WORK_DIR")) > 0 {
		envVars["GITEA_WORK_DIR"] = &envVar{"GITEA_WORK_DIR", os.Getenv("GITEA_WORK_DIR")}
	}
	if len(os.Getenv("GITEA_CUSTOM")) > 0 {
		envVars["GITEA_CUSTOM"] = &envVar{"GITEA_CUSTOM", os.Getenv("GITEA_CUSTOM")}
	}

	ctx.Data["EnvVars"] = envVars
	ctx.Data["Loggers"] = setting.GetLogDescriptions()
	ctx.Data["EnableAccessLog"] = setting.EnableAccessLog
	ctx.Data["AccessLogTemplate"] = setting.AccessLogTemplate
	ctx.Data["DisableRouterLog"] = setting.DisableRouterLog
	ctx.Data["EnableXORMLog"] = setting.EnableXORMLog
	ctx.Data["LogSQL"] = setting.Database.LogSQL

	ctx.HTML(http.StatusOK, tplConfig)
}

// Monitor show admin monitor page
func Monitor(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.monitor")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminMonitor"] = true
	ctx.Data["Processes"] = process.GetManager().Processes()
	ctx.Data["Entries"] = cron.ListTasks()
	ctx.Data["Queues"] = queue.GetManager().ManagedQueues()
	ctx.HTML(http.StatusOK, tplMonitor)
}

// MonitorCancel cancels a process
func MonitorCancel(ctx *context.Context) {
	pid := ctx.ParamsInt64("pid")
	process.GetManager().Cancel(pid)
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": setting.AppSubURL + "/admin/monitor",
	})
}

// Queue shows details for a specific queue
func Queue(ctx *context.Context) {
	qid := ctx.ParamsInt64("qid")
	mq := queue.GetManager().GetManagedQueue(qid)
	if mq == nil {
		ctx.Status(404)
		return
	}
	ctx.Data["Title"] = ctx.Tr("admin.monitor.queue", mq.Name)
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminMonitor"] = true
	ctx.Data["Queue"] = mq
	ctx.HTML(http.StatusOK, tplQueue)
}

// WorkerCancel cancels a worker group
func WorkerCancel(ctx *context.Context) {
	qid := ctx.ParamsInt64("qid")
	mq := queue.GetManager().GetManagedQueue(qid)
	if mq == nil {
		ctx.Status(404)
		return
	}
	pid := ctx.ParamsInt64("pid")
	mq.CancelWorkers(pid)
	ctx.Flash.Info(ctx.Tr("admin.monitor.queue.pool.cancelling"))
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10),
	})
}

// Flush flushes a queue
func Flush(ctx *context.Context) {
	qid := ctx.ParamsInt64("qid")
	mq := queue.GetManager().GetManagedQueue(qid)
	if mq == nil {
		ctx.Status(404)
		return
	}
	timeout, err := time.ParseDuration(ctx.Query("timeout"))
	if err != nil {
		timeout = -1
	}
	ctx.Flash.Info(ctx.Tr("admin.monitor.queue.pool.flush.added", mq.Name))
	go func() {
		err := mq.Flush(timeout)
		if err != nil {
			log.Error("Flushing failure for %s: Error %v", mq.Name, err)
		}
	}()
	ctx.Redirect(setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
}

// AddWorkers adds workers to a worker group
func AddWorkers(ctx *context.Context) {
	qid := ctx.ParamsInt64("qid")
	mq := queue.GetManager().GetManagedQueue(qid)
	if mq == nil {
		ctx.Status(404)
		return
	}
	number := ctx.QueryInt("number")
	if number < 1 {
		ctx.Flash.Error(ctx.Tr("admin.monitor.queue.pool.addworkers.mustnumbergreaterzero"))
		ctx.Redirect(setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
		return
	}
	timeout, err := time.ParseDuration(ctx.Query("timeout"))
	if err != nil {
		ctx.Flash.Error(ctx.Tr("admin.monitor.queue.pool.addworkers.musttimeoutduration"))
		ctx.Redirect(setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
		return
	}
	if _, ok := mq.Managed.(queue.ManagedPool); !ok {
		ctx.Flash.Error(ctx.Tr("admin.monitor.queue.pool.none"))
		ctx.Redirect(setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
		return
	}
	mq.AddWorkers(number, timeout)
	ctx.Flash.Success(ctx.Tr("admin.monitor.queue.pool.added"))
	ctx.Redirect(setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
}

// SetQueueSettings sets the maximum number of workers and other settings for this queue
func SetQueueSettings(ctx *context.Context) {
	qid := ctx.ParamsInt64("qid")
	mq := queue.GetManager().GetManagedQueue(qid)
	if mq == nil {
		ctx.Status(404)
		return
	}
	if _, ok := mq.Managed.(queue.ManagedPool); !ok {
		ctx.Flash.Error(ctx.Tr("admin.monitor.queue.pool.none"))
		ctx.Redirect(setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
		return
	}

	maxNumberStr := ctx.Query("max-number")
	numberStr := ctx.Query("number")
	timeoutStr := ctx.Query("timeout")

	var err error
	var maxNumber, number int
	var timeout time.Duration
	if len(maxNumberStr) > 0 {
		maxNumber, err = strconv.Atoi(maxNumberStr)
		if err != nil {
			ctx.Flash.Error(ctx.Tr("admin.monitor.queue.settings.maxnumberworkers.error"))
			ctx.Redirect(setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
			return
		}
		if maxNumber < -1 {
			maxNumber = -1
		}
	} else {
		maxNumber = mq.MaxNumberOfWorkers()
	}

	if len(numberStr) > 0 {
		number, err = strconv.Atoi(numberStr)
		if err != nil || number < 0 {
			ctx.Flash.Error(ctx.Tr("admin.monitor.queue.settings.numberworkers.error"))
			ctx.Redirect(setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
			return
		}
	} else {
		number = mq.BoostWorkers()
	}

	if len(timeoutStr) > 0 {
		timeout, err = time.ParseDuration(timeoutStr)
		if err != nil {
			ctx.Flash.Error(ctx.Tr("admin.monitor.queue.settings.timeout.error"))
			ctx.Redirect(setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
			return
		}
	} else {
		timeout = mq.BoostTimeout()
	}

	mq.SetPoolSettings(maxNumber, number, timeout)
	ctx.Flash.Success(ctx.Tr("admin.monitor.queue.settings.changed"))
	ctx.Redirect(setting.AppSubURL + "/admin/monitor/queue/" + strconv.FormatInt(qid, 10))
}
