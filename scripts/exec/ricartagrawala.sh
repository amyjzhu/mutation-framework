#!/bin/bash
# This exec script implements
# - execution of tests against the mutated file without deleting it,
# - (executes all tests originating from the package of the mutated file),
# - and the reporting whether the mutation was killed.

## usage 
# if calling this with --exec, you can specify only one file of tests to run
# by adding it as a command-line param after invoking the script
# as well as any files it needs to build.
# e.g. $ go-mutesting --exec "/mnt/c/Users/gijin/go/src/github.com/zimmski/go-mutesting/scripts/exec/preserve-mutants-test-package.sh mainBad_test.go main.go" main.go 
# where main.go is the file to mutate
#TODO actually properly expand arguments



DEFAULT_MUTANT_FOLDER="mutants/"

# Save the other arguments
echo "Mutant folder is $MUTANT_FOLDER"
if [ -z "$MUTANT_FOLDER" ]; then MUTANT_FOLDER="$DEFAULT_MUTANT_FOLDER"; fi

if [ -z ${MUTATE_CHANGED+x} ]; then echo "MUTATE_CHANGED is not set"; exit 1; fi
if [ -z ${MUTATE_ORIGINAL+x} ]; then echo "MUTATE_ORIGINAL is not set"; exit 1; fi
if [ -z ${MUTATE_PACKAGE+x} ]; then echo "MUTATE_PACKAGE is not set"; exit 1; fi


# set the name to something human-readable
# todo proper checking for empty
MUTATE_CHANGED_NAME="${MUTATE_CHANGED%/}"
MUTATE_CHANGED_NAME="${MUTATE_CHANGED_NAME##*/}"
MUTATE_CHANGED_FILE_PATH="$MUTANT_FOLDER$MUTATE_CHANGED_NAME"
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

mv $MUTATE_ORIGINAL $MUTATE_ORIGINAL.tmp
cp $MUTATE_CHANGED $MUTATE_ORIGINAL

export MUTATE_TIMEOUT=${MUTATE_TIMEOUT:-10}

if [ -n "$TEST_RECURSIVE" ]; then
	TEST_RECURSIVE="/..."
fi

#GOMUTESTING_TEST=$(go test $@ -v -timeout $(printf '%ds' $MUTATE_TIMEOUT) $MUTATE_PACKAGE$TEST_RECURSIVE 2>&1)
echo "Running test"
#GOMUTESTING_TEST=$(bash -x $GOPATH/src/bitbucket.org/bestchai/dinv/examples/mutation-ricartagrawala/onlytest.sh)
#GOMUTESTING_TEST=$(test)
#echo $(test)
#test

#===============
    HOSTS=3
    SLEEPTIME=10

    DINV=$GOPATH/src/bitbucket.org/bestchai/dinv
    testDir=$DINV/examples/mutation-ricartagrawala
    #ricart-agrawala test cases
    function shutdown {
        kill `ps | pgrep ricart | awk '{print $1}'` > /dev/null
    }

    function setup {
        for (( i=0; i<$1; i++))
        do
            go test $1 -id=$i -hosts=$2 &
        done
    }

    function runTest {
        pids=()
        for (( i=0; i<$2; i++))
        do
            go test $testDir/test/$1 -id=$i -hosts=$2 -time=$3 >> passfail.txt &
            pids[$i]=$!
        done


        for (( i=0; i<$2; i++))
        do
            wait ${pids[$i]}
        done
    }


    function testWrapper {
        echo testing $1
        echo testing $1 >> passfail.txt
        runTest $1 $2 $3
        mkdir "$1-txt"
        mv *.txt "$1-txt/"
        echo "results of test in $1"
        cat "$1-txt/passfail.txt"
        shutdown
    }


    function runTests {
        cd $testDir/test
        testWrapper "useless_test.go" $HOSTS $SLEEPTIME
        testWrapper "hoststartup_test.go" $HOSTS $SLEEPTIME
        testWrapper "onehostonecritical_test.go" $HOSTS $SLEEPTIME
        testWrapper "onehostmanycritical_test.go" $HOSTS $SLEEPTIME
        testWrapper "allhostsonecritical_test.go" $HOSTS $SLEEPTIME
        testWrapper "allhostsmanycriticals_test.go" $HOSTS $SLEEPTIME
        testWrapper "halfhostsonecritical_test.go" $HOSTS $SLEEPTIME
        testWrapper "halfhostsmanycriticals_test.go" $HOSTS $SLEEPTIME
    #   testWrapper "useless_test.go" $HOSTS $SLEEPTIME
    }

    runTests

    if [ ! -z "$1" ] && [ "$1" = "-c" ]
    then
        rm -r $testDir/test/*/
    fi

#===============

if grep -nr  --include="passfail.txt" FAIL *; then
    #exit 1
    export GOMUTESTING_RESULT=1
else
export GOMUTESTING_RESULT=0
fi


echo "done"


echo "Done test"
echo $GOMUTESTING_TEST

#export GOMUTESTING_RESULT=$?

#if [ "$MUTATE_DEBUG" = true ] ; then
echo "$GOMUTESTING_TEST"
#fi

clean_up

echo $GOMUTESTING_RESULT
case $GOMUTESTING_RESULT in
0) # tests passed -> FAIL
	echo "$GOMUTESTING_DIFF"
    echo "Tests passed"
	exit 1
	;;
1) # tests failed -> PASS
	if [ "$MUTATE_DEBUG" = true ] ; then
		echo "$GOMUTESTING_DIFF"
	fi
    echo "tests failed"
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
	echo "Unknown exit code $GOMUTESTING_RESULT"
	echo "$GOMUTESTING_DIFF"

	exit $GOMUTESTING_RESULT
	;;
esac




function test  {
    OSTS=3
    SLEEPTIME=20

    DINV=$GOPATH/src/bitbucket.org/bestchai/dinv
    testDir=$DINV/examples/mutation-ricartagrawala
    #ricart-agrawala test cases
    function shutdown {
        kill `ps | pgrep ricart | awk '{print $1}'` > /dev/null
    }

    function setup {
        for (( i=0; i<$1; i++))
        do
            go test $1 -id=$i -hosts=$2 &
        done
    }

    function runTest {
        pids=()
        for (( i=0; i<$2; i++))
        do
            go test $testDir/test/$1 -id=$i -hosts=$2 -time=$3 >> passfail.txt &
            pids[$i]=$!
        done


        for (( i=0; i<$2; i++))
        do
            wait ${pids[$i]}
        done
    }


    function testWrapper {
        echo testing $1
        echo testing $1 >> passfail.txt
        runTest $1 $2 $3
        mkdir "$1-txt"
        mv *.txt "$1-txt/"
        echo "results of test in $1"
        cat "$1-txt/passfail.txt"
        shutdown
    }


    function runTests {
        cd $testDir/test
        testWrapper "useless_test.go" $HOSTS $SLEEPTIME
        testWrapper "hoststartup_test.go" $HOSTS $SLEEPTIME
        testWrapper "onehostonecritical_test.go" $HOSTS $SLEEPTIME
        testWrapper "onehostmanycritical_test.go" $HOSTS $SLEEPTIME
        testWrapper "allhostsonecritical_test.go" $HOSTS $SLEEPTIME
        testWrapper "allhostsmanycriticals_test.go" $HOSTS $SLEEPTIME
        testWrapper "halfhostsonecritical_test.go" $HOSTS $SLEEPTIME
        testWrapper "halfhostsmanycriticals_test.go" $HOSTS $SLEEPTIME
    #   testWrapper "useless_test.go" $HOSTS $SLEEPTIME
    }

    runTests

    if [ ! -z "$1" ] && [ "$1" = "-c" ]
    then
        rm -r $testDir/test/*/
    fi


    # calculate whether test suite failed

#    if grep -nr  --include="passfail.txt" FAIL *; then
#        #exit 1
#        export GOMUTESTING_RESULT=1
#    else
#        export GOMUTESTING_RESULT=0
#    fi
#    echo "done"
#    #exit 0
}