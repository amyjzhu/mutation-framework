package distributed

import (
	"go/ast"
	"go/types"

	"github.com/amyjzhu/mutation-framework/mutator"
	"github.com/amyjzhu/mutation-framework/astutil"
)

func init() {
	mutator.Register("distributed/protocols", MutatorSwap)
}

func MutatorSwap(pkg *types.Package, info *types.Info, node ast.Node) []mutator.Mutation {
	// TODO better separation of concerns
	vert, revert := astutil.SwapProtocolVersion(node, info)

	return []mutator.Mutation{
		mutator.Mutation{
			Change: vert,
			Reset: revert,
		},
	}
}


