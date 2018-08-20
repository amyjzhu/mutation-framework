package astutil

import (
	"go/ast"
	"fmt"
	"go/token"
)

// find network calls
// I think we need a better interface since I want to be able to do different things
// with each type of networking function

// instrumenting multiple directories at a time?


// I think i should just do a for-each run function thing
// and then if you want to collect them, pass in your own thing by reference

// TODO probably consume an entire AST
// as well as functions to change/modify
// should it return, or simply map?
func captureInstances() {
	//capture.GetCommNodes()
}

func IsErrorHandlingCode(n ast.Node) bool {
	ret, ok := n.(*ast.IfStmt)
	if ok {

		//fmt.Printf("if statement found on line %d:\n\t", fset.Position(ret.Pos()).Line)
		testCond, ok := ret.Cond.(*ast.BinaryExpr)
		// TODO does not cover every error-handling case, but covers most common
		// err != nil or err == nil
		// another tactic could be dataflow following where errors are raised
		if ok {
			xId, ok := testCond.X.(*ast.Ident)
			if !ok {
				return false
			}
			yId, ok := testCond.Y.(*ast.Ident)
			if !ok {
				return false
			}
			x := xId.Name
			y := yId.Name

			if (x == "err" && y == "nil") || (y == "err" && x == "nil") {
				if testCond.Op == token.NEQ {
					// return the body
					fmt.Println("Found error handle")
					return true
				} else if testCond.Op == token.EQL {
					if ret.Else != nil {
						ifst, ok := ret.Else.(*ast.IfStmt)
						if ok {
							return IsErrorHandlingCode(ifst)
						} else {
							// the else block actually has error handling
							if _, ok := ret.Else.(*ast.BlockStmt); ok {
								fmt.Println("Found error handle")
								return true
							}
						}
					}
				}
			}

		}
	}

	return false
}

// TODO not sure which one is necessary.
// should rework this one to use one above
// handle by adding if is, ignoring if isn;t
func getErrorBlockNodes(node *ast.File, fset *token.FileSet) []*ast.BlockStmt {

	bodies := []*ast.BlockStmt{}

	var getErrorHandlingBodyFromIf func(ret *ast.IfStmt) bool

	getErrorHandlingBodyFromIf = func(ret *ast.IfStmt) bool {
		fmt.Printf("if statement found on line %d:\n\t", fset.Position(ret.Pos()).Line)
		testCond, ok := ret.Cond.(*ast.BinaryExpr)
		// TODO does not cover every error-handling case, but covers most common
		// err != nil or err == nil
		// another tactic could be dataflow following where errors are raised
		if ok {
			xId, ok := testCond.X.(*ast.Ident)
			if !ok {
				return true
			}
			yId, ok := testCond.Y.(*ast.Ident)
			if !ok {
				return true
			}
			x := xId.Name
			y := yId.Name

			if (x == "err" && y == "nil") || (y == "err" && x == "nil") {
				if testCond.Op == token.NEQ {
					// return the body
					fmt.Println("Found error handle")
					bodies = append(bodies, ret.Body)
				} else if testCond.Op == token.EQL {
					if ret.Else != nil {
						ifst, ok := ret.Else.(*ast.IfStmt)
						if ok {
							return getErrorHandlingBodyFromIf(ifst)
						} else {
							// the else block actually has error handling
							if block, ok := ret.Else.(*ast.BlockStmt); ok {
								fmt.Println("Found error handle")
								bodies = append(bodies, block)
							}
						}
					}
				}
			}

		}


		//printer.Fprint(os.Stdout, fset, ret)
		return true
	}


	ast.Inspect(node, func(n ast.Node) bool {
		// TODO necessary?
		if n == nil {
			return false
		}

		ret, ok := n.(*ast.IfStmt)
		if ok {
			getErrorHandlingBodyFromIf(ret)
			return true
		}
		return true
	})

	return bodies
}



func getTryBlockNodes() []*ast.Node {
	return nil
}

func getLoggingDenseAreas() {

}

// TODO perhaps specified by test administrator, since
// this could be difficult to automatically find
func getServiceStartupCode() {

}

func getServiceShutdownCode() {

}

func IsTimeoutCall(call *ast.CallExpr) bool {
	if fun, ok := call.Fun.(*ast.SelectorExpr); ok {
		funcName := fun.Sel.Name
		// TODO this is disgusting. can't I just retrieve the name?
		funcLibrary := fmt.Sprintf("%s", fun.X)

		if funcName == "Sleep" && funcLibrary == "time" {
			return true
		}
	}
	return false
}

