// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/log"

	"gopkg.in/ini.v1"
)

// ExternalMarkupParsers represents the external markup parsers
var (
	ExternalMarkupParsers  []MarkupParser
	ExternalSanitizerRules []MarkupSanitizerRule
)

// MarkupParser defines the external parser configured in ini
type MarkupParser struct {
	Enabled        bool
	MarkupName     string
	Command        string
	FileExtensions []string
	IsInputFile    bool
}

// MarkupSanitizerRule defines the policy for whitelisting attributes on
// certain elements.
type MarkupSanitizerRule struct {
	Element   string
	AllowAttr string
	Regexp    *regexp.Regexp
}

func newMarkup() {
	for _, sec := range Cfg.Section("markup").ChildSections() {
		name := strings.TrimPrefix(sec.Name(), "markup.")
		if name == "" {
			log.Warn("name is empty, markup " + sec.Name() + "ignored")
			continue
		}

		if name == "sanitizer" || strings.HasPrefix(name, "sanitizer.") {
			newMarkupSanitizer(name, sec)
		} else {
			newMarkupRenderer(name, sec)
		}
	}
}

func newMarkupSanitizer(name string, sec *ini.Section) {
	haveElement := sec.HasKey("ELEMENT")
	haveAttr := sec.HasKey("ALLOW_ATTR")
	haveRegexp := sec.HasKey("REGEXP")

	if !haveElement && !haveAttr && !haveRegexp {
		log.Warn("Skipping empty section: markup.%s.", name)
		return
	}

	if !haveElement || !haveAttr || !haveRegexp {
		log.Error("Missing required keys from markup.%s. Must have all three of ELEMENT, ALLOW_ATTR, and REGEXP defined!", name)
		return
	}

	elements := sec.Key("ELEMENT").Value()
	allowAttrs := sec.Key("ALLOW_ATTR").Value()
	regexpStr := sec.Key("REGEXP").Value()

	if regexpStr == "" {
		rule := MarkupSanitizerRule{
			Element:   elements,
			AllowAttr: allowAttrs,
			Regexp:    nil,
		}

		ExternalSanitizerRules = append(ExternalSanitizerRules, rule)
		return
	}

	// Validate when parsing the config that this is a valid regular
	// expression. Then we can use regexp.MustCompile(...) later.
	compiled, err := regexp.Compile(regexpStr)
	if err != nil {
		log.Error("In module.%s: REGEXP (%s) at definition %d failed to compile: %v", regexpStr, name, err)
		return
	}

	rule := MarkupSanitizerRule{
		Element:   elements,
		AllowAttr: allowAttrs,
		Regexp:    compiled,
	}

	ExternalSanitizerRules = append(ExternalSanitizerRules, rule)
}

func newMarkupRenderer(name string, sec *ini.Section) {
	extensionReg := regexp.MustCompile(`\.\w`)

	extensions := sec.Key("FILE_EXTENSIONS").Strings(",")
	exts := make([]string, 0, len(extensions))
	for _, extension := range extensions {
		if !extensionReg.MatchString(extension) {
			log.Warn(sec.Name() + " file extension " + extension + " is invalid. Extension ignored")
		} else {
			exts = append(exts, extension)
		}
	}

	if len(exts) == 0 {
		log.Warn(sec.Name() + " file extension is empty, markup " + name + " ignored")
		return
	}

	command := sec.Key("RENDER_COMMAND").MustString("")
	if command == "" {
		log.Warn(" RENDER_COMMAND is empty, markup " + name + " ignored")
		return
	}

	ExternalMarkupParsers = append(ExternalMarkupParsers, MarkupParser{
		Enabled:        sec.Key("ENABLED").MustBool(false),
		MarkupName:     name,
		FileExtensions: exts,
		Command:        command,
		IsInputFile:    sec.Key("IS_INPUT_FILE").MustBool(false),
	})
}
