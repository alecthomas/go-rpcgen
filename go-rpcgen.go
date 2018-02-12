// Copyright 2012 Alec Thomas
// Copyright (c) 2018 Samsung Electronics Co., Ltd All Rights Reserved
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/alecthomas/template"
)

var rpcTemplate = `// Generated by go-rpcgen. Do not modify.

package {{.Package}}

import (
{{range $key, $value := .Imports}}  "{{$key}}"
{{end}})
{{$type := .Type}}
type {{.Type}}Service struct {
	impl {{.Type}}
}

func New{{.Type}}Service(impl {{.Type}}) *{{.Type}}Service {
	return &{{.Type}}Service{impl}
}

func Register{{.Type}}Service(server *rpc.Server, impl {{.Type}}) error {
	return server.RegisterName("{{.Service}}", New{{.Type}}Service(impl))
}
{{range .Methods}}
type {{$type}}{{.Name}}Request struct {
	{{.Parameters | publicfields}}
}

type {{$type}}{{.Name}}Response struct {
	{{.Results | publicfields}}
}

func (s *{{$type}}Service) {{.Name}}(request *{{$type}}{{.Name}}Request, response *{{$type}}{{.Name}}Response) (err error) {
	{{.Results | publicrefswithprefix "response."}}{{if .Results}}, {{end}}err = s.impl.{{.Name}}({{.Parameters | publicrefswithprefix "request."}})
	return
}
{{end}}
type {{.Type}}Client struct {
	client {{.RPCType}}
}

func Dial{{.Type}}Client(addr string) (*{{.Type}}Client, error) {
	client, err := rpc.Dial("tcp", addr)
	return &{{.Type}}Client{client}, err
}

func New{{.Type}}Client(client {{.RPCType}}) *{{.Type}}Client {
	return &{{.Type}}Client{client}
}

func (_c *{{$type}}Client) Close() error {
	return _c.client.Close()
}
{{range .Methods}}
func (_c *{{$type}}Client) {{.Name}}({{.Parameters | functionargs}}) ({{.Results | functionargs}}{{if .Results}}, {{end}}err error) {
	_request := &{{$type}}{{.Name}}Request{{"{"}}{{.Parameters | refswithprefix ""}}{{"}"}}
	_response := &{{$type}}{{.Name}}Response{}
	err = _c.client.Call("{{$.Service}}.{{.Name}}", _request, _response)
	return {{.Results | publicrefswithprefix "_response."}}{{if .Results}}, {{end}}err
}
{{end}}`

var usage = `usage: %s --source=<source.go> --type=<interface_type_name>

This utility generates server and client RPC stubs from a Go interface.

If you had a file "arith.go" containing this interface:

  package arith

  type Arith interface {
  	Add(a, b int)
  }

The following command will generate stubs for the interface:

  ./%s --source=arith.go --type=Arith

That will generate a file containing two types, ArithService and ArithClient,
that can be used with the Go RPC system, and as a client for the system,
respectively.

Flags:
`

var source = flag.String("source", "", "source file to parse RPC interface from")
var rpcType = flag.String("type", "", "type to generate RPC interface from")
var target = flag.String("target", "", "target file to write stubs to")
var importsFlag = flag.String("imports", "net/rpc", "list of imports to add")
var packageFlag = flag.String("package", "", "package to export under")
var serviceName = flag.String("service", "", "service name to use (defaults to type name)")
var rpcClientTypeFlag = flag.String("rpc_client_type", "*rpc.Client", "type to use for RPC client interfaces")

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage, os.Args[0], os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if *source == "" || *rpcType == "" {
		fatalf("expected --source and --type")
	}
	if *target == "" {
		parts := strings.Split(*source, ".")
		parts = parts[:len(parts)-1]
		*target = strings.Join(parts, ".") + "rpc.go"
	}

	fileset := token.NewFileSet()
	f, err := parser.ParseFile(fileset, *source, nil, 0)
	if err != nil {
		fatalf("failed to parse %s: %s", *source, err)
	}
	imports := map[string]bool{}
	if *importsFlag != "" {
		for _, imp := range strings.Split(*importsFlag, ",") {
			imports[imp] = true
		}
	}
	if *packageFlag == "" {
		*packageFlag = f.Name.Name
	}
	if *serviceName == "" {
		*serviceName = *rpcType
	}
	gen := &RPCGen{
		Service: *serviceName,
		Type:    *rpcType,
		RPCType: *rpcClientTypeFlag,
		Package: *packageFlag,
		Imports: imports,
		fileset: fileset,
	}
	ast.Walk(gen, f)
	funcs := map[string]interface{}{
		"publicfields":         func(fields []*Type) string { return FieldList(fields, "", "\n\t", true, true) },
		"refswithprefix":       func(prefix string, fields []*Type) string { return FieldList(fields, prefix, ", ", false, false) },
		"publicrefswithprefix": func(prefix string, fields []*Type) string { return FieldList(fields, prefix, ", ", false, true) },
		"functionargs":         func(fields []*Type) string { return FieldList(fields, "", ", ", true, false) },
	}
	t, err := template.New("rpc").Funcs(funcs).Parse(rpcTemplate)
	if err != nil {
		fatalf("failed to parse template: %s", err)
	}
	out, err := os.Create(*target)
	if err != nil {
		fatalf("failed to create output file %s: %s", *target, err)
	}
	err = t.Execute(out, gen)
	if err != nil {
		fatalf("failed to execute template: %s", err)
	}
	fmt.Printf("%s: wrote RPC stubs for %s to %s\n", os.Args[0], *rpcType, *target)
	if out, err := exec.Command("go", "fmt", *target).CombinedOutput(); err != nil {
		fatalf("failed to run go fmt on %s: %s: %s", *target, err, string(out))
	}
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s: error: %s\n", os.Args[0], fmt.Sprintf(format, args...))
	os.Exit(1)
}

func fatalNode(fileset *token.FileSet, node ast.Node, format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s: error: %s: %s\n", os.Args[0], fileset.Position(node.Pos()).String(), fmt.Sprintf(format, args...))
	os.Exit(1)
}

type Type struct {
	Names      []string
	LowerNames []string
	Type       string
}

func (t *Type) NamesString() string {
	return strings.Join(t.Names, ", ")
}

func (t *Type) LowerNamesString() string {
	return strings.Join(t.LowerNames, ", ")
}

type Method struct {
	Name       string
	Parameters []*Type
	Results    []*Type
}

func FieldList(fields []*Type, prefix string, delim string, withTypes bool, public bool) string {
	var out []string
	for _, p := range fields {
		suffix := ""
		if withTypes {
			suffix = " " + p.Type
		}
		names := p.LowerNames
		if public {
			names = p.Names
		}
		var field []string
		for _, n := range names {
			field = append(field, prefix+n)
		}
		out = append(out, strings.Join(field, ", ")+suffix)
	}
	return strings.Join(out, delim)
}

type RPCGen struct {
	Service      string
	Type         string
	Package      string
	Methods      []*Method
	Imports      map[string]bool
	RPCType      string
	fileset      *token.FileSet
	CheckImports []*ast.ImportSpec
}

func (r *RPCGen) Visit(node ast.Node) (w ast.Visitor) {
	switch n := node.(type) {
	case *ast.ImportSpec:
		r.CheckImports = append(r.CheckImports, n)

	case *ast.TypeSpec:
		name := n.Name.Name
		if name == r.Type {
			return &InterfaceGen{RPCGen: r}
		}
	}
	return r
}

type InterfaceGen struct {
	*RPCGen
}

func (r *InterfaceGen) VisitMethodList(n *ast.InterfaceType) {
	for _, m := range n.Methods.List {
		switch t := m.Type.(type) {
		case *ast.FuncType:
			method := &Method{
				Name:       m.Names[0].Name,
				Parameters: make([]*Type, 0),
				Results:    make([]*Type, 0),
			}
			for _, v := range t.Params.List {
				method.Parameters = append(method.Parameters, r.formatType(r.fileset, v))
			}
			hasError := false
			if t.Results != nil {
				for _, v := range t.Results.List {
					result := r.formatType(r.fileset, v)
					if result.Type == "error" {
						hasError = true
					} else {
						method.Results = append(method.Results, result)
					}
				}
			}
			if !hasError {
				fatalNode(r.fileset, m, "method %s must have error as last return value", method.Name)
			}
			r.Methods = append(r.Methods, method)
		}
	}
}

func (r *InterfaceGen) Visit(node ast.Node) (w ast.Visitor) {
	switch n := node.(type) {
	case *ast.InterfaceType:
		r.VisitMethodList(n)
	}
	return r.RPCGen
}

func types(t ast.Expr) []string {
	switch n := t.(type) {
	case *ast.StarExpr:
		return types(n.X)
	case *ast.SelectorExpr:
		return []string{strings.Join(append(types(n.X), types(n.Sel)...), ".")}
	case *ast.MapType:
		keys := types(n.Key)
		return append(keys, types(n.Value)...)
	case *ast.ArrayType:
		return types(n.Elt)
	case *ast.Ident:
		return []string{n.Name}
	default:
		panic(fmt.Sprintf("unknown expression node %s %s\n", reflect.TypeOf(t), t))
	}
}

func (r *InterfaceGen) formatType(fileset *token.FileSet, field *ast.Field) *Type {
	var typeBuf bytes.Buffer
	_ = printer.Fprint(&typeBuf, fileset, field.Type)
	if len(field.Names) == 0 {
		fatalNode(fileset, field, "RPC interface parameters and results must all be named")
	}
	for _, typeName := range types(field.Type) {
		parts := strings.SplitN(typeName, ".", 2)
		if len(parts) > 1 {
			for _, imp := range r.CheckImports {
				importPath := imp.Path.Value[1 : len(imp.Path.Value)-1]
				if imp.Name != nil && imp.Name.String() == parts[0] {
					r.Imports[fmt.Sprintf("%s %s", imp.Name, importPath)] = true
				} else if filepath.Base(importPath) == parts[0] {
					r.Imports[importPath] = true
				}
			}
		}
	}
	t := &Type{Type: typeBuf.String()}
	for _, n := range field.Names {
		lowerName := n.Name
		name := strings.ToUpper(lowerName[0:1]) + lowerName[1:]
		t.Names = append(t.Names, name)
		t.LowerNames = append(t.LowerNames, lowerName)
	}
	return t
}
