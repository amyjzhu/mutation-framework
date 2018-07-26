package osutil

import (
	"io"
	"os"
	"fmt"
	"github.com/spf13/afero"
)

// CopyFile copies a file from src to dst
// Forked from github.com/zimmski/osutil because that package does not compile on Go 1.10
func CopyFile(src string, dst string) (err error) {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		e := s.Close()
		if err == nil {
			err = e
		}
	}()

	d, err := os.Create(dst)
	if err != nil {
		fmt.Println("3")
		return err
	}
	defer func() {
		e := d.Close()
		if err == nil {
			err = e
		}
	}()

	_, err = io.Copy(d, s)
	if err != nil {
		return err
	}

	i, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, i.Mode())
}

func AferoCopyFile(fs afero.Fs, src string, dst string) (err error) {
	s, err := fs.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		e := s.Close()
		if err == nil {
			err = e
		}
	}()

	d, err := fs.Create(dst)
	if err != nil {
		fmt.Println("3")
		return err
	}
	defer func() {
		e := d.Close()
		if err == nil {
			err = e
		}
	}()

	_, err = io.Copy(d, s)
	if err != nil {
		return err
	}

	i, err := fs.Stat(src)
	if err != nil {
		return err
	}

	return fs.Chmod(dst, i.Mode())
}
