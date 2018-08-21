package main

import (
	"github.com/amyjzhu/mutation-framework/osutil"
	"path/filepath"
	"strings"
	"os"
	"fmt"
	"github.com/spf13/afero"
	log "github.com/sirupsen/logrus"
)

func copyProject(config *MutationConfig, name string) (absoluteMutantPath string, err error) {
	log.WithField("mutant", name).Debug("Copying into mutants folder.")
	dir, err := os.Getwd()
	if err != nil {
		panic (err)
	}

	projectName := appendFolder(getAbsoluteMutationFolderPath(config), name)

	return projectName,
		copyRecursive(config.Mutate.Overwrite, dir, projectName, config.Mutate.MutantFolder)
}

func copyRecursive(overwrite bool, source string, dest string, mutantFolder string) error {
	destFile, err := FS.Open(dest)
	// did we get an error opening destination file?
	if err != nil {
		// we got an error, but not the expected one
		if !os.IsNotExist(err) {
			log.WithFields(log.Fields{"file": source, "error": err}).Info("Some error arose.")
			return err
		}
	} else {
		// a file must already exist at this path
		if overwrite {
			log.Debug("Overwriting destination mutants if they already exist.")
		} else {
			return fmt.Errorf("this action would overwrite %s", destFile)
		}
	}

	if destFile != nil {
		destFile.Close()
	}

	// source file must exist
	file, err := FS.Stat(source)
	if err != nil {
		// TODO is this right?
		return err
	}

	if file.IsDir() {
		err = FS.MkdirAll(dest, file.Mode())
		if err != nil {
			return err
		}

		// get all files in source directory
		files, err := afero.ReadDir(FS,source)
		if err != nil {
			return err
		}

		for _, entry := range files {
			newSource := appendFolder(source, entry.Name())
			newDest := appendFolder(dest, entry.Name())

			if entry.IsDir() {
				if doNotCopyDir(entry.Name(), mutantFolder) {
					// avoid recursively copying mutant directory into new directory
					continue
				}

				err = copyRecursive(overwrite, newSource, newDest, mutantFolder)
				if err != nil {
					return err
				}
			} else {
				err = osutil.AferoCopyFile(FS, newSource, newDest)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func doNotCopyDir(dir string, innerFolder string) bool {
	// don't copy git information or mutant folders
	return dir == filepath.Clean(innerFolder) ||
		dir == ".git" || //dir.Name() == filepath.Base(innerFolder)
		dir == getFirstElementInPath(innerFolder)
}


// Returns the first "meaningful piece" of the path
func getFirstElementInPath(path string) string {
	if path == "/" {
		return path
	}

	elts := strings.Split(path, string(os.PathSeparator))
	if len(elts) > 1 {
		if elts[0] == "" { // TODO assuming not //folder
			return elts[1]
		}
		return elts[0]
	} else {
		return path
	}
}

// Copy contents of one folder to another, ignoring mutants folder
func moveAllContentsExceptMutantFolder(sourceFolder string, dest string, mutantFolder string) error {
	log.WithFields(log.Fields{"source": sourceFolder, "dest": dest}).Info("Moving contents of folder.")
	isNotMutantsFolder := func(name string) bool {
		if strings.Contains(filepath.Clean(mutantFolder), string(os.PathSeparator)) {
			// Get first part of path, which matches what you see in directory
			// TODO absolute // paths
			mutantFolder = strings.Split(mutantFolder, string(os.PathSeparator))[0]
		}

		return !(strings.Contains(name, mutantFolder) || strings.Contains(mutantFolder, name))
	}

	return copyFolderContents(sourceFolder, dest, isNotMutantsFolder)
}

// Copy all the contents of one folder to another folder i.e. cp src/* dest/
// Creates destination if doesn't exist
func copyFolderContents(sourceFolder string, destFolder string, pred func(name string) bool) error {
	itemsToMove, err := afero.ReadDir(FS, sourceFolder)
	if err != nil {
		log.Error(err)
		return err
	}

	// TODO not sure what permissions
	err = FS.MkdirAll(destFolder, os.FileMode(0700))
	if err != nil {
		log.Error(err)
		return err
	}

	for _, item := range itemsToMove {
		itemPath := appendFolder(sourceFolder, item.Name())
		newItemPath:= appendFolder(destFolder, item.Name())
		if pred(item.Name()) {
			if item.IsDir() {
				copyFolderContents(itemPath, newItemPath, pred)
			} else {
				err = osutil.AferoCopyFile(FS, itemPath, newItemPath)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}