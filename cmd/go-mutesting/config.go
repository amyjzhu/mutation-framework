package main

import (
	"github.com/amyjzhu/mutation-framework/mutator"
	"encoding/json"
	"fmt"
	"strings"
)

type MutationConfig struct {
	operators []mutator.Mutator
	filesToInclude []string
	filesToExclude []string
}

type Options struct {
	composition int
	verbose bool
}

func (mutator *mutator.Mutator) UnmarshalJSON(data []byte) error {
	var roll string

	err := json.Unmarshal(data , &roll);
	if err != nil {
		fmt.Println("issue")
		return err
	}

	parts := strings.Split(roll, "d")
	dr.die = getInt(parts[1])
	dr.rolls = getInt(parts[0])
	return nil
}