package main

import (
	"github.com/spf13/afero"
	"os"
	"path/filepath"
	"fmt"
	"strings"
	"go/types"
	"go/token"
	"go/ast"
	"github.com/amyjzhu/mutation-framework/mutator"
	"github.com/amyjzhu/mutation-framework"
	"github.com/amyjzhu/mutation-framework/osutil"
	log "github.com/sirupsen/logrus"
)

type MutantInfo struct {
	pkg                      *types.Package
	originalFileRelativePath string
	mutantDirPathAbsPath     string
	mutationFileAbsPath      string
	checksum                 string
}

func mutateFiles(config *MutationConfig, files map[string]string, operators []mutator.Mutator) (map[string]*mutationStats, []MutantInfo, int) {
	log.Info("Mutating files.")
	allStats := make(map[string]*mutationStats)
	var mutantInfos []MutantInfo

	for relativeFileLocation, abs := range files {
		stats := &mutationStats{}
		allStats[relativeFileLocation] = stats
		log.WithField("file", relativeFileLocation).Debug("Mutating file.")

		src, fset, pkg, info, err := mutesting.ParseAndTypeCheckFile(abs)
		if err != nil {
			log.WithField("file", abs).Error("There was an error compiling the file.")
			return nil, nil, exitError(err.Error())
		}

		err = fs.MkdirAll(config.Mutate.MutantFolder, 0755)
		if err != nil {
			panic(err)
		}

		mutantFile := config.Mutate.MutantFolder + relativeFileLocation
		createMutantFolderPath(mutantFile)

		mutationID := 0

		// TODO match function names instead
		mutantInfo := mutate(config, mutationID, pkg, info, abs, relativeFileLocation,
			fset, src, src, stats)

		mutantInfos = append(mutantInfos, mutantInfo...)
	}

	return allStats, mutantInfos, returnOk
}

func createMutantFolderPath(file string) {
	if strings.Contains(file, string(os.PathSeparator)) {
		parentPath := filepath.Dir(file)
		err := fs.MkdirAll(parentPath, 0755)
		if err != nil {
			panic(err)
		}
	}
}

func mutate(config *MutationConfig, mutationID int, pkg *types.Package,
	info *types.Info, file string, relativeFilePath string, fset *token.FileSet,
	src ast.Node, node ast.Node, stats *mutationStats) []MutantInfo {

	var mutantInfos []MutantInfo

	for _, m := range config.Mutate.Operators {
		mutationID = 0 // reset the mutationid for each operator
		log.WithField("mutation_operator", m.Name).Info("Mutating.")

		changed := mutesting.MutateWalk(pkg, info, node, *m.MutationOperator)

		for {
			_, ok := <-changed

			if !ok {
				break
			}

			mutationBlackList := make(map[string]struct{},0) //TODO implement real blacklisting

			safeMutationName := strings.Replace(m.Name, string(os.PathSeparator), "-", -1)
			mutationFileId := fmt.Sprintf("%s.%s.%d", relativeFilePath, safeMutationName, mutationID)
			log.WithField("name", mutationFileId).Info("Creating mutant.")

			mutantPath, err := copyProject(config, mutationFileId) // TODO verify correctness of absolute file
			if err != nil {
				log.WithField("error", err).Error("Internal error.")
			}

			mutatedFilePath := filepath.Clean(mutantPath) + string(os.PathSeparator) + relativeFilePath
			checksum, duplicate, err := saveAST(mutationBlackList, mutatedFilePath, fset, src)

			if err != nil {
				log.WithField("error", err).Error("Internal error.")
			} else if duplicate {
				log.WithField("mutant", mutatedFilePath).Debug("Ignoring duplicate.")
				stats.duplicated++
			} else {
				log.WithFields(
					log.Fields{"mutant": mutatedFilePath, "checksum": checksum}).
					Debug("Saving mutated file.")

				mutantInfo := MutantInfo{pkg, relativeFilePath,
					filepath.Clean(mutantPath),
					mutatedFilePath, checksum}
				mutantInfos = append(mutantInfos, mutantInfo)
			}

			changed <- true

			// Ignore original state
			<-changed
			changed <- true

			mutationID++
		}
	}
	return mutantInfos
}


func copyProject(config *MutationConfig, name string) (string, error) {
	log.WithField("mutant", name).Debug("Copying into mutants folder.")
	dir, err := os.Getwd()
	if err != nil {
		panic (err)
	}

	projectName := config.FileBasePath + config.Mutate.MutantFolder + name

	return projectName,
		copyRecursive(config.Mutate.Overwrite, dir, projectName, config.Mutate.MutantFolder)
}

func copyRecursive(overwrite bool, source string, dest string, mutantFolder string) error {
	destFile, err := fs.Open(dest)
	if !os.IsNotExist(err) {
		if overwrite {
			log.Debug("Overwriting destination mutants if they already exist.")
		} else if err != nil {
			log.WithFields(log.Fields{"file":source, "error":err}).Info("Some error arose.")
			return fmt.Errorf("source file %s does not exist", source)
		}
	}

	if destFile != nil {
		destFile.Close()
	}

	file, err := fs.Stat(source)

	if err != nil {
		// TODO is this right?
		return err
	}

	if file.IsDir() {
		err = fs.MkdirAll(dest, file.Mode())
		if err != nil {
			return err
		}

		files, err := afero.ReadDir(fs,source)
		if err != nil {
			return err
		}

		for _, entry := range files {
			newSource := appendFolder(source, entry.Name())
			newDest := appendFolder(dest, entry.Name())

			if entry.IsDir() {
				if doNotCopyDir(entry, mutantFolder) {
					// avoid recursively copying mutant directory into new directory
					continue
				}

				err = copyRecursive(overwrite, newSource, newDest, mutantFolder)
				if err != nil {
					return err
				}
			} else {
				err = osutil.CopyFile(newSource, newDest)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func doNotCopyDir(dir os.FileInfo, innerFolder string) bool {
	return dir.Name() == filepath.Clean(innerFolder) || dir.Name() == ".git"
}