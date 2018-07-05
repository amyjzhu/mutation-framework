package statement

import (
	"go/ast"
	"go/types"

	"github.com/amyjzhu/mutation-framework/mutator"
	"go/token"
	"fmt"
)

func init() {
	mutator.Register("statement/timeout", MutatorTimeout)
}

// Doesn't have to be inspect; we can wholesale mutate
func MutatorTimeout(pkg *types.Package, info *types.Info, node ast.Node) []mutator.Mutation {
	n, ok := node.(*ast.CallExpr)
	if !ok {
		return nil
	}

	if (!isThisATimeoutFunction(n)) {
		return nil
	}

	oldTimeout := n.Args

	return []mutator.Mutation{
		mutator.Mutation{
			Change: func() {
				zeroTimeout := &ast.BasicLit{Kind:token.INT, Value:"0"}
				n.Args = []ast.Expr{zeroTimeout}
			},
			Reset: func() {
				n.Args = oldTimeout
			},
		},
	}
}

func isThisATimeoutFunction(call *ast.CallExpr) bool {
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
