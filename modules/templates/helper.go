// Copyright 2018 The Gitea Authors. All rights reserved.
// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package templates

import (
	"bytes"
	"container/list"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"mime"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/util"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
	"gopkg.in/editorconfig/editorconfig-core-go.v1"
)

// NewFuncMap returns functions for injecting to templates
func NewFuncMap() []template.FuncMap {
	return []template.FuncMap{map[string]interface{}{
		"GoVer": func() string {
			return strings.Title(runtime.Version())
		},
		"UseHTTPS": func() bool {
			return strings.HasPrefix(setting.AppURL, "https")
		},
		"AppName": func() string {
			return setting.AppName
		},
		"AppSubUrl": func() string {
			return setting.AppSubURL
		},
		"AppUrl": func() string {
			return setting.AppURL
		},
		"AppVer": func() string {
			return setting.AppVer
		},
		"AppBuiltWith": func() string {
			return setting.AppBuiltWith
		},
		"AppDomain": func() string {
			return setting.Domain
		},
		"DisableGravatar": func() bool {
			return setting.DisableGravatar
		},
		"DefaultShowFullName": func() bool {
			return setting.UI.DefaultShowFullName
		},
		"ShowFooterTemplateLoadTime": func() bool {
			return setting.ShowFooterTemplateLoadTime
		},
		"LoadTimes": func(startTime time.Time) string {
			return fmt.Sprint(time.Since(startTime).Nanoseconds()/1e6) + "ms"
		},
		"AvatarLink":    base.AvatarLink,
		"Safe":          Safe,
		"SafeJS":        SafeJS,
		"Str2html":      Str2html,
		"TimeSince":     base.TimeSince,
		"TimeSinceUnix": base.TimeSinceUnix,
		"RawTimeSince":  base.RawTimeSince,
		"FileSize":      base.FileSize,
		"Subtract":      base.Subtract,
		"EntryIcon":     base.EntryIcon,
		"Add": func(a, b int) int {
			return a + b
		},
		"ActionIcon": ActionIcon,
		"DateFmtLong": func(t time.Time) string {
			return t.Format(time.RFC1123Z)
		},
		"DateFmtShort": func(t time.Time) string {
			return t.Format("Jan 02, 2006")
		},
		"SizeFmt": func(s int64) string {
			return base.FileSize(s)
		},
		"List": List,
		"SubStr": func(str string, start, length int) string {
			if len(str) == 0 {
				return ""
			}
			end := start + length
			if length == -1 {
				end = len(str)
			}
			if len(str) < end {
				return str
			}
			return str[start:end]
		},
		"EllipsisString":        base.EllipsisString,
		"DiffTypeToStr":         DiffTypeToStr,
		"DiffLineTypeToStr":     DiffLineTypeToStr,
		"Sha1":                  Sha1,
		"ShortSha":              base.ShortSha,
		"MD5":                   base.EncodeMD5,
		"ActionContent2Commits": ActionContent2Commits,
		"PathEscape":            url.PathEscape,
		"EscapePound": func(str string) string {
			return strings.NewReplacer("%", "%25", "#", "%23", " ", "%20", "?", "%3F").Replace(str)
		},
		"PathEscapeSegments":       util.PathEscapeSegments,
		"URLJoin":                  util.URLJoin,
		"RenderCommitMessage":      RenderCommitMessage,
		"RenderCommitMessageLink":  RenderCommitMessageLink,
		"RenderCommitBody":         RenderCommitBody,
		"RenderNote":               RenderNote,
		"IsMultilineCommitMessage": IsMultilineCommitMessage,
		"ThemeColorMetaTag": func() string {
			return setting.UI.ThemeColorMetaTag
		},
		"MetaAuthor": func() string {
			return setting.UI.Meta.Author
		},
		"MetaDescription": func() string {
			return setting.UI.Meta.Description
		},
		"MetaKeywords": func() string {
			return setting.UI.Meta.Keywords
		},
		"FilenameIsImage": func(filename string) bool {
			mimeType := mime.TypeByExtension(filepath.Ext(filename))
			return strings.HasPrefix(mimeType, "image/")
		},
		"TabSizeClass": func(ec *editorconfig.Editorconfig, filename string) string {
			if ec != nil {
				def := ec.GetDefinitionForFilename(filename)
				if def.TabWidth > 0 {
					return fmt.Sprintf("tab-size-%d", def.TabWidth)
				}
			}
			return "tab-size-8"
		},
		"SubJumpablePath": func(str string) []string {
			var path []string
			index := strings.LastIndex(str, "/")
			if index != -1 && index != len(str) {
				path = append(path, str[0:index+1], str[index+1:])
			} else {
				path = append(path, str)
			}
			return path
		},
		"JsonPrettyPrint": func(in string) string {
			var out bytes.Buffer
			err := json.Indent(&out, []byte(in), "", "  ")
			if err != nil {
				return ""
			}
			return out.String()
		},
		"DisableGitHooks": func() bool {
			return setting.DisableGitHooks
		},
		"DisableImportLocal": func() bool {
			return !setting.ImportLocalPaths
		},
		"TrN": TrN,
		"Dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, errors.New("invalid dict call")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, errors.New("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"Printf":   fmt.Sprintf,
		"Escape":   Escape,
		"Sec2Time": models.SecToTime,
		"ParseDeadline": func(deadline string) []string {
			return strings.Split(deadline, "|")
		},
		"DefaultTheme": func() string {
			return setting.UI.DefaultTheme
		},
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values) == 0 {
				return nil, errors.New("invalid dict call")
			}

			dict := make(map[string]interface{})

			for i := 0; i < len(values); i++ {
				switch key := values[i].(type) {
				case string:
					i++
					if i == len(values) {
						return nil, errors.New("specify the key for non array values")
					}
					dict[key] = values[i]
				case map[string]interface{}:
					m := values[i].(map[string]interface{})
					for i, v := range m {
						dict[i] = v
					}
				default:
					return nil, errors.New("dict values must be maps")
				}
			}
			return dict, nil
		},
		"percentage": func(n int, values ...int) float32 {
			var sum = 0
			for i := 0; i < len(values); i++ {
				sum += values[i]
			}
			return float32(n) * 100 / float32(sum)
		},
		"Ternary": func(condition bool, truthyValue string, falsyValue string) string {
			if condition {
				return truthyValue
			}

			return falsyValue
		},
	}}
}

// Safe render raw as HTML
func Safe(raw string) template.HTML {
	return template.HTML(raw)
}

// SafeJS renders raw as JS
func SafeJS(raw string) template.JS {
	return template.JS(raw)
}

// Str2html render Markdown text to HTML
func Str2html(raw string) template.HTML {
	return template.HTML(markup.Sanitize(raw))
}

// Escape escapes a HTML string
func Escape(raw string) string {
	return html.EscapeString(raw)
}

// List traversings the list
func List(l *list.List) chan interface{} {
	e := l.Front()
	c := make(chan interface{})
	go func() {
		for e != nil {
			c <- e.Value
			e = e.Next()
		}
		close(c)
	}()
	return c
}

// Sha1 returns sha1 sum of string
func Sha1(str string) string {
	return base.EncodeSha1(str)
}

// ToUTF8WithErr converts content to UTF8 encoding
func ToUTF8WithErr(content []byte) (string, error) {
	charsetLabel, err := base.DetectEncoding(content)
	if err != nil {
		return "", err
	} else if charsetLabel == "UTF-8" {
		return string(base.RemoveBOMIfPresent(content)), nil
	}

	encoding, _ := charset.Lookup(charsetLabel)
	if encoding == nil {
		return string(content), fmt.Errorf("Unknown encoding: %s", charsetLabel)
	}

	// If there is an error, we concatenate the nicely decoded part and the
	// original left over. This way we won't lose data.
	result, n, err := transform.Bytes(encoding.NewDecoder(), content)
	if err != nil {
		result = append(result, content[n:]...)
	}

	result = base.RemoveBOMIfPresent(result)

	return string(result), err
}

// ToUTF8WithFallback detects the encoding of content and coverts to UTF-8 if possible
func ToUTF8WithFallback(content []byte) []byte {
	charsetLabel, err := base.DetectEncoding(content)
	if err != nil || charsetLabel == "UTF-8" {
		return base.RemoveBOMIfPresent(content)
	}

	encoding, _ := charset.Lookup(charsetLabel)
	if encoding == nil {
		return content
	}

	// If there is an error, we concatenate the nicely decoded part and the
	// original left over. This way we won't lose data.
	result, n, err := transform.Bytes(encoding.NewDecoder(), content)
	if err != nil {
		return append(result, content[n:]...)
	}

	return base.RemoveBOMIfPresent(result)
}

// ToUTF8 converts content to UTF8 encoding and ignore error
func ToUTF8(content string) string {
	res, _ := ToUTF8WithErr([]byte(content))
	return res
}

// ReplaceLeft replaces all prefixes 'oldS' in 's' with 'newS'.
func ReplaceLeft(s, oldS, newS string) string {
	oldLen, newLen, i, n := len(oldS), len(newS), 0, 0
	for ; i < len(s) && strings.HasPrefix(s[i:], oldS); n++ {
		i += oldLen
	}

	// simple optimization
	if n == 0 {
		return s
	}

	// allocating space for the new string
	curLen := n*newLen + len(s[i:])
	replacement := make([]byte, curLen)

	j := 0
	for ; j < n*newLen; j += newLen {
		copy(replacement[j:j+newLen], newS)
	}

	copy(replacement[j:], s[i:])
	return string(replacement)
}

// RenderCommitMessage renders commit message with XSS-safe and special links.
func RenderCommitMessage(msg, urlPrefix string, metas map[string]string) template.HTML {
	return RenderCommitMessageLink(msg, urlPrefix, "", metas)
}

// RenderCommitMessageLink renders commit message as a XXS-safe link to the provided
// default url, handling for special links.
func RenderCommitMessageLink(msg, urlPrefix, urlDefault string, metas map[string]string) template.HTML {
	cleanMsg := template.HTMLEscapeString(msg)
	// we can safely assume that it will not return any error, since there
	// shouldn't be any special HTML.
	fullMessage, err := markup.RenderCommitMessage([]byte(cleanMsg), urlPrefix, urlDefault, metas)
	if err != nil {
		log.Error("RenderCommitMessage: %v", err)
		return ""
	}
	msgLines := strings.Split(strings.TrimSpace(string(fullMessage)), "\n")
	if len(msgLines) == 0 {
		return template.HTML("")
	}
	return template.HTML(msgLines[0])
}

// RenderCommitBody extracts the body of a commit message without its title.
func RenderCommitBody(msg, urlPrefix string, metas map[string]string) template.HTML {
	cleanMsg := template.HTMLEscapeString(msg)
	fullMessage, err := markup.RenderCommitMessage([]byte(cleanMsg), urlPrefix, "", metas)
	if err != nil {
		log.Error("RenderCommitMessage: %v", err)
		return ""
	}
	body := strings.Split(strings.TrimSpace(string(fullMessage)), "\n")
	if len(body) == 0 {
		return template.HTML("")
	}
	return template.HTML(strings.Join(body[1:], "\n"))
}

// RenderNote renders the contents of a git-notes file as a commit message.
func RenderNote(msg, urlPrefix string, metas map[string]string) template.HTML {
	cleanMsg := template.HTMLEscapeString(msg)
	fullMessage, err := markup.RenderCommitMessage([]byte(cleanMsg), urlPrefix, "", metas)
	if err != nil {
		log.Error("RenderNote: %v", err)
		return ""
	}
	return template.HTML(string(fullMessage))
}

// IsMultilineCommitMessage checks to see if a commit message contains multiple lines.
func IsMultilineCommitMessage(msg string) bool {
	return strings.Count(strings.TrimSpace(msg), "\n") >= 1
}

// Actioner describes an action
type Actioner interface {
	GetOpType() models.ActionType
	GetActUserName() string
	GetRepoUserName() string
	GetRepoName() string
	GetRepoPath() string
	GetRepoLink() string
	GetBranch() string
	GetContent() string
	GetCreate() time.Time
	GetIssueInfos() []string
}

// ActionIcon accepts an action operation type and returns an icon class name.
func ActionIcon(opType models.ActionType) string {
	switch opType {
	case models.ActionCreateRepo, models.ActionTransferRepo:
		return "repo"
	case models.ActionCommitRepo, models.ActionPushTag, models.ActionDeleteTag, models.ActionDeleteBranch:
		return "git-commit"
	case models.ActionCreateIssue:
		return "issue-opened"
	case models.ActionCreatePullRequest:
		return "git-pull-request"
	case models.ActionCommentIssue:
		return "comment-discussion"
	case models.ActionMergePullRequest:
		return "git-merge"
	case models.ActionCloseIssue, models.ActionClosePullRequest:
		return "issue-closed"
	case models.ActionReopenIssue, models.ActionReopenPullRequest:
		return "issue-reopened"
	case models.ActionMirrorSyncPush, models.ActionMirrorSyncCreate, models.ActionMirrorSyncDelete:
		return "repo-clone"
	default:
		return "invalid type"
	}
}

// ActionContent2Commits converts action content to push commits
func ActionContent2Commits(act Actioner) *models.PushCommits {
	push := models.NewPushCommits()
	if err := json.Unmarshal([]byte(act.GetContent()), push); err != nil {
		log.Error("json.Unmarshal:\n%s\nERROR: %v", act.GetContent(), err)
	}
	return push
}

// DiffTypeToStr returns diff type name
func DiffTypeToStr(diffType int) string {
	diffTypes := map[int]string{
		1: "add", 2: "modify", 3: "del", 4: "rename",
	}
	return diffTypes[diffType]
}

// DiffLineTypeToStr returns diff line type name
func DiffLineTypeToStr(diffType int) string {
	switch diffType {
	case 2:
		return "add"
	case 3:
		return "del"
	case 4:
		return "tag"
	}
	return "same"
}

// Language specific rules for translating plural texts
var trNLangRules = map[string]func(int64) int{
	"en-US": func(cnt int64) int {
		if cnt == 1 {
			return 0
		}
		return 1
	},
	"lv-LV": func(cnt int64) int {
		if cnt%10 == 1 && cnt%100 != 11 {
			return 0
		}
		return 1
	},
	"ru-RU": func(cnt int64) int {
		if cnt%10 == 1 && cnt%100 != 11 {
			return 0
		}
		return 1
	},
	"zh-CN": func(cnt int64) int {
		return 0
	},
	"zh-HK": func(cnt int64) int {
		return 0
	},
	"zh-TW": func(cnt int64) int {
		return 0
	},
	"fr-FR": func(cnt int64) int {
		if cnt > -2 && cnt < 2 {
			return 0
		}
		return 1
	},
}

// TrN returns key to be used for plural text translation
func TrN(lang string, cnt interface{}, key1, keyN string) string {
	var c int64
	if t, ok := cnt.(int); ok {
		c = int64(t)
	} else if t, ok := cnt.(int16); ok {
		c = int64(t)
	} else if t, ok := cnt.(int32); ok {
		c = int64(t)
	} else if t, ok := cnt.(int64); ok {
		c = t
	} else {
		return keyN
	}

	ruleFunc, ok := trNLangRules[lang]
	if !ok {
		ruleFunc = trNLangRules["en-US"]
	}

	if ruleFunc(c) == 0 {
		return key1
	}
	return keyN
}
