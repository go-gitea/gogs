// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package templates

import (
	"os"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/unrolled/render"
)

// Vars represents variables to be render in golang templates
type Vars map[string]interface{}

// Merge merges another vars to the current, another Vars will override the current
func (vars Vars) Merge(another map[string]interface{}) Vars {
	for k, v := range another {
		vars[k] = v
	}
	return vars
}

// BaseVars returns all basic vars
func BaseVars() Vars {
	startTime := time.Now()
	return map[string]interface{}{
		"IsLandingPageHome":          setting.LandingPageURL == setting.LandingPageHome,
		"IsLandingPageExplore":       setting.LandingPageURL == setting.LandingPageExplore,
		"IsLandingPageOrganizations": setting.LandingPageURL == setting.LandingPageOrganizations,

		"ShowRegistrationButton":      setting.Service.ShowRegistrationButton,
		"ShowMilestonesDashboardPage": setting.Service.ShowMilestonesDashboardPage,
		"ShowFooterBranding":          setting.ShowFooterBranding,
		"ShowFooterVersion":           setting.ShowFooterVersion,

		"EnableSwagger":      setting.API.EnableSwagger,
		"EnableOpenIDSignIn": setting.Service.EnableOpenIDSignIn,
		"PageStartTime":      startTime,
		"TmplLoadTimes": func() string {
			return time.Since(startTime).String()
		},
	}
}

func getDirAssetNames(dir string) []string {
	var tmpls []string
	f, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return tmpls
		}
		log.Warn("Unable to check if templates dir %s is a directory. Error: %v", dir, err)
		return tmpls
	}
	if !f.IsDir() {
		log.Warn("Templates dir %s is a not directory.", dir)
		return tmpls
	}

	files, err := util.StatDir(dir)
	if err != nil {
		log.Warn("Failed to read %s templates dir. %v", dir, err)
		return tmpls
	}
	for _, filePath := range files {
		if strings.HasPrefix(filePath, "mail/") {
			continue
		}

		if !strings.HasSuffix(filePath, ".tmpl") {
			continue
		}

		tmpls = append(tmpls, "templates/"+filePath)
	}
	return tmpls
}

// HTMLRenderer returns a render.
func HTMLRenderer() *render.Render {
	return render.New(render.Options{
		Extensions:                []string{".tmpl"},
		Directory:                 "templates",
		Funcs:                     NewFuncMap(),
		Asset:                     GetAsset,
		AssetNames:                GetAssetNames,
		IsDevelopment:             !setting.IsProd(),
		DisableHTTPErrorRendering: true,
	})
}
