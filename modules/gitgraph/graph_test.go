// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitgraph

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/git"
)

func BenchmarkGetCommitGraph(b *testing.B) {
	currentRepo, err := git.OpenRepository(".")
	if err != nil || currentRepo == nil {
		b.Error("Could not open repository")
	}
	defer currentRepo.Close()

	for i := 0; i < b.N; i++ {
		graph, err := GetCommitGraph(currentRepo, 1, 0, false, nil, nil)
		if err != nil {
			b.Error("Could get commit graph")
		}

		if len(graph.Commits) < 100 {
			b.Error("Should get 100 log lines.")
		}
	}
}

func BenchmarkParseCommitString(b *testing.B) {
	testString := "* DATA:|4e61bacab44e9b4730e44a6615d04098dd3a8eaf|2016-12-20 21:10:41 +0100|4e61bac|Add route for graph"

	parser := &Parser{}
	parser.Reset()
	for i := 0; i < b.N; i++ {
		parser.Reset()
		graph := NewGraph()
		if err := parser.AddLineToGraph(graph, 0, []byte(testString)); err != nil {
			b.Error("could not parse teststring")
		}
		if graph.Flows[1].Commits[0].Rev != "4e61bacab44e9b4730e44a6615d04098dd3a8eaf" {
			b.Error("Did not get expected data")
		}
	}
}

func BenchmarkParseGlyphs(b *testing.B) {
	parser := &Parser{}
	parser.Reset()
	tgBytes := []byte(testglyphs)
	tg := tgBytes
	idx := bytes.Index(tg, []byte("\n"))
	for i := 0; i < b.N; i++ {
		parser.Reset()
		tg = tgBytes
		idx = bytes.Index(tg, []byte("\n"))
		for idx > 0 {
			parser.ParseGlyphs(tg[:idx])
			tg = tg[idx+1:]
			idx = bytes.Index(tg, []byte("\n"))
		}
	}
}

func TestReleaseUnusedColors(t *testing.T) {
	testcases := []struct {
		availableColors []int
		oldColors       []int
		firstInUse      int // these values have to be either be correct or suggest less is
		firstAvailable  int // available than possibly is - i.e. you cannot say 10 is available when it
	}{
		{
			availableColors: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			oldColors:       []int{1, 1, 1, 1, 1},
			firstAvailable:  -1,
			firstInUse:      1,
		},
		{
			availableColors: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			oldColors:       []int{1, 2, 3, 4},
			firstAvailable:  6,
			firstInUse:      0,
		},
		{
			availableColors: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			oldColors:       []int{6, 0, 3, 5, 3, 4, 0, 0},
			firstAvailable:  6,
			firstInUse:      0,
		},
		{
			availableColors: []int{1, 2, 3, 4, 5, 6, 7},
			oldColors:       []int{6, 1, 3, 5, 3, 4, 2, 7},
			firstAvailable:  -1,
			firstInUse:      0,
		},
		{
			availableColors: []int{1, 2, 3, 4, 5, 6, 7},
			oldColors:       []int{6, 0, 3, 5, 3, 4, 2, 7},
			firstAvailable:  -1,
			firstInUse:      0,
		},
	}
	for _, testcase := range testcases {
		parser := &Parser{}
		parser.Reset()
		parser.availableColors = append([]int{}, testcase.availableColors...)
		parser.oldColors = append(parser.oldColors, testcase.oldColors...)
		parser.firstAvailable = testcase.firstAvailable
		parser.firstInUse = testcase.firstInUse
		parser.releaseUnusedColors()

		if parser.firstAvailable == -1 {
			// All in use
			for _, color := range parser.availableColors {
				found := false
				for _, oldColor := range parser.oldColors {
					if oldColor == color {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("In testcase:\n%d\t%d\t%d %d =>\n%d\t%d\t%d %d: %d should be available but is not",
						testcase.availableColors,
						testcase.oldColors,
						testcase.firstAvailable,
						testcase.firstInUse,
						parser.availableColors,
						parser.oldColors,
						parser.firstAvailable,
						parser.firstInUse,
						color)
				}
			}
		} else if parser.firstInUse != -1 {
			// Some in use
			for i := parser.firstInUse; i != parser.firstAvailable; i = (i + 1) % len(parser.availableColors) {
				color := parser.availableColors[i]
				found := false
				for _, oldColor := range parser.oldColors {
					if oldColor == color {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("In testcase:\n%d\t%d\t%d %d =>\n%d\t%d\t%d %d: %d should be available but is not",
						testcase.availableColors,
						testcase.oldColors,
						testcase.firstAvailable,
						testcase.firstInUse,
						parser.availableColors,
						parser.oldColors,
						parser.firstAvailable,
						parser.firstInUse,
						color)
				}
			}
			for i := parser.firstAvailable; i != parser.firstInUse; i = (i + 1) % len(parser.availableColors) {
				color := parser.availableColors[i]
				found := false
				for _, oldColor := range parser.oldColors {
					if oldColor == color {
						found = true
						break
					}
				}
				if found {
					t.Errorf("In testcase:\n%d\t%d\t%d %d =>\n%d\t%d\t%d %d: %d should not be available but is",
						testcase.availableColors,
						testcase.oldColors,
						testcase.firstAvailable,
						testcase.firstInUse,
						parser.availableColors,
						parser.oldColors,
						parser.firstAvailable,
						parser.firstInUse,
						color)
				}
			}
		} else {
			// None in use
			for _, color := range parser.oldColors {
				if color != 0 {
					t.Errorf("In testcase:\n%d\t%d\t%d %d =>\n%d\t%d\t%d %d: %d should not be available but is",
						testcase.availableColors,
						testcase.oldColors,
						testcase.firstAvailable,
						testcase.firstInUse,
						parser.availableColors,
						parser.oldColors,
						parser.firstAvailable,
						parser.firstInUse,
						color)
				}
			}
		}
	}
}

func TestParseGlyphs(t *testing.T) {
	parser := &Parser{}
	parser.Reset()
	tgBytes := []byte(testglyphs)
	tg := tgBytes
	idx := bytes.Index(tg, []byte("\n"))
	row := 0
	for idx > 0 {
		parser.ParseGlyphs(tg[:idx])
		tg = tg[idx+1:]
		idx = bytes.Index(tg, []byte("\n"))
		if parser.flows[0] != 1 {
			t.Errorf("First column flow should be 1 but was %d", parser.flows[0])
		}
		colorToFlow := map[int]int64{}
		flowToColor := map[int64]int{}

		for i, flow := range parser.flows {
			if flow == 0 {
				continue
			}
			color := parser.colors[i]

			if fColor, in := flowToColor[flow]; in && fColor != color {
				t.Errorf("Row %d column %d flow %d has color %d but should be %d", row, i, flow, color, fColor)
			}
			flowToColor[flow] = color
			if cFlow, in := colorToFlow[color]; in && cFlow != flow {
				t.Errorf("Row %d column %d flow %d has color %d but conflicts with flow %d", row, i, flow, color, cFlow)
			}
			colorToFlow[color] = flow
		}
		row++
	}
	if len(parser.availableColors) != 9 {
		t.Errorf("Expected 9 colors but have %d", len(parser.availableColors))
	}
}

func TestCommitStringParsing(t *testing.T) {
	dataFirstPart := "* DATA:|4e61bacab44e9b4730e44a6615d04098dd3a8eaf|2016-12-20 21:10:41 +0100|4e61bac|"
	tests := []struct {
		shouldPass    bool
		testName      string
		commitMessage string
	}{
		{true, "normal", "not a fancy message"},
		{true, "extra pipe", "An extra pipe: |"},
		{true, "extra 'Data:'", "DATA: might be trouble"},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			testString := fmt.Sprintf("%s%s", dataFirstPart, test.commitMessage)
			idx := strings.Index(testString, "DATA:")
			commit, err := NewCommit(0, 0, []byte(testString[idx+5:]))
			if err != nil && test.shouldPass {
				t.Errorf("Could not parse %s", testString)
				return
			}

			if test.commitMessage != commit.Subject {
				t.Errorf("%s does not match %s", test.commitMessage, commit.Subject)
			}
		})
	}
}

var testglyphs = `* 
* 
* 
* 
* 
* 
* 
*   
|\  
* | 
* | 
* | 
* | 
* | 
| * 
* | 
| *   
| |\  
* | | 
| | *   
| | |\  
* | | \   
|\ \ \ \  
| * | | | 
| |\| | | 
* | | | | 
|/ / / /  
| | | * 
| * | | 
| * | | 
| * | | 
* | | | 
* | | | 
* | | | 
* | | | 
* | | |   
|\ \ \ \  
| | * | | 
| | |\| | 
| | | * | 
| | | | * 
* | | | | 
* | | | | 
* | | | | 
* | | | | 
* | | | |   
|\ \ \ \ \  
| * | | | | 
|/| | | | | 
| | |/ / /  
| |/| | |   
| | | | * 
| * | | | 
|/| | | | 
| * | | | 
|/| | | | 
| | |/ /  
| |/| |   
| * | | 
| * | |   
| |\ \ \  
| | * | | 
| |/| | | 
| | | |/  
| | |/|   
| * | | 
| * | | 
| * | | 
| | * |   
| | |\ \  
| | | * | 
| | |/| | 
| | | * |   
| | | |\ \  
| | | | * | 
| | | |/| | 
| | * | | | 
| | * | | |   
| | |\ \ \ \  
| | | * | | | 
| | |/| | | | 
| | | | | * | 
| | | | |/ /  
* | | | / / 
|/ / / / /  
* | | | |   
|\ \ \ \ \  
| * | | | | 
|/| | | | | 
| * | | | | 
| * | | | |   
| |\ \ \ \ \  
| | | * \ \ \   
| | | |\ \ \ \  
| | | | * | | | 
| | | |/| | | | 
| | | | | |/ /  
| | | | |/| |   
* | | | | | | 
* | | | | | | 
* | | | | | | 
| | | | * | | 
* | | | | | | 
| | * | | | | 
| |/| | | | | 
* | | | | | | 
| |/ / / / /  
|/| | | | |   
| | | | * | 
| | | |/ /  
| | |/| |   
| * | | | 
| | | | * 
| | * | |   
| | |\ \ \  
| | | * | | 
| | |/| | | 
| | | |/ /  
| | | * | 
| | * | |   
| | |\ \ \  
| | | * | | 
| | |/| | | 
| | | |/ /  
| | | * | 
* | | | |   
|\ \ \ \ \  
| * \ \ \ \   
| |\ \ \ \ \  
| | | |/ / /  
| | |/| | |   
| | | | * | 
| | | | * | 
* | | | | | 
* | | | | | 
|/ / / / /  
| | | * | 
* | | | | 
* | | | | 
* | | | | 
* | | | |   
|\ \ \ \ \  
| * | | | | 
|/| | | | | 
| | * | | |   
| | |\ \ \ \  
| | | * | | | 
| | |/| | | | 
| |/| | |/ /  
| | | |/| |   
| | | | | * 
| |_|_|_|/  
|/| | | |   
| | * | | 
| |/ / /  
* | | | 
* | | | 
| | * | 
* | | | 
* | | | 
| * | | 
| | * | 
| * | | 
* | | |   
|\ \ \ \  
| * | | | 
|/| | | | 
| |/ / /  
| * | |   
| |\ \ \  
| | * | | 
| |/| | | 
| | |/ /  
| | * |   
| | |\ \  
| | | * | 
| | |/| | 
* | | | | 
* | | | |   
|\ \ \ \ \  
| * | | | | 
|/| | | | | 
| | * | | | 
| | * | | | 
| | * | | | 
| |/ / / /  
| * | | |   
| |\ \ \ \  
| | * | | | 
| |/| | | | 
* | | | | | 
* | | | | | 
* | | | | | 
* | | | | | 
* | | | | | 
| | | | * | 
* | | | | |   
|\ \ \ \ \ \  
| * | | | | | 
|/| | | | | | 
| | | | | * | 
| | | | |/ /  
* | | | | |   
|\ \ \ \ \ \  
* | | | | | | 
* | | | | | | 
| | | | * | | 
* | | | | | | 
* | | | | | |   
|\ \ \ \ \ \ \  
| | |_|_|/ / /  
| |/| | | | |   
| | | | * | | 
| | | | * | | 
| | | | * | | 
| | | | * | | 
| | | | * | | 
| | | | * | | 
| | | |/ / /  
| | | * | | 
| | | * | | 
| | | * | | 
| | |/| | | 
| | | * | | 
| | |/| | | 
| | | |/ /  
| | * | | 
| |/| | | 
| | | * | 
| | |/ /  
| | * | 
| * | |   
| |\ \ \  
| * | | | 
| | * | | 
| |/| | | 
| | |/ /  
| | * |   
| | |\ \  
| | * | | 
* | | | | 
|\| | | | 
| * | | | 
| * | | | 
| * | | | 
| | * | | 
| * | | | 
| |\| | | 
| * | | | 
| | * | | 
| | * | | 
| * | | | 
| * | | | 
| * | | | 
| * | | | 
| * | | | 
| * | | | 
| * | | | 
| * | | | 
| | * | | 
| * | | | 
| * | | | 
| * | | | 
| * | | | 
| | * | | 
* | | | | 
|\| | | | 
| | * | | 
| * | | | 
| |\| | | 
| | * | | 
| | * | | 
| | * | | 
| | | * | 
* | | | | 
|\| | | | 
| | * | | 
| | |/ /  
| * | | 
| * | | 
| |\| | 
* | | | 
|\| | | 
| | * | 
| | * | 
| | * | 
| * | | 
| | * | 
| * | | 
| | * | 
| | * | 
| | * | 
| * | | 
| * | | 
| * | | 
| * | | 
| * | | 
| * | | 
| * | | 
* | | | 
|\| | | 
| * | | 
| |\| | 
| | * |   
| | |\ \  
* | | | | 
|\| | | | 
| * | | | 
| |\| | | 
| | * | | 
| | | * | 
| | |/ /  
* | | | 
* | | | 
|\| | | 
| * | | 
| |\| | 
| | * | 
| | * | 
| | * | 
| | | * 
* | | | 
|\| | | 
| * | | 
| * | | 
| | | *   
| | | |\  
* | | | | 
| |_|_|/  
|/| | |   
| * | | 
| |\| | 
| | * | 
| | * | 
| | * | 
| | * | 
| | * | 
| * | | 
* | | | 
|\| | | 
| * | | 
|/| | | 
| |/ /  
| * |   
| |\ \  
| * | | 
| * | | 
* | | | 
|\| | | 
| | * | 
| * | | 
| * | | 
| * | | 
* | | | 
|\| | | 
| * | | 
| * | | 
| | * |   
| | |\ \  
| | |/ /  
| |/| |   
| * | | 
* | | | 
|\| | | 
| * | | 
* | | | 
|\| | | 
| * | |   
| |\ \ \  
| * | | | 
| * | | | 
| | | * | 
| * | | | 
| * | | | 
| | |/ /  
| |/| |   
| | * | 
* | | | 
|\| | | 
| * | | 
| * | | 
| * | | 
| * | | 
| * | |   
| |\ \ \  
* | | | | 
|\| | | | 
| * | | | 
| * | | | 
* | | | | 
* | | | | 
|\| | | | 
| | | | *   
| | | | |\  
| |_|_|_|/  
|/| | | |   
| * | | | 
* | | | | 
* | | | | 
|\| | | | 
| * | | |   
| |\ \ \ \  
| | | |/ /  
| | |/| |   
| * | | | 
| * | | | 
| * | | | 
| * | | | 
| | * | | 
| | | * | 
| | |/ /  
| |/| |   
* | | | 
|\| | | 
| * | | 
| * | | 
| * | | 
| * | | 
| * | | 
* | | | 
|\| | | 
| * | | 
| * | | 
* | | | 
| * | | 
| * | | 
| * | | 
* | | | 
* | | | 
* | | | 
|\| | | 
| * | | 
* | | | 
* | | | 
* | | | 
* | | | 
| | | * 
* | | | 
|\| | | 
| * | | 
| * | | 
| * | | 
`
