// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package validation

import (
	"fmt"
	"regexp"
	"strings"

	"gitea.com/go-chi/binding"
	"github.com/gobwas/glob"
)

const (
	// ErrGitRefName is git reference name error
	ErrGitRefName = "GitRefNameError"

	// ErrGlobPattern is returned when glob pattern is invalid
	ErrGlobPattern = "GlobPattern"
)

// GitRefNamePatternInvalid is regular expression with unallowed characters in git reference name
// They cannot have ASCII control characters (i.e. bytes whose values are lower than \040, or \177 DEL), space, tilde ~, caret ^, or colon : anywhere.
// They cannot have question-mark ?, asterisk *, or open bracket [ anywhere
var GitRefNamePatternInvalid = regexp.MustCompile(`[\000-\037\177 \\~^:?*[]+`)

// CheckGitRefAdditionalRulesValid check name is valid on additional rules
func CheckGitRefAdditionalRulesValid(name string) bool {
	// Additional rules as described at https://www.kernel.org/pub/software/scm/git/docs/git-check-ref-format.html
	if strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") ||
		strings.HasSuffix(name, ".") || strings.Contains(name, "..") ||
		strings.Contains(name, "//") || strings.Contains(name, "@{") ||
		name == "@" {
		return false
	}
	parts := strings.Split(name, "/")
	for _, part := range parts {
		if strings.HasSuffix(part, ".lock") || strings.HasPrefix(part, ".") {
			return false
		}
	}

	return true
}

// AddBindingRules adds additional binding rules
func AddBindingRules() {
	addGitRefNameBindingRule()
	addValidURLBindingRule()
	addGlobPatternRule()
}

func addGitRefNameBindingRule() {
	// Git refname validation rule
	binding.AddRule(&binding.Rule{
		IsMatch: func(rule string) bool {
			return strings.HasPrefix(rule, "GitRefName")
		},
		IsValid: func(errs binding.Errors, name string, val interface{}) (bool, binding.Errors) {
			str := fmt.Sprintf("%v", val)

			if GitRefNamePatternInvalid.MatchString(str) {
				errs.Add([]string{name}, ErrGitRefName, "GitRefName")
				return false, errs
			}

			if !CheckGitRefAdditionalRulesValid(str) {
				errs.Add([]string{name}, ErrGitRefName, "GitRefName")
				return false, errs
			}

			return true, errs
		},
	})
}

func addValidURLBindingRule() {
	// URL validation rule
	binding.AddRule(&binding.Rule{
		IsMatch: func(rule string) bool {
			return strings.HasPrefix(rule, "ValidUrl")
		},
		IsValid: func(errs binding.Errors, name string, val interface{}) (bool, binding.Errors) {
			str := fmt.Sprintf("%v", val)
			if len(str) != 0 && !IsValidURL(str) {
				errs.Add([]string{name}, binding.ERR_URL, "Url")
				return false, errs
			}

			return true, errs
		},
	})
}

func addGlobPatternRule() {
	binding.AddRule(&binding.Rule{
		IsMatch: func(rule string) bool {
			return rule == "GlobPattern"
		},
		IsValid: func(errs binding.Errors, name string, val interface{}) (bool, binding.Errors) {
			str := fmt.Sprintf("%v", val)

			if len(str) != 0 {
				if _, err := glob.Compile(str); err != nil {
					errs.Add([]string{name}, ErrGlobPattern, err.Error())
					return false, errs
				}
			}

			return true, errs
		},
	})
}

func portOnly(hostport string) string {
	colon := strings.IndexByte(hostport, ':')
	if colon == -1 {
		return ""
	}
	if i := strings.Index(hostport, "]:"); i != -1 {
		return hostport[i+len("]:"):]
	}
	if strings.Contains(hostport, "]") {
		return ""
	}
	return hostport[colon+len(":"):]
}

func validPort(p string) bool {
	for _, r := range []byte(p) {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
