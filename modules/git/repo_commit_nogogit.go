// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build !gogit

package git

import (
	"bufio"
	"errors"
	"io"
	"io/ioutil"
	"strings"

	"code.gitea.io/gitea/modules/log"
)

// ResolveReference resolves a name to a reference
func (repo *Repository) ResolveReference(name string) (string, error) {
	stdout, err := NewCommand("show-ref", "--hash", name).RunInDir(repo.Path)
	if err != nil {
		if strings.Contains(err.Error(), "not a valid ref") {
			return "", ErrNotExist{name, ""}
		}
		return "", err
	}
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return "", ErrNotExist{name, ""}
	}

	return stdout, nil
}

// GetRefCommitID returns the last commit ID string of given reference (branch or tag).
func (repo *Repository) GetRefCommitID(name string) (string, error) {
	wr, rd, cancel := repo.CatFileBatchCheck()
	defer cancel()
	_, _ = wr.Write([]byte(name + "\n"))
	shaBs, _, _, err := ReadBatchLine(rd)
	if IsErrNotExist(err) {
		return "", ErrNotExist{name, ""}
	}

	return string(shaBs), nil
}

// IsCommitExist returns true if given commit exists in current repository.
func (repo *Repository) IsCommitExist(name string) bool {
	_, err := NewCommand("cat-file", "-e", name).RunInDir(repo.Path)
	return err == nil
}

func (repo *Repository) getCommit(id SHA1) (*Commit, error) {
	wr, rd, cancel := repo.CatFileBatch()
	defer cancel()

	_, _ = wr.Write([]byte(id.String() + "\n"))

	return repo.getCommitFromBatchReader(rd, id)
}

func (repo *Repository) getCommitFromBatchReader(rd *bufio.Reader, id SHA1) (*Commit, error) {
	_, typ, size, err := ReadBatchLine(rd)
	if err != nil {
		if errors.Is(err, io.EOF) || IsErrNotExist(err) {
			return nil, ErrNotExist{ID: id.String()}
		}
		return nil, err
	}

	switch typ {
	case "missing":
		return nil, ErrNotExist{ID: id.String()}
	case "tag":
		// then we need to parse the tag
		// and load the commit
		data, err := ioutil.ReadAll(io.LimitReader(rd, size))
		if err != nil {
			return nil, err
		}
		_, err = rd.Discard(1)
		if err != nil {
			return nil, err
		}
		tag, err := parseTagData(data)
		if err != nil {
			return nil, err
		}
		tag.repo = repo

		commit, err := tag.Commit()
		if err != nil {
			return nil, err
		}

		commit.CommitMessage = strings.TrimSpace(tag.Message)
		commit.Author = tag.Tagger
		commit.Signature = tag.Signature

		return commit, nil
	case "commit":
		commit, err := CommitFromReader(repo, id, io.LimitReader(rd, size))
		if err != nil {
			return nil, err
		}
		_, err = rd.Discard(1)
		if err != nil {
			return nil, err
		}

		return commit, nil
	default:
		log.Debug("Unknown typ: %s", typ)
		_, err = rd.Discard(int(size) + 1)
		if err != nil {
			return nil, err
		}
		return nil, ErrNotExist{
			ID: id.String(),
		}
	}
}

// ConvertToSHA1 returns a Hash object from a potential ID string
func (repo *Repository) ConvertToSHA1(commitID string) (SHA1, error) {
	if len(commitID) == 40 && SHAPattern.MatchString(commitID) {
		sha1, err := NewIDFromString(commitID)
		if err == nil {
			return sha1, nil
		}
	}

	wr, rd, cancel := repo.CatFileBatchCheck()
	defer cancel()
	_, err := wr.Write([]byte(commitID + "\n"))
	if err != nil {
		return SHA1{}, err
	}
	sha, _, _, err := ReadBatchLine(rd)
	if err != nil {
		if IsErrNotExist(err) {
			return SHA1{}, ErrNotExist{commitID, ""}
		}
		return SHA1{}, err
	}

	return MustIDFromString(string(sha)), nil
}
