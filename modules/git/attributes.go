// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bufio"
	"io"
	"path"
	"regexp"
	"strings"

	"github.com/gobwas/glob"
)

type attributePattern struct {
	pattern    glob.Glob
	attributes map[string]interface{}
}

// Attributes represents all attributes from a .gitattribute file
type Attributes []attributePattern

// ForFile returns the git attributes for the given path.
func (a Attributes) ForFile(filepath string) map[string]interface{} {
	filepath = path.Join("/", filepath)

	for _, pattern := range a {
		if pattern.pattern.Match(filepath) {
			return pattern.attributes
		}
	}

	return map[string]interface{}{}
}

var whitespaceSplit = regexp.MustCompile(`\s+`)

// ParseAttributes parses git attributes from the provided reader.
func ParseAttributes(reader io.Reader) (Attributes, error) {
	patterns := []attributePattern{}

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and comments
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		splitted := whitespaceSplit.Split(line, 2)

		pattern := path.Join("/", splitted[0])

		attributes := map[string]interface{}{}
		if len(splitted) == 2 {
			attributes = parseAttributes(splitted[1])
		}

		if g, err := glob.Compile(pattern, '/'); err == nil {
			patterns = append(patterns, attributePattern{
				g,
				attributes,
			})
		}
	}

	for i, j := 0, len(patterns)-1; i < j; i, j = i+1, j-1 {
		patterns[i], patterns[j] = patterns[j], patterns[i]
	}

	return Attributes(patterns), scanner.Err()
}

// parseAttributes parses an attribute string. Attributes can have the following formats:
// foo     => foo = true
// -foo    => foo = false
// foo=bar => foo = bar
func parseAttributes(attributes string) map[string]interface{} {
	values := make(map[string]interface{})

	for _, chunk := range whitespaceSplit.Split(attributes, -1) {
		if chunk == "=" { // "foo = bar" is treated as "foo" and "bar"
			continue
		}

		if strings.HasPrefix(chunk, "-") { // "-foo"
			values[chunk[1:]] = false
		} else if strings.Contains(chunk, "=") { // "foo=bar"
			splitted := strings.SplitN(chunk, "=", 2)
			values[splitted[0]] = splitted[1]
		} else { // "foo"
			values[chunk] = true
		}
	}

	return values
}
