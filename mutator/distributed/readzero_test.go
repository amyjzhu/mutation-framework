package distributed

import (
	"testing"
	"github.com/amyjzhu/mutation-framework/test"
)

func TestMutatorReadZero(t *testing.T) {
	test.Mutator(
		t,
		MutatorReadZero,
		"../../testdata/astutil/assign.go",
		1,
	)
}
