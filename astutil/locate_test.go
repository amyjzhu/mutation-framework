package astutil

import (
	"testing"
	"go/token"
	"go/parser"
	"go/ast"
	"github.com/stretchr/testify/assert"
	"github.com/amyjzhu/mutation-framework"
	"go/types"
	"fmt"
)

var tryCatchDataPath = "../testdata/astutil/trycatch.go"
var messageSendDataPath = "../testdata/astutil/trycatch.go"
var timeoutDataPath = "../testdata/astutil/timeout.go"
var loggingDataPath = "../testdata/astutil/trycatch.go"
var zeroRead = "../testdata/astutil/assign.go"

func loadAST(t *testing.T, path string) (*token.FileSet, *ast.File){
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		assert.Nil(t, err)
	}

	return fset, node
}

func TestIsReadOperation(t *testing.T) {
	expr, err := parser.ParseExpr("conn.Read(b)")
	assert.Nil(t, err)
	assert.True(t, isReadOperation(expr))

	expr, err = parser.ParseExpr("conn.read(b)")
	assert.Nil(t, err)
	assert.True(t, isReadOperation(expr))

	expr, err = parser.ParseExpr("conn.readDir(b)")
	assert.Nil(t, err)
	assert.False(t, isReadOperation(expr))

	expr, err = parser.ParseExpr("conn.Dial(b)")
	assert.Nil(t, err)
	assert.False(t, isReadOperation(expr))

	expr, err = parser.ParseExpr("sdfkjsd.read()")
	assert.Nil(t, err)
	assert.True(t, isReadOperation(expr))
}

/*
func TestCreateReadZeroAssignment(t *testing.T) {
	node, fset, _, info, err := mutesting.ParseAndTypeCheckFile(zeroRead)
	assert.Nil(t, err)

	actualAssign := createReadZeroAssignmentWrapper(node, info)

	assert.Equal(t, "n = 0", ast.Print(fset, actualAssign))
}
*/

func createReadZeroAssignmentWrapper(node ast.Node, info *types.Info) *ast.AssignStmt {
	var assignment *ast.AssignStmt
	ast.Inspect(node, func (node ast.Node) bool {
		if _, ok := node.(*ast.AssignStmt); ok {
			if assignment == nil {
				assignment = CreateReadZeroAssignment(node, info)
				if assignment != nil {
					return false
				}
			}
		}
		return true
	})

	return assignment
}

func TestInsertZeroAssignment(t *testing.T) {
	node, _, _, info, err := mutesting.ParseAndTypeCheckFile(zeroRead)
	assert.Nil(t, err)
	ast.Inspect(node, func (node ast.Node) bool {
		newAssign := CreateReadZeroAssignment(node, info)
		if newAssign != nil {
			fmt.Println(newAssign)
		} else {
			fmt.Println("not read")
		}
		return true
	})
}

func TestGetErrorBlockCode(t *testing.T) {
	node, _, _, info, err := mutesting.ParseAndTypeCheckFile(tryCatchDataPath)
	assert.Nil(t, err)

	bodies := getErrorBlockNodes(node, info)
	assert.Equal(t, 5, len(bodies))

	// make sure that no error handling block has a return statement
	// because this example has no returns in error-handling code
	// TODO fix this test and add more
	for _, block := range bodies {
		noReturnStatements(t, block)
	}
}

func TestGetNetworkVersion(t *testing.T) {
	assert.Equal(t, PLAIN, getNetworkVersionWrapper("udp", t))
	assert.Equal(t, PLAIN, getNetworkVersionWrapper("tcp", t))
	assert.Equal(t, FOUR, getNetworkVersionWrapper("tcp4", t))
	assert.Equal(t, FOUR, getNetworkVersionWrapper("udp4", t))
	assert.Equal(t, SIX, getNetworkVersionWrapper("tcp6", t))
	assert.Equal(t, SIX, getNetworkVersionWrapper("udp6", t))
	// TODO add some false tests
}

func getNetworkVersionWrapper(version string, t *testing.T) NetworkVersion {
	ver, err := getNetworkVersion(version)
	assert.Nil(t, err)
	return ver
}

/*
func TestIsThisErrorBlock(t *testing.T) {
	x := &ast.Ident{Name:"nil"}
	y := &ast.Ident{Name:"err"}

// TOOD until I figure out how to "create" a types.Info object
// I can't use this test anymore
	xObj := &types.Object{func Type() Type {}}
	defs := map[*ast.Ident]types.Object{x:xObj, y:}
	info := &types.Info{Defs:}

	conditional := &ast.BinaryExpr{X: x, Y: y, Op: token.NEQ}
	block := &ast.BlockStmt{}
	ifStmt := &ast.IfStmt{Cond: conditional, Body:block}

	isErrorHandler, actualBlock := IsErrorHandlingCode(ifStmt)
	assert.True(t, isErrorHandler)
	assert.Equal(t, block, actualBlock)

	x = &ast.Ident{Name:"data"}
	isErrorHandler, actualBlock = IsErrorHandlingCode(ifStmt)
	assert.False(t, isErrorHandler)
	assert.Nil(t, actualBlock)
}
*/

func TestMatchTimeLibrary(t *testing.T) {
	f, fset, _, info, err := mutesting.ParseAndTypeCheckFile(timeoutDataPath)

	x := &ast.CallExpr{}
	assert.False(t, IsTimeoutCall(x, info))
	// TODO add better false case

	time := &ast.Ident{Name:"t"}
	sleep := &ast.Ident{Name:"sleep"}
	selector := &ast.SelectorExpr{X: time, Sel: sleep}
	call := &ast.CallExpr{Fun: selector}
	assert.True(t, IsTimeoutCall(call, info))

	// shouldn't work because library is named t now
	time = &ast.Ident{Name:"time"}
	selector = &ast.SelectorExpr{X: time, Sel: sleep}
	call = &ast.CallExpr{Fun: selector}
	assert.False(t, IsTimeoutCall(call, info))

	_, _ = f, fset
	// easy/proper way to get package aliases, but mutants don't have access to
	// fset and ast.File
	/*imports := astutil.Imports(fset,f)
	fmt.Println(imports)
	for _, paragraph := range imports {
		for _, dependency := range paragraph {
			fmt.Println(dependency.Path.Value)
			fmt.Println(dependency.Name)
		}
	}*/

	assert.Nil(t, err)
}

func noReturnStatements(t *testing.T, node *ast.BlockStmt) {
	for _, stmt := range node.List {
		if _, ok := stmt.(*ast.ReturnStmt); ok {
			assert.Fail(t, "None of the error handling blocks should have return")
		}
	}
}

func TestIsTimeLibrary(t *testing.T) {
	assert.True(t, isTimeLibrary(`package sdfjkhsdfshdf ("time")`))
	assert.True(t, isTimeLibrary(`package t ("time")`))
	assert.True(t, isTimeLibrary(`package time ("time")`))
	assert.False(t, isTimeLibrary(`package sdf sdjfhsdf  ("time")`))
	assert.False(t, isTimeLibrary(` ("time")`))
	assert.False(t, isTimeLibrary(`package ("time")`))
	assert.False(t, isTimeLibrary(`package . ("fmt")`))
}