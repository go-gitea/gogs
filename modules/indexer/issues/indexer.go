// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issues

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// IndexerData data stored in the issue indexer
type IndexerData struct {
	ID       int64    `json:"id"`
	RepoID   int64    `json:"repo_id"`
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Comments []string `json:"comments"`
	IsDelete bool     `json:"is_delete"`
	IDs      []int64  `json:"ids"`
}

// Match represents on search result
type Match struct {
	ID    int64   `json:"id"`
	Score float64 `json:"score"`
}

// SearchResult represents search results
type SearchResult struct {
	Total int64
	Hits  []Match
}

// Indexer defines an interface to indexer issues contents
type Indexer interface {
	Init() (bool, error)
	Index(issue []*IndexerData) error
	Delete(ids ...int64) error
	Search(kw string, repoIDs []int64, limit, start int) (*SearchResult, error)
	Close()
}

type indexerHolder struct {
	indexer   Indexer
	mutex     sync.RWMutex
	cond      *sync.Cond
	cancelled bool
}

func newIndexerHolder() *indexerHolder {
	h := &indexerHolder{}
	h.cond = sync.NewCond(h.mutex.RLocker())
	return h
}

func (h *indexerHolder) cancel() {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.cancelled = true
	h.cond.Broadcast()
}

func (h *indexerHolder) set(indexer Indexer) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.indexer = indexer
	h.cond.Broadcast()
}

func (h *indexerHolder) get() Indexer {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	if h.indexer == nil && !h.cancelled {
		h.cond.Wait()
	}
	return h.indexer
}

var (
	// issueIndexerQueue queue of issue ids to be updated
	issueIndexerQueue queue.Queue
	holder            = newIndexerHolder()
)

// InitIssueIndexer initialize issue indexer, syncReindex is true then reindex until
// all issue index done.
func InitIssueIndexer(syncReindex bool) {
	waitChannel := make(chan time.Duration)

	// Create the Queue
	switch setting.Indexer.IssueType {
	case "bleve", "elasticsearch":
		handler := func(data ...queue.Data) []queue.Data {
			indexer := holder.get()
			if indexer == nil {
				log.Error("Issue indexer handler: unable to get indexer!")
				return data
			}

			iData := make([]*IndexerData, 0, setting.Indexer.IssueQueueBatchNumber)
			for _, datum := range data {
				indexerData, ok := datum.(*IndexerData)
				if !ok {
					log.Error("Unable to process provided datum: %v - not possible to cast to IndexerData", datum)
					continue
				}
				log.Trace("IndexerData Process: %d %v %t", indexerData.ID, indexerData.IDs, indexerData.IsDelete)
				if indexerData.IsDelete {
					_ = indexer.Delete(indexerData.IDs...)
					continue
				}
				iData = append(iData, indexerData)
			}
			if err := indexer.Index(iData); err != nil {
				log.Error("Error whilst indexing: %v Error: %v", iData, err)
			}
			return nil
		}

		issueIndexerQueue = queue.CreateQueue("issue_indexer", handler, &IndexerData{})

		if issueIndexerQueue == nil {
			log.Fatal("Unable to create issue indexer queue")
		}
	default:
		issueIndexerQueue = &queue.DummyQueue{}
	}

	// Create the Indexer
	go func() {
		start := time.Now()
		log.Info("PID %d: Initializing Issue Indexer: %s", os.Getpid(), setting.Indexer.IssueType)
		var populate bool
		switch setting.Indexer.IssueType {
		case "bleve":
			defer func() {
				if err := recover(); err != nil {
					log.Error("PANIC whilst initializing issue indexer: %v\nStacktrace: %s", err, log.Stack(2))
					log.Error("The indexer files are likely corrupted and may need to be deleted")
					log.Error("You can completely remove the %q directory to make Gitea recreate the indexes", setting.Indexer.IssuePath)
					holder.cancel()
					log.Fatal("PID: %d Unable to initialize the Bleve Issue Indexer at path: %s Error: %v", os.Getpid(), setting.Indexer.IssuePath, err)
				}
			}()
			issueIndexer := NewBleveIndexer(setting.Indexer.IssuePath)
			exist, err := issueIndexer.Init()
			if err != nil {
				holder.cancel()
				log.Fatal("Unable to initialize Bleve Issue Indexer at path: %s Error: %v", setting.Indexer.IssuePath, err)
			}
			populate = !exist
			holder.set(issueIndexer)
			graceful.GetManager().RunAtTerminate(func() {
				log.Debug("Closing issue indexer")
				issueIndexer := holder.get()
				if issueIndexer != nil {
					issueIndexer.Close()
				}
				log.Info("PID: %d Issue Indexer closed", os.Getpid())
			})
			log.Debug("Created Bleve Indexer")
		case "elasticsearch":
			graceful.GetManager().RunWithShutdownFns(func(_, atTerminate func(func())) {
				issueIndexer, err := NewElasticSearchIndexer(setting.Indexer.IssueConnStr, setting.Indexer.IssueIndexerName)
				if err != nil {
					log.Fatal("Unable to initialize Elastic Search Issue Indexer at connection: %s Error: %v", setting.Indexer.IssueConnStr, err)
				}
				exist, err := issueIndexer.Init()
				if err != nil {
					log.Fatal("Unable to issueIndexer.Init with connection %s Error: %v", setting.Indexer.IssueConnStr, err)
				}
				populate = !exist
				holder.set(issueIndexer)
			})
		case "db":
			issueIndexer := &DBIndexer{}
			holder.set(issueIndexer)
		default:
			holder.cancel()
			log.Fatal("Unknown issue indexer type: %s", setting.Indexer.IssueType)
		}

		// Start processing the queue
		go graceful.GetManager().RunWithShutdownFns(issueIndexerQueue.Run)

		// Populate the index
		if populate {
			if syncReindex {
				graceful.GetManager().RunWithShutdownContext(populateIssueIndexer)
			} else {
				go graceful.GetManager().RunWithShutdownContext(populateIssueIndexer)
			}
		}
		waitChannel <- time.Since(start)
		close(waitChannel)
	}()

	if syncReindex {
		select {
		case <-waitChannel:
		case <-graceful.GetManager().IsShutdown():
		}
	} else if setting.Indexer.StartupTimeout > 0 {
		go func() {
			timeout := setting.Indexer.StartupTimeout
			if graceful.GetManager().IsChild() && setting.GracefulHammerTime > 0 {
				timeout += setting.GracefulHammerTime
			}
			select {
			case duration := <-waitChannel:
				log.Info("Issue Indexer Initialization took %v", duration)
			case <-graceful.GetManager().IsShutdown():
				log.Warn("Shutdown occurred before issue index initialisation was complete")
			case <-time.After(timeout):
				if shutdownable, ok := issueIndexerQueue.(queue.Shutdownable); ok {
					shutdownable.Terminate()
				}
				log.Fatal("Issue Indexer Initialization timed-out after: %v", timeout)
			}
		}()
	}
}

// populateIssueIndexer populate the issue indexer with issue data
func populateIssueIndexer(ctx context.Context) {
	for page := 1; ; page++ {
		select {
		case <-ctx.Done():
			log.Warn("Issue Indexer population shutdown before completion")
			return
		default:
		}
		repos, _, err := models.SearchRepositoryByName(&models.SearchRepoOptions{
			ListOptions: models.ListOptions{Page: page, PageSize: models.RepositoryListDefaultPageSize},
			OrderBy:     models.SearchOrderByID,
			Private:     true,
			Collaborate: util.OptionalBoolFalse,
		})
		if err != nil {
			log.Error("SearchRepositoryByName: %v", err)
			continue
		}
		if len(repos) == 0 {
			log.Debug("Issue Indexer population complete")
			return
		}

		for _, repo := range repos {
			select {
			case <-ctx.Done():
				log.Info("Issue Indexer population shutdown before completion")
				return
			default:
			}
			UpdateRepoIndexer(repo)
		}
	}
}

// UpdateRepoIndexer add/update all issues of the repositories
func UpdateRepoIndexer(repo *models.Repository) {
	is, err := models.Issues(&models.IssuesOptions{
		RepoIDs:  []int64{repo.ID},
		IsClosed: util.OptionalBoolNone,
		IsPull:   util.OptionalBoolNone,
	})
	if err != nil {
		log.Error("Issues: %v", err)
		return
	}
	if err = models.IssueList(is).LoadDiscussComments(); err != nil {
		log.Error("LoadComments: %v", err)
		return
	}
	for _, issue := range is {
		UpdateIssueIndexer(issue)
	}
}

// UpdateIssueIndexer add/update an issue to the issue indexer
func UpdateIssueIndexer(issue *models.Issue) {
	var comments []string
	for _, comment := range issue.Comments {
		if comment.Type == models.CommentTypeComment {
			comments = append(comments, comment.Content)
		}
	}
	indexerData := &IndexerData{
		ID:       issue.ID,
		RepoID:   issue.RepoID,
		Title:    issue.Title,
		Content:  issue.Content,
		Comments: comments,
	}
	log.Debug("Adding to channel: %v", indexerData)
	if err := issueIndexerQueue.Push(indexerData); err != nil {
		log.Error("Unable to push to issue indexer: %v: Error: %v", indexerData, err)
	}
}

// DeleteRepoIssueIndexer deletes repo's all issues indexes
func DeleteRepoIssueIndexer(repo *models.Repository) {
	var ids []int64
	ids, err := models.GetIssueIDsByRepoID(repo.ID)
	if err != nil {
		log.Error("getIssueIDsByRepoID failed: %v", err)
		return
	}

	if len(ids) == 0 {
		return
	}
	indexerData := &IndexerData{
		IDs:      ids,
		IsDelete: true,
	}
	if err := issueIndexerQueue.Push(indexerData); err != nil {
		log.Error("Unable to push to issue indexer: %v: Error: %v", indexerData, err)
	}
}

// SearchIssuesByKeyword search issue ids by keywords and repo id
// WARNNING: You have to ensure user have permission to visit repoIDs' issues
func SearchIssuesByKeyword(repoIDs []int64, keyword string) ([]int64, error) {
	var issueIDs []int64
	indexer := holder.get()

	if indexer == nil {
		log.Error("SearchIssuesByKeyword(): unable to get indexer!")
		return nil, fmt.Errorf("unable to get issue indexer")
	}
	res, err := indexer.Search(keyword, repoIDs, 50, 0)
	if err != nil {
		return nil, err
	}
	for _, r := range res.Hits {
		issueIDs = append(issueIDs, r.ID)
	}
	return issueIDs, nil
}
