package main

import (
	"github.com/amyjzhu/mutation-framework/mutator"
	"encoding/json"
	"github.com/amyjzhu/mutation-framework"
	"fmt"
)

// Cannot extend types in other packages
// So this is an embedded type for unmarshalling JSON
type Operator struct {
	MutationOperator *mutator.Mutator
	MutatorName      string
}

type MutationConfig struct {
	Operators []Operator `json:"operators"`
	FilesToInclude []string `json:"files_to_include"`
	FilesToExclude []string `json:"files_to_exclude"`
	Options Options `json:"options"`
}

type Options struct {
	Composition int `json:"composition"`
	Verbose bool `json:"verbose"`
}

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
	operator.MutatorName = mutatorName

	return nil
}

func (operator *Operator) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", operator.MutatorName)), nil
}

func getConfig(configFilePath string) (*MutationConfig, error) {
	// TODO return error instead of panic maybe?
	data, err := mutesting.LoadFile(configFilePath)
	if err != nil {
		return nil, err
	}

	var config MutationConfig
	err = json.Unmarshal([]byte(data), &config)

	if err != nil {
		return nil, err
	}

	return &config, nil
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
	config, _ := getConfig("testdata/config/sample_config.json")
	fmt.Println(config.getString())
}