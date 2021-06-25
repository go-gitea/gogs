// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package process

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"sync"
	"time"
)

// TODO: This packages still uses a singleton for the Manager.
// Once there's a decent web framework and dependencies are passed around like they should,
// then we delete the singleton.

var (
	manager     *Manager
	managerInit sync.Once

	// DefaultContext is the default context to run processing commands in
	DefaultContext = context.Background()
)

// Process represents a working process inheriting from Gitea.
type Process struct {
	PID         int64 // Process ID, not system one.
	Description string
	Start       time.Time
	Cancel      context.CancelFunc
}

// Manager knows about all processes and counts PIDs.
type Manager struct {
	mutex sync.Mutex

	counter   int64
	processes map[int64]*Process
}

// GetManager returns a Manager and initializes one as singleton if there's none yet
func GetManager() *Manager {
	managerInit.Do(func() {
		manager = &Manager{
			processes: make(map[int64]*Process),
		}
	})
	return manager
}

// Add a process to the ProcessManager and returns its PID.
func (pm *Manager) Add(description string, cancel context.CancelFunc) int64 {
	pm.mutex.Lock()
	pid := pm.counter + 1
	pm.processes[pid] = &Process{
		PID:         pid,
		Description: description,
		Start:       time.Now(),
		Cancel:      cancel,
	}
	pm.counter = pid
	pm.mutex.Unlock()

	return pid
}

// AddContext adds a process to the ProcessManager using a base context and returns a context, cancel func and its PID
func (pm *Manager) AddContext(baseCtx context.Context, description string) (context.Context, context.CancelFunc, int64) {
	ctx, cancel := context.WithCancel(baseCtx)
	pid := pm.Add(description, cancel)

	return ctx, func() {
		defer pm.Remove(pid)
		cancel()
	}, pid
}

// Remove a process from the ProcessManager.
func (pm *Manager) Remove(pid int64) {
	pm.mutex.Lock()
	delete(pm.processes, pid)
	pm.mutex.Unlock()
}

// Cancel a process in the ProcessManager.
func (pm *Manager) Cancel(pid int64) {
	pm.mutex.Lock()
	process, ok := pm.processes[pid]
	pm.mutex.Unlock()
	if ok {
		process.Cancel()
	}
}

// Processes gets the processes in a thread safe manner
func (pm *Manager) Processes() []*Process {
	pm.mutex.Lock()
	processes := make([]*Process, 0, len(pm.processes))
	for _, process := range pm.processes {
		processes = append(processes, process)
	}
	pm.mutex.Unlock()
	sort.Sort(processList(processes))
	return processes
}

// Exec a command and use the default timeout.
func (pm *Manager) Exec(desc, cmdName string, args ...string) (string, string, error) {
	return pm.ExecDir(-1, "", desc, cmdName, args...)
}

// ExecTimeout a command and use a specific timeout duration.
func (pm *Manager) ExecTimeout(timeout time.Duration, desc, cmdName string, args ...string) (string, string, error) {
	return pm.ExecDir(timeout, "", desc, cmdName, args...)
}

// ExecDir a command and use the default timeout.
func (pm *Manager) ExecDir(timeout time.Duration, dir, desc, cmdName string, args ...string) (string, string, error) {
	return pm.ExecDirEnv(timeout, dir, desc, nil, cmdName, args...)
}

// ExecDirEnv runs a command in given path and environment variables, and waits for its completion
// up to the given timeout (or DefaultTimeout if -1 is given).
// Returns its complete stdout and stderr
// outputs and an error, if any (including timeout)
func (pm *Manager) ExecDirEnv(timeout time.Duration, dir, desc string, env []string, cmdName string, args ...string) (string, string, error) {
	return pm.ExecDirEnvStdIn(timeout, dir, desc, env, nil, cmdName, args...)
}

// ExecDirEnvStdIn runs a command in given path and environment variables with provided stdIN, and waits for its completion
// up to the given timeout (or DefaultTimeout if -1 is given).
// Returns its complete stdout and stderr
// outputs and an error, if any (including timeout)
func (pm *Manager) ExecDirEnvStdIn(timeout time.Duration, dir, desc string, env []string, stdIn io.Reader, cmdName string, args ...string) (string, string, error) {
	if timeout == -1 {
		timeout = 60 * time.Second
	}

	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)

	ctx, cancel := context.WithTimeout(DefaultContext, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdName, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if stdIn != nil {
		cmd.Stdin = stdIn
	}

	if err := cmd.Start(); err != nil {
		return "", "", err
	}

	pid := pm.Add(desc, cancel)
	err := cmd.Wait()
	pm.Remove(pid)

	if err != nil {
		err = &Error{
			PID:         pid,
			Description: desc,
			Err:         err,
			CtxErr:      ctx.Err(),
			Stdout:      stdOut.String(),
			Stderr:      stdErr.String(),
		}
	}

	return stdOut.String(), stdErr.String(), err
}

type processList []*Process

func (l processList) Len() int {
	return len(l)
}

func (l processList) Less(i, j int) bool {
	return l[i].PID < l[j].PID
}

func (l processList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

// Error is a wrapped error describing the error results of Process Execution
type Error struct {
	PID         int64
	Description string
	Err         error
	CtxErr      error
	Stdout      string
	Stderr      string
}

func (err *Error) Error() string {
	return fmt.Sprintf("exec(%d:%s) failed: %v(%v) stdout: %s stderr: %s", err.PID, err.Description, err.Err, err.CtxErr, err.Stdout, err.Stderr)
}

// Unwrap implements the unwrappable implicit interface for go1.13 Unwrap()
func (err *Error) Unwrap() error {
	return err.Err
}
