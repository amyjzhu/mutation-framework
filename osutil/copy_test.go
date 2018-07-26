package osutil

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/spf13/afero"
)

func TestCopyFile(t *testing.T) {
	src := "copy.go"
	dst := "copy.go.tmp"

	err := CopyFile(src, dst)
	assert.Nil(t, err)

	s, err := ioutil.ReadFile(src)
	assert.Nil(t, err)

	d, err := ioutil.ReadFile(dst)
	assert.Nil(t, err)

	assert.Equal(t, s, d)

	err = os.Remove(dst)
	assert.Nil(t, err)
}

var fs = afero.NewMemMapFs()

func TestAferoCopyFile(t *testing.T) {
	src := "copy.go"
	dst := "copy.go.tmp"

	file, err := fs.Create(src)
	assert.Nil(t, err)
	file.Close()

	err = AferoCopyFile(fs, src, dst)
	assert.Nil(t, err)

	s, err := afero.ReadFile(fs, src)
	assert.Nil(t, err)

	d, err := afero.ReadFile(fs, dst)
	assert.Nil(t, err)

	assert.Equal(t, s, d)

	err = fs.Remove(dst)
	assert.Nil(t, err)
}

