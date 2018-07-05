#!/bin/bash
# This exec script implements
# - execution of tests against the mutated file without deleting it,
# - (executes all tests originating from the package of the mutated file),
# - and the reporting whether the mutation was killed.

## usage 
# if calling this with --exec, you can specify only one file of tests to run
# by adding it as a command-line param after invoking the script
# as well as any files it needs to build.
# e.g. $ go-mutesting --exec "/mnt/c/Users/gijin/go/src/github.com/amyjzhu/mutation-framework/scripts/exec/preserve-mutants-test-package.sh mainBad_test.go main.go" main.go
# where main.go is the file to mutate
#TODO actually properly expand arguments



DEFAULT_MUTANT_FOLDER="mutants/"
USAGE="Run as -o [(optional) mutant-output-folder] -i [src path] [(optional) test-files-and-deps]"

if [ "$1" = "help" ] || [ "$1" = "usage" ] ; then
	echo $USAGE
	exit 1
fi

while getopts "o:i:" OPTION
do 
	case $OPTION in 
	  o) MUTANT_FOLDER=$OPTARG ; shift 1 ;;
	  i) SRC_PATH=$OPTARG ; shift 1 ;;
	  \?) break;;
	esac
done

# Save the other arguments
# TODO add a trailing slash to MUTANT_FOLDER
if [ -z "$MUTANT_FOLDER" ]; then MUTANT_FOLDER="$DEFAULT_MUTANT_FOLDER"; fi
echo "Mutant folder is $MUTANT_FOLDER"

if [ -z ${SRC_PATH+x} ]; then echo "SRC_PATH is not set"; exit 1; fi
if [ -z ${MUTATE_CHANGED+x} ]; then echo "MUTATE_CHANGED is not set"; exit 1; fi
if [ -z ${MUTATE_ORIGINAL+x} ]; then echo "MUTATE_ORIGINAL is not set"; exit 1; fi
if [ -z ${MUTATE_PACKAGE+x} ]; then echo "MUTATE_PACKAGE is not set"; exit 1; fi


# set the name to something human-readable
# todo proper checking for empty
MUTATE_CHANGED_NAME="${MUTATE_CHANGED%/}"
MUTATE_CHANGED_NAME="${MUTATE_CHANGED_NAME##*/}"
MUTATE_CHANGED_FILE_PATH="$SRCPATH$MUTANT_FOLDER$MUTATE_CHANGED_NAME"
echo $MUTATE_CHANGED_FILE_PATH

# TODO check that the folder doesn't already exist with stuff
if [ ! -d $MUTANT_FOLDER ]; then mkdir "$MUTANT_FOLDER"; fi

function clean_up {
	if [ -f $MUTATE_ORIGINAL.tmp ];
	then
		mv $MUTATE_ORIGINAL $MUTATE_CHANGED_FILE_PATH
		mv $MUTATE_ORIGINAL.tmp $MUTATE_ORIGINAL
	fi
}

function sig_handler {
	clean_up

	exit $GOMUTESTING_RESULT
}
trap sig_handler SIGHUP SIGINT SIGTERM

export GOMUTESTING_DIFF=$(diff -u $MUTATE_ORIGINAL $MUTATE_CHANGED)
# TODO add a new option and make the script treat MUTATE_CHANGED as a name, affix directory
# problem is that actually the go program doesn't know about where your mutants are
# should be passed in directly
#export GOMUTESTING_DIFF=$(diff -u $MUTATE_ORIGINAL $MUTATE_CHANGED_FILE_PATH)

mv $MUTATE_ORIGINAL $MUTATE_ORIGINAL.tmp
cp $MUTATE_CHANGED $MUTATE_ORIGINAL

export MUTATE_TIMEOUT=${MUTATE_TIMEOUT:-10}

if [ -n "$TEST_RECURSIVE" ]; then
	TEST_RECURSIVE="/..."
fi

GOMUTESTING_TEST=$(go test $@ -v -timeout $(printf '%ds' $MUTATE_TIMEOUT) $MUTATE_PACKAGE$TEST_RECURSIVE 2>&1)
export GOMUTESTING_RESULT=$?

#if [ "$MUTATE_DEBUG" = true ] ; then
	echo "$GOMUTESTING_TEST"
#fi

clean_up

case $GOMUTESTING_RESULT in
0) # tests passed -> FAIL
	echo "$GOMUTESTING_DIFF"

	exit 1
	;;
1) # tests failed -> PASS
	if [ "$MUTATE_DEBUG" = true ] ; then
		echo "$GOMUTESTING_DIFF"
	fi

	exit 0
	;;
2) # did not compile -> SKIP
	if [ "$MUTATE_VERBOSE" = true ] ; then
		echo "Mutation did not compile"
	fi

	if [ "$MUTATE_DEBUG" = true ] ; then
		echo "$GOMUTESTING_DIFF"
	fi

	exit 2
	;;
*) # Unkown exit code -> SKIP
	echo "Unknown exit code"
	echo "$GOMUTESTING_DIFF"

	exit $GOMUTESTING_RESULT
	;;
esac
