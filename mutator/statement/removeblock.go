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

	delete, stmts := astutil.IsErrorHandlingCode(node, info)
	if delete {
		mutations = MutatorRemoveStatement(pkg, info, stmts)
	}

	return mutations
}


