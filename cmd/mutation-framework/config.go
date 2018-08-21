package main

import (
	"github.com/amyjzhu/mutation-framework/mutator"
	"encoding/json"
	"github.com/amyjzhu/mutation-framework"
	"fmt"
	"regexp"
	"github.com/ghodss/yaml"
	"strings"
	"os"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"path/filepath"
)

// Cannot extend types in other packages
// So this is an embedded type for unmarshalling JSON
type Operator struct {
	MutationOperator *mutator.Mutator
	Name             string
}

type MutationConfig struct {
	Verbose      bool   `json:"verbose"`
	Json bool `json:"json"`
	FileBasePath   string     `json:"project_root"` // the root of project and appended to file paths
	Mutate Mutate `json:"mutate"`
	Test Test `json:"test"`
	Commands       Commands   `json:"commands"`
}

type Test struct {
	Disable bool `json:"disable"`
	Timeout      uint   `json:"timeout"`
	Composition  int    `json:"composition"`
}

type Mutate struct {
	Disable bool `json:"disable"`
	Operators      []Operator `json:"operators"`
	FilesToInclude []string   `json:"files_to_include"`
	FilesToExclude []string   `json:"files_to_exclude"`
	MutantFolder string `json:"mutant_folder"`
	Overwrite bool `json:"overwrite"`
}


// TODO rules
// Project Directory is necessary
// If mutant folder doesn't start with /, it is taken to be relative

type Commands struct {
	Test    string `json:"test"`
	Build string `json:"build"`
	CleanUp string `json:"clean_up"`
}

const DefaultMutationFolder = "mutants/"

// Bundle mutation operators together with their names
func (operator *Operator) UnmarshalJSON(data []byte) error {
	var mutatorName string
	err := json.Unmarshal(data , &mutatorName)
	// do I have to do this or can I simply
	//mutatorName := string(data)

	var mutationOperator mutator.Mutator
	mutationOperator, err = mutator.New(mutatorName)
	if err != nil {
		return err
	}

	operator.MutationOperator = &mutationOperator
	operator.Name = mutatorName

	return nil
}

func (operator *Operator) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", operator.Name)), nil
}

// TODO not convinced about infalibility of config; write more tests
func getConfig(configFilePath string) (*MutationConfig, error) {
	data, err := mutesting.LoadFile(configFilePath)
	if err != nil {
		return nil, err
	}

	if !isJson(data) {
		data, err = convertFromYaml(data)
	}

	return parseConfig(data)
}

func parseConfig(data []byte) (*MutationConfig, error) {
	var config MutationConfig
	err := json.Unmarshal([]byte(data), &config)

	if err != nil {
		return nil, err
	}

	return &config, nil
}

func (config *MutationConfig) UnmarshalJSON(data []byte) error {
	type unfurlConfig MutationConfig

	err := json.Unmarshal(data, (*unfurlConfig)(config))

	if err != nil {
		return err
	}

	err = validateImportantConfigFields(config)
	if err != nil {
		return err
	}

	appendMutantFolderSlashOrReplaceWithDefault(config)
	expandWildCards(config)
	config.FileBasePath = appendSlash(config.FileBasePath)

	configString, err := config.toString()
	if err != nil {
		return err
	}

	log.WithField("config", configString).Info("Finished parsing config.")

	return nil
}

func (config *MutationConfig) toString() (string, error) {
	configString, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(configString), nil
}

func validateImportantConfigFields(config *MutationConfig) error {
	noFilesSpecified := func(config *MutationConfig) bool {
		return !config.Mutate.Disable &&
			(len(config.Mutate.FilesToExclude) == 0 &&
				len(config.Mutate.FilesToInclude) == 0)}

	if noFilesSpecified(config) {
		log.Info("Mutate files are empty. Is your config correct?")
	}

	if config.FileBasePath == "" {
		return fmt.Errorf("project root is not set")
	}

	for _, file := range append(config.Mutate.FilesToInclude, config.Mutate.FilesToExclude...) {
		if strings.HasPrefix(file, string(os.PathSeparator)) {
			log.WithField("file", file).Debug( "Did you intend for %s to have path separator prefix?\n")
		}
	}

	if strings.HasPrefix(config.Mutate.MutantFolder, string(os.PathSeparator)) {
		log.Debug( "Did you intend for mutant folder to have path separator prefix?\n")
	}

	if config.Commands == (Commands{}) {
		log.Debug("Did you mean for Commands to be empty?")
	}


	return nil
}

func concatAddingSlashIfNeeded(parent string, child string) string {
	parentSuffixedWithSlash := strings.HasSuffix(parent, string(os.PathSeparator))
	childPrefixedWithSlash := strings.HasPrefix(child, string(os.PathSeparator))
	if parentSuffixedWithSlash && childPrefixedWithSlash {
		return parent + child[1:]
	} else if parentSuffixedWithSlash || childPrefixedWithSlash {
		return parent + child
	} else {
		return parent + string(os.PathSeparator) + child
	}
}

func appendMutantFolderSlashOrReplaceWithDefault(config *MutationConfig) {
	mutantFolderPath := config.Mutate.MutantFolder
	if mutantFolderPath == "" {
		config.Mutate.MutantFolder = DefaultMutationFolder
	} else {
		config.Mutate.MutantFolder = appendSlash(mutantFolderPath)
	}
}

func appendSlash(path string) string {
	if path == "" {
		return string(os.PathSeparator)
	}

	// TODO path.Join
	if path[len(path)-1:] != string(os.PathSeparator) {
		return path + string(os.PathSeparator)
	}
	return path
}

// Include files in include, then exclude files from exclude
func (config *MutationConfig) getIncludedFiles() []string {
	var filesToMutate = make(map[string]struct{},0)

	if len(config.Mutate.FilesToInclude) == 0 {
		// TODO add all files
	}

	for _, file := range config.Mutate.FilesToInclude {
		filesToMutate[file] = struct{}{}
	}

	for _, excludeFile := range config.Mutate.FilesToExclude {
		delete(filesToMutate, excludeFile)
	}

	fileNames := make([]string, len(filesToMutate))
	i := 0
	for name := range filesToMutate {
		fileNames[i] = name
		i++
	}

	return fileNames
}

// Maps the relative file paths within the project to their absolute paths
// given by project directory option
func (config *MutationConfig) getRelativeAndAbsoluteFiles() (fileMap map[string]string) {
	fileMap = make(map[string]string)
	files := config.getIncludedFiles()
	for _, file := range files {
		fileMap[file] = concatAddingSlashIfNeeded(config.FileBasePath, file)
	}

	return
}

// replaces wildcards with array of actual file paths
// that are found using the glob patterns
func expandWildCards(config *MutationConfig) {
	var expandedPaths []string
	for _, filePath := range config.Mutate.FilesToInclude {
		if strings.Contains(filePath, "*") {
			expandedPaths = append(expandedPaths,
				expandWildCard(filePath, config.FileBasePath)...)
		}
	}
	revisedFilesToInclude := removeWildCardPaths(config.Mutate.FilesToInclude)
	config.Mutate.FilesToInclude = append(revisedFilesToInclude, expandedPaths...)

	expandedPaths = []string{}
	for _, filePath := range config.Mutate.FilesToExclude {
		if strings.Contains(filePath, "*") {
			expandedPaths = append(expandedPaths,
				expandWildCard(filePath, config.FileBasePath)...)
		}
	}
	revisedFilesToExclude := removeWildCardPaths(config.Mutate.FilesToExclude)
	config.Mutate.FilesToExclude = append(revisedFilesToExclude, expandedPaths...)
}

func expandWildCard(path string, basepath string) []string {
	pieces := strings.Split(path, string(os.PathSeparator))
	return expandWildCardRecursive(0, pieces, basepath)
}

func removeWildCardPaths(paths []string) []string {
	var nonWildCards []string

	for _, file := range paths {
		if valid, _ := isValidWildCard(file); !valid {
			nonWildCards = append(nonWildCards, file)
		}
	}
	return nonWildCards
}

// TODO refactor
// TODO replace with filepath.Glob? facepalm
// Move through a string with wildcards to find all possible files
// Keep a "pathPieces" arg as a context accumulator for the paths we've visited
// thus far
// basepath is the relative path within the project folder
func expandWildCardRecursive(pathIndex int, pathPieces []string, basepath string) []string {

	// Function closures
	isDirectory := func(path string) bool {
		if path == "" {
			return true
			// TODO I suppose this could cause everything to break
		}

		currentFile, err := FS.Stat(basepath + path)
		if err != nil {
			return false
		}

		return currentFile.IsDir()
	}

	exists := func(path string) bool {
		_, err := FS.Stat(basepath + path)

		if err != nil && len(path) != 0 {
			// Try opening the absolute path we have so far, ignoring last slash
			_, err = FS.Stat(basepath + filepath.Clean(path))
			if err != nil && os.IsNotExist(err) {
				// eg. mutator/branch/remove.go does not exist
				return false
			}
		}

		return true
	}

	getAllFilesAndFoldersInPath := func(path string) (fileNames []string, dirNames []string) {
		fileInfo, err := afero.ReadDir(FS, basepath + path)
		if err != nil {
			panic(err)
		}

		for _, info := range fileInfo {
			if info.IsDir() {
				dirNames = append(dirNames, info.Name())
			} else {
				fileNames = append(fileNames, info.Name())
			}
		}

		return fileNames, dirNames
	}

	// The file base case version of expandWildCard
	expandWildCardFile := func(path string) []string {
		if exists(path) {
			return []string{path}
		}
		return []string{}
	}

	// actual function begins
	if pathIndex >= len(pathPieces) {
		return []string{}
	}
	currentPath := getCurrentPath(pathIndex, pathPieces)
	parentPath := getParentPath(pathPieces, pathIndex)

	if isDirectory(parentPath) {

		pathPiece := pathPieces[pathIndex]
		// is the current directory we're in a wildcard?
		if valid, glob := isValidWildCard(pathPiece); valid {
			fileNames, dirNames := getAllFilesAndFoldersInPath(parentPath)

			// is there something to filter? e.g. /*/ no, but /*.jpg yes
			if len(glob) > 1 {
				fileNames = filterFileNames(glob, fileNames)
				dirNames = filterFileNames(glob, dirNames)
			}

			var allPaths []string
			for _, dir := range dirNames {
				newPathPieces := pathPieces
				// replace wildcard character with real folders
				newPathPieces[pathIndex] = dir
				// recurse deeper into structure
				allPaths = append(allPaths,
					expandWildCardRecursive(pathIndex, newPathPieces, basepath)...)
			}

			// only add files if we're at the last piece of the path
			// otherwise we need to keep looking
			if pathIndex == len(pathPieces) - 1 {
				for _, file := range fileNames {
					path := getParentPath(pathPieces, pathIndex) + file
					allPaths = append(allPaths, expandWildCardFile(path)...)
				}
			}

			return allPaths
		}

		// we're not globs and don't have directory, so we're done here
		if !isDirectory(currentPath) {
			if exists(currentPath) {
				return []string{currentPath[:len(currentPath)-1]}
			}
		}

		// if we don't have globs are are a directory, keep going
		return expandWildCardRecursive(pathIndex+1, pathPieces, basepath)
	}

	// normal path, but doesn't exist
	if !exists(currentPath) {
		return []string{}
	}

	// normal path
	return []string{currentPath[:len(currentPath)-1]}

}

// TODO remove code duplication
// TODO replace with filepath.Dir and filepath.Parent
func getCurrentPath(index int, pathPieces []string) string {
	path := ""
	for i := 0; i <= index; i++ {
		path += pathPieces[i]
		path += string(os.PathSeparator)
	}
	return path
}

// Returns one directory up
// e.g. mushroom/*.go/
//                ^^^ we focus on this piece, but return mushroom/
func getParentPath(pathPieces []string, index int) string {
	path := ""
	for i := 0; i < index; i++ {
		path += pathPieces[i]
		path += string(os.PathSeparator)
	}
	return path
}

// Does not properly account for slashes
func isValidWildCard(piece string) (bool, string) {
	validWildCard := regexp.MustCompile(`([^\*]*\*[^\*]*)`)
	matches := validWildCard.FindAllStringSubmatch(piece,-1)
	CapturingGroupIndex := 1
	if len(matches) == 1 {
		return true, matches[0][CapturingGroupIndex]
	} else {
		return false, ""
	}
}

// Replace a glob with regex so we can match it to file names
func filterFileNames(globPattern string, files []string) []string {
	validFileName := `[\w\-. ]+` // should it be + or *? since replacing *
	validFilePattern := strings.Replace(globPattern, "*", validFileName, -1)
	validFileRegex := regexp.MustCompile(validFilePattern)

	var validFiles []string
	for _, file := range files {
		if validFileRegex.MatchString(file) {
			validFiles = append(validFiles, file)
		}
	}

	return validFiles
}

// Support YAML configuration files
func convertFromYaml(yamlData []byte) ([]byte, error) {
	return yaml.YAMLToJSON(yamlData)
}

func isJson(data []byte) bool {
	jsonPattern := regexp.MustCompile(`[\s]*{.*`)
	return jsonPattern.Match(data)
}