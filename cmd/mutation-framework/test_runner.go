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

	log "github.com/sirupsen/logrus"
	"path/filepath"
	"crypto/md5"
	"github.com/amyjzhu/mutation-framework"
	"github.com/spf13/afero"
)


// TODO count statistics per mutant
func printStats(config *MutationConfig, allStats map[string]*mutationStats) {
	if !config.Test.Disable {
		getRedundantCandidates()
		log.Info("Mutants killed by: ", testsToMutants)
		log.Info("Live mutants are: ", liveMutants)

		// print stats for each file
		for file, stats := range allStats {
			log.WithField("file", file).
				Info(fmt.Sprintf("For this file, the mutation score is %f (%d passed, %d failed, %d duplicated, %d skipped, total is %d)",
					stats.Score(), stats.passed, stats.failed, stats.duplicated, stats.skipped, stats.Total()))
		}
	} else {
		log.Info("Cannot do a mutation testing summary since no exec command was executed.")
	}

}

// TODO doesn;t work for some reason?
var liveMutants = make([]string, 0)

func findAllMutantsInFolder(config *MutationConfig, allStats map[string]*mutationStats, filesToExec map[string]string) ([]MutantInfo, error) {
	log.Info("Finding mutants and mutant files.")
	var mutants []MutantInfo

	var findMutantsRecursive func(folder string, pathSoFar string) error
	log.Info("What's wrong")

	// look for all mutant directories
	findMutantsRecursive = func(absolutePath string, pathSoFar string) error {
		directoryContents, err := afero.ReadDir(FS, absolutePath)
		if err != nil {
			return err
		}

		for _, fileInfo := range directoryContents {
			if fileInfo.IsDir() {
				if isMutant(fileInfo.Name()) {
					// if we've found a mutant directory, collect information about it
					mutantInfo, err := createNewMutantInfo(filesToExec, pathSoFar, fileInfo, absolutePath, allStats)
					if err != nil {
						return err
					}
					if mutantInfo != nil {
						mutants = append(mutants, *mutantInfo)
					}
				} else {
					// not a mutant directory, so let's keep looking
					findMutantsRecursive(appendFolder(absolutePath, fileInfo.Name()),
						appendFolder(pathSoFar, fileInfo.Name()))
				}
			}
		}

		return nil
	}

	mutationFolderAbsolutePath := getAbsoluteMutationFolderPath(config)
	fmt.Println("mutation folder path is " + mutationFolderAbsolutePath)

	// find all mutants in the mutation folder
	err := findMutantsRecursive(mutationFolderAbsolutePath, "")
	if err != nil {
		return nil, err
	}

	//fmt.Println(mutants)
	return mutants, nil
}

// TODO make configurable mutant patterns
func isMutant(candidate string) bool {
	mutantPattern := regexp.MustCompile(`([\w\-. ]+.go)[\w\-. ]*.[\d]+`)
	return mutantPattern.MatchString(filepath.Clean(candidate))
}

func createNewMutantInfo(acceptableFiles map[string]string, pathSoFar string, fileInfo os.FileInfo,
	absPath string, allStats map[string]*mutationStats) (*MutantInfo, error) {
	// the relative file path within the project, e.g. nsqd/nsqd.go
	originalFilePath := getMutatedFileRelativePath(pathSoFar, fileInfo.Name())
	// the directory name, e.g. nsqd.go.branch-if.1
	currentPath := appendFolder(absPath, fileInfo.Name())
	// absolute path to the mutated file, e.g. .../mutants/nsqd/nsqd.go.branch-if.1/nsqd/nsqd.go
	mutatedFileAbsolutePath := appendFolder(currentPath, originalFilePath)
	checksum, err := getChecksum(mutatedFileAbsolutePath)
	if err != nil {
		return nil, err
	}

	// don't add the file to be test if it doesn't fit specified files
	if _, ok := acceptableFiles[originalFilePath]; !ok {
		return nil, err
	}

	// create a corresponding mutationStats for this file
	stats := &mutationStats{}
	allStats[originalFilePath] = stats

	// check the original file package
	_, _, pkg, _, err := mutesting.ParseAndTypeCheckFile(originalFilePath)

	log.WithField("path", mutatedFileAbsolutePath).Debug("Found mutant.")
	mutantInfo := MutantInfo{pkg, originalFilePath,
		currentPath, mutatedFileAbsolutePath, checksum}
	return &mutantInfo, nil
}

// TODO replace with path.Join
func appendFolder(original string, folder string) string {
	if original == "" || original == "."{
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
	data, err := afero.ReadFile(FS, path)
	if err != nil {
		return "", err
	}

	checksum := md5.Sum(data)
	return fmt.Sprintf("%x", checksum), nil

}

func executeAllMutants(config *MutationConfig, mutantFiles []MutantInfo, allStats map[string]*mutationStats) int {
	fmt.Println(mutantFiles)
	log.Info("Executing tests against mutants.")
	exitCode := returnOk

	// move all contents into temp file
	// maybe in the future can use go move
	mutantFolder := config.Mutate.MutantFolder
	//mutantFolder := appendFolder(config.ProjectRoot, config.Mutate.MutantFolder)
	originalArtifactFolder := appendFolder(mutantFolder, "original")
	err := moveAllContentsExceptMutantFolder(".", originalArtifactFolder, mutantFolder)
	if err != nil {
		log.Error(err)
		return returnError
	}

	// TODO actually replace back with original rather than symlink, but oh well
	defer func() int {
		/*err = os.Symlink(".", originalArtifactFolder)
		if err != nil {
			return returnError
		}
		return returnOk*/

		moveAllContentsExceptMutantFolder(originalArtifactFolder, ".", mutantFolder)
		if err != nil {
			log.Error(err)
			return returnError
		}
		// remove original
		os.RemoveAll(originalArtifactFolder)
		return returnOk
	}()

	for _, file := range mutantFiles {
		stats := allStats[file.originalFileRelativePath]
		//moveAllContentsExceptMutantFolder(file.mutantDirPathAbsPath, ".", mutantFolder)
		os.Symlink(".", file.mutantDirPathAbsPath)
		if err != nil {
			log.Error(err)
			return returnError
		}
		exitCode = executeForMutant(config, file, stats)
	}

	printStats(config, allStats)
	return exitCode
}

// Run an execution for one mutant
func executeForMutant(config *MutationConfig, mutantInfo MutantInfo, stats *mutationStats) int {
	log.WithField("mutant", mutantInfo.mutationFileAbsPath).Debug("Running tests.")

	if !config.Test.Disable {
		execExitCode := runTestsForMutant(config, mutantInfo.pkg,
			mutantInfo.originalFileRelativePath, mutantInfo.mutantDirPathAbsPath,
			mutantInfo.mutationFileAbsPath)

		log.WithField("exit_code", execExitCode).Debug("Finished running tests.")

		msg := fmt.Sprintf("%q with checksum %s", mutantInfo.mutationFileAbsPath, mutantInfo.checksum)

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

func runTestsForMutant(config *MutationConfig, pkg *types.Package, originalFilePath string, file string, absMutationFile string) (execExitCode int) {
	/* // TODO might be worthwhile to check validity before running tests, because test execution can take a long time
	_, _, _, _, err := mutesting.ParseAndTypeCheckFile(absMutationFile)
	if err != nil {
		return execSkipped
	}*/

	// TODO probably want to put the whole thing in a docker container because you're gonna mess up your commands
	runBuildCommand(config.Test.Commands.Build)
	defer func() {
		runCleanUpCommand(config)
	}()

	if config.Test.Commands.Test != "" {
		return customTestMutateExec(originalFilePath, absMutationFile, config.Test.Commands.Test)
	}

	return defaultMutateExec(config, pkg, file, absMutationFile)
}

func runBuildCommand(buildCommand string) {
	log.WithField("command", buildCommand).Info("Running build command.")

	if buildCommand != "" {
		output, err := exec.Command(buildCommand).CombinedOutput()

		log.Debug(output) // TODO out-of-order with mutation 

		if err != nil {
			panic(err)
		}
	}
}

func customTestMutateExec(originalFilePath string, mutationFile string, testCommand string) (execExitCode int) {
	log.WithField("command", testCommand).Debug("Executing tests with custom test command.")

	/*err := os.Chdir(dirPath)
	if err != nil {
		log.Error(err)
		return returnError
	}*/

	execWithArgs := strings.Split(testCommand, " ")
	execCommand := exec.Command(execWithArgs[0], execWithArgs[1:]...)

	return executeTestCommand(originalFilePath, mutationFile, execCommand)
}

func defaultMutateExec(config *MutationConfig, pkg *types.Package, file string, mutationFile string) (execExitCode int) {
	log.Debug("Execute default test command.")

//	os.Chdir(file)

	pkgName := pkg.Path()

	testCommand := exec.Command("go", "test", "-timeout", fmt.Sprintf("%ds", config.Test.Timeout), pkgName)
	return executeTestCommand(file, mutationFile, testCommand)
}

func executeTestCommand(originalFilePath string, mutationFile string, testCommand *exec.Cmd) int {
	diff, execExitCode := showDiff(originalFilePath, mutationFile)

	test, err := testCommand.CombinedOutput()

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

func showDiff(file string, mutationFile string) (diff []byte, execExitCode int) {
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

		panic("Could not performMutationTesting diff on mutation file")
	}

	return
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
	log.WithField("command", config.Test.Commands.CleanUp).Info("Running clean up command.")

	if config.Test.Commands.CleanUp != "" {
		output, err := exec.Command(config.Test.Commands.CleanUp).CombinedOutput()

		log.Debug(output)

		if err != nil {
			panic(err)
		}
	}
}