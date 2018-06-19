package statement

import (
	"testing"

	"github.com/zimmski/go-mutesting/test"
)

func TestMutatorRemoveTimeout(t *testing.T) {
	test.Mutator(
		t,
		MutatorTimeout,
		"../../testdata/statement/remove_timeout_many.go",
		4,
	)

	test.Mutator(
		t,
		MutatorTimeout,
		"../../testdata/statement/remove.go",
		0,
	)
}
