// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build bindata

package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/public"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"

	"github.com/gobwas/glob"
	"github.com/urfave/cli"
)

// Cmdembedded represents the available extract sub-command.
var (
	Cmdembedded = cli.Command{
		Name:        "embedded",
		Usage:       "Extract embedded resources",
		Description: "A command for extracting embedded resources, like templates and images",
		Subcommands: []cli.Command{
			subcmdList,
			subcmdView,
			subcmdExtract,
		},
	}

	subcmdList = cli.Command{
		Name:   "list",
		Usage:  "List files matching the given pattern",
		Action: runList,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "include-vendored,vendor",
				Usage: "Include files under public/vendor as well",
			},
		},
	}

	subcmdView = cli.Command{
		Name:   "view",
		Usage:  "View a file matching the given pattern",
		Action: runView,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "include-vendored,vendor",
				Usage: "Include files under public/vendor as well",
			},
		},
	}

	subcmdExtract = cli.Command{
		Name:   "extract",
		Usage:  "Extract resources",
		Action: runExtract,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "include-vendored,vendor",
				Usage: "Include files under public/vendor as well",
			},
			cli.BoolFlag{
				Name:  "overwrite",
				Usage: "Overwrite files if they already exist",
			},
			cli.BoolFlag{
				Name:  "rename",
				Usage: "Rename files as {name}.bak if they already exist (overwrites previous .bak)",
			},
			cli.BoolFlag{
				Name:  "custom",
				Usage: "Extract to the 'custom' directory as per app.ini",
			},
			cli.StringFlag{
				Name:  "destination,dest-dir",
				Usage: "Extract to the specified directory",
			},
		},
	}

	sections map[string]*section
	assets   []asset
)

type section struct {
	Path  string
	Names func() []string
	IsDir func(string) (bool, error)
	Asset func(string) ([]byte, error)
}

type asset struct {
	Section *section
	Name    string
	Path    string
}

func initEmbeddedExtractor(c *cli.Context) error {

	// Silence the console logger
	log.DelNamedLogger("console")
	log.DelNamedLogger(log.DEFAULT)

	// Read configuration file
	setting.NewContext()

	pats, err := getPatterns(c.Args())
	if err != nil {
		return err
	}
	sections := make(map[string]*section, 3)

	sections["public"] = &section{Path: "public", Names: public.AssetNames, IsDir: public.AssetIsDir, Asset: public.Asset}
	sections["options"] = &section{Path: "options", Names: options.AssetNames, IsDir: options.AssetIsDir, Asset: options.Asset}
	sections["templates"] = &section{Path: "templates", Names: templates.AssetNames, IsDir: templates.AssetIsDir, Asset: templates.Asset}

	for _, sec := range sections {
		assets = append(assets, buildAssetList(sec, pats, c)...)
	}

	// Sort assets
	sort.SliceStable(assets, func(i, j int) bool {
		return assets[i].Path < assets[j].Path
	})

	return nil
}

func runList(c *cli.Context) error {
	if err := runListDo(c); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return err
	}
	return nil
}

func runView(c *cli.Context) error {
	if err := runViewDo(c); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return err
	}
	return nil
}

func runExtract(c *cli.Context) error {
	if err := runExtractDo(c); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return err
	}
	return nil
}

func runListDo(c *cli.Context) error {
	if err := initEmbeddedExtractor(c); err != nil {
		return err
	}

	for _, a := range assets {
		fmt.Println(a.Path)
	}

	return nil
}

func runViewDo(c *cli.Context) error {
	if err := initEmbeddedExtractor(c); err != nil {
		return err
	}

	if len(assets) == 0 {
		return fmt.Errorf("No files matched the given pattern")
	} else if len(assets) > 1 {
		return fmt.Errorf("Too many files matched the given pattern; try to be more specific")
	}

	data, err := assets[0].Section.Asset(assets[0].Name)
	if err != nil {
		return fmt.Errorf("%s: %v", assets[0].Path, err)
	}

	if _, err = os.Stdout.Write(data); err != nil {
		return fmt.Errorf("%s: %v", assets[0].Path, err)
	}

	return nil
}

func runExtractDo(c *cli.Context) error {
	if err := initEmbeddedExtractor(c); err != nil {
		return err
	}

	if len(c.Args()) == 0 {
		return fmt.Errorf("A list of pattern of files to extract is mandatory (e.g. '**' for all)")
	}

	destdir := "."

	if c.IsSet("destination") {
		destdir = c.String("destination")
	} else if c.Bool("custom") {
		destdir = setting.CustomPath
		fmt.Println("Using app.ini at", setting.CustomConf)
	}

	fi, err := os.Stat(destdir)
	if errors.Is(err, os.ErrNotExist) {
		// In case Windows users attempt to provide a forward-slash path
		wdestdir := filepath.FromSlash(destdir)
		if wfi, werr := os.Stat(wdestdir); werr == nil {
			destdir = wdestdir
			fi = wfi
			err = nil
		}
	}
	if err != nil {
		return fmt.Errorf("%s: %s", destdir, err)
	} else if !fi.IsDir() {
		return fmt.Errorf("%s is not a directory.", destdir)
	}

	fmt.Printf("Extracting to %s:\n", destdir)

	overwrite := c.Bool("overwrite")
	rename := c.Bool("rename")

	for _, a := range assets {
		if err := extractAsset(destdir, a, overwrite, rename); err != nil {
			// Non-fatal error
			fmt.Fprintf(os.Stderr, "%s: %v", a.Path, err)
		}
	}

	return nil
}

func extractAsset(d string, a asset, overwrite, rename bool) error {
	dest := filepath.Join(d, filepath.FromSlash(a.Path))
	dir := filepath.Dir(dest)

	data, err := a.Section.Asset(a.Name)
	if err != nil {
		return fmt.Errorf("%s: %v", a.Path, err)
	}

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("%s: %v", dir, err)
	}

	perms := os.ModePerm & 0666

	fi, err := os.Lstat(dest)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%s: %v", dest, err)
		}
	} else if !overwrite && !rename {
		fmt.Printf("%s already exists; skipped.\n", dest)
		return nil
	} else if !fi.Mode().IsRegular() {
		return fmt.Errorf("%s already exists, but it's not a regular file", dest)
	} else if rename {
		if err := util.Rename(dest, dest+".bak"); err != nil {
			return fmt.Errorf("Error creating backup for %s: %v", dest, err)
		}
		// Attempt to respect file permissions mask (even if user:group will be set anew)
		perms = fi.Mode()
	}

	file, err := os.OpenFile(dest, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, perms)
	if err != nil {
		return fmt.Errorf("%s: %v", dest, err)
	}
	defer file.Close()

	if _, err = file.Write(data); err != nil {
		return fmt.Errorf("%s: %v", dest, err)
	}

	fmt.Println(dest)

	return nil
}

func buildAssetList(sec *section, globs []glob.Glob, c *cli.Context) []asset {
	var results = make([]asset, 0, 64)
	for _, name := range sec.Names() {
		if isdir, err := sec.IsDir(name); !isdir && err == nil {
			if sec.Path == "public" &&
				strings.HasPrefix(name, "vendor/") &&
				!c.Bool("include-vendored") {
				continue
			}
			matchName := sec.Path + "/" + name
			for _, g := range globs {
				if g.Match(matchName) {
					results = append(results, asset{Section: sec,
						Name: name,
						Path: sec.Path + "/" + name})
					break
				}
			}
		}
	}
	return results
}

func getPatterns(args []string) ([]glob.Glob, error) {
	if len(args) == 0 {
		args = []string{"**"}
	}
	pat := make([]glob.Glob, len(args))
	for i := range args {
		if g, err := glob.Compile(args[i], '/'); err != nil {
			return nil, fmt.Errorf("'%s': Invalid glob pattern: %v", args[i], err)
		} else {
			pat[i] = g
		}
	}
	return pat, nil
}
