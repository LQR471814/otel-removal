package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"os"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/ast/inspector"
)

func printNode(n ast.Node) string {
	buff := bytes.NewBuffer(nil)
	err := ast.Fprint(buff, nil, n, nil)
	if err != nil {
		panic(err)
	}
	return buff.String()
}

type Transformer struct {
	typesInfo *types.Info
	fset      *token.FileSet
	files     []*ast.File
	filepaths []string
	inspector *inspector.Inspector

	Debug bool
}

func NewTransformer(
	typesInfo *types.Info,
	fset *token.FileSet,
	files []*ast.File,
	filepaths []string,
) Transformer {
	inspect := inspector.New(files)
	return Transformer{
		typesInfo: typesInfo,
		fset:      fset,
		files:     files,
		filepaths: filepaths,
		inspector: inspect,
	}
}

func (transform Transformer) handleFunc(fn *ast.FuncDecl, startingStack []ast.Node) {
	hasContextParam := false
	for _, param := range fn.Type.Params.List {
		sel, ok := param.Type.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		pkgId, ok := sel.X.(*ast.Ident)
		if !ok {
			continue
		}
		if pkgId.Name == "context" && sel.Sel.Name == "Context" {
			hasContextParam = true
			break
		}
	}
	if !hasContextParam {
		return
	}

	spanId := ""
	for _, stmt := range fn.Body.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok {
			continue
		}

		// find the name of the span:
		// ctx, span := tracer.Start(ctx, "SpanName")
		if len(assign.Rhs) != 1 || len(assign.Lhs) != 2 {
			continue
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok {
			continue
		}

		if len(call.Args) != 2 {
			continue
		}

		nameType, ok := call.Args[1].(*ast.BasicLit)
		if !ok {
			continue
		}
		if nameType.Kind != token.STRING {
			continue
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		if sel.Sel.Name != "Start" {
			continue
		}

		spanIdObj, ok := assign.Lhs[1].(*ast.Ident)
		if !ok {
			continue
		}
		spanId = spanIdObj.Name
	}

	if spanId == "" {
		return
	}

	var removedStatements []ast.Stmt
	transform.inspector.WithStack(
		[]ast.Node{&ast.Ident{}},
		func(n ast.Node, push bool, stack []ast.Node) (proceed bool) {
			if len(stack) <= len(startingStack) {
				for i := range stack {
					if stack[i] != startingStack[i] {
						return false
					}
				}
				return true
			}

			id := n.(*ast.Ident)
			if id.Name != spanId {
				return true
			}

			for i := len(stack) - 1; i >= len(startingStack); i-- {
				closestStmt, ok := stack[i].(ast.Stmt)
				if !ok {
					continue
				}

				if transform.Debug {
					fmt.Print("removed statement: ")
					format.Node(os.Stdout, transform.fset, closestStmt)
					fmt.Print("\n")
				}

				removedStatements = append(removedStatements, closestStmt)
				return false
			}
			return true
		},
	)

	if len(removedStatements) == 0 {
		return
	}

	offset := 0
	astutil.Apply(
		fn.Body,
		func(c *astutil.Cursor) bool {
			if offset >= len(removedStatements) {
				return false
			}
			node := c.Node()
			for i := offset; i < len(removedStatements); i++ {
				if node == removedStatements[i] {
					c.Delete()
					// we can do this because it is guaranteed that we will come across the
					// removed list of statements in the order it is in the list (because the
					// traversal method is the same and the tree hasn't changed)
					offset++
					return false
				}
			}
			return true
		},
		nil,
	)

	if transform.Debug {
		fmt.Println("\n---------- modified function:\n", fn.Name.Name)
		format.Node(os.Stdout, transform.fset, fn)
		fmt.Println("\n------------------------\n")
	}
}

func (transform Transformer) Run() error {
	transform.inspector.WithStack(
		[]ast.Node{&ast.FuncDecl{}},
		func(n ast.Node, push bool, stack []ast.Node) (proceed bool) {
			transform.handleFunc(n.(*ast.FuncDecl), stack)
			return false
		},
	)

	for i, file := range transform.files {
		f, err := os.OpenFile(transform.filepaths[i], os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return err
		}
		err = format.Node(f, transform.fset, file)
		if err != nil {
			return err
		}
		err = f.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
