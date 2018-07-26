package main

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/spf13/afero"
	"os"
)

func TestDoNotCopyDir (t *testing.T) {
	assert.True(t, doNotCopyDir("world", "world"))
	assert.True(t, doNotCopyDir(".git", ".git"))
	assert.True(t, doNotCopyDir("mutants", "/mutants/"))
	assert.True(t, doNotCopyDir("mutants", "mutants/"))
	assert.True(t, doNotCopyDir("mutants", "mutants"))
	assert.True(t, doNotCopyDir("mutants", "mutants/other/stuff"))

	assert.False(t, doNotCopyDir("worldwide", "world"))
	assert.False(t, doNotCopyDir("ball python", "world"))
}

func TestGetFirstElementInPath(t *testing.T) {
	assert.Equal(t, "hello", getFirstElementInPath("/hello/world"))
	assert.Equal(t, "hello", getFirstElementInPath("hello/world"))
	assert.Equal(t, "world", getFirstElementInPath("world"))
	assert.Equal(t, "world", getFirstElementInPath("/world"))
	assert.Equal(t, "/", getFirstElementInPath("/"))
}

func TestCopy(t *testing.T) {
	fs = afero.NewMemMapFs()

	testFileParent := "/tmp/mutation-testing/"
	testFile := appendFolder(testFileParent, "testcopy/")
	// No idea what permission mode this is
	err := fs.MkdirAll(testFile, os.FileMode(700))
	assert.Nil(t, err)

	mutationFolder := "copied-mutants/"
	mutationPath := appendFolder(testFileParent, mutationFolder)
	defer fs.RemoveAll(testFileParent)

	// original file is /tmp/mutation-testing/testcopy/sample.txt
	// new file is /tmp/mutation-testing/copied-mutatns/testcopy/sample.txt
	sample, err := fs.Create(testFile + "sample.txt")
	assert.Nil(t, err)
	fs.Mkdir(testFile + ".git", os.FileMode(700))
	defer sample.Close()

	err = copyRecursive(true, testFileParent, mutationPath, mutationFolder)
	assert.Nil(t, err)

	// read from /tmp/mutation-testing/copied-mutants
	entries, err := afero.ReadDir(fs, mutationPath)
	assert.Nil(t, err)

	testFolderExists := false
	gitDoesNotExist := true
	mutantFolderDoesNotExist := true
	for _, entry := range entries {
		switch entry.Name() {
		case "testcopy":
			testFolderExists = true
			break
		case mutationFolder:
			mutantFolderDoesNotExist = false
			break
		case ".git":
			gitDoesNotExist = false
			break
		}

		if entry.Name() == "testcopy" {
			moreEntries, err := afero.ReadDir(fs, mutationPath + "testcopy")
			assert.Nil(t, err)
			assert.Equal(t, "sample.txt", moreEntries[0].Name())
		}
	}

	assert.True(t, testFolderExists)
	assert.True(t, gitDoesNotExist)
	assert.True(t, mutantFolderDoesNotExist)

	// entries should contain copied-mutants
	// inside that should be testcopy/sample.txt
	// there should not be git inside testcopy/
}

func TestCopyWithoutOverwrite(t *testing.T) {
	fs = afero.NewMemMapFs()
	testFileParent := "/tmp/mutation-testing/"

	destination := appendFolder(testFileParent, "mutant")

	err := fs.MkdirAll(destination, os.FileMode(777)) // live dangerously
	assert.Nil(t, err)

	err = copyRecursive(false, testFileParent, destination, "mutant")
	assert.NotNil(t, err)
}

func TestSimpleCopyFolderContents(t *testing.T) {
	fs = afero.NewMemMapFs()
	files := []string{"soba", "passionfruit", "udon", "vermicelli", "mango"}
	newFolder := "/newFolder"
	oldFolder := "/oldFolder/innerFolder/"

	// Need to make old folder, but function should handle new folder
	fs.MkdirAll(oldFolder, os.FileMode(700))
	for _, file := range files {
		reiFiled, err := fs.Create(oldFolder + file)
		assert.Nil(t, err)
		reiFiled.Close()
	}

	_, err := fs.Stat(oldFolder)
	assert.Nil(t, err)


	isNoodleNotFruit := func(name string) bool {
		return name == "soba" || name == "udon" || name == "vermicelli"
	}

	copyFolderContents(oldFolder, newFolder, isNoodleNotFruit)
	expectedCopiedFiles := []string{"soba", "udon", "vermicelli"}

	var actualCopiedFiles []string
	copiedFiles, err := afero.ReadDir(fs, newFolder)
	assert.Nil(t, err)

	for _, info := range copiedFiles {
		actualCopiedFiles = append(actualCopiedFiles, info.Name())
	}

	assert.ElementsMatch(t, expectedCopiedFiles, actualCopiedFiles)
}