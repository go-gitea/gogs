// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"

	"github.com/hashicorp/go-version"
)

var (
	// Prefix the log prefix
	Prefix = "[git-module] "
	// GitVersionRequired is the minimum Git version required
	GitVersionRequired = "1.7.2"

	// GitExecutable is the command name of git
	// Could be updated to an absolute path while initialization
	GitExecutable = "git"

	// DefaultContext is the default context to run git commands in
	// will be overwritten by Init with HammerContext
	DefaultContext = context.Background()

	gitVersion *version.Version

	// will be checked on Init
	goVersionLessThan115 = true

	// SupportProcReceive version >= 2.29.0
	SupportProcReceive bool
)

// LocalVersion returns current Git version from shell.
func LocalVersion() (*version.Version, error) {
	if err := LoadGitVersion(); err != nil {
		return nil, err
	}
	return gitVersion, nil
}

// LoadGitVersion returns current Git version from shell.
func LoadGitVersion() error {
	// doesn't need RWMutex because its exec by Init()
	if gitVersion != nil {
		return nil
	}

	stdout, err := NewCommand("version").Run()
	if err != nil {
		return err
	}

	fields := strings.Fields(stdout)
	if len(fields) < 3 {
		return fmt.Errorf("not enough output: %s", stdout)
	}

	var versionString string

	// Handle special case on Windows.
	i := strings.Index(fields[2], "windows")
	if i >= 1 {
		versionString = fields[2][:i-1]
	} else {
		versionString = fields[2]
	}

	gitVersion, err = version.NewVersion(versionString)
	return err
}

// SetExecutablePath changes the path of git executable and checks the file permission and version.
func SetExecutablePath(path string) error {
	// If path is empty, we use the default value of GitExecutable "git" to search for the location of git.
	if path != "" {
		GitExecutable = path
	}
	absPath, err := exec.LookPath(GitExecutable)
	if err != nil {
		return fmt.Errorf("Git not found: %v", err)
	}
	GitExecutable = absPath

	err = LoadGitVersion()
	if err != nil {
		return fmt.Errorf("Git version missing: %v", err)
	}

	versionRequired, err := version.NewVersion(GitVersionRequired)
	if err != nil {
		return err
	}

	if gitVersion.LessThan(versionRequired) {
		return fmt.Errorf("Git version not supported. Requires version > %v", GitVersionRequired)
	}

	return nil
}

// VersionInfo returns git version information
func VersionInfo() string {
	var format = "Git Version: %s"
	var args = []interface{}{gitVersion.Original()}
	// Since git wire protocol has been released from git v2.18
	if setting.Git.EnableAutoGitWireProtocol && CheckGitVersionAtLeast("2.18") == nil {
		format += ", Wire Protocol %s Enabled"
		args = append(args, "Version 2") // for focus color
	}

	return fmt.Sprintf(format, args...)
}

// Init initializes git module
func Init(ctx context.Context) error {
	DefaultContext = ctx

	defaultCommandExecutionTimeout = time.Duration(setting.Git.Timeout.Default) * time.Second

	if err := SetExecutablePath(setting.Git.Path); err != nil {
		return err
	}

	// force cleanup args
	GlobalCommandArgs = []string{}

	if CheckGitVersionAtLeast("2.9") == nil {
		// Explicitly disable credential helper, otherwise Git credentials might leak
		GlobalCommandArgs = append(GlobalCommandArgs, "-c", "credential.helper=")
	}

	// Since git wire protocol has been released from git v2.18
	if setting.Git.EnableAutoGitWireProtocol && CheckGitVersionAtLeast("2.18") == nil {
		GlobalCommandArgs = append(GlobalCommandArgs, "-c", "protocol.version=2")
	}

	// Save current git version on init to gitVersion otherwise it would require an RWMutex
	if err := LoadGitVersion(); err != nil {
		return err
	}

	// Save if the go version used to compile gitea is greater or equal 1.15
	runtimeVersion, err := version.NewVersion(strings.TrimPrefix(runtime.Version(), "go"))
	if err != nil {
		return err
	}
	version115, _ := version.NewVersion("1.15")
	goVersionLessThan115 = runtimeVersion.LessThan(version115)

	// Git requires setting user.name and user.email in order to commit changes - if they're not set just add some defaults
	for configKey, defaultValue := range map[string]string{"user.name": "Gitea", "user.email": "gitea@fake.local"} {
		if err := checkAndSetConfig(configKey, defaultValue, false); err != nil {
			return err
		}
	}

	// Set git some configurations - these must be set to these values for gitea to work correctly
	if err := checkAndSetConfig("core.quotePath", "false", true); err != nil {
		return err
	}

	if CheckGitVersionAtLeast("2.10") == nil {
		if err := checkAndSetConfig("receive.advertisePushOptions", "true", true); err != nil {
			return err
		}
	}

	if CheckGitVersionAtLeast("2.18") == nil {
		if err := checkAndSetConfig("core.commitGraph", "true", true); err != nil {
			return err
		}
		if err := checkAndSetConfig("gc.writeCommitGraph", "true", true); err != nil {
			return err
		}
	}

	if CheckGitVersionAtLeast("2.29") == nil {
		// set support for AGit flow
		if err := checkAndAddConfig("receive.procReceiveRefs", "refs/for"); err != nil {
			return err
		}
		SupportProcReceive = true
	} else {
		if err := checkAndRemoveConfig("receive.procReceiveRefs", "refs/for"); err != nil {
			return err
		}
		SupportProcReceive = false
	}

	if runtime.GOOS == "windows" {
		if err := checkAndSetConfig("core.longpaths", "true", true); err != nil {
			return err
		}
	}
	return nil
}

// CheckGitVersionAtLeast check git version is at least the constraint version
func CheckGitVersionAtLeast(atLeast string) error {
	if err := LoadGitVersion(); err != nil {
		return err
	}
	atLeastVersion, err := version.NewVersion(atLeast)
	if err != nil {
		return err
	}
	if gitVersion.Compare(atLeastVersion) < 0 {
		return fmt.Errorf("installed git binary version %s is not at least %s", gitVersion.Original(), atLeast)
	}
	return nil
}

func checkAndSetConfig(key, defaultValue string, forceToDefault bool) error {
	stdout, stderr, err := process.GetManager().Exec("git.Init(get setting)", GitExecutable, "config", "--get", key)
	if err != nil {
		perr, ok := err.(*process.Error)
		if !ok {
			return fmt.Errorf("Failed to get git %s(%v) errType %T: %s", key, err, err, stderr)
		}
		eerr, ok := perr.Err.(*exec.ExitError)
		if !ok || eerr.ExitCode() != 1 {
			return fmt.Errorf("Failed to get git %s(%v) errType %T: %s", key, err, err, stderr)
		}
	}

	currValue := strings.TrimSpace(stdout)

	if currValue == defaultValue || (!forceToDefault && len(currValue) > 0) {
		return nil
	}

	if _, stderr, err = process.GetManager().Exec(fmt.Sprintf("git.Init(set %s)", key), "git", "config", "--global", key, defaultValue); err != nil {
		return fmt.Errorf("Failed to set git %s(%s): %s", key, err, stderr)
	}

	return nil
}

func checkAndAddConfig(key, value string) error {
	_, stderr, err := process.GetManager().Exec("git.Init(get setting)", GitExecutable, "config", "--get", key, value)
	if err != nil {
		perr, ok := err.(*process.Error)
		if !ok {
			return fmt.Errorf("Failed to get git %s(%v) errType %T: %s", key, err, err, stderr)
		}
		eerr, ok := perr.Err.(*exec.ExitError)
		if !ok || eerr.ExitCode() != 1 {
			return fmt.Errorf("Failed to get git %s(%v) errType %T: %s", key, err, err, stderr)
		}
		if eerr.ExitCode() == 1 {
			if _, stderr, err = process.GetManager().Exec(fmt.Sprintf("git.Init(set %s)", key), "git", "config", "--global", "--add", key, value); err != nil {
				return fmt.Errorf("Failed to set git %s(%s): %s", key, err, stderr)
			}
			return nil
		}
	}

	return nil
}

func checkAndRemoveConfig(key, value string) error {
	_, stderr, err := process.GetManager().Exec("git.Init(get setting)", GitExecutable, "config", "--get", key, value)
	if err != nil {
		perr, ok := err.(*process.Error)
		if !ok {
			return fmt.Errorf("Failed to get git %s(%v) errType %T: %s", key, err, err, stderr)
		}
		eerr, ok := perr.Err.(*exec.ExitError)
		if !ok || eerr.ExitCode() != 1 {
			return fmt.Errorf("Failed to get git %s(%v) errType %T: %s", key, err, err, stderr)
		}
		if eerr.ExitCode() == 1 {
			return nil
		}
	}

	if _, stderr, err = process.GetManager().Exec(fmt.Sprintf("git.Init(set %s)", key), "git", "config", "--global", "--unset-all", key, value); err != nil {
		return fmt.Errorf("Failed to set git %s(%s): %s", key, err, stderr)
	}

	return nil
}

// Fsck verifies the connectivity and validity of the objects in the database
func Fsck(ctx context.Context, repoPath string, timeout time.Duration, args ...string) error {
	// Make sure timeout makes sense.
	if timeout <= 0 {
		timeout = -1
	}
	_, err := NewCommandContext(ctx, "fsck").AddArguments(args...).RunInDirTimeout(timeout, repoPath)
	return err
}
