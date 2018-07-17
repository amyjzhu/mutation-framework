package main

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"github.com/spf13/afero"
)

func TestMain(t *testing.T) {
	testMain(
		t,
		"../../example",
		[]string{"--debug", "--exec-timeout", "1"},
		returnOk,
		"The mutation score is 0.500000 (8 passed, 8 failed, 8 duplicated, 0 skipped, total is 16)",
	)
}

func TestMainRecursive(t *testing.T) {
	testMain(
		t,
		"../../example",
		[]string{"--debug", "--exec-timeout", "1", "./..."},
		returnOk,
		"The mutation score is 0.529412 (9 passed, 8 failed, 8 duplicated, 0 skipped, total is 17)",
	)
}

func TestMainFromOtherDirectory(t *testing.T) {
	testMain(
		t,
		"../..",
		[]string{"--debug", "--exec-timeout", "1", "github.com/amyjzhu/mutation-framework/example"},
		returnOk,
		"The mutation score is 0.500000 (8 passed, 8 failed, 8 duplicated, 0 skipped, total is 16)",
	)
}

func TestMainMatch(t *testing.T) {
	testMain(
		t,
		"../../example",
		[]string{"--debug", "--exec", "../scripts/exec/test-mutated-package.sh", "--exec-timeout", "1", "--match", "baz", "./..."},
		returnOk,
		"The mutation score is 0.500000 (1 passed, 1 failed, 0 duplicated, 0 skipped, total is 2)",
	)
}

func testMain(t *testing.T, root string, exec []string, expectedExitCode int, contains string) {
	saveStderr := os.Stderr
	saveStdout := os.Stdout
	saveCwd, err := os.Getwd()
	assert.Nil(t, err)

	r, w, err := os.Pipe()
	assert.Nil(t, err)

	os.Stderr = w
	os.Stdout = w
	assert.Nil(t, os.Chdir(root))

	bufChannel := make(chan string)

	go func() {
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, r)
		assert.Nil(t, err)
		assert.Nil(t, r.Close())

		bufChannel <- buf.String()
	}()

	exitCode := mainCmd(exec)

	assert.Nil(t, w.Close())

	os.Stderr = saveStderr
	os.Stdout = saveStdout
	assert.Nil(t, os.Chdir(saveCwd))

	out := <-bufChannel

	assert.Equal(t, expectedExitCode, exitCode)
	assert.Contains(t, out, contains)
}

// TODO file paths
// TODO should write more tests with mocks or abstract file systems (afero?)
func TestCopy(t *testing.T) {
	fs = afero.NewMemMapFs()

	testFileParent := "/tmp/mutation-testing/"
	testFile := testFileParent + "testcopy/"
	// No idea what permission mode this is
	err := fs.MkdirAll(testFile, os.FileMode(077))
	assert.Nil(t, err)

	mutationFolder := "copied-mutants/"
	mutationPath := testFileParent + mutationFolder
	defer fs.RemoveAll(testFileParent)

	sample, err := fs.Create(testFile + "sample.txt")
	assert.Nil(t, err)
	fs.Mkdir(testFile + ".git", os.FileMode(077))
	defer sample.Close()

	err = copy(true, testFileParent, mutationPath, mutationFolder)
	assert.Nil(t, err)

	entries, err := ioutil.ReadDir(mutationFolder)
	assert.Nil(t, err)

	testFolderExists := false
	sampleFileExists := false
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
			moreEntries, err := ioutil.ReadDir(testFile)
			assert.Nil(t, err)
			assert.Equal(t, moreEntries[0].Name(), "sample.txt")
		}
	}

	assert.True(t, testFolderExists)
	assert.True(t, sampleFileExists)
	assert.True(t, gitDoesNotExist)
	assert.True(t, mutantFolderDoesNotExist)

	// entries should contain copied-mutants
	// inside that should be testcopy/sample.txt
	// there should not be git inside testcopy/

}
