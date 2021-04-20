// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/models"
)

// To generate derivative fixtures, execute the following from Gitea's repository base dir:
// go run contrib/fixtures/fixture_generation.go [fixture...]

var (
	generators = []struct {
		gen  func() (string, error)
		name string
	}{
		{
			models.GetYamlFixturesAccess, "access",
		},
	}
	fixturesDir string
)

func main() {
	pathToGiteaRoot := "."
	fixturesDir = filepath.Join(pathToGiteaRoot, "models", "fixtures")
	if err := models.CreateTestEngine(fixturesDir); err != nil {
		fmt.Printf("CreateTestEngine: %+v", err)
		os.Exit(1)
	}
	if err := models.PrepareTestDatabase(); err != nil {
		fmt.Printf("PrepareTestDatabase: %+v\n", err)
		os.Exit(1)
	}
	if len(os.Args) == 0 {
		for _, r := range os.Args {
			if err := generate(r); err != nil {
				fmt.Printf("generate '%s': %+v\n", r, err)
				os.Exit(1)
			}
		}
	} else {
		for _, g := range generators {
			if err := generate(g.name); err != nil {
				fmt.Printf("generate '%s': %+v\n", g.name, err)
				os.Exit(1)
			}
		}
	}
}

func generate(name string) error {
	for _, g := range generators {
		if g.name == name {
			data, err := g.gen()
			if err != nil {
				return err
			}
			path := filepath.Join(fixturesDir, name+".yml")
			if err := ioutil.WriteFile(path, []byte(data), 0644); err != nil {
				return fmt.Errorf("%s: %+v", path, err)
			}
			fmt.Printf("%s created.\n", path)
			return nil
		}
	}

	return fmt.Errorf("generator not found")
}
