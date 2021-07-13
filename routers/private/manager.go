// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	jsoniter "github.com/json-iterator/go"
)

// FlushQueues flushes all the Queues
func FlushQueues(ctx *context.PrivateContext) {
	opts := web.GetForm(ctx).(*private.FlushOptions)
	if opts.NonBlocking {
		// Save the hammer ctx here - as a new one is created each time you call this.
		baseCtx := graceful.GetManager().HammerContext()
		go func() {
			err := queue.GetManager().FlushAll(baseCtx, opts.Timeout)
			if err != nil {
				log.Error("Flushing request timed-out with error: %v", err)
			}
		}()
		ctx.JSON(http.StatusAccepted, private.Response{
			Err: "Flushing",
		})
		return
	}
	err := queue.GetManager().FlushAll(ctx, opts.Timeout)
	if err != nil {
		ctx.JSON(http.StatusRequestTimeout, private.Response{
			Err: fmt.Sprintf("%v", err),
		})
	}
	ctx.PlainText(http.StatusOK, []byte("success"))
}

// PauseLogging pauses logging
func PauseLogging(ctx *context.PrivateContext) {
	log.Pause()
	ctx.PlainText(http.StatusOK, []byte("success"))
}

// ResumeLogging resumes logging
func ResumeLogging(ctx *context.PrivateContext) {
	log.Resume()
	ctx.PlainText(http.StatusOK, []byte("success"))
}

// ReleaseReopenLogging releases and reopens logging files
func ReleaseReopenLogging(ctx *context.PrivateContext) {
	if err := log.ReleaseReopen(); err != nil {
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: fmt.Sprintf("Error during release and reopen: %v", err),
		})
		return
	}
	ctx.PlainText(http.StatusOK, []byte("success"))
}

// RemoveLogger removes a logger
func RemoveLogger(ctx *context.PrivateContext) {
	group := ctx.Params("group")
	name := ctx.Params("name")
	ok, err := log.GetLogger(group).DelLogger(name)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: fmt.Sprintf("Failed to remove logger: %s %s %v", group, name, err),
		})
		return
	}
	if ok {
		setting.RemoveSubLogDescription(group, name)
	}
	ctx.PlainText(http.StatusOK, []byte(fmt.Sprintf("Removed %s %s", group, name)))
}

// AddLogger adds a logger
func AddLogger(ctx *context.PrivateContext) {
	opts := web.GetForm(ctx).(*private.LoggerOptions)
	if len(opts.Group) == 0 {
		opts.Group = log.DEFAULT
	}
	if _, ok := opts.Config["flags"]; !ok {
		switch opts.Group {
		case "access":
			opts.Config["flags"] = log.FlagsFromString("")
		case "router":
			opts.Config["flags"] = log.FlagsFromString("date,time")
		default:
			opts.Config["flags"] = log.FlagsFromString("stdflags")
		}
	}

	if _, ok := opts.Config["colorize"]; !ok && opts.Mode == "console" {
		if _, ok := opts.Config["stderr"]; ok {
			opts.Config["colorize"] = log.CanColorStderr
		} else {
			opts.Config["colorize"] = log.CanColorStdout
		}
	}

	if _, ok := opts.Config["level"]; !ok {
		opts.Config["level"] = setting.LogLevel
	}

	if _, ok := opts.Config["stacktraceLevel"]; !ok {
		opts.Config["stacktraceLevel"] = setting.StacktraceLogLevel
	}

	if opts.Mode == "file" {
		if _, ok := opts.Config["maxsize"]; !ok {
			opts.Config["maxsize"] = 1 << 28
		}
		if _, ok := opts.Config["maxdays"]; !ok {
			opts.Config["maxdays"] = 7
		}
		if _, ok := opts.Config["compressionLevel"]; !ok {
			opts.Config["compressionLevel"] = -1
		}
	}

	bufferLen := setting.Cfg.Section("log").Key("BUFFER_LEN").MustInt64(10000)
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	byteConfig, err := json.Marshal(opts.Config)
	if err != nil {
		log.Error("Failed to marshal log configuration: %v %v", opts.Config, err)
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: fmt.Sprintf("Failed to marshal log configuration: %v %v", opts.Config, err),
		})
		return
	}
	config := string(byteConfig)

	if err := log.NewNamedLogger(opts.Group, bufferLen, opts.Name, opts.Mode, config); err != nil {
		log.Error("Failed to create new named logger: %s %v", config, err)
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: fmt.Sprintf("Failed to create new named logger: %s %v", config, err),
		})
		return
	}

	setting.AddSubLogDescription(opts.Group, setting.SubLogDescription{
		Name:     opts.Name,
		Provider: opts.Mode,
		Config:   config,
	})

	ctx.PlainText(http.StatusOK, []byte("success"))
}
