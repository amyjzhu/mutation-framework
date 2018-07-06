package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"github.com/amyjzhu/mutation-framework/mutator"
)

// test that configs are properly loaded

func testConfigLoading() {
	// read all files in testdata/config/
	//
}

func TestSimpleConfig(t *testing.T) {
	expectedMutator, err := mutator.New("statement/remove")
	assert.Nil(t, err)

	expectedConfig := &MutationConfig{
		 []Operator{Operator{&expectedMutator, "statement/remove"}},
		[]string{"primary.go", "secondary.go"},
		[]string{},
		Options{1, false}}

	expectedString, err := expectedConfig.getString()
	assert.Nil(t, err)


	testFile := "../../testdata/config/sample_config.json"
	actualConfig, err := getConfig(testFile)
	assert.Nil(t, err)

	actualString, err := actualConfig.getString()
	assert.Nil(t, err)

	assert.Equal(t, expectedString, actualString)
}
