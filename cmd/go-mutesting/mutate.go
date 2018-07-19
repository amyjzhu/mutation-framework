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
)

type MutantInfo struct {
	pkg *types.Package
	info *types.Info
	originalFile string
	mutantDirPath string
	mutationFile string
	checksum string
}

func mutateFiles(config *MutationConfig, files map[string]string, operators []mutator.Mutator) (*mutationStats, int) {
	stats := &mutationStats{}

	for rel, abs := range files {
		debug(config, "Mutate %q", abs)

		src, fset, pkg, info, err := mutesting.ParseAndTypeCheckFile(abs)
		if err != nil {
			fmt.Printf("There was an error compiling %s. Is the file correct?\n", abs)
			return nil, exitError(err.Error())
		}

		err = fs.MkdirAll(config.Mutate.MutantFolder, 0755)
		if err != nil {
			panic(err)
		}

		mutantFile := config.Mutate.MutantFolder + rel
		createMutantFolderPath(mutantFile)

		mutationID := 0

		// TODO match function names instead
		mutationID = mutate(config, mutationID, pkg, info, abs, rel, fset, src, src, mutantFile, stats)
	}

	return stats, returnOk
}

func createMutantFolderPath(file string) {
	if strings.Contains(file, string(os.PathSeparator)) {
		paths := strings.Split(file, string(os.PathSeparator))
		paths = paths[:len(paths)-1]
		parentPath := strings.Join(paths, string(os.PathSeparator))
		err := fs.MkdirAll(parentPath, 0755)
		if err != nil {
			panic(err)
		}
	}
}

func mutate(config *MutationConfig, mutationID int, pkg *types.Package, info *types.Info, file string, relPath string, fset *token.FileSet, src ast.Node, node ast.Node, tmpFile string, stats *mutationStats) int {
	for _, m := range config.Mutate.Operators {
		debug(config, "Mutator %s", m.Name)

		changed := mutesting.MutateWalk(pkg, info, node, *m.MutationOperator)

		for {
			_, ok := <-changed

			if !ok {
				break
			}

			mutationBlackList := make(map[string]struct{},0) //TODO implement real blacklisting

			//mutationFile := fmt.Sprintf("%s.%d", tmpFile, mutationID)
			mutationFileID := fmt.Sprintf("%s.%d", relPath, mutationID)

			//mutationFile := fmt.Sprintf("%s%s", config.FileBasePath, tmpFile)
			//fmt.Printf("mutationFileID: %s mutationFile:%s\n", mutationFileID, mutationFile)
			mutantPath, err := copyProject(config, mutationFileID) // TODO verify correctness of absolute file
			fmt.Printf("mutant path is %s\n", mutantPath)
			if err != nil {
				fmt.Printf("INTERNAL ERROR %s\n", err.Error())
			}

			mutatedFilePath := filepath.Clean(mutantPath) + "/" + relPath
			checksum, duplicate, err := saveAST(mutationBlackList, mutatedFilePath, fset, src)
			if err != nil {
				fmt.Printf("INTERNAL ERROR %s\n", err.Error())
			} else if duplicate {
				debug(config, "%q is a duplicate, we ignore it", mutatedFilePath)

				stats.duplicated++
			} else {
				debug(config, "Save mutation into %q with checksum %s", mutatedFilePath, checksum)
				mutantInfo := MutantInfo{pkg, info, file, relPath, mutatedFilePath, checksum}
				//runExecution(config, mutantInfo, stats)
				mutantPaths = append(mutantPaths, mutantInfo)
			}

			changed <- true

			// Ignore original state
			<-changed
			changed <- true

			mutationID++
		}
	}

	getRedundantCandidates()
	fmt.Printf("Live muatants: %s\n", liveMutants)
	fmt.Println(testsToMutants)
	return mutationID
}

var mutantPaths []MutantInfo

func copyProject(config *MutationConfig, name string) (string, error) {
	//projectRoot := config.FileBasePath

	debug(config, "copying %s to mutants folder", name)
	dir, err := os.Getwd()
	if err != nil {
		panic (err)
	}
	/*
	pathParts := strings.Split(projectRoot, string(os.PathSeparator))
	// TODO do I even really need pathparts? seems redundant since that's the directory I'm in
	// all projectRoots end with a / so the last element is ""
	// we're interested in the non-empty string part before it, thus index is length - 2
	PathPartPieceIndex := len(pathParts)-2
	projectName := config.FileBasePath + config.Mutate.MutantFolder +
		pathParts[PathPartPieceIndex] + "_" + name*/
	projectName := config.FileBasePath + config.Mutate.MutantFolder + name

	return projectName,
		copyRecursive(config.Mutate.Overwrite, dir, projectName, config.Mutate.MutantFolder)
}

func copyRecursive(overwrite bool, source string, dest string, mutantFolder string) error {
	destFile, err := fs.Open(dest)
	if !os.IsNotExist(err) {
		if overwrite {
			fmt.Println("Overwriting destination mutants if they already exist.")
		} else if err != nil {
			fmt.Println(err)
			return fmt.Errorf("source file %s does not exist", source)
		}
	}

	if destFile != nil {
		destFile.Close()
	}

	file, err := fs.Stat(source)
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
			newSource := source + string(os.PathSeparator) + entry.Name()
			newDest := dest + string(os.PathSeparator) + entry.Name()

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
