package expression

import (
	"testing"

	"github.com/amyjzhu/mutation-framework/test"
)

func TestMutatorRemoveTerm(t *testing.T) {
	test.Mutator(
		t,
		MutatorRemoveTerm,
		"../../testdata/expression/remove.go",
		6,
	)
}
