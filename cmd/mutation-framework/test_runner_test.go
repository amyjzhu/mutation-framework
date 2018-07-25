package main

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestIsMutant(t *testing.T) {
	validMutants := []string{"nsqd.go.branch-if.1",
		"blah.blag.nldfjsd.go.statement-remove.2",
		"-.go.a.1239048",
	}

	invalidMutants := []string{"nsqd.branch-if.1",
	"nsqd.branch-else.go", "nsqd.go.expression-remove",
	".go.branch-remove.1"}

	for _, mutantPath := range validMutants {
		assert.True(t, isMutant(mutantPath))
	}

	for _, mutantPath := range invalidMutants {
		assert.False(t, isMutant(mutantPath))
	}
}

func TestAppendFolder(t *testing.T) {
	assert.Equal(t,"folder", appendFolder("", "folder"))
	assert.Equal(t,"/folder", appendFolder("", "/folder"))
	assert.Equal(t,"/folder", appendFolder("/", "folder"))
	assert.Equal(t,"/folder", appendFolder("/", "/folder"))
}