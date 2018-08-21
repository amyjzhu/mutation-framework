package statement

import (
	"go/ast"
	"go/types"

	"github.com/amyjzhu/mutation-framework/mutator"
	"go/token"
	"github.com/amyjzhu/mutation-framework/astutil"
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

	if !astutil.IsTimeoutCall(n, info) {
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


