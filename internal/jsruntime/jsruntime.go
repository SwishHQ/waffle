// Package jsruntime is feast's in-process JavaScript engine: it transpiles a
// user's JSX/TSX React document and runs it entirely inside the Go process to
// produce a feast-tree/v1 JSON document — no Node.js at render time.
//
// The pipeline is pure Go end to end:
//
//	esbuild (github.com/evanw/esbuild)  — transpile JSX/TSX + bundle, in memory
//	goja    (github.com/dop251/goja)    — execute the bundled React program
//
// React itself (react.production.min.js) and the @feast/react runtime are
// embedded and resolved by an esbuild plugin from strings, so nothing is read
// from disk or node_modules. goja is the single JS engine; there is no sidecar.
package jsruntime

import (
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"github.com/dop251/goja"
	"github.com/evanw/esbuild/pkg/api"
)

//go:embed assets/react.production.min.js
var reactUMD string

//go:embed assets/feast-react.js
var feastRuntime string

// jsxRuntime is a tiny automatic-JSX-runtime shim so user files need no React
// import: esbuild rewrites <X/> to jsx(X, props) importing from "react/jsx-runtime".
// React.createElement merges a config object (including children) into props, so
// forwarding props verbatim reproduces classic createElement semantics.
const jsxRuntime = `import React from 'react';
export function jsx(type, props, key){
  return React.createElement(type, key === undefined ? props : Object.assign({}, props, { key: key }));
}
export const jsxs = jsx;
export const jsxDEV = jsx;
export const Fragment = React.Fragment;
`

// entry is the bundler entry point. It imports the user's default export and the
// serializer, then exposes render(propsJSON)->treeJSON on the IIFE global.
//
// A function default export is wrapped as a React element (not called directly)
// so it is invoked *inside* serialize()'s render pass, where the hooks
// dispatcher is installed — otherwise hooks in the top-level component would run
// with no dispatcher and throw.
const entry = `import UserDefault from 'feast:user';
import React from 'react';
import { serializeString, evalCallbackString, evalPaintString } from '@feast/react';
export function render(propsJSON){
  var props = propsJSON ? JSON.parse(propsJSON) : {};
  var el = (typeof UserDefault === 'function') ? React.createElement(UserDefault, props) : UserDefault;
  return serializeString(el);
}
export function evalCallback(id, ctxJSON){
  return evalCallbackString(id, ctxJSON);
}
export function evalPaint(id, w, h){
  return evalPaintString(id, w, h);
}
`

// Options configures how a source is transpiled.
type Options struct {
	// TypeScript selects the TSX loader for the user source (JSX by default).
	TypeScript bool
	// Filename labels the user source in error messages (default "document.jsx").
	Filename string
}

// Program is a compiled React document, ready to render with varying props. It
// holds an esbuild-bundled, goja-compiled program; it is safe to Render many
// times (each Render runs on a fresh goja VM).
type Program struct {
	prog *goja.Program
	src  string // bundled JS, retained for debugging
}

// Compile transpiles and bundles a JSX/TSX React document into an executable
// program. The source's default export must be a <Document> element or a
// function returning one (given props).
func Compile(source []byte, opts Options) (*Program, error) {
	js, err := bundle(source, opts)
	if err != nil {
		return nil, err
	}
	name := opts.Filename
	if name == "" {
		name = "document.jsx"
	}
	src := string(js)
	prog, err := goja.Compile(name, src, true)
	if err != nil {
		return nil, fmt.Errorf("jsruntime: compile bundled program: %w", err)
	}
	return &Program{prog: prog, src: src}, nil
}

// Render executes the program on a fresh VM with the given props (already
// JSON-encoded; pass nil for none) and returns the feast-tree/v1 JSON. It is a
// one-shot convenience over Instantiate for documents with no render callbacks.
func (p *Program) Render(propsJSON []byte) ([]byte, error) {
	inst, err := p.Instantiate(propsJSON)
	if err != nil {
		return nil, err
	}
	return inst.Tree(), nil
}

// Instance is a rendered document on a live goja VM. Unlike Render, it keeps the
// runtime alive so function render-props (kept as closures in the VM) can be
// evaluated per page via EvalCallback. It is NOT safe for concurrent use — the
// underlying goja VM is single-threaded — but layout/pagination is sequential.
type Instance struct {
	vm       *goja.Runtime
	feastObj *goja.Object // the __feast object
	tree     []byte
}

// Instantiate runs the program with props and returns a live Instance whose
// Tree() is the feast-tree/v1 JSON.
func (p *Program) Instantiate(propsJSON []byte) (inst *Instance, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("jsruntime: panic during instantiate: %v", r)
		}
	}()

	vm := goja.New()
	if _, err = vm.RunProgram(p.prog); err != nil {
		return nil, fmt.Errorf("jsruntime: run program: %w", jsErr(err))
	}
	global := vm.Get("__feast")
	if global == nil || goja.IsUndefined(global) {
		return nil, fmt.Errorf("jsruntime: bundle did not expose __feast global")
	}
	obj := global.ToObject(vm)
	renderFn, ok := goja.AssertFunction(obj.Get("render"))
	if !ok {
		return nil, fmt.Errorf("jsruntime: __feast.render is not a function")
	}
	arg := goja.Undefined()
	if len(propsJSON) > 0 {
		arg = vm.ToValue(string(propsJSON))
	}
	res, err := renderFn(goja.Undefined(), arg)
	if err != nil {
		return nil, fmt.Errorf("jsruntime: evaluate document: %w", jsErr(err))
	}
	return &Instance{vm: vm, feastObj: obj, tree: []byte(res.String())}, nil
}

// Tree returns the rendered feast-tree/v1 JSON.
func (i *Instance) Tree() []byte { return i.tree }

// EvalCallback evaluates the render-prop callback id with ctxJSON (the page
// context: pageNumber, totalPages, …) and returns a JSON array of feast-tree
// nodes produced by the callback.
func (i *Instance) EvalCallback(id string, ctxJSON []byte) (nodes []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("jsruntime: panic during callback %q: %v", id, r)
		}
	}()
	fn, ok := goja.AssertFunction(i.feastObj.Get("evalCallback"))
	if !ok {
		return nil, fmt.Errorf("jsruntime: __feast.evalCallback is not a function")
	}
	ctxArg := goja.Undefined()
	if len(ctxJSON) > 0 {
		ctxArg = i.vm.ToValue(string(ctxJSON))
	}
	res, err := fn(goja.Undefined(), i.vm.ToValue(id), ctxArg)
	if err != nil {
		return nil, fmt.Errorf("jsruntime: eval callback %q: %w", id, jsErr(err))
	}
	return []byte(res.String()), nil
}

// EvalPaint evaluates a Canvas paint callback id against a painter of size w×h
// and returns the recorded ops as a JSON array ({op,args} objects).
func (i *Instance) EvalPaint(id string, w, h float64) (ops []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("jsruntime: panic during paint %q: %v", id, r)
		}
	}()
	fn, ok := goja.AssertFunction(i.feastObj.Get("evalPaint"))
	if !ok {
		return nil, fmt.Errorf("jsruntime: __feast.evalPaint is not a function")
	}
	res, err := fn(goja.Undefined(), i.vm.ToValue(id), i.vm.ToValue(w), i.vm.ToValue(h))
	if err != nil {
		return nil, fmt.Errorf("jsruntime: eval paint %q: %w", id, jsErr(err))
	}
	return []byte(res.String()), nil
}

// BundledSource returns the transpiled+bundled JS (for debugging/tests).
func (p *Program) BundledSource() string { return p.src }

// bundle transpiles the user source and bundles it with React + @feast/react
// into a single IIFE that assigns `var __feast = { render }`. Everything is
// resolved from embedded strings via a plugin: no disk or node_modules access.
func bundle(source []byte, opts Options) ([]byte, error) {
	userLoader := api.LoaderJSX
	if opts.TypeScript {
		userLoader = api.LoaderTSX
	}
	userSrc := string(source)

	plugin := api.Plugin{
		Name: "feast",
		Setup: func(b api.PluginBuild) {
			b.OnResolve(api.OnResolveOptions{
				Filter: `^(react|react/jsx-runtime|react/jsx-dev-runtime|@feast/react|feast:user)$`,
			}, func(a api.OnResolveArgs) (api.OnResolveResult, error) {
				return api.OnResolveResult{Path: a.Path, Namespace: "feast"}, nil
			})
			b.OnLoad(api.OnLoadOptions{
				Filter: `.*`, Namespace: "feast",
			}, func(a api.OnLoadArgs) (api.OnLoadResult, error) {
				var contents string
				loader := api.LoaderJS
				switch a.Path {
				case "react":
					contents = reactUMD
				case "react/jsx-runtime", "react/jsx-dev-runtime":
					contents = jsxRuntime
				case "@feast/react":
					contents = feastRuntime
				case "feast:user":
					contents, loader = userSrc, userLoader
				default:
					return api.OnLoadResult{}, fmt.Errorf("jsruntime: cannot resolve %q", a.Path)
				}
				return api.OnLoadResult{Contents: &contents, Loader: loader}, nil
			})
		},
	}

	result := api.Build(api.BuildOptions{
		Stdin: &api.StdinOptions{
			Contents:   entry,
			Sourcefile: "feast-entry.js",
			ResolveDir: "/",
			Loader:     api.LoaderJS,
		},
		Bundle:          true,
		Write:           false,
		Format:          api.FormatIIFE,
		GlobalName:      "__feast",
		Target:          api.ES2015,
		Platform:        api.PlatformNeutral,
		JSX:             api.JSXAutomatic,
		JSXImportSource: "react",
		Plugins:         []api.Plugin{plugin},
		LogLevel:        api.LogLevelSilent,
	})
	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("jsruntime: transpile: %s", esbuildErrors(result.Errors))
	}
	if len(result.OutputFiles) == 0 {
		return nil, fmt.Errorf("jsruntime: transpile produced no output")
	}
	return result.OutputFiles[0].Contents, nil
}

func esbuildErrors(msgs []api.Message) string {
	parts := make([]string, 0, len(msgs))
	for _, m := range msgs {
		if m.Location != nil {
			parts = append(parts, fmt.Sprintf("%s:%d: %s", m.Location.File, m.Location.Line, m.Text))
		} else {
			parts = append(parts, m.Text)
		}
	}
	return strings.Join(parts, "; ")
}

// jsErr unwraps a goja exception to its JS message/stack for readable errors.
func jsErr(err error) error {
	var ex *goja.Exception
	if errors.As(err, &ex) {
		return fmt.Errorf("%s", ex.String())
	}
	return err
}
