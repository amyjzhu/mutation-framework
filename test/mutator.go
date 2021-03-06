package test

import (
	"bytes"
	"fmt"
	"go/printer"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/amyjzhu/mutation-framework"
	"github.com/amyjzhu/mutation-framework/mutator"
	"os"
)

// Mutator tests a mutator.
// It mutates the given original file with the given mutator. Every mutation is then validated with the given changed file. The mutation overall count is validated with the given count.
func Mutator(t *testing.T, m mutator.Mutator, testFile string, count int) {
	// Test if mutator is not nil
	assert.NotNil(t, m)

	// Read the origianl source code
	originalSrcData, err := ioutil.ReadFile(testFile)
	assert.Nil(t, err)

	// Parse and type-check the original source code
	src, fset, pkg, info, err := mutesting.ParseAndTypeCheckFile(testFile)
	assert.Nil(t, err)

	// Mutate a non relevant node
	assert.Nil(t, m(pkg, info, src))

	// Count the actual mutations
	n := mutesting.CountWalk(pkg, info, src, m)
	assert.Equal(t, count, n)

	// Mutate all relevant nodes -> test whole mutation process
	changed := mutesting.MutateWalk(pkg, info, src, m)

	for i := 0; i < count; i++ {
		assert.True(t, <-changed)

		buf := new(bytes.Buffer)
		err = printer.Fprint(buf, fset, src)
		assert.Nil(t, err)

		// If this file isn't written, it breaks somehow
		// and it doesn't work currently
		changedFilename := fmt.Sprintf("%s.%d.go", testFile, i)
		_, err := os.Stat(changedFilename)
		assert.Nil(t, err)

		changedFile, err := ioutil.ReadFile(changedFilename)
		assert.Nil(t, err)

		if !assert.Equal(t, string(changedFile), buf.String(), fmt.Sprintf("For change file %q", changedFilename)) {
			err = ioutil.WriteFile(fmt.Sprintf("%s.%d.go.new", testFile, i), []byte(buf.String()), 0644)
			assert.Nil(t, err)
		}

		changed <- true

		assert.True(t, <-changed)

		buf = new(bytes.Buffer)
		err = printer.Fprint(buf, fset, src)
		assert.Nil(t, err)

		assert.Equal(t, string(originalSrcData), buf.String())

		changed <- true
	}

	_, ok := <-changed
	assert.False(t, ok)
}
