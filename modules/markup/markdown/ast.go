// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markdown

import (
	"strconv"

	"github.com/yuin/goldmark/ast"
)

// Details is a block that contains Summary and details
type Details struct {
	ast.BaseBlock
}

// Dump implements Node.Dump .
func (n *Details) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

// KindDetails is the NodeKind for Details
var KindDetails = ast.NewNodeKind("Details")

// Kind implements Node.Kind.
func (n *Details) Kind() ast.NodeKind {
	return KindDetails
}

// NewDetails returns a new Paragraph node.
func NewDetails() *Details {
	return &Details{
		BaseBlock: ast.BaseBlock{},
	}
}

// IsDetails returns true if the given node implements the Details interface,
// otherwise false.
func IsDetails(node ast.Node) bool {
	_, ok := node.(*Details)
	return ok
}

// Summary is a block that contains the summary of details block
type Summary struct {
	ast.BaseBlock
}

// Dump implements Node.Dump .
func (n *Summary) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

// KindSummary is the NodeKind for Summary
var KindSummary = ast.NewNodeKind("Summary")

// Kind implements Node.Kind.
func (n *Summary) Kind() ast.NodeKind {
	return KindSummary
}

// NewSummary returns a new Summary node.
func NewSummary() *Summary {
	return &Summary{
		BaseBlock: ast.BaseBlock{},
	}
}

// IsSummary returns true if the given node implements the Summary interface,
// otherwise false.
func IsSummary(node ast.Node) bool {
	_, ok := node.(*Summary)
	return ok
}

// TaskCheckBoxListItem is a block that represents a list item of a markdown block with a checkbox
type TaskCheckBoxListItem struct {
	*ast.ListItem
	IsChecked bool
}

// KindTaskCheckBoxListItem is the NodeKind for TaskCheckBoxListItem
var KindTaskCheckBoxListItem = ast.NewNodeKind("TaskCheckBoxListItem")

// Dump implements Node.Dump .
func (n *TaskCheckBoxListItem) Dump(source []byte, level int) {
	m := map[string]string{}
	m["IsChecked"] = strconv.FormatBool(n.IsChecked)
	ast.DumpHelper(n, source, level, m, nil)
}

// Kind implements Node.Kind.
func (n *TaskCheckBoxListItem) Kind() ast.NodeKind {
	return KindTaskCheckBoxListItem
}

// NewTaskCheckBoxListItem returns a new TaskCheckBoxListItem node.
func NewTaskCheckBoxListItem(listItem *ast.ListItem) *TaskCheckBoxListItem {
	return &TaskCheckBoxListItem{
		ListItem: listItem,
	}
}

// IsTaskCheckBoxListItem returns true if the given node implements the TaskCheckBoxListItem interface,
// otherwise false.
func IsTaskCheckBoxListItem(node ast.Node) bool {
	_, ok := node.(*TaskCheckBoxListItem)
	return ok
}

// Icon is an inline for a fomantic icon
type Icon struct {
	ast.BaseInline
	Name []byte
}

// Dump implements Node.Dump .
func (n *Icon) Dump(source []byte, level int) {
	m := map[string]string{}
	m["Name"] = string(n.Name)
	ast.DumpHelper(n, source, level, m, nil)
}

// KindIcon is the NodeKind for Icon
var KindIcon = ast.NewNodeKind("Icon")

// Kind implements Node.Kind.
func (n *Icon) Kind() ast.NodeKind {
	return KindIcon
}

// NewIcon returns a new Paragraph node.
func NewIcon(name string) *Icon {
	return &Icon{
		BaseInline: ast.BaseInline{},
		Name:       []byte(name),
	}
}

// IsIcon returns true if the given node implements the Icon interface,
// otherwise false.
func IsIcon(node ast.Node) bool {
	_, ok := node.(*Icon)
	return ok
}
