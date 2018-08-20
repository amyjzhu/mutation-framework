package astutil

import (
	"testing"
	"go/token"
	"go/parser"
	"go/ast"
	"github.com/stretchr/testify/assert"
)

var tryCatchDataPath = "../testdata/astutil/trycatch.go"
var messageSendDataPath = "../testdata/astutil/trycatch.go"
var loggingDataPath = "../testdata/astutil/trycatch.go"

func loadAST(t *testing.T, path string) (*token.FileSet, *ast.File){
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		assert.Nil(t, err)
	}

	return fset, node
}

func TestGetErrorBlockCode(t *testing.T) {
	fset, node := loadAST(t, tryCatchDataPath)

	bodies := getErrorBlockNodes(node, fset)
	assert.Equal(t, 4, len(bodies))

	// make sure that no error handling block has a return statement
	// because this example has no returns in error-handling code
	// TODO fix this test and add more
	for _, block := range bodies {
		noReturnStatements(t, block)
	}
}

func noReturnStatements(t *testing.T, node *ast.BlockStmt) {
	for _, stmt := range node.List {
		if _, ok := stmt.(*ast.ReturnStmt); ok {
			assert.Fail(t, "None of the error handling blocks should have return")
		}
	}
}