// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// ArchiveType archive types
type ArchiveType int

const (
	// ZIP zip archive type
	ZIP ArchiveType = iota + 1
	// TARGZ tar gz archive type
	TARGZ
	// BUNDLE bundle archive type
	BUNDLE
)

// String converts an ArchiveType to string
func (a ArchiveType) String() string {
	switch a {
	case ZIP:
		return "zip"
	case TARGZ:
		return "tar.gz"
	case BUNDLE:
		return "bundle"
	}
	return "unknown"
}

// CreateArchiveOpts represents options for creating an archive
type CreateArchiveOpts struct {
	Format ArchiveType
	Prefix bool
}

// CreateArchive create archive content to the target path
func (c *Commit) CreateArchive(ctx context.Context, target string, opts CreateArchiveOpts) error {
	if opts.Format.String() == "unknown" {
		return fmt.Errorf("unknown format: %v", opts.Format)
	}

	args := []string{
		"archive",
	}
	if opts.Prefix {
		args = append(args, "--prefix="+filepath.Base(strings.TrimSuffix(c.repo.Path, ".git"))+"/")
	}

	args = append(args,
		"--format="+opts.Format.String(),
		"-o",
		target,
		c.ID.String(),
	)

	_, err := NewCommandContext(ctx, args...).RunInDir(c.repo.Path)
	return err
}
