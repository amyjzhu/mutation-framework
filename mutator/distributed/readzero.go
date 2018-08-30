package distributed

import (
	"go/types"
	"go/ast"
	"github.com/amyjzhu/mutation-framework/mutator"
	"github.com/amyjzhu/mutation-framework/astutil"
)

func init() {
	mutator.Register("distributed/readzero", MutatorReadZero)
}

func MutatorReadZero(pkg *types.Package, info *types.Info, node ast.Node) []mutator.Mutation {
	var mutationList []mutator.Mutation

	if blocks, ok := node.(*ast.BlockStmt); ok {
		for i, block := range blocks.List {
			newAssign := astutil.CreateReadZeroAssignment(block, info)
			if newAssign != nil {
				mutation := createMutant(blocks, blocks.List, newAssign, i+1)
				mutationList = append(mutationList, mutation)
			}
		}
	}

	return mutationList
}

func createMutant(blockToAugment *ast.BlockStmt, oldStmtList []ast.Stmt, newAssign *ast.AssignStmt, index int) mutator.Mutation {
	return mutator.Mutation{
		Change: func() {
			var newList = make([]ast.Stmt, len(oldStmtList))
			copy(newList, oldStmtList)
			// increase size of list/capacity
			newList = append(newList, newAssign)
			// was the initial statement the last one?
			if len(newList) != (index + 1) {
				// if it's in the middle, we need to shift the list
				copy(newList[index+1:], newList[index:])
				newList[index] = newAssign
			}
			blockToAugment.List = newList
		},
		Reset: func() {
			blockToAugment.List = oldStmtList

		},
	}
}
