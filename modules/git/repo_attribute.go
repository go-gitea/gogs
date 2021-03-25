// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"fmt"
)

// CheckAttributeOpts represents the possible options to CheckAttribute
type CheckAttributeOpts struct {
	CachedOnly    bool
	AllAttributes bool
	Attributes    []string
	Filenames     []string
}

// CheckAttribute return the Blame object of file
func (repo *Repository) CheckAttribute(opts CheckAttributeOpts) (map[string]map[string]string, error) {
	err := LoadGitVersion()
	if err != nil {
		return nil, fmt.Errorf("Git version missing: %v", err)
	}

	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)

	cmdArgs := []string{"check-attr", "-z"}

	if opts.AllAttributes {
		cmdArgs = append(cmdArgs, "-a")
	} else {
		for _, attribute := range opts.Attributes {
			if attribute != "" {
				cmdArgs = append(cmdArgs, attribute)
			}
		}
	}

	// git check-attr --cached first appears in git 1.7.8
	if opts.CachedOnly && CheckGitVersionAtLeast("1.7.8") == nil {
		cmdArgs = append(cmdArgs, "--cached")
	}

	cmdArgs = append(cmdArgs, "--")

	for _, arg := range opts.Filenames {
		if arg != "" {
			cmdArgs = append(cmdArgs, arg)
		}
	}

	cmd := NewCommand(cmdArgs...)

	if err := cmd.RunInDirPipeline(repo.Path, stdOut, stdErr); err != nil {
		return nil, fmt.Errorf("Failed to run check-attr: %v\n%s\n%s", err, stdOut.String(), stdErr.String())
	}

	fields := bytes.Split(stdOut.Bytes(), []byte{'\000'})

	if len(fields)%3 != 1 {
		return nil, fmt.Errorf("Wrong number of fields in return from check-attr")
	}

	name2attribute2info := make(map[string]map[string]string)

	for i := 0; i < (len(fields) / 3); i++ {
		filename := string(fields[3*i])
		attribute := string(fields[3*i+1])
		info := string(fields[3*i+2])
		attribute2info := name2attribute2info[filename]
		if attribute2info == nil {
			attribute2info = make(map[string]string)
		}
		attribute2info[attribute] = info
		name2attribute2info[filename] = attribute2info
	}

	return name2attribute2info, nil
}
