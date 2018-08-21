package statement

import (
	"testing"

	"github.com/amyjzhu/mutation-framework/test"
)

func TestMutatorRemoveBlock(t *testing.T) {
	test.Mutator(
		t,
		MutatorRemoveBlock,
		"../../testdata/statement/removeblock.go",
		4,
	)
}
