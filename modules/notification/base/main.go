// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"sort"
	"strings"
	"text/template"
	"time"
)

// funcDef are a semi-generic function definitions
type funcDef struct {
	Name string
	Args []funcDefArg
}

// funcDefArg describe an argument to a function
type funcDefArg struct {
	Name string
	Type string
}

// main will generate two files that implement the Notifier interface
// defined in notifier.go
//
// * the NullNotifier which is a basic Notifier that does nothing
// when each of its methods is called
//
// * the QueueNotifier which is a notifier that sends its commands down
// a queue.
//
// The main benefit of this generation is that we never need to keep these
// up to date again. Add a new function to Notifier and the NullNotifier and
// the NotifierQueue will gain these functions automatically.
//
// There are two caveat:
// * All notifier functions must not return anything.
// * If you add a new import you will need to add it to the templates below
func main() {

	// OK build the AST from the notifier.go file
	fset := token.NewFileSet() // positions are relative to fset
	f, err := parser.ParseFile(fset, "notifier.go", nil, 0)
	if err != nil {
		panic(err)
	}

	// func will collect all the function definitions from the Notifier interface
	funcs := make([]funcDef, 0)

	ast.Inspect(f, func(n ast.Node) bool {
		spec, ok := n.(*ast.TypeSpec)
		if !ok || spec.Name.Name != "Notifier" { // We only care about the Notifier interface declaration
			return true // If we are not a type decl or aren't looking at the Notifier decl keep looking
		}

		// We're at: `type Notifier ...` so now we need check that it's an interface
		child, ok := spec.Type.(*ast.InterfaceType)
		if !ok {
			return false // There's no point looking in non interface types.
		}

		// OK we're in the declaration of the Notifier, e.g.
		// type Notifier interface { ... }

		// Let's look at each Method in turn, but first we redefine
		// funcs now we know how big it's supposed to be
		funcs = make([]funcDef, len(child.Methods.List))
		for i, method := range child.Methods.List {
			// example: NotifyPushCommits(pusher *models.User, repo *models.Repository, refName, oldCommitID, newCommitID string, commits *repository.PushCommits)

			// method here is looking at the NotifyPushCommits...

			// We know that interfaces have FuncType for the method
			methodFuncDef := method.Type.(*ast.FuncType) // eg. (...)

			// Extract the function definition from the method
			def := funcDef{}
			def.Name = method.Names[0].Name // methods only have one name in interfaces <- NotifyPushCommits

			// Now construct the args
			def.Args = make([]funcDefArg, 0, len(methodFuncDef.Params.List))
			for j, param := range methodFuncDef.Params.List {

				// interfaces don't have to name their arguments e.g. NotifyNewIssue(*models.Issue)
				// but we need a name to make a function call
				defaultName := fmt.Sprintf("unknown%d", j)

				// Now get the type - here we will just use what is used in source file. (See caveat 2.)
				sb := strings.Builder{}
				format.Node(&sb, fset, param.Type)

				// If our parameter is unnamed
				if len(param.Names) == 0 {
					def.Args = append(def.Args, funcDefArg{
						Name: defaultName,
						Type: sb.String(),
					})
				} else {
					// Now in the example NotifyPushCommits we see that refname, oldCommitID etc don't have type following them
					// The AST creates these as a single param with mulitple names
					// Therefore iterate through the param.Names
					for _, ident := range param.Names {
						def.Args = append(def.Args, funcDefArg{
							Name: ident.Name,
							Type: sb.String(),
						})
					}
				}
			}
			funcs[i] = def
		}

		// We're done so stop walking
		return false
	})

	sort.Slice(funcs, func(i, j int) bool {
		return funcs[i].Name < funcs[j].Name
	})

	// First lets create the NullNotifier
	buf := bytes.Buffer{}
	nullTemplate.Execute(&buf, struct {
		Timestamp time.Time
		Funcs     []funcDef
	}{
		Timestamp: time.Now(),
		Funcs:     funcs,
	})

	bs, err := format.Source(buf.Bytes())
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile("null.go", bs, 0644)
	if err != nil {
		panic(err)
	}

	// Then create the NotifierQueue
	buf = bytes.Buffer{}
	queueTemplate.Execute(&buf, struct {
		Timestamp time.Time
		Funcs     []funcDef
	}{
		Timestamp: time.Now(),
		Funcs:     funcs,
	})

	bs, err = format.Source(buf.Bytes())
	if err != nil {
		ioutil.WriteFile("queue.go", buf.Bytes(), 0644)
		panic(err)
	}

	err = ioutil.WriteFile("queue.go", bs, 0644)
	if err != nil {
		panic(err)
	}

}

var queueTemplate = template.Must(template.New("").Parse(`// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// Code generated by go generate; DO NOT EDIT.
package base

import (
	"encoding/json"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/queue"
)

// FunctionCall represents a function call with json.Marshaled arguments
type FunctionCall struct {
	Name string
	Args [][]byte
}

// QueueNotifier is a notifier queue
type QueueNotifier struct {
	name string
	notifiers []Notifier
	internal queue.Queue
}

// Ensure that QueueNotifier fulfils the Notifier interface
var (
	_ Notifier = &QueueNotifier{}
)

// NewQueueNotifier creates a notifier that queues notifications and on dequeueing sends them to the provided notifiers
func NewQueueNotifier(name string, notifiers []Notifier) Notifier {
	q := &QueueNotifier{
		name: name,
		notifiers: notifiers,
	}
	q.internal = queue.CreateQueue(name, q.handle, &FunctionCall{})
	return q
}

// NewQueueNotifierWithHandle creates a notifier queue with a specific handler function
func NewQueueNotifierWithHandle(name string, handle queue.HandlerFunc) Notifier {
	q := &QueueNotifier{
		name: name,
	}
	q.internal = queue.CreateQueue(name, handle, &FunctionCall{})
	return q
}

func (q *QueueNotifier) handle(data ...queue.Data) {
	for _, datum := range data {
		call := datum.(*FunctionCall)
		var err error
		switch call.Name {
		{{- range .Funcs }}
		case "{{.Name}}":
			{{$p := .Name}}
			{{- range $i, $e := .Args }}
			var {{$e.Name}} {{$e.Type}}
			err = json.Unmarshal(call.Args[{{$i}}], &{{$e.Name}})
			if err != nil {
				log.Error("Unable to unmarshal %s to %s in call to %s: %v", string(call.Args[{{$i}}]), "{{$e.Type}}", "{{$p}}", err)
				continue
			}
			{{- end }}
			for _, notifier := range q.notifiers {
				notifier.{{.Name}}({{- range $i, $e := .Args}}{{ if $i }}, {{ end }}{{$e.Name}}{{end}})
			}
		{{- end }}
		default:
			log.Error("Unknown notifier function %s with %d arguments", call.Name, len(call.Args))
		}
	}
}

func (q *QueueNotifier) Run() {
	for _, notifier := range q.notifiers {
		go notifier.Run()
	}
	graceful.GetManager().RunWithShutdownFns(q.internal.Run)
}
{{- range .Funcs}}
{{if ne .Name "Run"}}

// {{ .Name }} is a placeholder function
func (q *QueueNotifier) {{ .Name }}({{ range $i, $e := .Args }}{{ if $i }}, {{ end }}{{$e.Name}} {{$e.Type}}{{end}}) {
	args := make([][]byte, 0)
	var err error
	var bs []byte
	{{- range .Args }}
	bs, err = json.Marshal(&{{.Name}})
	if err != nil {
		log.Error("Unable to marshall {{.Name}}: %v", err)
		return
	}
	args = append(args, bs)
	{{- end }}

	q.internal.Push(&FunctionCall{
		Name: "{{.Name}}",
		Args: args,
	})
}
{{end}}
{{- end }}
`))

var nullTemplate = template.Must(template.New("").Parse(`// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// Code generated by go generate; DO NOT EDIT.
package base

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/repository"
)

// NullNotifier implements a blank notifier
type NullNotifier struct {
}

// Ensure that NullNotifier fulfils the Notifier interface
var (
	_ Notifier = &NullNotifier{}
)
{{- range .Funcs}}

// {{ .Name }} is a placeholder function
func (*NullNotifier) {{ .Name }}({{ range $i, $e := .Args }}{{ if $i }}, {{ end }}{{$e.Name}} {{$e.Type}}{{end}}) {}
{{- end }}
`))
