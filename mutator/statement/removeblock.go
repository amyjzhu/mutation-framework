package statement

import (
	"go/ast"
	"go/types"

	"github.com/amyjzhu/mutation-framework/mutator"
	"github.com/amyjzhu/mutation-framework/astutil"
)

func init() {
	mutator.Register("statement/removeblock", MutatorRemoveBlock)
}

// Doesn't have to be inspect; we can wholesale mutate
func MutatorRemoveBlock(pkg *types.Package, info *types.Info, node ast.Node) []mutator.Mutation {

	var mutations []mutator.Mutation

	shouldDelete, statements := astutil.IsErrorHandlingCode(node, info)
	if shouldDelete {
		mutations = MutatorRemoveStatement(pkg, info, statements)
	}

	return mutations
}


