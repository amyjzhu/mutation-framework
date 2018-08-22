package main

import (
	"fmt"
	"os"
	"github.com/jessevdk/go-flags"

	"github.com/amyjzhu/mutation-framework/mutator"
	_ "github.com/amyjzhu/mutation-framework/mutator/branch"
	_ "github.com/amyjzhu/mutation-framework/mutator/expression"
	_ "github.com/amyjzhu/mutation-framework/mutator/statement"
	"github.com/spf13/afero"
	"strings"
	log "github.com/sirupsen/logrus"
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

var FS = afero.NewOsFs()

type Args struct {
	General struct {
		Debug                bool `long:"debug" description:"Debug log output"`
		Help                 bool `long:"help" description:"Show this help message"`
		Verbose              bool `long:"verbose" description:"Verbose log output"`
		ConfigPath 		string `long:"config" descriptionL:"Path to mutation config file" required:"true"`
		ListMutators    bool     `long:"list-mutators" description:"List all available mutators"`
		Json bool `long:"json" description:"Log events in json format"`
	} `group:"General Args"`

	Files struct {
		Blacklist []string `long:"blacklist" description:"List of MD5 checksums of mutations which should be ignored. Each checksum must end with a new line character."`
		ListFiles bool     `long:"list-files" description:"List found files"`
	} `group:"File Args"`

	Exec struct {
		Composition int `long:"composition" description:"Describe how many nodes should contain the mutation"`
		MutateOnly bool   `long:"no-exec" description:"Skip the built-in exec command and just generate the mutations"`
		Timeout    uint   `long:"exec-timeout" description:"Sets a timeout for the command execution (in seconds)" default:"10"`
		ExecOnly   bool   `long:"no-mutate" description:"Does not mutate the files, only executes existing mutations"`
		CustomTest string   `string:"custom-test" description:"Specifies location of test script"`
		Overwrite bool `long:"overwrite" description:"True if want to overwrite existing mutants in name clash"`
	} `group:"Exec Args"`
}

type mutationStats struct {
	passed     int
	failed     int
	duplicated int
	skipped    int
}

func exitError(format string, args ...interface{}) (exitCode int) {
	errorMessage := fmt.Sprintf(format+"\n", args...)
	log.Error(errorMessage)

	return returnError
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

func checkArguments(args []string, opts *Args) (bool, int) {
	p := flags.NewNamedParser("mutation-framework", flags.None)

	p.ShortDescription = "Mutation testing for Go source code"

	if _, err := p.AddGroup("mutation-framework", "mutation-framework arguments", opts); err != nil {
		return true, exitError(err.Error())
	}

	completion := len(os.Getenv("GO_FLAGS_COMPLETION")) > 0

	_, err := p.ParseArgs(args)
	if (opts.General.Help || len(args) == 0) && !completion {
		p.WriteHelp(os.Stdout)

		return true, returnHelp

	} else if opts.General.ListMutators {
		fmt.Println(mutator.List())

		return true, returnHelp
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

// TODO variable levels of logging
func setUpLogging(config *MutationConfig) {
	if config.Json {
		log.SetFormatter(&log.JSONFormatter{})
	} else {
		log.SetFormatter(&log.TextFormatter{})
	}

	if config.Verbose {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	log.SetOutput(os.Stdout)
}

// Command-line arguments are higher-priority than config file options
// TODO environment variables?
func consolidateArgsIntoConfig(opts *Args, config *MutationConfig) {
	if strings.TrimSpace(opts.Exec.CustomTest) != "" {
		config.Commands.Test = opts.Exec.CustomTest
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

	if opts.General.Json {
		config.Json = true
	}

	if opts.Exec.Overwrite {
		config.Mutate.Overwrite = true
	}
}

func mainCmd(args []string) (exitCode int) {
	config, files, exitCode := initializeExecution(args)
	if exitCode != returnOk {
		return
	}

	exitCode = performMutationTesting(config, files)
	return
}

func initializeExecution(args []string) (*MutationConfig, map[string]string, int){
	var opts= &Args{}
	if exit, exitCode := checkArguments(args, opts); exit {
		return nil, nil, exitCode
	}

	pathToConfig := opts.General.ConfigPath
	config, err := getConfig(pathToConfig)
	if err != nil {
		exitError(err.Error())
	}

	consolidateArgsIntoConfig(opts, config)
	setUpLogging(config)
	files := config.getRelativeAndAbsoluteFiles()

	return config, files, returnOk
}

func performMutationTesting(config *MutationConfig, files map[string]string) (exitCode int) {
	var mutantPaths []MutantInfo
	var stats map[string]*mutationStats
	var err error

	if !config.Mutate.Disable {
		stats, mutantPaths, exitCode = mutateFiles(config, files)
		if exitCode == returnError {
			return exitCode
		}
	} else {
		stats = make(map[string]*mutationStats)
		log.Info("Running tests without mutating.")
		mutantPaths, err = findAllMutantsInFolder(config, stats, files)
		if err != nil {
			log.Error(err)
			return returnError
		}
	}

	// TODO implement listfiles
	if !config.Test.Disable {
		exitCode = executeAllMutants(config, mutantPaths, stats)
	} else {
		exitCode = returnOk
	}

	return exitCode

}

func main() {
	os.Exit(mainCmd(os.Args[1:]))
}
