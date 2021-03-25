// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"net/url"
	"strconv"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestGenAPILinks(t *testing.T) {
	setting.AppURL = "http://localhost:3000/"
	kases := map[string][]string{
		"api/v1/repos/jerrykan/example-repo/issues?state=all": {
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=2&state=all>; rel="next"`,
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=5&state=all>; rel="last"`,
		},
		"api/v1/repos/jerrykan/example-repo/issues?state=all&page=1": {
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=2&state=all>; rel="next"`,
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=5&state=all>; rel="last"`,
		},
		"api/v1/repos/jerrykan/example-repo/issues?state=all&page=2": {
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=3&state=all>; rel="next"`,
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=5&state=all>; rel="last"`,
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=1&state=all>; rel="first"`,
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=1&state=all>; rel="prev"`,
		},
		"api/v1/repos/jerrykan/example-repo/issues?state=all&page=5": {
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=1&state=all>; rel="first"`,
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=4&state=all>; rel="prev"`,
		},
	}

	for req, response := range kases {
		u, err := url.Parse(setting.AppURL + req)
		assert.NoError(t, err)

		p := u.Query().Get("page")
		curPage, _ := strconv.Atoi(p)

		links := genAPILinks(u, 100, 20, curPage)

		assert.EqualValues(t, links, response)
	}
}
