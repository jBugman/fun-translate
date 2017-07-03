package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"reflect"
)

func main() {
	if len(os.Args) != 2 {
		exit("Usage: goast [filename.go]")
	}
	infile := os.Args[1]
	var err error

	// Read from source
	source, err := ioutil.ReadFile(infile)
	if err != nil {
		exit(err)
	}

	// Parse Go AST
	fset := token.NewFileSet()
	tree, err := parser.ParseFile(fset, infile, source, 0)
	if err != nil {
		exit(err)
	}

	// Pretty print AST
	err = ast.Fprint(os.Stdout, fset, tree, fieldFilter)
	if err != nil {
		exit(err)
	}
}

func fieldFilter(name string, v reflect.Value) bool {
	switch name {
	// Micro positions
	case "NamePos", "Lbrack", "Rbrack", "Lbrace", "Rbrace", "Lparen", "Rparen", "Opening", "Closing":
		return false
	// Macro positions
	case "Func", "Return":
		return false
	// Metadata
	case "Package", "Scope", "Unresolved":
		return false
	}
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return !v.IsNil()
	}
	return true
}

func exit(err interface{}) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
