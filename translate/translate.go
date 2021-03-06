// Package translate provides methods for convert Go AST to Fun AST.
package translate

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"strings"

	"strconv"

	"github.com/jBugman/fun-lang/fun"
)

// NewTranslator creates new translator with provided fileset.
func NewTranslator(fset *token.FileSet) Translator {
	return funC{fset}
}

// Translator provides methods for translation.
type Translator interface {
	Package(src *ast.File) (fun.Package, error)
	Import(imp *ast.ImportSpec) (fun.Import, error)
	Function(fd *ast.FuncDecl) (fun.FuncDecl, error)
	Statement(stmt ast.Stmt) (fun.Expr, error)
	Expression(expr ast.Expr) (fun.Expr, error)
}

type funC struct {
	fset *token.FileSet
}

// Module converts Go ast.File to Fun Module.
func (conv funC) Package(src *ast.File) (fun.Package, error) {
	var pack fun.Package
	// Package name
	pack.Name = identToString(src.Name)
	// Imports
	for _, imp := range src.Imports {
		funImp, err := conv.Import(imp)
		if err != nil {
			return pack, err
		}
		pack.Imports = append(pack.Imports, funImp)
	}
	// Top-level declarations
	for _, gd := range src.Decls {
		switch d := gd.(type) {
		case *ast.FuncDecl:
			fn, err := conv.Function(d)
			if err != nil {
				return pack, err
			}
			pack.TopLevels = append(pack.TopLevels, fn)
		}
	}
	return pack, nil
}

// Import converts Go import to Fun Import.
func (conv funC) Import(imp *ast.ImportSpec) (fun.Import, error) {
	var result fun.Import
	ex, err := conv.Expression(imp.Path)
	if err != nil {
		return result, err
	}
	s, ok := ex.(fun.StringLit)
	if !ok {
		return result, conv.errorWithAST("not a string or char literal", imp.Path)
	}
	result.Path = string(s)
	// TODO aliases
	return result, nil
}

// Function converts Go function declaration to the Fun one.
func (conv funC) Function(fd *ast.FuncDecl) (fun.FuncDecl, error) {
	// Name
	fn := fun.FuncDecl{Name: identToString(fd.Name)}
	// Parameters
	if fd.Type.Params.List != nil {
		for _, p := range fd.Type.Params.List {
			ex, ok := p.Type.(*ast.Ident)
			if !ok {
				return fn, conv.errorWithAST("not supported type", p.Type)
			}
			tp := identToString(ex)
			for _, n := range p.Names {
				fn.Params = append(fn.Params, fun.Param{V: fun.VarSpec{Name: identToString(n), Type: fun.Atomic(tp)}})
			}
		}
	}
	// Results
	if fd.Type.Results != nil {
		for _, p := range fd.Type.Results.List {
			tp := identToString(p.Type.(*ast.Ident))
			fn.Results = append(fn.Results, fun.Atomic(tp))
		}
	}
	// Body
	if fd.Body == nil {
		return fn, conv.errorWithAST("empty function body is not supported", fd)
	}
	if len(fd.Body.List) == 1 {
		// Convert to FuncApplication
		stmt, err := conv.Statement(fd.Body.List[0])
		if err != nil {
			return fn, err
		}
		fn.Body = fun.Single{Expr: stmt}
	} else {
		// Convert to Fun DoBlock
		db := fun.Inline{}
		for _, stmt := range fd.Body.List {
			var buf bytes.Buffer
			printer.Fprint(&buf, conv.fset, stmt)
			db.Block = append(db.Block, buf.String())
		}
		fn.Body = db
	}

	return fn, nil
}

// Statement converts Go statement to a corresponding Fun Expression depending on type.
func (conv funC) Statement(stmt ast.Stmt) (fun.Expr, error) {
	switch st := stmt.(type) {
	case *ast.ReturnStmt:
		lr := len(st.Results)
		switch lr {
		case 0:
			return nil, conv.errorWithAST("result list of zero length is not supported", st)
		case 1:
			// Single expression
			result, err := conv.Expression(st.Results[0])
			if err != nil {
				return nil, err
			}
			return result, nil
		default:
			//  ReturnList
			result := make(fun.Results, lr)
			for i := 0; i < lr; i++ {
				expr, err := conv.Expression(st.Results[i])
				if err != nil {
					return nil, err
				}
				result[i] = expr
			}
			return result, nil
		}
	case *ast.ExprStmt:
		result, err := conv.Expression(st.X)
		if err != nil {
			return nil, err
		}
		return result, nil
	default:
		return nil, conv.errorWithAST("ast.Stmt type not supported", st)
	}
}

// Expression converts Go expression to a Fun one.
func (conv funC) Expression(expr ast.Expr) (fun.Expr, error) {
	switch ex := expr.(type) {
	case *ast.Ident:
		if ex.Obj == nil {
			return nil, conv.errorWithAST("Ident with empty Obj is not supported", ex)
		}
		switch ex.Obj.Kind {
		case ast.Var:
			return fun.Var(ex.Name), nil
		default:
			return nil, conv.errorWithAST("unsupported Obj kind", ex.Obj)
		}
	case *ast.BinaryExpr:
		x, err := conv.Expression(ex.X)
		if err != nil {
			return nil, err
		}
		y, err := conv.Expression(ex.Y)
		if err != nil {
			return nil, err
		}
		op := ex.Op.String()
		result := fun.BinaryOp{X: x, Y: y, Op: fun.Operator(op)}
		return result, nil
	case *ast.SelectorExpr:
		var name, pkg string
		name = identToString(ex.Sel)
		switch x := ex.X.(type) {
		case *ast.Ident:
			pkg = identToString(x)
		default:
			return nil, conv.errorWithAST("argument type not supported", x)
		}
		return fun.FuncName{V: pkg + "." + name}, nil
	case *ast.CallExpr:
		e, err := conv.Expression(ex.Fun)
		if err != nil {
			return nil, err
		}
		name, ok := e.(fun.FuncName)
		if !ok {
			return nil, conv.errorWithAST("expected FunctionVal but got", e)
		}
		result := fun.Application{Name: name}
		for _, aex := range ex.Args {
			arg, err := conv.Expression(aex)
			if err != nil {
				return nil, err
			}
			result.Args = append(result.Args, arg)
		}
		return result, nil
	case *ast.BasicLit:
		switch ex.Kind {
		case token.INT:
			x, err := strconv.Atoi(ex.Value)
			return fun.IntegerLit(x), err
		case token.FLOAT:
			x, err := strconv.ParseFloat(ex.Value, 64)
			return fun.DoubleLit(x), err
		case token.STRING:
			return fun.StringLit(strings.Trim(ex.Value, `"`)), nil
		case token.CHAR:
			runes := []rune(strings.Trim(ex.Value, "'"))
			return fun.CharLit(runes[0]), nil
		// case token.IMAG:
		// 	return fun.Imaginary(ex.Value), nil
		default:
			return nil, conv.errorWithAST("unexpected literal type", ex)
		}
	default:
		return nil, conv.errorWithAST("Expr type not supported", ex)
	}
}

func (conv funC) errorWithAST(message string, obj interface{}) error {
	var buf bytes.Buffer
	ast.Fprint(&buf, conv.fset, obj, ast.NotNilFilter)
	return fmt.Errorf("%s:\n%s", message, buf.String())
}

// Shortcut for cases there Ident.Obj is not relevant.
func identToString(ident *ast.Ident) string {
	return ident.Name
}
