package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"github.com/amyjzhu/mutation-framework/mutator"
	"go/types"
	"go/ast"
)

// test that configs are properly loaded

func testConfigLoading() {
	// read all files in testdata/config/
	//
}

var expectedConfig MutationConfig
var expectedMutator mutator.Mutator
var testFile = "../../testdata/config/sample_config.json"
var testFileYaml = "../../testdata/config/sample_config.yaml"

func mockMutator(pkg *types.Package, info *types.Info, node ast.Node) []mutator.Mutation {
	// Do nothing

	return nil
}

func initialize() {

	expectedMutator = mockMutator
	mutator.Register("mutator/mock", expectedMutator)

	expectedConfig = MutationConfig{
		[]Operator{Operator{&expectedMutator, "mutator/mock"}},
		[]string{"primary.go", "secondary.go"},
		[]string{},
		Options{1, false, false, false, "mutants/",10},
		Scripts{"go test", ""}}
}

func TestJsonConfig(t *testing.T) {
	initialize()
	expectedString, err := expectedConfig.getString()
	assert.Nil(t, err)


	actualConfig, err := getConfig(testFile)
	assert.Nil(t, err)

	actualString, err := actualConfig.getString()
	assert.Nil(t, err)

	assert.Equal(t, expectedString, actualString)
}

// TODO fix null cases where one key isn't included, .e.g "files to exclude" is completely omitted
func TestYamlConfig(t *testing.T) {
	actualConfig, err := getConfig(testFileYaml)
	assert.Nil(t, err)

	actualString, err := actualConfig.getString()
	assert.Nil(t, err)

	expectedString, err := expectedConfig.getString()
	assert.Nil(t, err)

	assert.Equal(t, expectedString, actualString)
}

func TestWildcardConfig(t *testing.T) {
	wildcardConfig, err:= getConfig("../../testdata/config/wildcard_config.json")
	assert.Nil(t, err)



}

func TestDefaultMutantFunctionality(t *testing.T) {
	//initialize()
	expectedConfig.Options.MutantFolder = ""

	appendMutantFolderSlashOrReplaceWithDefault(&expectedConfig)
	assert.Equal(t, DEFAULT_MUTATION_FOLDER, expectedConfig.Options.MutantFolder)
}

func TestMutantFolderFormatting(t *testing.T) {
	//initialize()
	expectedConfig.Options.MutantFolder = "deep fried pickles"

	appendMutantFolderSlashOrReplaceWithDefault(&expectedConfig)
	assert.Equal(t, "deep fried pickles/", expectedConfig.Options.MutantFolder)


	expectedConfig.Options.MutantFolder = "deep fried pickles/"

	appendMutantFolderSlashOrReplaceWithDefault(&expectedConfig)
	assert.Equal(t, "deep fried pickles/", expectedConfig.Options.MutantFolder)
}
