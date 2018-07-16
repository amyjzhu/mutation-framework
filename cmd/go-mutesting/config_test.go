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

// TODO it should be that empty slices are initialized instead of null
func initialize() {

	expectedMutator = mockMutator
	mutator.Register("mutator/mock", expectedMutator)

	expectedConfig = MutationConfig{
		[]Operator{Operator{&expectedMutator, "mutator/mock"}},
		[]string{"primary.go", "secondary.go"},
		//[]string{},
		nil,
		"",
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

// TODO tests are highly dependent on file structure of OS
// TODO get working directory perhaps
func TestWildcardConfig(t *testing.T) {
	wildcardConfig, err:= getConfig("../../testdata/config/wildcard_config.json")
	assert.Nil(t, err)

	expectedIncludedFiles := []string{
		"/home/amy/go/src/github.com/amyjzhu/mutation-framework/cmd/go-mutesting/config.go",
		"/home/amy/go/src/github.com/amyjzhu/mutation-framework/cmd/go-mutesting/config_test.go",
		"/home/amy/go/src/github.com/amyjzhu/mutation-framework/cmd/go-mutesting/main.go",
		"/home/amy/go/src/github.com/amyjzhu/mutation-framework/cmd/go-mutesting/main_test.go"}

	assert.ElementsMatch(t, wildcardConfig.FilesToInclude, expectedIncludedFiles)

	expectedExcludedFiles := []string{
		"/home/amy/go/src/github.com/amyjzhu/mutation-framework/mutator/mutation.go",
		"/home/amy/go/src/github.com/amyjzhu/mutation-framework/mutator/mutator.go",
		"/home/amy/go/src/github.com/amyjzhu/mutation-framework/mutator/mutator_test.go",
		"/home/amy/go/src/github.com/amyjzhu/mutation-framework/mutator/expression/remove.go",
		"/home/amy/go/src/github.com/amyjzhu/mutation-framework/mutator/statement/remove.go"}

	assert.ElementsMatch(t, wildcardConfig.FilesToExclude, expectedExcludedFiles)
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

func TestConcatAndAddSlashIfNeeded(t *testing.T) {
	assert.Equal(t, "hello/world", concatAddingSlashIfNeeded("hello", "world"))
	assert.Equal(t, "hello/world", concatAddingSlashIfNeeded("hello/", "world"))
	assert.Equal(t, "hello/world", concatAddingSlashIfNeeded("hello/", "/world"))
	assert.Equal(t, "hello/world", concatAddingSlashIfNeeded("hello", "/world"))
}

func getIsValidWildCardValue(piece string) bool {
	result, _ := isValidWildCard(piece)
	return result
}

func TestIsValidWildCard(t *testing.T) {
	assert.True(t, getIsValidWildCardValue("dsjfsd*.go"))
	assert.True(t, getIsValidWildCardValue("*"))
	assert.True(t, getIsValidWildCardValue("*_test"))
	assert.False(t, getIsValidWildCardValue("*_test**"))
	assert.False(t, getIsValidWildCardValue("fskjdfsdf"))
}

func TestGetParentDirectory(t *testing.T) {
	pathPieces := []string{"apple", "banana", "watermelon", "chive"}
	actualPath := getParentPath(pathPieces, 1)
	expectedPath := "apple/"
	assert.Equal(t, expectedPath, actualPath)

	actualPath = getParentPath(pathPieces, 2)
	expectedPath = "apple/banana/"
	assert.Equal(t, expectedPath, actualPath)


	actualPath = getParentPath(pathPieces, 3)
	expectedPath = "apple/banana/watermelon/"
	assert.Equal(t, expectedPath, actualPath)


	actualPath = getParentPath(pathPieces, -1)
	expectedPath = ""
	assert.Equal(t, expectedPath, actualPath)

	actualPath = getParentPath(pathPieces, 0)
	expectedPath = ""
	assert.Equal(t, expectedPath, actualPath)
}