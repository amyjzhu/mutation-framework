package example

import (
	"testing"

	. "github.com/stretchr/testify/assert"
)

func TestFoo(t *testing.T) {
	//Equal(t, 16, foo()) // this is the correct quantity
	Equal(t, 16, foo())
}
