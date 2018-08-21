package distributed

import (
	"go/ast"
	"go/types"

	"github.com/amyjzhu/mutation-framework/mutator"
)

func init() {
	mutator.Register("statement/timeout", MutatorTimeout)
}

// Doesn't have to be inspect; we can wholesale mutate
func MutatorTimeout(pkg *types.Package, info *types.Info, node ast.Node) []mutator.Mutation {

	// is this node a communication node
	// what type of node is it?
	// swap it with another type

	return []mutator.Mutation{
		mutator.Mutation{
			Change: func() {
			},
			Reset: func() {
			},
		},
	}
}


