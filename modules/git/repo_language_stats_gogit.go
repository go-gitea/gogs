// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build gogit

package git

import (
	"bytes"
	"io"
	"io/ioutil"

	"code.gitea.io/gitea/modules/analyze"

	"github.com/go-enry/go-enry/v2"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GetLanguageStats calculates language stats for git repository at specified commit
func (repo *Repository) GetLanguageStats(commitID string, preCheck func(path string) (string, bool)) (map[string]int64, error) {
	r, err := git.PlainOpen(repo.Path)
	if err != nil {
		return nil, err
	}

	rev, err := r.ResolveRevision(plumbing.Revision(commitID))
	if err != nil {
		return nil, err
	}

	commit, err := r.CommitObject(*rev)
	if err != nil {
		return nil, err
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}

	sizes := make(map[string]int64)
	err = tree.Files().ForEach(func(f *object.File) error {
		if f.Size == 0 || analyze.IsVendor(f.Name) || enry.IsDotFile(f.Name) ||
			enry.IsDocumentation(f.Name) || enry.IsConfiguration(f.Name) {
			return nil
		}

		// If content can not be read or file is too big just do detection by filename
		var content []byte
		if f.Size <= bigFileSize {
			content, _ = readFile(f, fileSizeLimit)
		}
		if enry.IsGenerated(f.Name, content) {
			return nil
		}

		language := ""
		skip := false
		if preCheck != nil {
			language, skip = preCheck(f.Name)
			if skip {
				return nil
			}
		}

		if len(language) == 0 {
			language = analyze.GetCodeLanguage(f.Name, content)
		}
		if language == enry.OtherLanguage || language == "" {
			return nil
		}

		// group languages, such as Pug -> HTML; SCSS -> CSS
		group := enry.GetLanguageGroup(language)
		if group != "" {
			language = group
		}

		sizes[language] += f.Size

		return nil
	})
	if err != nil {
		return nil, err
	}

	// filter special languages unless they are the only language
	if len(sizes) > 1 {
		for language := range sizes {
			langtype := enry.GetLanguageType(language)
			if langtype != enry.Programming && langtype != enry.Markup {
				delete(sizes, language)
			}
		}
	}

	return sizes, nil
}

func readFile(f *object.File, limit int64) ([]byte, error) {
	r, err := f.Reader()
	if err != nil {
		return nil, err
	}
	defer r.Close()

	if limit <= 0 {
		return ioutil.ReadAll(r)
	}

	size := f.Size
	if limit > 0 && size > limit {
		size = limit
	}
	buf := bytes.NewBuffer(nil)
	buf.Grow(int(size))
	_, err = io.Copy(buf, io.LimitReader(r, limit))
	return buf.Bytes(), err
}
