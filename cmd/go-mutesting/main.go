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
	"os/exec"
	"regexp"
	"strings"
	"syscall"

	"github.com/jessevdk/go-flags"

	"github.com/amyjzhu/mutation-framework"
	"github.com/amyjzhu/mutation-framework/osutil"
	"github.com/amyjzhu/mutation-framework/mutator"
	_ "github.com/amyjzhu/mutation-framework/mutator/branch"
	_ "github.com/amyjzhu/mutation-framework/mutator/expression"
	_ "github.com/amyjzhu/mutation-framework/mutator/statement"
	"sort"
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
	if config.Options.Verbose {
		fmt.Printf(format+"\n", args...)
	}
}

func verbose(opts *Args, format string, args ...interface{}) {
	if opts.General.Verbose || opts.General.Debug {
		fmt.Printf(format+"\n", args...)
	}
}

func exitError(format string, args ...interface{}) int {
	fmt.Fprintf(os.Stderr, format+"\n", args...)

	return returnError
}

type mutatorItem struct {
	Name    string
	Mutator mutator.Mutator
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
	files := getCandidateFiles(mutationConfig)

	return mutateFiles(mutationConfig, files, operators)
}

func consolidateArgsIntoConfig(opts *Args, config *MutationConfig) {
	if opts.Exec.CustomTest != "" {
		config.Scripts.Test = opts.Exec.CustomTest // TODO fix for arguments
	}

	if opts.Exec.ExecOnly {
		config.Options.ExecOnly = true
	}

	if opts.Exec.MutateOnly {
		config.Options.MutateOnly = true
	}

	if opts.General.Verbose {
		config.Options.Verbose = true
	}

	if opts.Exec.Composition != 0 {
		config.Options.Composition = opts.Exec.Composition
	}

	if opts.Exec.Timeout != 0 {
		config.Options.Timeout = opts.Exec.Timeout
	}
}

func retrieveMutationOperators(config *MutationConfig) []mutator.Mutator {
	var operators []mutator.Mutator
	for _, operator := range config.Operators {
		operators = append(operators, *operator.MutationOperator)
	}
	return operators
}

func getCandidateFiles(config *MutationConfig) []string {
	var filesToMutate = make(map[string]struct{},0)

	if len(config.FilesToInclude) == 0 {
		// TODO add all files
	}

	for _, file := range config.FilesToInclude {
		filesToMutate[file] = struct{}{}
	}

	// TODO exclude is more powerful than include
	for _, excludeFile := range config.FilesToExclude {
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

func mutateFiles(config *MutationConfig, files []string, operators []mutator.Mutator) int {
	stats := &mutationStats{}

	for _, file := range files {
		debug(config, "Mutate %q", file)

		src, fset, pkg, info, err := mutesting.ParseAndTypeCheckFile(file)
		if err != nil {
			return exitError(err.Error())
		}

		err = os.MkdirAll(config.Options.MutantFolder, 0755)
		if err != nil {
			panic(err)
		}

		mutantFile := config.Options.MutantFolder + file

		originalFile := fmt.Sprintf("%s.original", mutantFile)
		err = osutil.CopyFile(file, originalFile)
		if err != nil {
			panic(err)
		}
		debug(config, "Save original into %q", originalFile)

		mutationID := 0

		// TODO match function names instead
		mutationID = mutate(config, mutationID, pkg, info, file, fset, src, src, mutantFile, stats)
	}

	printStats(config, stats)

	return returnOk
}

func mutate(config *MutationConfig, mutationID int, pkg *types.Package, info *types.Info, file string, fset *token.FileSet, src ast.Node, node ast.Node, tmpFile string, stats *mutationStats) int {
	for _, m := range config.Operators {
		debug(config, "Mutator %s", m.Name)

		changed := mutesting.MutateWalk(pkg, info, node, *m.MutationOperator)

		for {
			_, ok := <-changed

			if !ok {
				break
			}

			mutationBlackList := make(map[string]struct{},0) //TODO implement real blacklisting
			mutationFile := fmt.Sprintf("%s.%d", tmpFile, mutationID)
			checksum, duplicate, err := saveAST(mutationBlackList, mutationFile, fset, src)
			if err != nil {
				fmt.Printf("INTERNAL ERROR %s\n", err.Error())
			} else if duplicate {
				debug(config, "%q is a duplicate, we ignore it", mutationFile)

				stats.duplicated++
			} else {
				debug(config, "Save mutation into %q with checksum %s", mutationFile, checksum)

				if !config.Options.MutateOnly {
					execExitCode := mutateExec(config, pkg, file, mutationFile)

					debug(config, "Exited with %d", execExitCode)

					msg := fmt.Sprintf("%q with checksum %s", mutationFile, checksum)

					switch execExitCode {
					case execPassed:
						fmt.Printf("PASS %s\n", msg)

						stats.passed++
					case execFailed:
						fmt.Printf("FAIL %s\n", msg)

						stats.failed++
					case execSkipped:
						fmt.Printf("SKIP %s\n", msg)

						stats.skipped++
					default:
						fmt.Printf("UNKNOWN exit code for %s\n", msg)
					}
				}
			}

			changed <- true

			// Ignore original state
			<-changed
			changed <- true

			mutationID++
		}
	}

	getRedundantCandidates()
	fmt.Printf("Live muatants: %s\n", liveMutants)
	fmt.Println(testsToMutants)
	return mutationID
}

func printStats(config *MutationConfig, stats *mutationStats) {
	if !config.Options.MutateOnly {
		fmt.Printf("The mutation score is %f (%d passed, %d failed, %d duplicated, %d skipped, total is %d)\n", stats.Score(), stats.passed, stats.failed, stats.duplicated, stats.skipped, stats.Total())
	} else {
		fmt.Println("Cannot do a mutation testing summary since no exec command was executed.")
	}
}



var liveMutants = make([]string, 0)

func mutateExec(config *MutationConfig, pkg *types.Package, file string, mutationFile string) (execExitCode int) {
	if config.Scripts.Test != "" {
		return customTestMutateExec(config, pkg, file, mutationFile, config.Scripts.Test)
	}

	//if len(execs) == 0 {
		//defaultMutateExec(opts, pkg, file, mutationFile)
		return customMutateExec(config, pkg, file, mutationFile)
	//}
}

func customTestMutateExec(config *MutationConfig, pkg *types.Package, file string, mutationFile string, testCommand string) (execExitCode int) {
	const MUTATION_FOLDER = "mutants/"
	debug(config, "Execute built-in exec command with custom test script for mutation")

	diff, err := exec.Command("diff", "-u", file, mutationFile).CombinedOutput()
	if err == nil {
		execExitCode = execPassed
	} else if e, ok := err.(*exec.ExitError); ok {
		execExitCode = e.Sys().(syscall.WaitStatus).ExitStatus()
	} else {
		panic(err)
	}
	if execExitCode != execPassed && execExitCode != execFailed {
		fmt.Printf("%s\n", diff)

		panic("Could not execute diff on mutation file")
	}

	defer func() {
		_ = os.Rename(file+".tmp", file)
	}()

	err = os.Rename(file, file+".tmp")
	if err != nil {
		panic(err)
	}
	err = osutil.CopyFile(mutationFile, file)
	if err != nil {
		panic(err)
	}

	err = moveIntoMutantsFolder(MUTATION_FOLDER, mutationFile)
	if err != nil {
		panic(err)
	}


	execWithArgs := strings.Split(testCommand, " ")

	test, err := exec.Command(execWithArgs[0], execWithArgs[1:]...).CombinedOutput()

	if err == nil {
		execExitCode = execPassed
	} else if e, ok := err.(*exec.ExitError); ok {
		execExitCode = e.Sys().(syscall.WaitStatus).ExitStatus()
	} else {
		panic(err)
	}

	debug(config, "%s\n", test)

	putFailedTestsInMap(mutationFile, test)

	execExitCode = determinePassOrFail(config, diff, mutationFile, execExitCode)

	return execExitCode
}

func determinePassOrFail(config *MutationConfig, diff []byte, mutationFile string, execExitCode int) (int) {
	switch execExitCode {
	case 0: // Tests passed -> FAIL
		fmt.Printf("%s\n", diff)


		return execFailed
		liveMutants = append(liveMutants, mutationFile)
	case 1: // Tests failed -> PASS
		debug(config,"%s\n", diff)

		return execPassed
	case 2: // Did not compile -> SKIP
		debug(config, "Mutation did not compile")
		debug(config, "%s\n", diff)

		return execSkipped
	default: // Unknown exit code -> SKIP
		fmt.Println("Unknown exit code")
		fmt.Printf("%s\n", diff)
	}
	return execExitCode
}

func customMutateExec(config *MutationConfig, pkg *types.Package, file string, mutationFile string) (execExitCode int) {
	debug(config, "Execute custom exec command for mutation")

	diff, err := exec.Command("diff", "-u", file, mutationFile).CombinedOutput()
	if err == nil {
		execExitCode = execPassed
	} else if e, ok := err.(*exec.ExitError); ok {
		execExitCode = e.Sys().(syscall.WaitStatus).ExitStatus()
	} else {
		panic(err)
	}
	if execExitCode != execPassed && execExitCode != execFailed {
		fmt.Printf("%s\n", diff)

		panic("Could not execute diff on mutation file")
	}

	defer func() {
		_ = os.Rename(file+".tmp", file)
	}()

	err = os.Rename(file, file+".tmp")
	if err != nil {
		panic(err)
	}
	err = osutil.CopyFile(mutationFile, file)
	if err != nil {
		panic(err)
	}

	err = moveIntoMutantsFolder(config.Options.MutantFolder, mutationFile)
	if err != nil {
		panic(err)
	}

	pkgName := pkg.Path()

	test, err := exec.Command("go", "test", "-timeout", fmt.Sprintf("%ds", config.Options.Timeout), pkgName).CombinedOutput()

	if err == nil {
		execExitCode = execPassed
	} else if e, ok := err.(*exec.ExitError); ok {
		execExitCode = e.Sys().(syscall.WaitStatus).ExitStatus()
	} else {
		panic(err)
	}

	debug(config, "%s\n", test)

	putFailedTestsInMap(mutationFile, test)

	execExitCode = determinePassOrFail(config, diff, mutationFile, execExitCode)

	return execExitCode
}

// TODO may not be necessary anymore
func moveIntoMutantsFolder(folder string, file string) error {
	relevantMutationFileName := regexp.MustCompile(`\/?([\w-]*\/)*([\w.-]*)`)
	matches := relevantMutationFileName.FindStringSubmatch(file)
	CAPTURING_GROUP_INDEX := 2
	prettyMutationFileName := matches[CAPTURING_GROUP_INDEX]
	fmt.Println(prettyMutationFileName)

	if _, err := os.Stat(folder); os.IsNotExist(err) {
		os.Mkdir(folder, os.ModePerm)
	}

	return osutil.CopyFile(file, folder + prettyMutationFileName)
}

func getFailedTests(output []byte) []string {
	testOutput := string(output[:])

	// use capturing group to get the name of the Test
	testNameRegex := regexp.MustCompile(`FAIL:?[\s]*([\w]*)`)
	matches := testNameRegex.FindAllStringSubmatch(testOutput, -1)

	failedTests := make([]string, 0)
	for _, match := range matches {
		// FindAllStringSubmatch puts the capturing group in 2nd position
		failedTests = append(failedTests, match[1])
	}

	return failedTests

}

var testsToMutants = make(map[string][]string)

func putFailedTestsInMap(mutationFile string, testOutput []byte) {
	failedTests := getFailedTests(testOutput)
	// if they don't fail, don't add

	if len(failedTests) == 0 {
		return
	}
	// does it have to be deduplicated? I feel like no
	testsKey := getTestKey(failedTests)
	existingMutants, exists := testsToMutants[testsKey]
	var newValue []string
	if (exists) {
		newValue = append(existingMutants, mutationFile)
	} else {
	}

	testsToMutants[testsKey] = newValue
}

func getRedundantCandidates() {
//	fmt.Println(testsToMutants)
	for _, mutants := range testsToMutants {
		if len(mutants) > 1 {
			fmt.Println(len(mutants))
			fmt.Printf("Potential duplicates: %s", mutants)
			fmt.Println(testsToMutants)
		}
	}
}

func getTestKey(tests []string) string {
	sort.Strings(tests)
	return strings.Join(tests, ", ")
}

func main() {
	//os.Exit(mainCmd(os.Args[1:]))
	 fmt.Println("Running config test instead of real program")
	test()
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
