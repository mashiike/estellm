package jsonutil

import (
	"fmt"
	"os"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
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

func MakeVM() *jsonnet.VM {
	vm := jsonnet.MakeVM()
	for _, f := range nativeFunctions {
		vm.NativeFunction(f)
	}
	return vm
}
