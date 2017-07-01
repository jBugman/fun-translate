package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/jBugman/fun-translate/translate"
)

func main() {
	if len(os.Args) != 2 {
		exit("Usage: fun-translate [filename.go]")
	}
	infile := os.Args[1]
	outfile := strings.TrimSuffix(infile, filepath.Ext(infile)) + ".fun"
	var err error

	// Read from source
	source, err := ioutil.ReadFile(infile)
	if err != nil {
		exit(err)
	}

	// Parse Go AST
	fset := token.NewFileSet()
	ast, err := parser.ParseFile(fset, infile, source, 0)
	if err != nil {
		exit(err)
	}

	// Translate to Fun
	pkg, err := translate.NewTranslator(fset).Package(ast)
	if err != nil {
		exit(err)
	}
	// result =

	// Write result to file
	code := []byte(fmt.Sprint(pkg)) // This won't work since package printer is commented out
	err = ioutil.WriteFile(outfile, code, 0644)
	if err != nil {
		exit(err)
	}
}

func exit(err interface{}) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
