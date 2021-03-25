// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
package markup

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

const (
	AppURL    = "http://localhost:3000/"
	Repo      = "gogits/gogs"
	AppSubURL = AppURL + Repo + "/"
)

// alphanumLink an HTML link to an alphanumeric-style issue
func alphanumIssueLink(baseURL, class, name string) string {
	return link(util.URLJoin(baseURL, name), class, name)
}

// numericLink an HTML to a numeric-style issue
func numericIssueLink(baseURL, class string, index int, marker string) string {
	return link(util.URLJoin(baseURL, strconv.Itoa(index)), class, fmt.Sprintf("%s%d", marker, index))
}

// link an HTML link
func link(href, class, contents string) string {
	if class != "" {
		class = " class=\"" + class + "\""
	}

	return fmt.Sprintf("<a href=\"%s\"%s>%s</a>", href, class, contents)
}

var numericMetas = map[string]string{
	"format": "https://someurl.com/{user}/{repo}/{index}",
	"user":   "someUser",
	"repo":   "someRepo",
	"style":  IssueNameStyleNumeric,
}

var alphanumericMetas = map[string]string{
	"format": "https://someurl.com/{user}/{repo}/{index}",
	"user":   "someUser",
	"repo":   "someRepo",
	"style":  IssueNameStyleAlphanumeric,
}

// these values should match the Repo const above
var localMetas = map[string]string{
	"user": "gogits",
	"repo": "gogs",
}

func TestRender_IssueIndexPattern(t *testing.T) {
	// numeric: render inputs without valid mentions
	test := func(s string) {
		testRenderIssueIndexPattern(t, s, s, nil)
		testRenderIssueIndexPattern(t, s, s, &postProcessCtx{metas: numericMetas})
	}

	// should not render anything when there are no mentions
	test("")
	test("this is a test")
	test("test 123 123 1234")
	test("#")
	test("# # #")
	test("# 123")
	test("#abcd")
	test("test#1234")
	test("#1234test")
	test("#abcd")
	test("test!1234")
	test("!1234test")
	test(" test !1234test")
	test("/home/gitea/#1234")
	test("/home/gitea/!1234")

	// should not render issue mention without leading space
	test("test#54321 issue")

	// should not render issue mention without trailing space
	test("test #54321issue")
}

func TestRender_IssueIndexPattern2(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL

	// numeric: render inputs with valid mentions
	test := func(s, expectedFmt, marker string, indices ...int) {
		var path, prefix string
		if marker == "!" {
			path = "pulls"
			prefix = "http://localhost:3000/someUser/someRepo/pulls/"
		} else {
			path = "issues"
			prefix = "https://someurl.com/someUser/someRepo/"
		}

		links := make([]interface{}, len(indices))
		for i, index := range indices {
			links[i] = numericIssueLink(util.URLJoin(setting.AppSubURL, path), "ref-issue", index, marker)
		}
		expectedNil := fmt.Sprintf(expectedFmt, links...)
		testRenderIssueIndexPattern(t, s, expectedNil, &postProcessCtx{metas: localMetas})

		for i, index := range indices {
			links[i] = numericIssueLink(prefix, "ref-issue", index, marker)
		}
		expectedNum := fmt.Sprintf(expectedFmt, links...)
		testRenderIssueIndexPattern(t, s, expectedNum, &postProcessCtx{metas: numericMetas})
	}

	// should render freestanding mentions
	test("#1234 test", "%s test", "#", 1234)
	test("test #8 issue", "test %s issue", "#", 8)
	test("!1234 test", "%s test", "!", 1234)
	test("test !8 issue", "test %s issue", "!", 8)
	test("test issue #1234", "test issue %s", "#", 1234)
	test("fixes issue #1234.", "fixes issue %s.", "#", 1234)

	// should render mentions in parentheses / brackets
	test("(#54321 issue)", "(%s issue)", "#", 54321)
	test("[#54321 issue]", "[%s issue]", "#", 54321)
	test("test (#9801 extra) issue", "test (%s extra) issue", "#", 9801)
	test("test (!9801 extra) issue", "test (%s extra) issue", "!", 9801)
	test("test (#1)", "test (%s)", "#", 1)

	// should render multiple issue mentions in the same line
	test("#54321 #1243", "%s %s", "#", 54321, 1243)
	test("wow (#54321 #1243)", "wow (%s %s)", "#", 54321, 1243)
	test("(#4)(#5)", "(%s)(%s)", "#", 4, 5)
	test("#1 (#4321) test", "%s (%s) test", "#", 1, 4321)

	// should render with :
	test("#1234: test", "%s: test", "#", 1234)
	test("wow (#54321: test)", "wow (%s: test)", "#", 54321)
}

func TestRender_IssueIndexPattern3(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL

	// alphanumeric: render inputs without valid mentions
	test := func(s string) {
		testRenderIssueIndexPattern(t, s, s, &postProcessCtx{metas: alphanumericMetas})
	}
	test("")
	test("this is a test")
	test("test 123 123 1234")
	test("#")
	test("# 123")
	test("#abcd")
	test("test #123")
	test("abc-1234")         // issue prefix must be capital
	test("ABc-1234")         // issue prefix must be _all_ capital
	test("ABCDEFGHIJK-1234") // the limit is 10 characters in the prefix
	test("ABC1234")          // dash is required
	test("test ABC- test")   // number is required
	test("test -1234 test")  // prefix is required
	test("testABC-123 test") // leading space is required
	test("test ABC-123test") // trailing space is required
	test("ABC-0123")         // no leading zero
}

func TestRender_IssueIndexPattern4(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL

	// alphanumeric: render inputs with valid mentions
	test := func(s, expectedFmt string, names ...string) {
		links := make([]interface{}, len(names))
		for i, name := range names {
			links[i] = alphanumIssueLink("https://someurl.com/someUser/someRepo/", "ref-issue", name)
		}
		expected := fmt.Sprintf(expectedFmt, links...)
		testRenderIssueIndexPattern(t, s, expected, &postProcessCtx{metas: alphanumericMetas})
	}
	test("OTT-1234 test", "%s test", "OTT-1234")
	test("test T-12 issue", "test %s issue", "T-12")
	test("test issue ABCDEFGHIJ-1234567890", "test issue %s", "ABCDEFGHIJ-1234567890")
}

func testRenderIssueIndexPattern(t *testing.T, input, expected string, ctx *postProcessCtx) {
	if ctx == nil {
		ctx = new(postProcessCtx)
	}
	ctx.procs = []processor{issueIndexPatternProcessor}
	if ctx.urlPrefix == "" {
		ctx.urlPrefix = AppSubURL
	}

	res, err := ctx.postProcess([]byte(input))
	assert.NoError(t, err)
	assert.Equal(t, expected, string(res))
}

func TestRender_AutoLink(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL

	test := func(input, expected string) {
		buffer, err := PostProcess([]byte(input), setting.AppSubURL, localMetas, false)
		assert.Equal(t, err, nil)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(buffer)))
		buffer, err = PostProcess([]byte(input), setting.AppSubURL, localMetas, true)
		assert.Equal(t, err, nil)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(buffer)))
	}

	// render valid issue URLs
	test(util.URLJoin(setting.AppSubURL, "issues", "3333"),
		numericIssueLink(util.URLJoin(setting.AppSubURL, "issues"), "ref-issue", 3333, "#"))

	// render valid commit URLs
	tmp := util.URLJoin(AppSubURL, "commit", "d8a994ef243349f321568f9e36d5c3f444b99cae")
	test(tmp, "<a href=\""+tmp+"\" class=\"commit\"><code class=\"nohighlight\">d8a994ef24</code></a>")
	tmp += "#diff-2"
	test(tmp, "<a href=\""+tmp+"\" class=\"commit\"><code class=\"nohighlight\">d8a994ef24 (diff-2)</code></a>")

	// render other commit URLs
	tmp = "https://external-link.gitea.io/go-gitea/gitea/commit/d8a994ef243349f321568f9e36d5c3f444b99cae#diff-2"
	test(tmp, "<a href=\""+tmp+"\" class=\"commit\"><code class=\"nohighlight\">d8a994ef24 (diff-2)</code></a>")
}

func TestRender_FullIssueURLs(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL

	test := func(input, expected string) {
		ctx := new(postProcessCtx)
		ctx.procs = []processor{fullIssuePatternProcessor}
		if ctx.urlPrefix == "" {
			ctx.urlPrefix = AppSubURL
		}
		ctx.metas = localMetas
		result, err := ctx.postProcess([]byte(input))
		assert.NoError(t, err)
		assert.Equal(t, expected, string(result))
	}
	test("Here is a link https://git.osgeo.org/gogs/postgis/postgis/pulls/6",
		"Here is a link https://git.osgeo.org/gogs/postgis/postgis/pulls/6")
	test("Look here http://localhost:3000/person/repo/issues/4",
		`Look here <a href="http://localhost:3000/person/repo/issues/4" class="ref-issue">person/repo#4</a>`)
	test("http://localhost:3000/person/repo/issues/4#issuecomment-1234",
		`<a href="http://localhost:3000/person/repo/issues/4#issuecomment-1234" class="ref-issue">person/repo#4</a>`)
	test("http://localhost:3000/gogits/gogs/issues/4",
		`<a href="http://localhost:3000/gogits/gogs/issues/4" class="ref-issue">#4</a>`)
}

func TestRegExp_sha1CurrentPattern(t *testing.T) {
	trueTestCases := []string{
		"d8a994ef243349f321568f9e36d5c3f444b99cae",
		"abcdefabcdefabcdefabcdefabcdefabcdefabcd",
		"(abcdefabcdefabcdefabcdefabcdefabcdefabcd)",
		"[abcdefabcdefabcdefabcdefabcdefabcdefabcd]",
		"abcdefabcdefabcdefabcdefabcdefabcdefabcd.",
	}
	falseTestCases := []string{
		"test",
		"abcdefg",
		"e59ff077-2d03-4e6b-964d-63fbaea81f",
		"abcdefghijklmnopqrstuvwxyzabcdefghijklmn",
		"abcdefghijklmnopqrstuvwxyzabcdefghijklmO",
	}

	for _, testCase := range trueTestCases {
		assert.True(t, sha1CurrentPattern.MatchString(testCase))
	}
	for _, testCase := range falseTestCases {
		assert.False(t, sha1CurrentPattern.MatchString(testCase))
	}
}

func TestRegExp_anySHA1Pattern(t *testing.T) {
	testCases := map[string][]string{
		"https://github.com/jquery/jquery/blob/a644101ed04d0beacea864ce805e0c4f86ba1cd1/test/unit/event.js#L2703": {
			"a644101ed04d0beacea864ce805e0c4f86ba1cd1",
			"/test/unit/event.js",
			"#L2703",
		},
		"https://github.com/jquery/jquery/blob/a644101ed04d0beacea864ce805e0c4f86ba1cd1/test/unit/event.js": {
			"a644101ed04d0beacea864ce805e0c4f86ba1cd1",
			"/test/unit/event.js",
			"",
		},
		"https://github.com/jquery/jquery/commit/0705be475092aede1eddae01319ec931fb9c65fc": {
			"0705be475092aede1eddae01319ec931fb9c65fc",
			"",
			"",
		},
		"https://github.com/jquery/jquery/tree/0705be475092aede1eddae01319ec931fb9c65fc/src": {
			"0705be475092aede1eddae01319ec931fb9c65fc",
			"/src",
			"",
		},
		"https://try.gogs.io/gogs/gogs/commit/d8a994ef243349f321568f9e36d5c3f444b99cae#diff-2": {
			"d8a994ef243349f321568f9e36d5c3f444b99cae",
			"",
			"#diff-2",
		},
	}

	for k, v := range testCases {
		assert.Equal(t, anySHA1Pattern.FindStringSubmatch(k)[1:], v)
	}
}

func TestRegExp_shortLinkPattern(t *testing.T) {
	trueTestCases := []string{
		"[[stuff]]",
		"[[]]",
		"[[stuff|title=Difficult name with spaces*!]]",
	}
	falseTestCases := []string{
		"test",
		"abcdefg",
		"[[]",
		"[[",
		"[]",
		"]]",
		"abcdefghijklmnopqrstuvwxyz",
	}

	for _, testCase := range trueTestCases {
		assert.True(t, shortLinkPattern.MatchString(testCase))
	}
	for _, testCase := range falseTestCases {
		assert.False(t, shortLinkPattern.MatchString(testCase))
	}
}
