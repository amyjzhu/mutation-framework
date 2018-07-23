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
	"path/filepath"
	"io/ioutil"
	"crypto/md5"
	"github.com/amyjzhu/mutation-framework"
)

func printStats(config *MutationConfig, stats *mutationStats) {
	// TODO show stats for different files
	if !config.Test.Disable {
		// TODO parameterize
		getRedundantCandidates()
		log.Info("Mutants killed by: ", testsToMutants)
		log.Info("Live mutants are: ", liveMutants)
		log.Info(
			fmt.Sprintf("The mutation score is %f (%d passed, %d failed, %d duplicated, %d skipped, total is %d)",
				stats.Score(), stats.passed, stats.failed, stats.duplicated, stats.skipped, stats.Total()))
	} else {
		log.Info("Cannot do a mutation testing summary since no exec command was executed.")
	}

}

var liveMutants = make([]string, 0)

func findAllMutantsInFolder(config *MutationConfig, stats *mutationStats) ([]MutantInfo, error) {
	log.Info("Finding mutants and mutant files.")
	var mutants []MutantInfo

	var findMutantsRecursive func(folder string, pathSoFar string) error

	findMutantsRecursive = func(absolutePath string, pathSoFar string) error {
		directoryContents, err := ioutil.ReadDir(absolutePath)
		if err != nil {
			return err
		}

		for _, fileInfo := range directoryContents {
			if fileInfo.IsDir() {
				if isMutant(fileInfo.Name()) {
					mutantInfo, err := createNewMutantInfo(pathSoFar, fileInfo, absolutePath, stats)
					if err != nil {
						return err
					}
					if mutantInfo != nil {
						mutants = append(mutants, *mutantInfo)
					}
				} else {
					findMutantsRecursive(appendFolder(absolutePath, fileInfo.Name()),
						appendFolder(pathSoFar, fileInfo.Name()))
				}
			}
		}

		return nil
	}

	mutationFolderAbsolutePath := config.FileBasePath + config.Mutate.MutantFolder

	err := findMutantsRecursive(mutationFolderAbsolutePath, "")
	if err != nil {
		return nil, err
	}

	return mutants, nil
}

func isMutant(candidate string) bool {
	mutantPattern := regexp.MustCompile(`([\w\-. ]+.go)[\w\-. ]*.[\d]+`)
	return mutantPattern.MatchString(filepath.Clean(candidate))
}

func createNewMutantInfo(pathSoFar string, fileInfo os.FileInfo, absPath string, stats *mutationStats) (*MutantInfo, error) {
	mutatedFileRelativePath := getMutatedFileRelativePath(pathSoFar, fileInfo.Name())
	currentPath := appendFolder(absPath, fileInfo.Name())
	mutatedFileAbsolutePath := appendFolder(currentPath, mutatedFileRelativePath)
	checksum, err := getChecksum(mutatedFileAbsolutePath)
	if err != nil {
		return nil, err
	}
	_, _, pkg, _, err := mutesting.ParseAndTypeCheckFile(mutatedFileAbsolutePath)
	if err != nil {
		stats.skipped++
		log.WithField("mutant", mutatedFileRelativePath).Info("Skip this mutant.")
		return nil, nil
	}
	log.WithField("path", mutatedFileAbsolutePath).Debug("Found mutant.")
	mutantInfo := MutantInfo{pkg, mutatedFileRelativePath,
		currentPath, mutatedFileAbsolutePath, checksum}
	return &mutantInfo, nil
}

func appendFolder(original string, folder string) string {
	if original == "" {
		return folder
	}

	return concatAddingSlashIfNeeded(original, folder)
}

func getMutatedFileRelativePath(pathSoFar string, mutantFolder string) string {
	mutantNamePattern := regexp.MustCompile(`([\w\-. ]+.go)[\w\-. ]*.[\d]+`)
	mutantName := mutantNamePattern.FindAllStringSubmatch(mutantFolder, -1)[0][1]

	return appendFolder(pathSoFar, mutantName)
}

func getChecksum(path string) (string, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}

	checksum := md5.Sum(data)
	return fmt.Sprintf("%x", checksum), nil

}

func runMutants(config *MutationConfig, mutantFiles []MutantInfo, stats *mutationStats) int {
	log.Info("Executing tests against mutants.")
	exitCode := returnOk
	for _, file := range mutantFiles {
		exitCode = runExecution(config, file, stats)
	}

	printStats(config, stats)
	return exitCode
}

func runExecution(config *MutationConfig, mutantInfo MutantInfo, stats *mutationStats) int {
	log.WithField("mutant", mutantInfo.mutationFile).Debug("Running tests.")

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

func oneMutantRunTests(config *MutationConfig, pkg *types.Package, originalFilePath string, file string, absMutationFile string) (execExitCode int) {
	_, _, _, _, err := mutesting.ParseAndTypeCheckFile(absMutationFile)
	if err != nil {
		return execSkipped
	}

	if config.Commands.Test != "" {
		return customTestMutateExec(config, originalFilePath, file, absMutationFile, config.Commands.Test)
	}

	return customMutateExec(config, pkg, file, absMutationFile)
}

func customTestMutateExec(config *MutationConfig, originalFilePath string, dirPath string, mutationFile string, testCommand string) (execExitCode int) {
	log.WithField("command", testCommand).Debug("Executing built-in execution steps with custom test command")
	defer func() {
		log.WithField("command", config.Commands.CleanUp).Info("Running clean up command.")
		runCleanUpCommand(config)
	}()

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

	log.Debug("Test output: ", string(test))

	putFailedTestsInMap(mutationFile, test)

	execExitCode = determinePassOrFail(diff, mutationFile, execExitCode)

	return execExitCode
}

func determinePassOrFail(diff []byte, mutationFile string, execExitCode int) (int) {
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

	execExitCode = determinePassOrFail(diff, mutationFile, execExitCode)

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
	if exists {
		newValue = append(existingMutants, mutationFile)
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