package main

import (
	"github.com/amyjzhu/mutation-framework/mutator"
	"encoding/json"
	"fmt"
)

type MutationConfig struct {
	Operators []mutator.Mutator
	FilesToInclude []string
	FilesToExclude []string
	Options Options
}

type Options struct {
	composition int
	verbose bool
}

func (mutator *mutator.Mutator) UnmarshalJSON(data []byte) error {
	var mutatorName string
	err := json.Unmarshal(data , &mutatorName)
	// do I have to do this or can I simply
	//mutatorName := string(data)

	mutator, err := mutator.New(mutatorName)

	if err != nil {
		panic(err)
	}
	return nil
}