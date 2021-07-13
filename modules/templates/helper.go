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
	"reflect"
	"regexp"
	"runtime"
	"strings"
	texttmpl "text/template"
	"time"
	"unicode"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/emoji"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/svg"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/gitdiff"

	"github.com/editorconfig/editorconfig-core-go/v2"
	jsoniter "github.com/json-iterator/go"
)

// Used from static.go && dynamic.go
var mailSubjectSplit = regexp.MustCompile(`(?m)^-{3,}[\s]*$`)

// NewFuncMap returns functions for injecting to templates
func NewFuncMap() []template.FuncMap {
	jsonED := jsoniter.ConfigCompatibleWithStandardLibrary
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
		"AssetUrlPrefix": func() string {
			return setting.StaticURLPrefix + "/assets"
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
		"AllowedReactions": func() []string {
			return setting.UI.Reactions
		},
		"CustomEmojis": func() map[string]string {
			return setting.UI.CustomEmojisMap
		},
		"Safe":          Safe,
		"SafeJS":        SafeJS,
		"JSEscape":      JSEscape,
		"Str2html":      Str2html,
		"TimeSince":     timeutil.TimeSince,
		"TimeSinceUnix": timeutil.TimeSinceUnix,
		"RawTimeSince":  timeutil.RawTimeSince,
		"FileSize":      base.FileSize,
		"PrettyNumber":  base.PrettyNumber,
		"Subtract":      base.Subtract,
		"EntryIcon":     base.EntryIcon,
		"MigrationIcon": MigrationIcon,
		"Add": func(a ...int) int {
			sum := 0
			for _, val := range a {
				sum += val
			}
			return sum
		},
		"Mul": func(a ...int) int {
			sum := 1
			for _, val := range a {
				sum *= val
			}
			return sum
		},
		"ActionIcon": ActionIcon,
		"DateFmtLong": func(t time.Time) string {
			return t.Format(time.RFC1123Z)
		},
		"DateFmtShort": func(t time.Time) string {
			return t.Format("Jan 02, 2006")
		},
		"SizeFmt":  base.FileSize,
		"CountFmt": base.FormatNumberSI,
		"List":     List,
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
		"PathEscapeSegments":             util.PathEscapeSegments,
		"URLJoin":                        util.URLJoin,
		"RenderCommitMessage":            RenderCommitMessage,
		"RenderCommitMessageLink":        RenderCommitMessageLink,
		"RenderCommitMessageLinkSubject": RenderCommitMessageLinkSubject,
		"RenderCommitBody":               RenderCommitBody,
		"RenderIssueTitle":               RenderIssueTitle,
		"RenderEmoji":                    RenderEmoji,
		"RenderEmojiPlain":               emoji.ReplaceAliases,
		"ReactionToEmoji":                ReactionToEmoji,
		"RenderNote":                     RenderNote,
		"IsMultilineCommitMessage":       IsMultilineCommitMessage,
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
		"UseServiceWorker": func() bool {
			return setting.UI.UseServiceWorker
		},
		"EnableTimetracking": func() bool {
			return setting.Service.EnableTimetracking
		},
		"FilenameIsImage": func(filename string) bool {
			mimeType := mime.TypeByExtension(filepath.Ext(filename))
			return strings.HasPrefix(mimeType, "image/")
		},
		"TabSizeClass": func(ec interface{}, filename string) string {
			var (
				value *editorconfig.Editorconfig
				ok    bool
			)
			if ec != nil {
				if value, ok = ec.(*editorconfig.Editorconfig); !ok || value == nil {
					return "tab-size-8"
				}
				def, err := value.GetDefinitionForFilename(filename)
				if err != nil {
					log.Error("tab size class: getting definition for filename: %v", err)
					return "tab-size-8"
				}
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
		"DiffStatsWidth": func(adds int, dels int) string {
			return fmt.Sprintf("%f", float64(adds)/(float64(adds)+float64(dels))*100)
		},
		"Json": func(in interface{}) string {
			out, err := jsonED.Marshal(in)
			if err != nil {
				return ""
			}
			return string(out)
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
		"DisableWebhooks": func() bool {
			return setting.DisableWebhooks
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
		// pass key-value pairs to a partial template which receives them as a dict
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values) == 0 {
				return nil, errors.New("invalid dict call")
			}

			dict := make(map[string]interface{})
			return util.MergeInto(dict, values...)
		},
		/* like dict but merge key-value pairs into the first dict and return it */
		"mergeinto": func(root map[string]interface{}, values ...interface{}) (map[string]interface{}, error) {
			if len(values) == 0 {
				return nil, errors.New("invalid mergeinto call")
			}

			dict := make(map[string]interface{})
			for key, value := range root {
				dict[key] = value
			}

			return util.MergeInto(dict, values...)
		},
		"percentage": func(n int, values ...int) float32 {
			var sum = 0
			for i := 0; i < len(values); i++ {
				sum += values[i]
			}
			return float32(n) * 100 / float32(sum)
		},
		"CommentMustAsDiff":   gitdiff.CommentMustAsDiff,
		"MirrorRemoteAddress": mirrorRemoteAddress,
		"CommitType": func(commit interface{}) string {
			switch commit.(type) {
			case models.SignCommitWithStatuses:
				return "SignCommitWithStatuses"
			case models.SignCommit:
				return "SignCommit"
			case models.UserCommit:
				return "UserCommit"
			default:
				return ""
			}
		},
		"NotificationSettings": func() map[string]interface{} {
			return map[string]interface{}{
				"MinTimeout":            int(setting.UI.Notification.MinTimeout / time.Millisecond),
				"TimeoutStep":           int(setting.UI.Notification.TimeoutStep / time.Millisecond),
				"MaxTimeout":            int(setting.UI.Notification.MaxTimeout / time.Millisecond),
				"EventSourceUpdateTime": int(setting.UI.Notification.EventSourceUpdateTime / time.Millisecond),
			}
		},
		"containGeneric": func(arr interface{}, v interface{}) bool {
			arrV := reflect.ValueOf(arr)
			if arrV.Kind() == reflect.String && reflect.ValueOf(v).Kind() == reflect.String {
				return strings.Contains(arr.(string), v.(string))
			}

			if arrV.Kind() == reflect.Slice {
				for i := 0; i < arrV.Len(); i++ {
					iV := arrV.Index(i)
					if !iV.CanInterface() {
						continue
					}
					if iV.Interface() == v {
						return true
					}
				}
			}

			return false
		},
		"contain": func(s []int64, id int64) bool {
			for i := 0; i < len(s); i++ {
				if s[i] == id {
					return true
				}
			}
			return false
		},
		"svg":            SVG,
		"avatar":         Avatar,
		"avatarHTML":     AvatarHTML,
		"avatarByAction": AvatarByAction,
		"avatarByEmail":  AvatarByEmail,
		"repoAvatar":     RepoAvatar,
		"SortArrow": func(normSort, revSort, urlSort string, isDefault bool) template.HTML {
			// if needed
			if len(normSort) == 0 || len(urlSort) == 0 {
				return ""
			}

			if len(urlSort) == 0 && isDefault {
				// if sort is sorted as default add arrow tho this table header
				if isDefault {
					return SVG("octicon-triangle-down", 16)
				}
			} else {
				// if sort arg is in url test if it correlates with column header sort arguments
				if urlSort == normSort {
					// the table is sorted with this header normal
					return SVG("octicon-triangle-down", 16)
				} else if urlSort == revSort {
					// the table is sorted with this header reverse
					return SVG("octicon-triangle-up", 16)
				}
			}
			// the table is NOT sorted with this header
			return ""
		},
		"RenderLabels": func(labels []*models.Label) template.HTML {
			html := `<span class="labels-list">`
			for _, label := range labels {
				// Protect against nil value in labels - shouldn't happen but would cause a panic if so
				if label == nil {
					continue
				}
				html += fmt.Sprintf("<div class='ui label' style='color: %s; background-color: %s'>%s</div> ",
					label.ForegroundColor(), label.Color, RenderEmoji(label.Name))
			}
			html += "</span>"
			return template.HTML(html)
		},
	}}
}

// NewTextFuncMap returns functions for injecting to text templates
// It's a subset of those used for HTML and other templates
func NewTextFuncMap() []texttmpl.FuncMap {
	return []texttmpl.FuncMap{map[string]interface{}{
		"GoVer": func() string {
			return strings.Title(runtime.Version())
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
		"TimeSince":     timeutil.TimeSince,
		"TimeSinceUnix": timeutil.TimeSinceUnix,
		"RawTimeSince":  timeutil.RawTimeSince,
		"DateFmtLong": func(t time.Time) string {
			return t.Format(time.RFC1123Z)
		},
		"DateFmtShort": func(t time.Time) string {
			return t.Format("Jan 02, 2006")
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
		"EllipsisString": base.EllipsisString,
		"URLJoin":        util.URLJoin,
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
		"Add": func(a ...int) int {
			sum := 0
			for _, val := range a {
				sum += val
			}
			return sum
		},
		"Mul": func(a ...int) int {
			sum := 1
			for _, val := range a {
				sum *= val
			}
			return sum
		},
	}}
}

var widthRe = regexp.MustCompile(`width="[0-9]+?"`)
var heightRe = regexp.MustCompile(`height="[0-9]+?"`)

func parseOthers(defaultSize int, defaultClass string, others ...interface{}) (int, string) {
	size := defaultSize
	if len(others) > 0 && others[0].(int) != 0 {
		size = others[0].(int)
	}

	class := defaultClass
	if len(others) > 1 && others[1].(string) != "" {
		if defaultClass == "" {
			class = others[1].(string)
		} else {
			class = defaultClass + " " + others[1].(string)
		}
	}

	return size, class
}

// AvatarHTML creates the HTML for an avatar
func AvatarHTML(src string, size int, class string, name string) template.HTML {
	sizeStr := fmt.Sprintf(`%d`, size)

	if name == "" {
		name = "avatar"
	}

	return template.HTML(`<img class="` + class + `" src="` + src + `" title="` + html.EscapeString(name) + `" width="` + sizeStr + `" height="` + sizeStr + `"/>`)
}

// SVG render icons - arguments icon name (string), size (int), class (string)
func SVG(icon string, others ...interface{}) template.HTML {
	size, class := parseOthers(16, "", others...)

	if svgStr, ok := svg.SVGs[icon]; ok {
		if size != 16 {
			svgStr = widthRe.ReplaceAllString(svgStr, fmt.Sprintf(`width="%d"`, size))
			svgStr = heightRe.ReplaceAllString(svgStr, fmt.Sprintf(`height="%d"`, size))
		}
		if class != "" {
			svgStr = strings.Replace(svgStr, `class="`, fmt.Sprintf(`class="%s `, class), 1)
		}
		return template.HTML(svgStr)
	}
	return template.HTML("")
}

// Avatar renders user avatars. args: user, size (int), class (string)
func Avatar(item interface{}, others ...interface{}) template.HTML {
	size, class := parseOthers(models.DefaultAvatarPixelSize, "ui avatar image", others...)

	if user, ok := item.(*models.User); ok {
		src := user.RealSizedAvatarLink(size * models.AvatarRenderedSizeFactor)
		if src != "" {
			return AvatarHTML(src, size, class, user.DisplayName())
		}
	}
	if user, ok := item.(*models.Collaborator); ok {
		src := user.RealSizedAvatarLink(size * models.AvatarRenderedSizeFactor)
		if src != "" {
			return AvatarHTML(src, size, class, user.DisplayName())
		}
	}
	return template.HTML("")
}

// AvatarByAction renders user avatars from action. args: action, size (int), class (string)
func AvatarByAction(action *models.Action, others ...interface{}) template.HTML {
	action.LoadActUser()
	return Avatar(action.ActUser, others...)
}

// RepoAvatar renders repo avatars. args: repo, size(int), class (string)
func RepoAvatar(repo *models.Repository, others ...interface{}) template.HTML {
	size, class := parseOthers(models.DefaultAvatarPixelSize, "ui avatar image", others...)

	src := repo.RelAvatarLink()
	if src != "" {
		return AvatarHTML(src, size, class, repo.FullName())
	}
	return template.HTML("")
}

// AvatarByEmail renders avatars by email address. args: email, name, size (int), class (string)
func AvatarByEmail(email string, name string, others ...interface{}) template.HTML {
	size, class := parseOthers(models.DefaultAvatarPixelSize, "ui avatar image", others...)
	src := models.SizedAvatarLink(email, size*models.AvatarRenderedSizeFactor)

	if src != "" {
		return AvatarHTML(src, size, class, name)
	}

	return template.HTML("")
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

// JSEscape escapes a JS string
func JSEscape(raw string) string {
	return template.JSEscapeString(raw)
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
	fullMessage, err := markup.RenderCommitMessage(&markup.RenderContext{
		URLPrefix:   urlPrefix,
		DefaultLink: urlDefault,
		Metas:       metas,
	}, cleanMsg)
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

// RenderCommitMessageLinkSubject renders commit message as a XXS-safe link to
// the provided default url, handling for special links without email to links.
func RenderCommitMessageLinkSubject(msg, urlPrefix, urlDefault string, metas map[string]string) template.HTML {
	msgLine := strings.TrimLeftFunc(msg, unicode.IsSpace)
	lineEnd := strings.IndexByte(msgLine, '\n')
	if lineEnd > 0 {
		msgLine = msgLine[:lineEnd]
	}
	msgLine = strings.TrimRightFunc(msgLine, unicode.IsSpace)
	if len(msgLine) == 0 {
		return template.HTML("")
	}

	// we can safely assume that it will not return any error, since there
	// shouldn't be any special HTML.
	renderedMessage, err := markup.RenderCommitMessageSubject(&markup.RenderContext{
		URLPrefix:   urlPrefix,
		DefaultLink: urlDefault,
		Metas:       metas,
	}, template.HTMLEscapeString(msgLine))
	if err != nil {
		log.Error("RenderCommitMessageSubject: %v", err)
		return template.HTML("")
	}
	return template.HTML(renderedMessage)
}

// RenderCommitBody extracts the body of a commit message without its title.
func RenderCommitBody(msg, urlPrefix string, metas map[string]string) template.HTML {
	msgLine := strings.TrimRightFunc(msg, unicode.IsSpace)
	lineEnd := strings.IndexByte(msgLine, '\n')
	if lineEnd > 0 {
		msgLine = msgLine[lineEnd+1:]
	} else {
		return template.HTML("")
	}
	msgLine = strings.TrimLeftFunc(msgLine, unicode.IsSpace)
	if len(msgLine) == 0 {
		return template.HTML("")
	}

	renderedMessage, err := markup.RenderCommitMessage(&markup.RenderContext{
		URLPrefix: urlPrefix,
		Metas:     metas,
	}, template.HTMLEscapeString(msgLine))
	if err != nil {
		log.Error("RenderCommitMessage: %v", err)
		return ""
	}
	return template.HTML(renderedMessage)
}

// RenderIssueTitle renders issue/pull title with defined post processors
func RenderIssueTitle(text, urlPrefix string, metas map[string]string) template.HTML {
	renderedText, err := markup.RenderIssueTitle(&markup.RenderContext{
		URLPrefix: urlPrefix,
		Metas:     metas,
	}, template.HTMLEscapeString(text))
	if err != nil {
		log.Error("RenderIssueTitle: %v", err)
		return template.HTML("")
	}
	return template.HTML(renderedText)
}

// RenderEmoji renders html text with emoji post processors
func RenderEmoji(text string) template.HTML {
	renderedText, err := markup.RenderEmoji(template.HTMLEscapeString(text))
	if err != nil {
		log.Error("RenderEmoji: %v", err)
		return template.HTML("")
	}
	return template.HTML(renderedText)
}

//ReactionToEmoji renders emoji for use in reactions
func ReactionToEmoji(reaction string) template.HTML {
	val := emoji.FromCode(reaction)
	if val != nil {
		return template.HTML(val.Emoji)
	}
	val = emoji.FromAlias(reaction)
	if val != nil {
		return template.HTML(val.Emoji)
	}
	return template.HTML(fmt.Sprintf(`<img alt=":%s:" src="%s/assets/img/emoji/%s.png"></img>`, reaction, setting.StaticURLPrefix, reaction))
}

// RenderNote renders the contents of a git-notes file as a commit message.
func RenderNote(msg, urlPrefix string, metas map[string]string) template.HTML {
	cleanMsg := template.HTMLEscapeString(msg)
	fullMessage, err := markup.RenderCommitMessage(&markup.RenderContext{
		URLPrefix: urlPrefix,
		Metas:     metas,
	}, cleanMsg)
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
	case models.ActionCommentIssue, models.ActionCommentPull:
		return "comment-discussion"
	case models.ActionMergePullRequest:
		return "git-merge"
	case models.ActionCloseIssue, models.ActionClosePullRequest:
		return "issue-closed"
	case models.ActionReopenIssue, models.ActionReopenPullRequest:
		return "issue-reopened"
	case models.ActionMirrorSyncPush, models.ActionMirrorSyncCreate, models.ActionMirrorSyncDelete:
		return "mirror"
	case models.ActionApprovePullRequest:
		return "check"
	case models.ActionRejectPullRequest:
		return "diff"
	case models.ActionPublishRelease:
		return "tag"
	case models.ActionPullReviewDismissed:
		return "x"
	default:
		return "question"
	}
}

// ActionContent2Commits converts action content to push commits
func ActionContent2Commits(act Actioner) *repository.PushCommits {
	push := repository.NewPushCommits()

	if act == nil || act.GetContent() == "" {
		return push
	}

	json := jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal([]byte(act.GetContent()), push); err != nil {
		log.Error("json.Unmarshal:\n%s\nERROR: %v", act.GetContent(), err)
	}
	return push
}

// DiffTypeToStr returns diff type name
func DiffTypeToStr(diffType int) string {
	diffTypes := map[int]string{
		1: "add", 2: "modify", 3: "del", 4: "rename", 5: "copy",
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

// MigrationIcon returns a Font Awesome name matching the service an issue/comment was migrated from
func MigrationIcon(hostname string) string {
	switch hostname {
	case "github.com":
		return "fa-github"
	default:
		return "fa-git-alt"
	}
}

func buildSubjectBodyTemplate(stpl *texttmpl.Template, btpl *template.Template, name string, content []byte) {
	// Split template into subject and body
	var subjectContent []byte
	bodyContent := content
	loc := mailSubjectSplit.FindIndex(content)
	if loc != nil {
		subjectContent = content[0:loc[0]]
		bodyContent = content[loc[1]:]
	}
	if _, err := stpl.New(name).
		Parse(string(subjectContent)); err != nil {
		log.Warn("Failed to parse template [%s/subject]: %v", name, err)
	}
	if _, err := btpl.New(name).
		Parse(string(bodyContent)); err != nil {
		log.Warn("Failed to parse template [%s/body]: %v", name, err)
	}
}

type remoteAddress struct {
	Address  string
	Username string
	Password string
}

func mirrorRemoteAddress(m models.RemoteMirrorer) remoteAddress {
	a := remoteAddress{}

	u, err := git.GetRemoteAddress(m.GetRepository().RepoPath(), m.GetRemoteName())
	if err != nil {
		log.Error("GetRemoteAddress %v", err)
		return a
	}

	if u.User != nil {
		a.Username = u.User.Username()
		a.Password, _ = u.User.Password()
	}
	u.User = nil
	a.Address = u.String()

	return a
}
