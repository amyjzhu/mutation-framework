package main

import (
	"fmt"
	"go/types"
	"os/exec"
	"syscall"
	"strings"
	"regexp"
	"os"
	"sort"

	"github.com/amyjzhu/mutation-framework/osutil"
)

func printStats(config *MutationConfig, stats *mutationStats) {
	if !config.Test.Disable {
		fmt.Printf("The mutation score is %f (%d passed, %d failed, %d duplicated, %d skipped, total is %d)\n", stats.Score(), stats.passed, stats.failed, stats.duplicated, stats.skipped, stats.Total())
	} else {
		fmt.Println("Cannot do a mutation testing summary since no exec command was executed.")
	}
}

var liveMutants = make([]string, 0)

func runMutants(config *MutationConfig, mutantFiles []MutantInfo, stats *mutationStats) int {
	exitCode := returnOk
	for _, file := range mutantFiles {
		exitCode = runExecution(config, file, stats)
	}

	printStats(config, stats)
	return exitCode
}



func runExecution(config *MutationConfig, mutantInfo MutantInfo, stats *mutationStats) int {
	if !config.Test.Disable {
		execExitCode := oneMutantRunTests(config, mutantInfo.pkg, mutantInfo.absFile, mutantInfo.mutationFile)

		debug(config, "Exited with %d", execExitCode)

		msg := fmt.Sprintf("%q with checksum %s", mutantInfo.mutationFile, mutantInfo.checksum)

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

		return execExitCode
	}

	return returnOk
}

func oneMutantRunTests(config *MutationConfig, pkg *types.Package, file string, mutationFile string) (execExitCode int) {
	if config.Commands.Test != "" {
		return customTestMutateExec(config, pkg, file, mutationFile, config.Commands.Test)
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
		_ = fs.Rename(file+".tmp", file)
	}()

	err = fs.Rename(file, file+".tmp")
	if err != nil {
		panic(err)
	}

	fmt.Printf("Mutationfile: %s, file: %s", mutationFile, file)

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
		_ = fs.Rename(file+".tmp", file)
	}()

	err = fs.Rename(file, file+".tmp")
	if err != nil {
		panic(err)
	}
	err = osutil.CopyFile(mutationFile, file)
	if err != nil {
		panic(err)
	}

	err = moveIntoMutantsFolder(config.Mutate.MutantFolder, mutationFile)
	if err != nil {
		panic(err)
	}

	pkgName := pkg.Path()

	test, err := exec.Command("go", "test", "-timeout", fmt.Sprintf("%ds", config.Test.Timeout), pkgName).CombinedOutput()

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

	if _, err := fs.Stat(folder); os.IsNotExist(err) {
		fs.Mkdir(folder, os.ModePerm)
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

func runCleanUpCommand(config *MutationConfig) {
	if config.Commands.CleanUp != "" {
		_, err := exec.Command(config.Commands.CleanUp).CombinedOutput()

		if err != nil {
			panic(err)
		}
	}
}