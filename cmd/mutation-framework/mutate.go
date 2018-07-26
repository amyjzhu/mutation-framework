package main

import (
	"os"
	"path/filepath"
	"fmt"
	"strings"
	"go/types"
	"go/token"
	"go/ast"
	"github.com/amyjzhu/mutation-framework/mutator"
	"github.com/amyjzhu/mutation-framework"
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

		// make sure the source is valid before mutating
		src, fset, pkg, info, err := mutesting.ParseAndTypeCheckFile(abs)
		if err != nil {
			log.WithField("file", abs).Error("There was an error compiling the file.")
			return nil, nil, exitError(err.Error())
		}

		mutantFolderName := config.Mutate.MutantFolder
		err = fs.MkdirAll(mutantFolderName, 0755)
		if err != nil {
			panic(err)
		}

		/*
		// if the path specified is multiple folders deep, we should only use last one for name
		if strings.Contains(mutantFolderName, string(os.PathSeparator)) {
			mutantFolderName = filepath.Base(config.Mutate.MutantFolder)
		}*/

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

func getAbsoluteMutationFolderPath(config *MutationConfig) (projectName string) {
	if strings.HasPrefix(config.Mutate.MutantFolder, string(os.PathSeparator)) {
		// don't add project base path to it thenm
		projectName = config.Mutate.MutantFolder
	} else {
		projectName = appendFolder(config.FileBasePath, config.Mutate.MutantFolder)
	}

	log.Info(projectName)
	return
}

