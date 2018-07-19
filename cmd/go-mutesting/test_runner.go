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
	log "github.com/sirupsen/logrus"
)

func printStats(config *MutationConfig, stats *mutationStats) {
	if !config.Test.Disable {
		// TODO parameterize
		log.Info(
			fmt.Sprintf("The mutation score is %f (%d passed, %d failed, %d duplicated, %d skipped, total is %d)",
				stats.Score(), stats.passed, stats.failed, stats.duplicated, stats.skipped, stats.Total()))
	} else {
		log.Info("Cannot do a mutation testing summary since no exec command was executed.")
	}
}

var liveMutants = make([]string, 0)

func runAllMutantsInFolder(config *MutationConfig, folder string) {

}

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
		execExitCode := oneMutantRunTests(config, mutantInfo.pkg,
			mutantInfo.originalFile, mutantInfo.mutantDirPath, mutantInfo.mutationFile)

		log.WithField("exit_code", execExitCode).Debug("Finished running tests.")

		msg := fmt.Sprintf("%q with checksum %s", mutantInfo.mutationFile, mutantInfo.checksum)

		switch execExitCode {
		case execPassed:
			log.Info(fmt.Sprintf("PASS %s", msg))

			stats.passed++
		case execFailed:
			log.Info(fmt.Sprintf("FAIL %s", msg))

			stats.failed++
		case execSkipped:
			log.Info(fmt.Sprintf("SKIP %s", msg))

			stats.skipped++
		default:
			log.Info(fmt.Sprintf("UNKNOWN exit code for %s", msg))
		}

		return execExitCode
	}

	return returnOk
}

func oneMutantRunTests(config *MutationConfig, pkg *types.Package, originalFilePath string, file string, mutationFile string) (execExitCode int) {
	if config.Commands.Test != "" {
		return customTestMutateExec(config, originalFilePath, file, mutationFile, config.Commands.Test)
	}

	return customMutateExec(config, pkg, file, mutationFile)
}

func customTestMutateExec(config *MutationConfig, originalFilePath string, dirPath string, mutationFile string, testCommand string) (execExitCode int) {
	log.WithField("command", testCommand).Debug("Executing built-in execution steps with custom test command")
	defer runCleanUpCommand(config)

	// TODO not supported by afero
	os.Chdir(dirPath)

	diff, err := exec.Command("diff", "-u", originalFilePath, mutationFile).CombinedOutput()
	if err == nil {
		execExitCode = execPassed
	} else if e, ok := err.(*exec.ExitError); ok {
		execExitCode = e.Sys().(syscall.WaitStatus).ExitStatus()
	} else {
		panic(err)
	}
	if execExitCode != execPassed && execExitCode != execFailed {
		log.Info(diff)

		panic("Could not execute diff on mutation file")
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

	log.Debug(string(test))

	putFailedTestsInMap(mutationFile, test)

	execExitCode = determinePassOrFail(config, diff, mutationFile, execExitCode)

	return execExitCode

}

func determinePassOrFail(config *MutationConfig, diff []byte, mutationFile string, execExitCode int) (int) {
	switch execExitCode {
	case 0: // Tests passed -> FAIL
		log.Info(string(diff))


		return execFailed
		liveMutants = append(liveMutants, mutationFile)
	case 1: // Tests failed -> PASS
		log.Debug(string(diff))

		return execPassed
	case 2: // Did not compile -> SKIP
		log.Debug("Mutation did not compile")
		log.Info(string(diff))

		return execSkipped
	default: // Unknown exit code -> SKIP
		log.WithField("exit_code", execExitCode).Info("Unknown exit code")
		log.Debug(string(diff))
	}
	return execExitCode
}

func customMutateExec(config *MutationConfig, pkg *types.Package, file string, mutationFile string) (execExitCode int) {
	log.Debug("Execute custom exec command for mutation")

	diff, err := exec.Command("diff", "-u", file, mutationFile).CombinedOutput()
	if err == nil {
		execExitCode = execPassed
	} else if e, ok := err.(*exec.ExitError); ok {
		execExitCode = e.Sys().(syscall.WaitStatus).ExitStatus()
	} else {
		panic(err)
	}
	if execExitCode != execPassed && execExitCode != execFailed {
		log.Info(diff)

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

	log.Debug(test)

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
	for _, mutants := range testsToMutants {
		if len(mutants) > 1 {
			log.WithField("mutants", mutants).Info("Potential duplicates")
			log.Debug(testsToMutants)
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