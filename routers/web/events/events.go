// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package events

import (
	"net/http"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/eventsource"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/web/user"

	"github.com/goccy/go-json"
)

// Events listens for events
func Events(ctx *context.Context) {
	// FIXME: Need to check if resp is actually a http.Flusher! - how though?

	// Set the headers related to event streaming.
	ctx.Resp.Header().Set("Content-Type", "text/event-stream")
	ctx.Resp.Header().Set("Cache-Control", "no-cache")
	ctx.Resp.Header().Set("Connection", "keep-alive")
	ctx.Resp.Header().Set("X-Accel-Buffering", "no")
	ctx.Resp.WriteHeader(http.StatusOK)

	if !ctx.IsSigned {
		// Return unauthorized status event
		event := &eventsource.Event{
			Name: "close",
			Data: "unauthorized",
		}
		_, _ = event.WriteTo(ctx)
		ctx.Resp.Flush()
		return
	}

	// Listen to connection close and un-register messageChan
	notify := ctx.Done()
	ctx.Resp.Flush()

	shutdownCtx := graceful.GetManager().ShutdownContext()

	uid := ctx.User.ID

	messageChan := eventsource.GetManager().Register(uid)

	unregister := func() {
		eventsource.GetManager().Unregister(uid, messageChan)
		// ensure the messageChan is closed
		for {
			_, ok := <-messageChan
			if !ok {
				break
			}
		}
	}

	if _, err := ctx.Resp.Write([]byte("\n")); err != nil {
		log.Error("Unable to write to EventStream: %v", err)
		unregister()
		return
	}

	timer := time.NewTicker(30 * time.Second)

	stopwatchTimer := time.NewTicker(setting.UI.Notification.MinTimeout)

loop:
	for {
		select {
		case <-timer.C:
			event := &eventsource.Event{
				Name: "ping",
			}
			_, err := event.WriteTo(ctx.Resp)
			if err != nil {
				log.Error("Unable to write to EventStream for user %s: %v", ctx.User.Name, err)
				go unregister()
				break loop
			}
			ctx.Resp.Flush()
		case <-notify:
			go unregister()
			break loop
		case <-shutdownCtx.Done():
			go unregister()
			break loop
		case <-stopwatchTimer.C:
			sws, err := models.GetUserStopwatches(ctx.User.ID, models.ListOptions{})
			if err != nil {
				log.Error("Unable to GetUserStopwatches: %v", err)
				continue
			}
			apiSWs, err := convert.ToStopWatches(sws)
			if err != nil {
				log.Error("Unable to APIFormat stopwatches: %v", err)
				continue
			}
			dataBs, err := json.Marshal(apiSWs)
			if err != nil {
				log.Error("Unable to marshal stopwatches: %v", err)
				continue
			}
			_, err = (&eventsource.Event{
				Name: "stopwatches",
				Data: string(dataBs),
			}).WriteTo(ctx.Resp)
			if err != nil {
				log.Error("Unable to write to EventStream for user %s: %v", ctx.User.Name, err)
				go unregister()
				break loop
			}
			ctx.Resp.Flush()
		case event, ok := <-messageChan:
			if !ok {
				break loop
			}

			// Handle logout
			if event.Name == "logout" {
				if ctx.Session.ID() == event.Data {
					_, _ = (&eventsource.Event{
						Name: "logout",
						Data: "here",
					}).WriteTo(ctx.Resp)
					ctx.Resp.Flush()
					go unregister()
					user.HandleSignOut(ctx)
					break loop
				}
				// Replace the event - we don't want to expose the session ID to the user
				event = &eventsource.Event{
					Name: "logout",
					Data: "elsewhere",
				}
			}

			_, err := event.WriteTo(ctx.Resp)
			if err != nil {
				log.Error("Unable to write to EventStream for user %s: %v", ctx.User.Name, err)
				go unregister()
				break loop
			}
			ctx.Resp.Flush()
		}
	}
	timer.Stop()
}
