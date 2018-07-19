package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"go/ast"
	"go/format"
	"go/printer"
	"go/token"
	"go/types"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"github.com/jessevdk/go-flags"

	"github.com/amyjzhu/mutation-framework"
	"github.com/amyjzhu/mutation-framework/osutil"
	"github.com/amyjzhu/mutation-framework/mutator"
	_ "github.com/amyjzhu/mutation-framework/mutator/branch"
	_ "github.com/amyjzhu/mutation-framework/mutator/expression"
	_ "github.com/amyjzhu/mutation-framework/mutator/statement"
	"github.com/spf13/afero"
	"path/filepath"
)

const (
	returnOk = iota
	returnHelp
	returnBashCompletion
	returnError
)

const (
	execPassed  = 0
	execFailed  = 1
	execSkipped = 2
)

var fs = afero.NewOsFs()

type Args struct {
	General struct {
		Debug                bool `long:"debug" description:"Debug log output"`
		Help                 bool `long:"help" description:"Show this help message"`
		Verbose              bool `long:"verbose" description:"Verbose log output"`
		ConfigPath 		string `long:"config" descriptionL:"Path to mutation config file" required:"true"`
		ListMutators    bool     `long:"list-mutators" description:"List all available mutators"`
	} `group:"General Args"`

	Files struct {
		Blacklist []string `long:"blacklist" description:"List of MD5 checksums of mutations which should be ignored. Each checksum must end with a new line character."`
		ListFiles bool     `long:"list-files" description:"List found files"`
		PrintAST  bool     `long:"print-ast" description:"Print the ASTs of all given files and exit"`
	} `group:"File Args"`

	Exec struct {
		Composition int `long:"composition" description:"Describe how many nodes should contain the mutation"`
		MutateOnly bool   `long:"mutate-only" description:"Skip the built-in exec command and just generate the mutations"`
		Timeout    uint   `long:"exec-timeout" description:"Sets a timeout for the command execution (in seconds)" default:"10"`
		ExecOnly   bool   `long:"no-mutate" description:"Does not mutate the files, only executes existing mutations"`
		CustomTest string   `string:"custom-test" description:"Specifies location of test script"`
	} `group:"Exec Args"`
}

func checkArguments(args []string, opts *Args) (bool, int) {
	p := flags.NewNamedParser("go-mutesting", flags.None)

	p.ShortDescription = "Mutation testing for Go source code"

	if _, err := p.AddGroup("go-mutesting", "go-mutesting arguments", opts); err != nil {
		return true, exitError(err.Error())
	}

	completion := len(os.Getenv("GO_FLAGS_COMPLETION")) > 0

	_, err := p.ParseArgs(args)
	if (opts.General.Help || len(args) == 0) && !completion {
		p.WriteHelp(os.Stdout)

		return true, returnHelp
	} else if opts.General.ListMutators {
		for _, name := range mutator.List() {
			fmt.Println(name)
		}

		return true, returnOk
	}

	if err != nil {
		return true, exitError(err.Error())
	}

	if completion {
		return true, returnBashCompletion
	}

	if opts.General.Debug {
		opts.General.Verbose = true
	}

	return false, 0
}

func debug(config *MutationConfig, format string, args ...interface{}) {
	if config.Verbose {
		fmt.Printf(format+"\n", args...)
	}
}

func exitError(format string, args ...interface{}) int {
	fmt.Fprintf(os.Stderr, format+"\n", args...)

	return returnError
}

type mutationStats struct {
	passed     int
	failed     int
	duplicated int
	skipped    int
}

func (ms *mutationStats) Score() float64 {
	total := ms.Total()

	if total == 0 {
		return 0.0
	}

	return float64(ms.passed) / float64(total)
}

func (ms *mutationStats) Total() int {
	return ms.passed + ms.failed + ms.skipped
}


func mainCmd(args []string) int {
	// get config path
	// get overrides
	var opts = &Args{}
	if exit, exitCode := checkArguments(args, opts); exit {
		return exitCode
	}

	pathToConfig := opts.General.ConfigPath
	mutationConfig, err := getConfig(pathToConfig)
	if err != nil {
		exitError(err.Error())
	}

	consolidateArgsIntoConfig(opts, mutationConfig)
	operators := retrieveMutationOperators(mutationConfig)
	files := mutationConfig.getRelativeAndAbsoluteFiles()

	stats, exitCode := mutateFiles(mutationConfig, files, operators)
	exitCode = runMutants(mutationConfig, mutantPaths, stats)
	return exitCode
}

func consolidateArgsIntoConfig(opts *Args, config *MutationConfig) {
	if opts.Exec.CustomTest != "" {
		config.Commands.Test = opts.Exec.CustomTest // TODO fix for arguments
	}

	if opts.Exec.ExecOnly {
		config.Mutate.Disable = true
	}

	if opts.Exec.MutateOnly {
		config.Test.Disable = true
	}

	if opts.General.Verbose {
		config.Verbose = true
	}

	if opts.Exec.Composition != 0 {
		config.Test.Composition = opts.Exec.Composition
	}

	if opts.Exec.Timeout != 0 {
		config.Test.Timeout = opts.Exec.Timeout
	}
}

func retrieveMutationOperators(config *MutationConfig) []mutator.Mutator {
	var operators []mutator.Mutator
	for _, operator := range config.Mutate.Operators {
		operators = append(operators, *operator.MutationOperator)
	}
	return operators
}

func main() {
	os.Exit(mainCmd(os.Args[1:]))
	// fmt.Println("Running config test instead of real program")
	//test()

	//fmt.Println("Running main nonsense instead of real program")
	//doNonsense()
}

func doNonsense() {
	copyRecursive(true, "/home/amy/go/src/github.com/amyjzhu/mutation-framework", "mutant-copyRecursive", "mutant-copyRecursive")
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

	err = ioutil.WriteFile(file, src, 0666)
	if err != nil {
		return "", false, err
	}

	return checksum, false, nil
}
