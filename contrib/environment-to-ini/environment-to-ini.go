// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"regexp"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/urfave/cli/v2"
	ini "gopkg.in/ini.v1"
)

// EnvironmentPrefix environment variables prefixed with this represent ini values to write
const EnvironmentPrefix = "GITEA"

func main() {
	app := cli.NewApp()
	app.Name = "environment-to-ini"
	app.Usage = "Use provided environment to update configuration ini"
	app.Description = `As a helper to allow docker users to update the gitea configuration
	through the environment, this command allows environment variables to
	be mapped to values in the ini.

	Environment variables of the form "GITEA__SECTION_NAME__KEY_NAME"
	will be mapped to the ini section "[section_name]" and the key
	"KEY_NAME" with the value as provided.

	Environment variables are usually restricted to a reduced character
	set "0-9A-Z_" - in order to allow the setting of sections with
	characters outside of that set, they should be escaped as following:
	"_0X2E_" for ".". The entire section and key names can be escaped as
	a UTF8 byte string if necessary. E.g. to configure:

		"""
		...
		[log.console]
		COLORIZE=false
		STDERR=true
		...
		"""

	You would set the environment variables: "GITEA__LOG_0x2E_CONSOLE__COLORIZE=false"
	and "GITEA__LOG_0x2E_CONSOLE__STDERR=false". Other examples can be found
	on the configuration cheat sheet.`
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "custom-path",
			Aliases: []string{"C"},
			Value:   setting.CustomPath,
			Usage:   "Custom path file path",
		},
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   setting.CustomConf,
			Usage:   "Custom configuration file path",
		},
		&cli.StringFlag{
			Name:    "work-path",
			Aliases: []string{"w"},
			Value:   setting.AppWorkPath,
			Usage:   "Set the gitea working path",
		},
		&cli.StringFlag{
			Name:    "out",
			Aliases: []string{"o"},
			Value:   "",
			Usage:   "Destination file to write to",
		},
		&cli.BoolFlag{
			Name:  "clear",
			Usage: "Clears the matched variables from the environment",
		},
		&cli.StringFlag{
			Name:    "prefix",
			Aliases: []string{"p"},
			Value:   EnvironmentPrefix,
			Usage:   "Environment prefix to look for - will be suffixed by __ (2 underscores)",
		},
	}
	app.Action = runEnvironmentToIni
	setting.SetCustomPathAndConf("", "", "")

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal("Failed to run app with %s: %v", os.Args, err)
	}
}

func runEnvironmentToIni(c *cli.Context) error {
	providedCustom := c.String("custom-path")
	providedConf := c.String("config")
	providedWorkPath := c.String("work-path")
	setting.SetCustomPathAndConf(providedCustom, providedConf, providedWorkPath)

	cfg := ini.Empty()
	isFile, err := util.IsFile(setting.CustomConf)
	if err != nil {
		log.Fatal("Unable to check if %s is a file. Error: %v", setting.CustomConf, err)
	}
	if isFile {
		if err := cfg.Append(setting.CustomConf); err != nil {
			log.Fatal("Failed to load custom conf '%s': %v", setting.CustomConf, err)
		}
	} else {
		log.Warn("Custom config '%s' not found, ignore this if you're running first time", setting.CustomConf)
	}
	cfg.NameMapper = ini.SnackCase

	changed := false

	prefix := c.String("prefix") + "__"

	for _, kv := range os.Environ() {
		idx := strings.IndexByte(kv, '=')
		if idx < 0 {
			continue
		}
		eKey := kv[:idx]
		value := kv[idx+1:]
		if !strings.HasPrefix(eKey, prefix) {
			continue
		}
		eKey = eKey[len(prefix):]
		sectionName, keyName := DecodeSectionKey(eKey)
		if len(keyName) == 0 {
			continue
		}
		section, err := cfg.GetSection(sectionName)
		if err != nil {
			section, err = cfg.NewSection(sectionName)
			if err != nil {
				log.Error("Error creating section: %s : %v", sectionName, err)
				continue
			}
		}
		key := section.Key(keyName)
		if key == nil {
			key, err = section.NewKey(keyName, value)
			if err != nil {
				log.Error("Error creating key: %s in section: %s with value: %s : %v", keyName, sectionName, value, err)
				continue
			}
		}
		oldValue := key.Value()
		if !changed && oldValue != value {
			changed = true
		}
		key.SetValue(value)
	}
	destination := c.String("out")
	if len(destination) == 0 {
		destination = setting.CustomConf
	}
	if destination != setting.CustomConf || changed {
		err = cfg.SaveTo(destination)
		if err != nil {
			return err
		}
	}
	if c.Bool("clear") {
		for _, kv := range os.Environ() {
			idx := strings.IndexByte(kv, '=')
			if idx < 0 {
				continue
			}
			eKey := kv[:idx]
			if strings.HasPrefix(eKey, prefix) {
				_ = os.Unsetenv(eKey)
			}
		}
	}
	return nil
}

const escapeRegexpString = "_0[xX](([0-9a-fA-F][0-9a-fA-F])+)_"

var escapeRegex = regexp.MustCompile(escapeRegexpString)

// DecodeSectionKey will decode a portable string encoded Section__Key pair
// Portable strings are considered to be of the form [A-Z0-9_]*
// We will encode a disallowed value as the UTF8 byte string preceded by _0X and
// followed by _. E.g. _0X2C_ for a '-' and _0X2E_ for '.'
// Section and Key are separated by a plain '__'.
// The entire section can be encoded as a UTF8 byte string
func DecodeSectionKey(encoded string) (string, string) {
	section := ""
	key := ""

	inKey := false
	last := 0
	escapeStringIndices := escapeRegex.FindAllStringIndex(encoded, -1)
	for _, unescapeIdx := range escapeStringIndices {
		preceding := encoded[last:unescapeIdx[0]]
		if !inKey {
			if splitter := strings.Index(preceding, "__"); splitter > -1 {
				section += preceding[:splitter]
				inKey = true
				key += preceding[splitter+2:]
			} else {
				section += preceding
			}
		} else {
			key += preceding
		}
		toDecode := encoded[unescapeIdx[0]+3 : unescapeIdx[1]-1]
		decodedBytes := make([]byte, len(toDecode)/2)
		for i := 0; i < len(toDecode)/2; i++ {
			// Can ignore error here as we know these should be hexadecimal from the regexp
			byteInt, _ := strconv.ParseInt(toDecode[2*i:2*i+2], 16, 0)
			decodedBytes[i] = byte(byteInt)
		}
		if inKey {
			key += string(decodedBytes)
		} else {
			section += string(decodedBytes)
		}
		last = unescapeIdx[1]
	}
	remaining := encoded[last:]
	if !inKey {
		if splitter := strings.Index(remaining, "__"); splitter > -1 {
			section += remaining[:splitter]
			inKey = true
			key += remaining[splitter+2:]
		} else {
			section += remaining
		}
	} else {
		key += remaining
	}
	section = strings.ToLower(section)
	return section, key
}
