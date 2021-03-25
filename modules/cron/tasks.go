// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cron

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
)

var (
	lock     = sync.Mutex{}
	started  = false
	tasks    = []*Task{}
	tasksMap = map[string]*Task{}
)

// Task represents a Cron task
type Task struct {
	lock      sync.Mutex
	Name      string
	config    Config
	fun       func(context.Context, *models.User, Config) error
	ExecTimes int64
}

// DoRunAtStart returns if this task should run at the start
func (t *Task) DoRunAtStart() bool {
	return t.config.DoRunAtStart()
}

// IsEnabled returns if this task is enabled as cron task
func (t *Task) IsEnabled() bool {
	return t.config.IsEnabled()
}

// GetConfig will return a copy of the task's config
func (t *Task) GetConfig() Config {
	if reflect.TypeOf(t.config).Kind() == reflect.Ptr {
		// Pointer:
		return reflect.New(reflect.ValueOf(t.config).Elem().Type()).Interface().(Config)
	}
	// Not pointer:
	return reflect.New(reflect.TypeOf(t.config)).Elem().Interface().(Config)
}

// Run will run the task incrementing the cron counter with no user defined
func (t *Task) Run() {
	t.RunWithUser(&models.User{
		ID:        -1,
		Name:      "(Cron)",
		LowerName: "(cron)",
	}, t.config)
}

// RunWithUser will run the task incrementing the cron counter at the time with User
func (t *Task) RunWithUser(doer *models.User, config Config) {
	if !taskStatusTable.StartIfNotRunning(t.Name) {
		return
	}
	t.lock.Lock()
	if config == nil {
		config = t.config
	}
	t.ExecTimes++
	t.lock.Unlock()
	defer func() {
		taskStatusTable.Stop(t.Name)
		if err := recover(); err != nil {
			// Recover a panic within the
			combinedErr := fmt.Errorf("%s\n%s", err, log.Stack(2))
			log.Error("PANIC whilst running task: %s Value: %v", t.Name, combinedErr)
		}
	}()
	graceful.GetManager().RunWithShutdownContext(func(baseCtx context.Context) {
		ctx, cancel := context.WithCancel(baseCtx)
		defer cancel()
		pm := process.GetManager()
		pid := pm.Add(config.FormatMessage(t.Name, "process", doer), cancel)
		defer pm.Remove(pid)
		if err := t.fun(ctx, doer, config); err != nil {
			if models.IsErrCancelled(err) {
				message := err.(models.ErrCancelled).Message
				if err := models.CreateNotice(models.NoticeTask, config.FormatMessage(t.Name, "aborted", doer, message)); err != nil {
					log.Error("CreateNotice: %v", err)
				}
				return
			}
			if err := models.CreateNotice(models.NoticeTask, config.FormatMessage(t.Name, "error", doer, err)); err != nil {
				log.Error("CreateNotice: %v", err)
			}
			return
		}
		if config.DoNoticeOnSuccess() {
			if err := models.CreateNotice(models.NoticeTask, config.FormatMessage(t.Name, "finished", doer)); err != nil {
				log.Error("CreateNotice: %v", err)
			}
		}
	})
}

// GetTask gets the named task
func GetTask(name string) *Task {
	lock.Lock()
	defer lock.Unlock()
	log.Info("Getting %s in %v", name, tasksMap[name])

	return tasksMap[name]
}

// RegisterTask allows a task to be registered with the cron service
func RegisterTask(name string, config Config, fun func(context.Context, *models.User, Config) error) error {
	log.Debug("Registering task: %s", name)
	_, err := setting.GetCronSettings(name, config)
	if err != nil {
		log.Error("Unable to register cron task with name: %s Error: %v", name, err)
		return err
	}

	task := &Task{
		Name:   name,
		config: config,
		fun:    fun,
	}
	lock.Lock()
	locked := true
	defer func() {
		if locked {
			lock.Unlock()
		}
	}()
	if _, has := tasksMap[task.Name]; has {
		log.Error("A task with this name: %s has already been registered", name)
		return fmt.Errorf("duplicate task with name: %s", task.Name)
	}

	if config.IsEnabled() {
		// We cannot use the entry return as there is no way to lock it
		if _, err = c.AddJob(name, config.GetSchedule(), task); err != nil {
			log.Error("Unable to register cron task with name: %s Error: %v", name, err)
			return err
		}
	}

	tasks = append(tasks, task)
	tasksMap[task.Name] = task
	if started && config.IsEnabled() && config.DoRunAtStart() {
		lock.Unlock()
		locked = false
		task.Run()
	}

	return nil
}

// RegisterTaskFatal will register a task but if there is an error log.Fatal
func RegisterTaskFatal(name string, config Config, fun func(context.Context, *models.User, Config) error) {
	if err := RegisterTask(name, config, fun); err != nil {
		log.Fatal("Unable to register cron task %s Error: %v", name, err)
	}
}
