package main

import (
	"github.com/amyjzhu/mutation-framework/mutator"
	"encoding/json"
	"github.com/amyjzhu/mutation-framework"
	"fmt"
	"regexp"
	"github.com/ghodss/yaml"
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

	appendMutantFolderSlashOrReplaceWithDefault(&config)

	return &config, nil
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