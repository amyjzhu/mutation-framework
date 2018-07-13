package main

import (
	"github.com/amyjzhu/mutation-framework/mutator"
	"encoding/json"
	"github.com/amyjzhu/mutation-framework"
	"fmt"
	"regexp"
	"github.com/ghodss/yaml"
	"strings"
	"io/ioutil"
	"os"
)

// Cannot extend types in other packages
// So this is an embedded type for unmarshalling JSON
type Operator struct {
	MutationOperator *mutator.Mutator
	Name             string
}

type MutationConfig struct {
	Operators []Operator `json:"operators"`
	FilesToInclude []string `json:"files_to_include"`
	FilesToExclude []string `json:"files_to_exclude"`
	FileBasePath string `json:"file_basepath"`
	Options Options `json:"options"`
	Scripts Scripts `json:"scripts"`
}

type Options struct {
	Composition  int    `json:"composition"`
	Verbose      bool   `json:"verbose"`
	MutateOnly   bool   `json:"mutate_only"`
	ExecOnly     bool   `json:"exec_only"`
	MutantFolder string `json:"mutant_folder"`
	Timeout      uint   `json:"timeout"`
}

type Scripts struct {
	Test    string `json:"test"`
	CleanUp string `json:"clean_up"`
} // todo required group

const DEFAULT_MUTATION_FOLDER = "mutants/"

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

func getConfig(configFilePath string) (*MutationConfig, error) {
	// TODO return error instead of panic maybe?
	data, err := mutesting.LoadFile(configFilePath)
	if err != nil {
		return nil, err
	}

	var config MutationConfig

	if !isJson(data) {
		data, err = convertFromYaml(data)
	}

	err = json.Unmarshal([]byte(data), &config)

	if err != nil {
		return nil, err
	}

	appendBasepathToAllFiles(&config)
	appendMutantFolderSlashOrReplaceWithDefault(&config)
	expandWildCards(&config)

	return &config, nil
}

func appendBasepathToAllFiles(config *MutationConfig) {
	basepath := config.FileBasePath
	if basepath == "" {
		return
	}

	var newPaths []string
	for _, file := range config.FilesToInclude {
		newPaths = append(newPaths, concatAddingSlashIfNeeded(basepath, file))
	}
	config.FilesToInclude = newPaths

	newPaths = make([]string, 0)
	for _, file := range config.FilesToExclude {
		newPaths = append(newPaths, concatAddingSlashIfNeeded(basepath, file))
	}
	config.FilesToExclude = newPaths
}

func concatAddingSlashIfNeeded(parent string, child string) string {
	parentSuffixedWithSlash := strings.HasSuffix(parent, "/")
	childPrefixedWithSlash := strings.HasPrefix(child, "/")
	if parentSuffixedWithSlash && childPrefixedWithSlash {
		return parent + child[1:]
	} else if parentSuffixedWithSlash || childPrefixedWithSlash {
		return parent + child
	} else {
		return parent + "/" + child
	}
}

func appendMutantFolderSlashOrReplaceWithDefault(config *MutationConfig) {
	mutantFolderPath := config.Options.MutantFolder
	if mutantFolderPath == "" {
		config.Options.MutantFolder = DEFAULT_MUTATION_FOLDER
	} else {
		if mutantFolderPath[len(mutantFolderPath)-1:] != "/" {
			config.Options.MutantFolder = mutantFolderPath + "/"
		}
	}
}

func expandWildCards(config *MutationConfig) {
	var expandedPaths []string
	for _, filePath := range config.FilesToInclude {
		if strings.Contains(filePath, "*") {
			expandedPaths = append(expandedPaths, expandWildCard(filePath)...)
		}
	}
	config.FilesToInclude = append(config.FilesToInclude, expandedPaths...)

	expandedPaths = []string{}
	for _, filePath := range config.FilesToExclude {
		if strings.Contains(filePath, "*") {
			expandedPaths = append(expandedPaths, expandWildCard(filePath)...)
		}
	}
	config.FilesToExclude = append(config.FilesToExclude, expandedPaths...)
}

func expandWildCard(path string) []string {
	pieces := strings.Split(path, "/")
	return expandWildCardRecursive(0, pieces)
}

func expandWildCardRecursive(pathIndex int, pathPieces []string) []string {
	if pathIndex >= len(pathPieces) {
		return []string{}
	}

	pathPiece := pathPieces[pathIndex]
	if valid, globPattern := isValidWildCard(pathPiece); valid {
		fileNames := getAllFilesInPath(getParentDirectory(pathPieces, pathIndex))

		if len(globPattern) > 1 {
			fileNames = filterFileNames(globPattern, fileNames)
		}

		// we have all the file names, now need to traverse each tree
		var completePaths []string
		for _, name := range fileNames {
			// remove the wildcard piece with real folder
			newPathPieces := pathPieces
			newPathPieces[pathIndex] = name
			completePaths = append(completePaths,
				expandWildCardRecursive(pathIndex+1, newPathPieces)...)
		}
		return completePaths
	} else {
		// not a glob, so we don't need to return any expanded file paths
		// but if it is a directory, we should keep going.
		currentPath := getParentDirectory(pathPieces, pathIndex + 1)
		currentFile, err := os.Stat(currentPath)
		if err != nil {
			fmt.Println("Unable to locate files for mutating.")
			panic(err)
		}

		if currentFile.IsDir() {
			return expandWildCardRecursive(pathIndex+1, pathPieces)
		}

		return[]string{getParentDirectory(pathPieces, pathIndex + 1)}
	}
}

// Does not properly account for slashes
func isValidWildCard(piece string) (bool, string) {
	validWildCard := regexp.MustCompile(`([^\*]*\*[^\*]*)`)
	matches := validWildCard.FindAllStringSubmatch(piece,-1)
	CAPTURING_GROUP_INDEX := 1
	if len(matches) == 1 {
		return true, matches[0][CAPTURING_GROUP_INDEX]
	} else {
		return false, ""
	}
}

func filterFileNames(globPattern string, files []string) []string {
	validFileName := `(@\"^[\w\-. ]+$)*\"`
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

// Returns one directory up
// e.g. mushroom/*.go/
//                ^^^ we focus on this piece, but return mushroom/
func getParentDirectory(pathPieces []string, index int) string {
	path := ""
	for i := 0; i < index; i++ {
		path += pathPieces[i]
		path += "/"
	}
	return path
}

func getAllFilesInPath(path string) []string {
	fileInfo, err := ioutil.ReadDir(path)
	if err != nil {
		// TODO??
	}

	var names []string
	for _, info := range fileInfo {
		names = append(names, info.Name())
	}

	fmt.Println("I am in getAllFilesInPath")
	fmt.Println(names)
	return names
}

func convertFromYaml(yamlData []byte) ([]byte, error) {
	return yaml.YAMLToJSON(yamlData)
}

func isJson(data []byte) bool {
	jsonPattern := regexp.MustCompile(`[\s]*{.*`)
	return jsonPattern.Match(data)

}

func (config *MutationConfig) getString() (string, error) {
	result, err := json.Marshal(config)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

func test() {
	fmt.Println(mutator.List())
	config, _ := getConfig("testdata/config/sample_config.yaml")
	fmt.Println(config.getString())
}