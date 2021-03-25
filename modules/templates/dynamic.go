// +build !bindata

// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package templates

import (
	"html/template"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	texttmpl "text/template"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

var (
	subjectTemplates = texttmpl.New("")
	bodyTemplates    = template.New("")
)

// GetAsset returns asset content via name
func GetAsset(name string) ([]byte, error) {
	bs, err := ioutil.ReadFile(filepath.Join(setting.CustomPath, name))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	} else if err == nil {
		return bs, nil
	}

	return ioutil.ReadFile(filepath.Join(setting.StaticRootPath, name))
}

// GetAssetNames returns assets list
func GetAssetNames() []string {
	tmpls := getDirAssetNames(filepath.Join(setting.CustomPath, "templates"))
	tmpls2 := getDirAssetNames(filepath.Join(setting.StaticRootPath, "templates"))
	return append(tmpls, tmpls2...)
}

// Mailer provides the templates required for sending notification mails.
func Mailer() (*texttmpl.Template, *template.Template) {
	for _, funcs := range NewTextFuncMap() {
		subjectTemplates.Funcs(funcs)
	}
	for _, funcs := range NewFuncMap() {
		bodyTemplates.Funcs(funcs)
	}

	staticDir := path.Join(setting.StaticRootPath, "templates", "mail")

	isDir, err := util.IsDir(staticDir)
	if err != nil {
		log.Warn("Unable to check if templates dir %s is a directory. Error: %v", staticDir, err)
	}
	if isDir {
		files, err := util.StatDir(staticDir)

		if err != nil {
			log.Warn("Failed to read %s templates dir. %v", staticDir, err)
		} else {
			for _, filePath := range files {
				if !strings.HasSuffix(filePath, ".tmpl") {
					continue
				}

				content, err := ioutil.ReadFile(path.Join(staticDir, filePath))
				if err != nil {
					log.Warn("Failed to read static %s template. %v", filePath, err)
					continue
				}

				buildSubjectBodyTemplate(subjectTemplates, bodyTemplates, strings.TrimSuffix(filePath, ".tmpl"), content)
			}
		}
	}

	customDir := path.Join(setting.CustomPath, "templates", "mail")

	isDir, err = util.IsDir(customDir)
	if err != nil {
		log.Warn("Unable to check if templates dir %s is a directory. Error: %v", customDir, err)
	}
	if isDir {
		files, err := util.StatDir(customDir)

		if err != nil {
			log.Warn("Failed to read %s templates dir. %v", customDir, err)
		} else {
			for _, filePath := range files {
				if !strings.HasSuffix(filePath, ".tmpl") {
					continue
				}

				content, err := ioutil.ReadFile(path.Join(customDir, filePath))
				if err != nil {
					log.Warn("Failed to read custom %s template. %v", filePath, err)
					continue
				}

				buildSubjectBodyTemplate(subjectTemplates, bodyTemplates, strings.TrimSuffix(filePath, ".tmpl"), content)
			}
		}
	}

	return subjectTemplates, bodyTemplates
}
