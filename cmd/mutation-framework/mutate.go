package main

import (
	"os"
	"path/filepath"
	"fmt"
	"strings"
	"go/types"
	"go/token"
	"go/ast"
	"github.com/amyjzhu/mutation-framework"
	log "github.com/sirupsen/logrus"
	"bytes"
	"crypto/md5"
	"go/printer"
	"io"
	"go/format"
	"github.com/spf13/afero"
)

type MutantInfo struct {
	pkg                      *types.Package
	originalFileRelativePath string
	mutantDirPathAbsPath     string
	mutationFileAbsPath      string
	checksum                 string
}

func mutateFiles(config *MutationConfig, files map[string]string) (map[string]*mutationStats, []MutantInfo, int) {
	log.Info("Mutating files.")
	allStats := make(map[string]*mutationStats)
	var allMutantInfo []MutantInfo

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
		err = FS.MkdirAll(mutantFolderName, 0755)
		if err != nil {
			panic(err)
		}

		mutantFile := appendFolder(config.Mutate.MutantFolder, relativeFileLocation)
		createMutantFolderPath(mutantFile)

		mutationID := 0

		mutantInfo := mutate(config, mutationID, pkg, info, abs, relativeFileLocation,
			fset, src, src, stats)

		allMutantInfo = append(allMutantInfo, mutantInfo...)
	}

	return allStats, allMutantInfo, returnOk
}

func createMutantFolderPath(file string) {
	if strings.Contains(file, string(os.PathSeparator)) {
		parentPath := filepath.Dir(file)
		err := FS.MkdirAll(parentPath, 0755)
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
		mutationID = 0
		log.WithField("mutation_operator", m.Name).Info("Mutating.")

		changed := mutesting.MutateWalk(pkg, info, node, *m.MutationOperator)

		for {
			_, ok := <-changed

			if !ok {
				break
			}

			mutationBlackList := make(map[string]struct{},0) //TODO implement real blacklisting

			mutationFileId := buildMutantName(m.Name, relativeFilePath, mutationID)
			log.WithField("name", mutationFileId).Info("Creating mutant.")

			mutantPath, err := copyProject(config, mutationFileId) // TODO verify correctness of absolute file
			if err != nil {
				log.WithField("error", err).Error("Internal error.")
			}

			// get the absolute path of the mutated file inside the mutant
			mutatedFilePath := appendFolder(filepath.Clean(mutantPath), relativeFilePath)
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

				// Bundle up information about the mutant and send to exec
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

func buildMutantName(operatorName string, filePath string, mutationId int) string {
	// replace slash so "branch/go" becomes "branch-go" and doesn't create new directory
	safeMutationName := strings.Replace(operatorName, string(os.PathSeparator), "-", -1)
	return fmt.Sprintf("%s.%s.%d", filePath, safeMutationName, mutationId)
}

func getAbsoluteMutationFolderPath(config *MutationConfig) (projectName string) {
	if strings.HasPrefix(config.Mutate.MutantFolder, string(os.PathSeparator)) {
		// don't add project base path to it then
		projectName = config.Mutate.MutantFolder
	} else {
		projectName = appendFolder(config.ProjectRoot, config.Mutate.MutantFolder)
	}

	return
}

func saveAST(mutationBlackList map[string]struct{}, file string, fset *token.FileSet, node ast.Node) (string, bool, error) { // TODO blacklists -- don't currently have this capability
	var buf bytes.Buffer

	h := md5.New()

	err := printer.Fprint(io.MultiWriter(h, &buf), fset, node)
	if err != nil {
		return "", false, err
	}

	checksum := fmt.Sprintf("%x", h.Sum(nil))

	if _, ok := mutationBlackList[checksum]; ok {
		return checksum, true, nil
	}

	mutationBlackList[checksum] = struct{}{}

	src, err := format.Source(buf.Bytes())
	if err != nil {
		return "", false, err
	}

	err = afero.WriteFile(FS, file, src, 0666)
	fmt.Println("Made change: ", src)
	if err != nil {
		return "", false, err
	}

	return checksum, false, nil
}


