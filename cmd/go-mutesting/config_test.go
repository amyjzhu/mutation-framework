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
		false,
		false,
		"/home/",
		Mutate{false, []Operator{Operator{&expectedMutator, "mutator/mock"}},
			[]string{"primary.go", "secondary.go"},
			//[]string{},
			nil,"mutants/",
			false},
		Test{false, 10, 1},
		Commands{"go test", "", ""}}
}

func TestJsonConfig(t *testing.T) {
	initialize()
	expectedString, err := expectedConfig.toString()
	assert.Nil(t, err)


	actualConfig, err := getConfig(testFile)
	assert.Nil(t, err)

	actualString, err := actualConfig.toString()
	assert.Nil(t, err)

	assert.Equal(t, expectedString, actualString)
}

// TODO fix null cases where one key isn't included, .e.g "files to exclude" is completely omitted
func TestYamlConfig(t *testing.T) {
	actualConfig, err := getConfig(testFileYaml)
	assert.Nil(t, err)

	actualString, err := actualConfig.toString()
	assert.Nil(t, err)

	expectedString, err := expectedConfig.toString()
	assert.Nil(t, err)

	assert.Equal(t, expectedString, actualString)
}

// TODO tests are highly dependent on file structure of OS
// TODO get working directory perhaps
func TestWildcardConfig(t *testing.T) {
	wildcardConfig, err:= getConfig("../../testdata/config/wildcard_config.json")
	assert.Nil(t, err)

	expectedIncludedFiles := []string{
		"cmd/go-mutesting/config.go",
		"cmd/go-mutesting/config_test.go",
		"cmd/go-mutesting/main.go",
		"cmd/go-mutesting/main_test.go",
		"cmd/go-mutesting/mutate.go",
		"cmd/go-mutesting/test_runner.go",
		"cmd/go-mutesting/test_runner_test.go"}

	assert.ElementsMatch(t, wildcardConfig.Mutate.FilesToInclude, expectedIncludedFiles)

	expectedExcludedFiles := []string{
		"/mutator/mutation.go",
		"/mutator/mutator.go",
		"/mutator/mutator_test.go",
		"/mutator/expression/remove.go",
		"/mutator/statement/remove.go"}

	assert.ElementsMatch(t, wildcardConfig.Mutate.FilesToExclude, expectedExcludedFiles)
}

func TestMinimalParseConfig(t *testing.T) {
	configString := `{"project_root":"home"}`

	actualConfig, err := parseConfig([]byte(configString))
	assert.Nil(t, err)

	expectedConfig = MutationConfig{FileBasePath:"home/",
	Mutate:Mutate{MutantFolder:"mutants/"}}
	assert.EqualValues(t, *actualConfig, expectedConfig)
}

func TestDefaultMutantFunctionality(t *testing.T) {
	//initialize()
	expectedConfig.Mutate.MutantFolder = ""

	appendMutantFolderSlashOrReplaceWithDefault(&expectedConfig)
	assert.Equal(t, DefaultMutationFolder, expectedConfig.Mutate.MutantFolder)
}

func TestMutantFolderFormatting(t *testing.T) {
	//initialize()
	expectedConfig.Mutate.MutantFolder = "deep fried pickles"

	appendMutantFolderSlashOrReplaceWithDefault(&expectedConfig)
	assert.Equal(t, "deep fried pickles/", expectedConfig.Mutate.MutantFolder)


	expectedConfig.Mutate.MutantFolder = "deep fried pickles/"

	appendMutantFolderSlashOrReplaceWithDefault(&expectedConfig)
	assert.Equal(t, "deep fried pickles/", expectedConfig.Mutate.MutantFolder)
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

func TestFilterFileNames(t *testing.T) {
	patternWithGlob := "foo*"
	candidateFiles := []string{"foobar", "foo%", "foo-", "foo.jpg", "bar.jpg",
	"baz", "baz*", "foo*", "foo....", "maryfoo"}

	expectedFiles := []string{"foobar", "foo-", "foo.jpg", "foo...."}
	actualFiles := filterFileNames(patternWithGlob, candidateFiles)

	assert.ElementsMatch(t, expectedFiles, actualFiles)
	assert.NotContains(t, expectedFiles, []string{"maryfoo", "bar.jpg", "baz*", "baz"})
}