// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package label

import (
	"fmt"
	"io/ioutil"
	"path"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/setting"

	"github.com/unknwon/com"
	"gopkg.in/yaml.v2"
)

type labelFile struct {
	Labels []*Label `yaml:"labels"`
}

// ColorPattern is a regexp witch can validate LabelColor
var ColorPattern = regexp.MustCompile("^#[0-9a-fA-F]{6}$")

// GetTemplateFile loads the label template file by given name,
// then parses and returns a list of labels.
func GetTemplateFile(name string) ([]*Label, error) {
	// Get yaml file
	data, isYaml, err := getRepoLabelFile(name)
	if err != nil {
		return nil, fmt.Errorf("getRepoInitFile: %v", err)
	}
	if isYaml {
		return parseYamlFormat(data)
	}

	return parseDefaultFormat(data)
}

func getRepoLabelFile(name string) ([]byte, bool, error) {
	cleanedName := strings.TrimLeft(path.Clean("/"+name), "/")
	relPath := path.Join("options", "label", cleanedName)

	// Check if custom yaml file exists
	customPath := path.Join(setting.CustomPath, relPath)
	var isYaml bool
	if com.IsFile(customPath + ".yml") {
		customPath += ".yml"
		isYaml = true
	}
	if com.IsFile(customPath + ".yaml") {
		customPath += ".yaml"
		isYaml = true
	}
	// Use custom file when available.
	if isYaml || com.IsFile(customPath) {
		data, err := ioutil.ReadFile(customPath)
		return data, isYaml, err
	}

	// Try built-in yaml file first
	if data, err := options.Labels(cleanedName + ".yaml"); err == nil {
		return data, true, nil
	}

	// Fallback to built-in default format file
	data, err := options.Labels(cleanedName)
	return data, false, err
}

func parseYamlFormat(data []byte) ([]*Label, error) {
	lf := &labelFile{}

	if err := yaml.Unmarshal(data, lf); err != nil {
		return nil, err
	}

	// Validate label data and fix colors
	for _, l := range lf.Labels {
		if len(l.Name) == 0 || len(l.Color) == 0 {
			return nil, fmt.Errorf("label name and color are required fields")
		}
		if !l.Priority.IsValid() {
			return nil, fmt.Errorf("invalid priority: %s", l.Priority)
		}
		l.Color = "#" + l.Color
		if !ColorPattern.MatchString(l.Color) {
			return nil, fmt.Errorf("bad HTML color code in label: %s", l.Name)
		}
	}

	return lf.Labels, nil
}

func parseDefaultFormat(data []byte) ([]*Label, error) {
	lines := strings.Split(string(data), "\n")
	list := make([]*Label, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if len(line) == 0 {
			continue
		}

		parts := strings.SplitN(line, ";", 2)

		fields := strings.SplitN(parts[0], " ", 2)
		if len(fields) != 2 {
			return nil, fmt.Errorf("line is malformed: %s", line)
		}

		if !ColorPattern.MatchString(fields[0]) {
			return nil, fmt.Errorf("bad HTML color code in line: %s", line)
		}

		var description string

		if len(parts) > 1 {
			description = strings.TrimSpace(parts[1])
		}

		fields[1] = strings.TrimSpace(fields[1])
		list = append(list, &Label{
			Name:        fields[1],
			Color:       fields[0],
			Description: description,
		})
	}
	return list, nil
}
