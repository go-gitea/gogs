// +build bindata

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

// GetAsset get a special asset, only for chi
func GetAsset(name string) ([]byte, error) {
	bs, err := ioutil.ReadFile(filepath.Join(setting.CustomPath, name))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	} else if err == nil {
		return bs, nil
	}
	return Asset(strings.TrimPrefix(name, "templates/"))
}

// GetAssetNames only for chi
func GetAssetNames() []string {
	realFS := Assets.(vfsgen۰FS)
	tmpls := make([]string, 0, len(realFS))
	for k := range realFS {
		tmpls = append(tmpls, "templates/"+k[1:])
	}

	customDir := path.Join(setting.CustomPath, "templates")
	customTmpls := getDirAssetNames(customDir)
	return append(tmpls, customTmpls...)
}

// Mailer provides the templates required for sending notification mails.
func Mailer() (*texttmpl.Template, *template.Template) {
	for _, funcs := range NewTextFuncMap() {
		subjectTemplates.Funcs(funcs)
	}
	for _, funcs := range NewFuncMap() {
		bodyTemplates.Funcs(funcs)
	}

	for _, assetPath := range AssetNames() {
		if !strings.HasPrefix(assetPath, "mail/") {
			continue
		}

		if !strings.HasSuffix(assetPath, ".tmpl") {
			continue
		}

		content, err := Asset(assetPath)
		if err != nil {
			log.Warn("Failed to read embedded %s template. %v", assetPath, err)
			continue
		}

		buildSubjectBodyTemplate(subjectTemplates,
			bodyTemplates,
			strings.TrimPrefix(
				strings.TrimSuffix(
					assetPath,
					".tmpl",
				),
				"mail/",
			),
			content)
	}

	customDir := path.Join(setting.CustomPath, "templates", "mail")
	isDir, err := util.IsDir(customDir)
	if err != nil {
		log.Warn("Failed to check if custom directory %s is a directory. %v", err)
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

				buildSubjectBodyTemplate(subjectTemplates,
					bodyTemplates,
					strings.TrimSuffix(
						filePath,
						".tmpl",
					),
					content)
			}
		}
	}

	return subjectTemplates, bodyTemplates
}

func Asset(name string) ([]byte, error) {
	f, err := Assets.Open("/" + name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ioutil.ReadAll(f)
}

func AssetNames() []string {
	realFS := Assets.(vfsgen۰FS)
	results := make([]string, 0, len(realFS))
	for k := range realFS {
		results = append(results, k[1:])
	}
	return results
}

func AssetIsDir(name string) (bool, error) {
	if f, err := Assets.Open("/" + name); err != nil {
		return false, err
	} else {
		defer f.Close()
		if fi, err := f.Stat(); err != nil {
			return false, err
		} else {
			return fi.IsDir(), nil
		}
	}
}
