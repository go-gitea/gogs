// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"encoding/json"
	"reflect"
	"sync"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/queue"
)

var notifierListener *NotifierListener

var once = sync.Once{}

type NotifierListener struct {
	lock      sync.RWMutex
	callbacks map[string][]*func(string, [][]byte)
	notifier  base.Notifier
}

func NotifierListenerInit() {
	once.Do(func() {
		notifierListener = &NotifierListener{
			callbacks: map[string][]*func(string, [][]byte){},
		}
		notifierListener.notifier = base.NewQueueNotifierWithHandle("test-notifier", notifierListener.handle)
		notification.RegisterNotifier(notifierListener.notifier)
	})
}

// Register will register a callback with the provided notifier function
func (n *NotifierListener) Register(functionName string, callback *func(string, [][]byte)) {
	n.lock.Lock()
	n.callbacks[functionName] = append(n.callbacks[functionName], callback)
	n.lock.Unlock()
}

// Deregister will remove the provided callback from the provided notifier function
func (n *NotifierListener) Deregister(functionName string, callback *func(string, [][]byte)) {
	n.lock.Lock()
	defer n.lock.Unlock()
	for i, callbackPtr := range n.callbacks[functionName] {
		if callbackPtr == callback {
			n.callbacks[functionName] = append(n.callbacks[functionName][0:i], n.callbacks[functionName][i+1:]...)
			return
		}
	}
}

// RegisterChannel will return a registered channel with function name and return a function to deregister it and close the channel at the end
func (n *NotifierListener) RegisterChannel(name string, argNumber int, exemplar interface{}) (<-chan interface{}, func()) {
	t := reflect.TypeOf(exemplar)
	channel := make(chan interface{}, 10)
	callback := func(_ string, args [][]byte) {
		n := reflect.New(t).Elem()
		err := json.Unmarshal(args[argNumber], n.Addr().Interface())
		if err != nil {
			log.Error("Wrong Argument passed to register channel: %v ", err)
		}
		channel <- n.Interface()
	}
	n.Register(name, &callback)

	return channel, func() {
		n.Deregister(name, &callback)
		close(channel)
	}
}

func (n *NotifierListener) handle(data ...queue.Data) {
	n.lock.RLock()
	defer n.lock.RUnlock()
	for _, datum := range data {
		call := datum.(*base.FunctionCall)
		callbacks, ok := n.callbacks[call.Name]
		if ok && len(callbacks) > 0 {
			for _, callback := range callbacks {
				(*callback)(call.Name, call.Args)
			}
		}
	}
}

func TestNotifierListener(t *testing.T) {
	defer prepareTestEnv(t)()

	createPullNotified, deregister := notifierListener.RegisterChannel("NotifyNewPullRequest", 0, &models.PullRequest{})

	bs, _ := json.Marshal(&models.PullRequest{})
	notifierListener.handle(&base.FunctionCall{
		Name: "NotifyNewPullRequest",
		Args: [][]byte{
			bs,
		},
	})
	<-createPullNotified

	notifierListener.notifier.NotifyNewPullRequest(&models.PullRequest{})
	<-createPullNotified

	notification.NotifyNewPullRequest(&models.PullRequest{})
	<-createPullNotified

	deregister()

	notification.NotifyNewPullRequest(&models.PullRequest{})
	// would panic if not deregistered
}
