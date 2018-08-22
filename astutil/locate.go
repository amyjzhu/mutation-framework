package astutil

import (
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"
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


func IsSendingMessageNode() {

}

func isConditionalErrorHandling(testCond *ast.BinaryExpr, info *types.Info) bool {
	xId, ok := testCond.X.(*ast.Ident)
	if !ok {
		return false
	}
	yId, ok := testCond.Y.(*ast.Ident)
	if !ok {
		return false
	}

	xObj := info.Uses[xId]
	yObj := info.Uses[yId]

	if xObj != nil && yObj != nil {
		if xObj.Type().String() == "error" &&
			isUntypedNil(yObj) ||
			yObj.Type().String() == "error" &&
				isUntypedNil(xObj) {
			return true
		}
	}
	return false
}

func isUntypedNil(candidate types.Object) bool {
	// TODO can candidate.Type() return nil?
	basicType, ok := candidate.Type().(*types.Basic)
	if ok {
		if basicType.Kind() == types.UntypedNil {
			return true
		}
	}
	return false
}

func IsErrorHandlingCode(n ast.Node, info *types.Info) (bool, *ast.BlockStmt) {
	ret, ok := n.(*ast.IfStmt)
	if ok {

		//fmt.Printf("if statement found on line %d:\n\t", fset.Position(ret.Pos()).Line)
		testCond, ok := ret.Cond.(*ast.BinaryExpr)
		// TODO does not cover every error-handling case, but covers most common
		// err != nil or err == nil
		// another tactic could be dataflow following where errors are raised
		if ok {
			if isConditionalErrorHandling(testCond, info) {

				// err != nil
				if testCond.Op == token.NEQ {
					// return the body
					return true, ret.Body

				// err == nil
				} else if testCond.Op == token.EQL {
					if ret.Else != nil {
						ifst, ok := ret.Else.(*ast.IfStmt)
						if ok {
							// err == nil {} else if { ... }
							return IsErrorHandlingCode(ifst, info)
						} else {
							// the else block actually has error handling
							// err == nil {} else { ... }
							if block, ok := ret.Else.(*ast.BlockStmt); ok {
								return true, block
							}
						}
					}
				}
			}
		}
	}

	return false, nil
}

// TODO not sure which one is necessary.
// should rework this one to use one above
// handle by adding if is, ignoring if isn;t
func getErrorBlockNodes(node *ast.File, info *types.Info) []*ast.BlockStmt {

	bodies := []*ast.BlockStmt{}

	ast.Inspect(node, func(n ast.Node) bool {
		// TODO necessary?
		if n == nil {
			return false
		}

		errorHandling, block := IsErrorHandlingCode(n, info)
		if errorHandling && block != nil {
			bodies = append(bodies, block)
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

// check if a call is a time.Sleep
func IsTimeoutCall(call *ast.CallExpr, info *types.Info) bool {
	// don't waste time computing if selector doesn't match
	if fun, ok := call.Fun.(*ast.SelectorExpr); ok {
		funcName := fun.Sel.Name
		if !strings.EqualFold(funcName, "Sleep") {
			return false
		} else {
			return selectorMatchesTimeLibrary(fun, info)
		}
	} else {
		// not a selector expression
		return false
	}

}

func selectorMatchesTimeLibrary(fun *ast.SelectorExpr, info *types.Info) bool {
	const TimePackagePath = "time"
	timeLibraryName := "time"

	// implicits is where you find non-renamed package imports
	for ident, obj := range info.Implicits {
		// if there are any imports here
		if dependency, ok := ident.(*ast.ImportSpec); ok {
			// check that they correspond to path of time package "time"
			objName := obj.Name()
			if dependency.Path.Value == TimePackagePath {
				timeLibraryName = objName
			}
		}
	}

	// otherwise, maybe package was renamed
	for identifiers, obj := range info.Defs {
		if obj != nil {
			if isTimeLibrary(obj.String()) {
				// ideally interchangeable
				timeLibraryName = identifiers.Name
				// timeLibraryName = obj.Name()
			}
		}
	}

	// find the libraryName of the library
	// assuming it's an ident and not another expression
	libraryName, ok := fun.X.(*ast.Ident)
	if ok {
		funcLibrary := libraryName.Name

		// then we compare the library and function call
		if funcLibrary == timeLibraryName {
			return true
		}
	}
	return false
}

// object string apparently resembles "package alias ("time")"
// where alias is user-defined variable
func isTimeLibrary(testString string) bool {
	const timeLibraryObjString = `package ([\w]*) \("time"\)`
	timeLibraryRegex := regexp.MustCompile(timeLibraryObjString)

	return timeLibraryRegex.MatchString(testString)
}


