package jsonnetutil

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	aliasimporter "github.com/mashiike/go-jsonnet-alias-importer"
)

var nativeFunctions = []*jsonnet.NativeFunction{
	{
		Name:   "env",
		Params: []ast.Identifier{"name", "default"},
		Func: func(args []any) (any, error) {
			key, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("env: name must be a string")
			}
			if v := os.Getenv(key); v != "" {
				return v, nil
			}
			return args[1], nil
		},
	},
	{
		Name:   "mustEnv",
		Params: []ast.Identifier{"name"},
		Func: func(args []any) (any, error) {
			key, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("mustEnv: name must be a string")
			}
			if v, ok := os.LookupEnv(key); ok {
				return v, nil
			}
			return nil, fmt.Errorf("mustEnv: %s is not set", key)
		},
	},
}

type VM struct {
	includesFS fs.FS
	promptsFS  fs.FS
	vm         *jsonnet.VM
	importer   *aliasimporter.AliasImpoter
}

func MakeVM() VM {
	vm := jsonnet.MakeVM()
	importer := aliasimporter.New()
	vm.Importer(importer)
	for _, f := range nativeFunctions {
		vm.NativeFunction(f)
	}
	return VM{
		vm:       vm,
		importer: importer,
	}
}

func (vm *VM) ExtVars(extVars map[string]string) {
	for k, v := range extVars {
		vm.vm.ExtVar(k, v)
	}
}

func (vm *VM) ExtCodes(extCodes map[string]string) {
	for k, v := range extCodes {
		vm.vm.ExtCode(k, v)
	}
}

func (vm *VM) NativeFunction(f *jsonnet.NativeFunction) {
	vm.vm.NativeFunction(f)
}

func (vm *VM) NativeFunctions(fs ...*jsonnet.NativeFunction) {
	for _, f := range fs {
		vm.NativeFunction(f)
	}
}

func (vm *VM) Impl() *jsonnet.VM {
	return vm.vm
}

func (vm *VM) Includes(fsys fs.FS) {
	vm.includesFS = fsys
	vm.importer.Register("includes", fsys)
	vm.importer.ClearCache()
}

func (vm *VM) Prompts(fsys fs.FS) {
	vm.promptsFS = fsys
	vm.importer.Register("prompts", fsys)
	vm.importer.ClearCache()
}
