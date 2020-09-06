// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"math"
	"strings"

	"code.gitea.io/gitea/modules/timeutil"

	"github.com/go-enry/go-enry/v2"
)

// LanguageStat describes language statistics of a repository
type LanguageStat struct {
	ID          int64 `xorm:"pk autoincr"`
	RepoID      int64 `xorm:"UNIQUE(s) INDEX NOT NULL"`
	CommitID    string
	IsPrimary   bool
	Language    string             `xorm:"VARCHAR(50) UNIQUE(s) INDEX NOT NULL"`
	Percentage  float32            `xorm:"-"`
	Size        int64              `xorm:"NOT NULL DEFAULT 0"`
	Color       string             `xorm:"-"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX CREATED"`
}

// LanguageStatList defines a list of language statistics
type LanguageStatList []*LanguageStat

// TableName sets the table name to `language_stat`
func (stat LanguageStat) TableName() string {
	return tbLanguageStat[1 : len(tbLanguageStat)-1]
}

func (stats LanguageStatList) loadAttributes() {
	for i := range stats {
		stats[i].Color = enry.GetColor(stats[i].Language)
	}
}

func (stats LanguageStatList) getLanguagePercentages() map[string]float32 {
	langPerc := make(map[string]float32)
	var otherPerc float32 = 100
	var total int64

	for _, stat := range stats {
		total += stat.Size
	}
	if total > 0 {
		for _, stat := range stats {
			perc := float32(math.Round(float64(stat.Size)/float64(total)*1000) / 10)
			if perc <= 0.1 {
				continue
			}
			otherPerc -= perc
			langPerc[stat.Language] = perc
		}
		otherPerc = float32(math.Round(float64(otherPerc)*10) / 10)
	}
	if otherPerc > 0 {
		langPerc["other"] = otherPerc
	}
	return langPerc
}

func (repo *Repository) getLanguageStats(e Engine) (LanguageStatList, error) {
	stats := make(LanguageStatList, 0, 6)
	if err := e.Where("`repo_id` = ?", repo.ID).Desc("`size`").Find(&stats); err != nil {
		return nil, err
	}
	return stats, nil
}

// GetLanguageStats returns the language statistics for a repository
func (repo *Repository) GetLanguageStats() (LanguageStatList, error) {
	return repo.getLanguageStats(x)
}

// GetTopLanguageStats returns the top language statistics for a repository
func (repo *Repository) GetTopLanguageStats(limit int) (LanguageStatList, error) {
	stats, err := repo.getLanguageStats(x)
	if err != nil {
		return nil, err
	}
	perc := stats.getLanguagePercentages()
	topstats := make(LanguageStatList, 0, limit)
	var other float32
	for i := range stats {
		if _, ok := perc[stats[i].Language]; !ok {
			continue
		}
		if stats[i].Language == "other" || len(topstats) >= limit {
			other += perc[stats[i].Language]
			continue
		}
		stats[i].Percentage = perc[stats[i].Language]
		topstats = append(topstats, stats[i])
	}
	if other > 0 {
		topstats = append(topstats, &LanguageStat{
			RepoID:     repo.ID,
			Language:   "other",
			Color:      "#cccccc",
			Percentage: float32(math.Round(float64(other)*10) / 10),
		})
	}
	topstats.loadAttributes()
	return topstats, nil
}

// UpdateLanguageStats updates the language statistics for repository
func (repo *Repository) UpdateLanguageStats(commitID string, stats map[string]int64) error {
	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		return err
	}
	defer sess.Close()

	oldstats, err := repo.getLanguageStats(sess)
	if err != nil {
		return err
	}
	var topLang string
	var s int64
	for lang, size := range stats {
		if size > s {
			s = size
			topLang = strings.ToLower(lang)
		}
	}

	for lang, size := range stats {
		upd := false
		llang := strings.ToLower(lang)
		for _, s := range oldstats {
			// Update already existing language
			if strings.ToLower(s.Language) == llang {
				s.CommitID = commitID
				s.IsPrimary = llang == topLang
				s.Size = size
				if _, err := sess.ID(s.ID).Cols("`commit_id`", "`size`", "`is_primary`").Update(s); err != nil {
					return err
				}
				upd = true
				break
			}
		}
		// Insert new language
		if !upd {
			if _, err := sess.Insert(&LanguageStat{
				RepoID:    repo.ID,
				CommitID:  commitID,
				IsPrimary: llang == topLang,
				Language:  lang,
				Size:      size,
			}); err != nil {
				return err
			}
		}
	}
	// Delete old languages
	statsToDelete := make([]int64, 0, len(oldstats))
	for _, s := range oldstats {
		if s.CommitID != commitID {
			statsToDelete = append(statsToDelete, s.ID)
		}
	}
	if len(statsToDelete) > 0 {
		if _, err := sess.In("`id`", statsToDelete).Delete(&LanguageStat{}); err != nil {
			return err
		}
	}

	// Update indexer status
	if err = repo.updateIndexerStatus(sess, RepoIndexerTypeStats, commitID); err != nil {
		return err
	}

	return sess.Commit()
}

// CopyLanguageStat Copy originalRepo language stat information to destRepo (use for forked repo)
func CopyLanguageStat(originalRepo, destRepo *Repository) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	RepoLang := make(LanguageStatList, 0, 6)
	if err := sess.Where("`repo_id` = ?", originalRepo.ID).Desc("`size`").Find(&RepoLang); err != nil {
		return err
	}
	if len(RepoLang) > 0 {
		for i := range RepoLang {
			RepoLang[i].ID = 0
			RepoLang[i].RepoID = destRepo.ID
			RepoLang[i].CreatedUnix = timeutil.TimeStampNow()
		}
		//update destRepo's indexer status
		tmpCommitID := RepoLang[0].CommitID
		if err := destRepo.updateIndexerStatus(sess, RepoIndexerTypeStats, tmpCommitID); err != nil {
			return err
		}
		if _, err := sess.Insert(&RepoLang); err != nil {
			return err
		}
	}
	return sess.Commit()
}
