// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package stats

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/services"
)

// Indexer defines an interface to index repository stats
type Indexer interface {
	Index(id int64) error
	Close()
}

// indexer represents a indexer instance
var indexer Indexer

// Init initialize the repo indexer
func Init() error {
	indexer = &DBIndexer{}

	if err := initStatsQueue(); err != nil {
		return err
	}

	go populateRepoIndexer()

	return nil
}

func init() {
	services.RegisterService("indexer/stats", Init, "setting", "models")
}

// populateRepoIndexer populate the repo indexer with pre-existing data. This
// should only be run when the indexer is created for the first time.
func populateRepoIndexer() {
	log.Info("Populating the repo stats indexer with existing repositories")

	isShutdown := graceful.GetManager().IsShutdown()

	exist, err := models.IsTableNotEmpty("repository")
	if err != nil {
		log.Fatal("System error: %v", err)
	} else if !exist {
		return
	}

	var maxRepoID int64
	if maxRepoID, err = models.GetMaxID("repository"); err != nil {
		log.Fatal("System error: %v", err)
	}

	// start with the maximum existing repo ID and work backwards, so that we
	// don't include repos that are created after gitea starts; such repos will
	// already be added to the indexer, and we don't need to add them again.
	for maxRepoID > 0 {
		select {
		case <-isShutdown:
			log.Info("Repository Stats Indexer population shutdown before completion")
			return
		default:
		}
		ids, err := models.GetUnindexedRepos(models.RepoIndexerTypeStats, maxRepoID, 0, 50)
		if err != nil {
			log.Error("populateRepoIndexer: %v", err)
			return
		} else if len(ids) == 0 {
			break
		}
		for _, id := range ids {
			select {
			case <-isShutdown:
				log.Info("Repository Stats Indexer population shutdown before completion")
				return
			default:
			}
			if err := statsQueue.Push(id); err != nil {
				log.Error("statsQueue.Push: %v", err)
			}
			maxRepoID = id - 1
		}
	}
	log.Info("Done (re)populating the repo stats indexer with existing repositories")
}
