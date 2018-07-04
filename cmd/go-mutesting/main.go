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
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/jessevdk/go-flags"
	"github.com/zimmski/go-tool/importing"

	"github.com/zimmski/go-mutesting"
	"github.com/zimmski/go-mutesting/astutil"
	"github.com/zimmski/go-mutesting/osutil"
	"github.com/zimmski/go-mutesting/mutator"
	_ "github.com/zimmski/go-mutesting/mutator/branch"
	_ "github.com/zimmski/go-mutesting/mutator/expression"
	_ "github.com/zimmski/go-mutesting/mutator/statement"
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

type options struct {
	General struct {
		Debug                bool `long:"debug" description:"Debug log output"`
		DoNotRemoveTmpFolder bool `long:"do-not-remove-tmp-folder" description:"Do not remove the tmp folder where all mutations are saved to"`
		Help                 bool `long:"help" description:"Show this help message"`
		Verbose              bool `long:"verbose" description:"Verbose log output"`
	} `group:"General options"`

	Files struct {
		Blacklist []string `long:"blacklist" description:"List of MD5 checksums of mutations which should be ignored. Each checksum must end with a new line character."`
		ListFiles bool     `long:"list-files" description:"List found files"`
		PrintAST  bool     `long:"print-ast" description:"Print the ASTs of all given files and exit"`
	} `group:"File options"`

	Mutator struct {
		DisableMutators []string `long:"disable" description:"Disable mutator by their name or using * as a suffix pattern"`
		ListMutators    bool     `long:"list-mutators" description:"List all available mutators"`
	} `group:"Mutator options"`

	Filter struct {
		Match string `long:"match" description:"Only functions are mutated that confirm to the arguments regex"`
	} `group:"Filter options"`

	Exec struct {
		Exec    string `long:"exec" description:"Execute this command for every mutation (by default the built-in exec command is used)"`
		NoExec  bool   `long:"no-exec" description:"Skip the built-in exec command and just generate the mutations"`
		Timeout uint   `long:"exec-timeout" description:"Sets a timeout for the command execution (in seconds)" default:"10"`
		NoMutate bool  `long:"no-mutate" description:"Does not mutate the files, only executes existing mutations"`
		CustomTest bool `long:"custom-test" description:"If true, executes exec as test commend"`
	} `group:"Exec options"`

	Test struct {
		Recursive bool `long:"test-recursive" description:"Defines if the executer should test recursively"`
	} `group:"Test options"`

	Remaining struct {
		Targets []string `description:"Packages, directories and files even with patterns (by default the current directory)"`
	} `positional-args:"true" required:"true"`
}

func checkArguments(args []string, opts *options) (bool, int) {
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
	} else if opts.Mutator.ListMutators {
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

func debug(opts *options, format string, args ...interface{}) {
	if opts.General.Debug {
		fmt.Printf(format+"\n", args...)
	}
}

func verbose(opts *options, format string, args ...interface{}) {
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
	var opts = &options{}
	var mutationBlackList = map[string]struct{}{}

	if exit, exitCode := checkArguments(args, opts); exit {
		return exitCode
	}

	files := importing.FilesOfArgs(opts.Remaining.Targets)
	if len(files) == 0 {
		return exitError("Could not find any suitable Go source files")
	}

	if opts.Files.ListFiles {
		for _, file := range files {
			fmt.Println(file)
		}

		return returnOk
	} else if opts.Files.PrintAST {
		for _, file := range files {
			fmt.Println(file)

			src, _, err := mutesting.ParseFile(file)
			if err != nil {
				return exitError("Could not open file %q: %v", file, err)
			}

			mutesting.PrintWalk(src)

			fmt.Println()
		}

		return returnOk
	}

	if len(opts.Files.Blacklist) > 0 {
		for _, f := range opts.Files.Blacklist {
			c, err := ioutil.ReadFile(f)
			if err != nil {
				return exitError("Cannot read blacklist file %q: %v", f, err)
			}

			for _, line := range strings.Split(string(c), "\n") {
				if line == "" {
					continue
				}

				if len(line) != 32 {
					return exitError("%q is not a MD5 checksum", line)
				}

				mutationBlackList[line] = struct{}{}
			}
		}
	}

	var mutators []mutatorItem

	var execs []string
	if opts.Exec.Exec != "" {
		execs = strings.Split(opts.Exec.Exec, " ")
	}

	if (opts.Exec.NoMutate) {
		execWithoutMutating(opts, execs)
		return returnOk
	}

MUTATOR:
	for _, name := range mutator.List() {
		if len(opts.Mutator.DisableMutators) > 0 {
			for _, d := range opts.Mutator.DisableMutators {
				pattern := strings.HasSuffix(d, "*")

				if (pattern && strings.HasPrefix(name, d[:len(d)-2])) || (!pattern && name == d) {
					continue MUTATOR
				}
			}
		}

		debug(opts, "Enable mutator %q", name)

		m, _ := mutator.New(name)
		mutators = append(mutators, mutatorItem{
			Name:    name,
			Mutator: m,
		})
	}

	tmpDir, err := ioutil.TempDir("", "go-mutesting-")
	if err != nil {
		panic(err)
	}
	debug(opts, "Save mutations into %q", tmpDir)

	stats := &mutationStats{}

	for _, file := range files {
		debug(opts, "Mutate %q", file)

		src, fset, pkg, info, err := mutesting.ParseAndTypeCheckFile(file)
		if err != nil {
			return exitError(err.Error())
		}

		err = os.MkdirAll(tmpDir+"/"+filepath.Dir(file), 0755)
		if err != nil {
			panic(err)
		}

		tmpFile := tmpDir + "/" + file

		originalFile := fmt.Sprintf("%s.original", tmpFile)
		err = osutil.CopyFile(file, originalFile)
		if err != nil {
			panic(err)
		}
		debug(opts, "Save original into %q", originalFile)

		mutationID := 0

		if opts.Filter.Match != "" {
			m, err := regexp.Compile(opts.Filter.Match)
			if err != nil {
				return exitError("Match regex is not valid: %v", err)
			}

			for _, f := range astutil.Functions(src) {
				if m.MatchString(f.Name.Name) {
					mutationID = mutate(opts, mutators, mutationBlackList, mutationID, pkg, info, file, fset, src, f, tmpFile, execs, stats)
				}
			}
		} else {
			mutationID = mutate(opts, mutators, mutationBlackList, mutationID, pkg, info, file, fset, src, src, tmpFile, execs, stats)
		}
	}

	if !opts.General.DoNotRemoveTmpFolder {
		err = os.RemoveAll(tmpDir)
		if err != nil {
			panic(err)
		}
		debug(opts, "Remove %q", tmpDir)
	}

	printStats(opts, stats)

	return returnOk
}

func printStats(opts *options, stats *mutationStats) {
	if !opts.Exec.NoExec {
		fmt.Printf("The mutation score is %f (%d passed, %d failed, %d duplicated, %d skipped, total is %d)\n", stats.Score(), stats.passed, stats.failed, stats.duplicated, stats.skipped, stats.Total())
	} else {
		fmt.Println("Cannot do a mutation testing summary since no exec command was executed.")
	}
}

func execWithoutMutating(opts *options, execs []string)  {
	fmt.Println("Not mutating!")
	// TODO configurable
	files, err := ioutil.ReadDir("mutants/")
	if err != nil {
		panic(err)
	}

	/*ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	currentDir := filepath.Dir(ex)*/
	currentDir := "/mnt/c/Users/gijin/go/src/github.com/amyjzhu/mutation-test/"
	fmt.Println(currentDir)


	stats := &mutationStats{}
	// TODO configurable/autodetected

	// get files in mutants/ folder
	for _, file := range files {
		filename := file.Name()
		filePath := currentDir + "mutants/" + filename // TODO don't hardcode
		if ok, _ := regexp.MatchString(`.*\.go\.[\d]*`, filename); ok {
			if !opts.Exec.NoExec {
				// TODO inefficient, should hash to main file name
				_, _, pkg, _, _ := mutesting.ParseAndTypeCheckFile(filePath)
				//pkg := types.NewPackage("$GOPATH/src/bitbucket.org/bestchai/dinv/examples/mutation-ricartagrawala", "ricartagrawala")
//				pkg.SetImports([]*types.Package{&{}, })
// goddammit, I try to get out of parsing the file, but for some reason I still need to know the package why
// well, I only need the package path to pass to the command, so it should be okay...

				originalFileNamePattern := regexp.MustCompile(`(.*\.go)\.[\d]*`)
				originalFileName := originalFileNamePattern.FindStringSubmatch(filename)[1]
				fmt.Println(originalFileName)

				// TODO following code would not work for multi-package projects
				execExitCode := mutateExec(opts, pkg, currentDir + originalFileName, filePath, execs)

				debug(opts, "Exited with %d", execExitCode)

				//msg := fmt.Sprintf("%q with checksum %s", mutationFile, checksum)
				msg := fmt.Sprintf("%q with checksum %s", filename, "none haha")

				switch execExitCode {
				case 0:
					fmt.Printf("PASS %s\n", msg)

					stats.passed++
				case 1:
					fmt.Printf("FAIL %s\n", msg)

					stats.failed++
				case 2:
					fmt.Printf("SKIP %s\n", msg)

					stats.skipped++
				default:
					fmt.Printf("UNKOWN exit code for %s\n", msg)
				}
			} else {
				fmt.Println("You are not mutating the files nor executing them. What do you want?")
			}
		}
	}

	getRedundantCandidates()
	printStats(opts, stats)

}

func mutate(opts *options, mutators []mutatorItem, mutationBlackList map[string]struct{}, mutationID int, pkg *types.Package, info *types.Info, file string, fset *token.FileSet, src ast.Node, node ast.Node, tmpFile string, execs []string, stats *mutationStats) int {
	for _, m := range mutators {
		debug(opts, "Mutator %s", m.Name)

		changed := mutesting.MutateWalk(pkg, info, node, m.Mutator)

		for {
			_, ok := <-changed

			if !ok {
				break
			}

			mutationFile := fmt.Sprintf("%s.%d", tmpFile, mutationID)
			checksum, duplicate, err := saveAST(mutationBlackList, mutationFile, fset, src)
			if err != nil {
				fmt.Printf("INTERNAL ERROR %s\n", err.Error())
			} else if duplicate {
				debug(opts, "%q is a duplicate, we ignore it", mutationFile)

				stats.duplicated++
			} else {
				debug(opts, "Save mutation into %q with checksum %s", mutationFile, checksum)

				if !opts.Exec.NoExec {
					execExitCode := mutateExec(opts, pkg, file, mutationFile, execs)

					debug(opts, "Exited with %d", execExitCode)

					msg := fmt.Sprintf("%q with checksum %s", mutationFile, checksum)

					switch execExitCode {
					case 0:
						fmt.Printf("PASS %s\n", msg)

						stats.passed++
					case 1:
						fmt.Printf("FAIL %s\n", msg)

						stats.failed++
					case 2:
						fmt.Printf("SKIP %s\n", msg)

						stats.skipped++
					default:
						fmt.Printf("UNKOWN exit code for %s\n", msg)
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

var liveMutants = make([]string, 0)

func mutateExec(opts *options, pkg *types.Package, file string, mutationFile string, execs []string) (execExitCode int) {
	if len(execs) == 0 {
		//defaultMutateExec(opts, pkg, file, mutationFile)
		return customMutateExec(opts, pkg, file, mutationFile)
	}

	fmt.Println(opts.Exec.CustomTest)
	if opts.Exec.CustomTest {
		return customTestMutateExec(opts, pkg, file, mutationFile, execs)
	}

	fmt.Printf("Script mutate %s", execs)
	return scriptMutateExec(opts, pkg, file, mutationFile, execs)
}

func customTestMutateExec(opts *options, pkg *types.Package, file string, mutationFile string, execs []string) (execExitCode int) {
	const MUTATION_FOLDER = "mutants/"
	debug(opts, "Execute built-in exec command with custom test script for mutation")

	diff, err := exec.Command("diff", "-u", file, mutationFile).CombinedOutput()
	if err == nil {
		execExitCode = 0
	} else if e, ok := err.(*exec.ExitError); ok {
		execExitCode = e.Sys().(syscall.WaitStatus).ExitStatus()
	} else {
		panic(err)
	}
	if execExitCode != 0 && execExitCode != 1 {
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

	pkgName := pkg.Path()
	if opts.Test.Recursive {
		pkgName += "/..."
	}

	test, err := exec.Command(execs[0], execs[1:]...).CombinedOutput()

	if err == nil {
		execExitCode = 0
	} else if e, ok := err.(*exec.ExitError); ok {
		execExitCode = e.Sys().(syscall.WaitStatus).ExitStatus()
	} else {
		panic(err)
	}

	if opts.General.Debug {
		fmt.Printf("%s\n", test)
	}

	putFailedTestsInMap(mutationFile, test)

	execExitCode = determinePassOrFail(opts, diff, mutationFile, execExitCode)

	return execExitCode
}

func determinePassOrFail(opts *options, diff []byte, mutationFile string, execExitCode int) (int) {
	switch execExitCode {
	case 0: // Tests passed -> FAIL
		fmt.Printf("%s\n", diff)


		return 1
		liveMutants = append(liveMutants, mutationFile)
	case 1: // Tests failed -> PASS
		if opts.General.Debug {
			fmt.Printf("%s\n", diff)
		}

		return 0
	case 2: // Did not compile -> SKIP
		if opts.General.Verbose {
			fmt.Println("Mutation did not compile")
		}

		if opts.General.Debug {
			fmt.Printf("%s\n", diff)
		}
		return 2
	default: // Unknown exit code -> SKIP
		fmt.Println("Unknown exit code")
		fmt.Printf("%s\n", diff)
	}
	return execExitCode
}

func customMutateExec(opts *options, pkg *types.Package, file string, mutationFile string) (execExitCode int) {
	const MUTATION_FOLDER = "mutants/"
	debug(opts, "Execute custom exec command for mutation")

	diff, err := exec.Command("diff", "-u", file, mutationFile).CombinedOutput()
	if err == nil {
		execExitCode = 0
	} else if e, ok := err.(*exec.ExitError); ok {
		execExitCode = e.Sys().(syscall.WaitStatus).ExitStatus()
	} else {
		panic(err)
	}
	if execExitCode != 0 && execExitCode != 1 {
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

	pkgName := pkg.Path()
	if opts.Test.Recursive {
		pkgName += "/..."
	}

	test, err := exec.Command("go", "test", "-timeout", fmt.Sprintf("%ds", opts.Exec.Timeout), pkgName).CombinedOutput()

	if err == nil {
		execExitCode = 0
	} else if e, ok := err.(*exec.ExitError); ok {
		execExitCode = e.Sys().(syscall.WaitStatus).ExitStatus()
	} else {
		panic(err)
	}

	if opts.General.Debug {
		fmt.Printf("%s\n", test)
	}

	putFailedTestsInMap(mutationFile, test)

	execExitCode = determinePassOrFail(opts, diff, mutationFile, execExitCode)

	return execExitCode
}

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

func scriptMutateExec(opts *options, pkg *types.Package, file string, mutationFile string, execs []string) (execExitCode int) {
	debug(opts, "Execute %q for mutation", opts.Exec.Exec)

	execCommand := exec.Command(execs[0], execs[1:]...)

	execCommand.Stderr = os.Stderr
	//execCommand.Stdout = os.Stdout

	execCommand.Env = append(os.Environ(), []string{
		"MUTATE_CHANGED=" + mutationFile,
		fmt.Sprintf("MUTATE_DEBUG=%t", opts.General.Debug),
		"MUTATE_ORIGINAL=" + file,
		"MUTATE_PACKAGE=" + pkg.Path(),
		fmt.Sprintf("MUTATE_TIMEOUT=%d", opts.Exec.Timeout),
		fmt.Sprintf("MUTATE_VERBOSE=%t", opts.General.Verbose),
	}...)
	if opts.Test.Recursive {
		execCommand.Env = append(execCommand.Env, "TEST_RECURSIVE=true")
	}

	testOutput, err := execCommand.Output()
	/*err := execCommand.Start()
	if err != nil {
		panic(err)
	}*/

	// TODO timeout here

	//err = execCommand.Wait()

	if err == nil {
		execExitCode = 0
	} else if e, ok := err.(*exec.ExitError); ok {
		execExitCode = e.Sys().(syscall.WaitStatus).ExitStatus()
	} else {
		fmt.Println(err)
		panic(err)
	}

	fmt.Printf("%s", testOutput)
	putFailedTestsInMap(mutationFile, testOutput)
	getRedundantCandidates()

	return execExitCode
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
		newValue = []string{mutationFile}
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
	os.Exit(mainCmd(os.Args[1:]))
}

func saveAST(mutationBlackList map[string]struct{}, file string, fset *token.FileSet, node ast.Node) (string, bool, error) {
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
