package distributed

import (
	"go/types"
	"go/ast"
	"github.com/amyjzhu/mutation-framework/mutator"
	"github.com/amyjzhu/mutation-framework/astutil"
	"fmt"
)

func init() {
	mutator.Register("distributed/readzero", MutatorReadZero)
}

// Doesn't have to be inspect; we can wholesale mutate
func MutatorReadZero(pkg *types.Package, info *types.Info, node ast.Node) []mutator.Mutation {

	var newAssign *ast.AssignStmt
	var blockToAugment *ast.BlockStmt
	var oldStmtList []ast.Stmt
	if blocks, ok := node.(*ast.BlockStmt); ok {
		for _, block := range blocks.List {
			newAssign = astutil.IsReadAssignment(block, info)
			if newAssign != nil {
				fmt.Println(newAssign)
				blockToAugment = blocks
				oldStmtList = blocks.List
			}
		}
	}
	// is this node a communication node
	// what type of node is it?
	// swap it with another type

	return []mutator.Mutation{
		mutator.Mutation{
			Change: func() {
				if newAssign != nil {
					fmt.Println("Appending n I think", newAssign)
					blockToAugment.List = append(oldStmtList, newAssign)
					fmt.Println(blockToAugment)
				}
			},
			Reset: func() {
				if newAssign != nil && blockToAugment != nil {
					blockToAugment.List = oldStmtList
				}
			},
		},
	}
}

