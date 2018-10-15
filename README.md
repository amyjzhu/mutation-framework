# go-mutesting [![GoDoc](https://godoc.org/github.com/zimmski/go-mutesting?status.png)](https://godoc.org/github.com/zimmski/go-mutesting) [![Build Status](https://travis-ci.org/zimmski/go-mutesting.svg?branch=master)](https://travis-ci.org/zimmski/go-mutesting) [![Coverage Status](https://coveralls.io/repos/amyjzhu/mutation-framework/badge.png?branch=master)](https://coveralls.io/r/zimmski/go-mutesting?branch=master)

This framework performs mutation testing, focusing specifically on distributed systems.

## Quick example

If we have a config file `config.json`, we only need to run

```bash
mutation-framework --config config.json
``` 

A sample config:
```json
{
  "test": {
    "disable": false,
    "timeout":10,
    "commands":{
      "test": "go test",
      "clean_up":""
    }
  },
  "mutate": {
    "disable": false,
    "operators": [
      "branch/case", "branch/else", "branch/if", "expression/remove", "statement/remove", "statement/timeout", "statement/removeblock"
    ],
    "files_to_include": [
      "primary.go", "secondary.go"
    ]
  },
  "verbose": false,
  "project_root":"/home/"
}
```

The framework can also be invoked with different overriding flags, such as `debug` or `list-mutators` (which prints mutators and exits). For a full list of flags, run `mutation-framework --help`.

If mutation is disabled, then all the mutants in the specified `mutant_folder` are used for execution.

If the mutant is still live after being run against the tests, the source code of the mutated file is printed out. 

```diff
for _, d := range opts.Mutator.DisableMutators {
	pattern := strings.HasSuffix(d, "*")

-	if (pattern && strings.HasPrefix(name, d[:len(d)-2])) || (!pattern && name == d) {
+	if (pattern && strings.HasPrefix(name, d[:len(d)-2])) || false {
		continue MUTATOR
	}
}
```

The example shows that the right term `(!pattern && name == d)` of the `||` operator is made irrelevant by substituting it with `false`. Since this change of the source code is not detected by the test suite, meaning the test suite did not fail, we can mark it as untested code.

The next mutation shows code from the `removeNode` method of a [linked list](https://github.com/zimmski/container/blob/master/list/linkedlist/linkedlist.go) implementation.

```diff
	}

	l.first = nil
-	l.last = nil
+
	l.len = 0
}
```

We know that the code originates from a remove method which means that the mutation introduces a leak by ignoring the removal of a reference. This can be [tested](https://github.com/zimmski/container/commit/142c3e16a249095b0d63f2b41055d17cf059f045) with [go-leaks](https://github.com/zimmski/go-leak).

## <a name="table-of-content"></a>Table of contents

- [What is mutation testing?](#what-is-mutation-testing)
- [How do I use go-mutesting?](#how-do-i-use-go-mutesting)
- [How do I write my own mutation exec commands?](#write-mutation-exec-commands)
- [Which mutators are implemented?](#list-of-mutators)
- [Other mutation testing projects and their flaws](#other-projects)
- [Can I make feature requests and report bugs and problems?](#feature-request)

## <a name="what-is-mutation-testing"></a>What is mutation testing?

Mutation testing is a form of error seeding used in order to evaluate the quality (i.e. efficacy) of test suites. A mutation operator generates a number of *mutants* from some initial artifact, and each mutant is executed against a set of tests. If 
any of the tests fail, then the mutant is *killed*; otherwise, the mutant is still *live*. This indicates that a similar bug would not be uncovered by the test suite.

The process may also be used in an auxiliary sense for fault localization as well as finding dead or duplicated code. 

## <a name="how-do-i-use-go-mutesting"></a>How do I use go-mutesting?

go-mutesting includes a binary which is go-getable.

```bash
go get -t -v github.com/amyjzhu/mutation-framework/...
```

> **Note**: This README describes only a few of the available arguments. It is therefore advisable to examine the output of the `--help` argument.

The targets of the mutation testing can be defined as arguments to the binary. Every target can be either a Go source file, a directory or a package. Directories and packages can also include the `...` wildcard pattern which will search recursively for Go source files. Test source files with the suffix `_test` are excluded, since this would interfere with the testing process most of the time.

The following example gathers all Go files which are defined by the targets and generate mutations with all available mutators of the binary.

```bash
go-mutesting parse.go example/ github.com/amyjzhu/mutation-framework/mutator/...
```

If no test command is specified, 

Mutation score

### <a name="black-list-false-positives"></a>Blacklist false positives

Mutation testing can generate many false positives since mutation algorithms do not fully understand the given source code. `early exits` are one common example. They can be implemented as optimizations and will almost always trigger a false-positive since the unoptimized code path will be used which will lead to the same result. go-mutesting is meant to be used as an addition to automatic test suites. It is therefore necessary to mark such mutations as false-positives. This is done with the `--blacklist` argument. The argument defines a file which contains in every line a MD5 checksum of a mutation. These checksums can then be used to ignore mutations.

> **Note**: The blacklist feature is currently badly implemented as a change in the original source code will change all checksums.

The example output of the [How do I use go-mutesting?](#how-do-i-use-go-mutesting) section describes a mutation `example.go.6` which has the checksum `5b1ca0cfedd786d9df136a0e042df23a`. If we want to mark this mutation as a false-positive, we simple create a file with the following content.

```
5b1ca0cfedd786d9df136a0e042df23a
```

The blacklist file, which is named `example.blacklist` in this example, can then be used to invoke go-mutesting.

```bash
go-mutesting --blacklist example.blacklist github.com/amyjzhu/mutation-framework/example
```

The execution will print the following output.

> **Note**: This output is from an older version of go-mutesting. Up to date versions of go-mutesting will have different mutations.

```diff
PASS "/tmp/go-mutesting-208240643/example.go.0" with checksum b705f4c99e6d572de509609eb0a625be
PASS "/tmp/go-mutesting-208240643/example.go.1" with checksum eb54efffc5edfc7eba2b276371b29836
PASS "/tmp/go-mutesting-208240643/example.go.2" with checksum 011df9567e5fee9bf75cbe5d5dc1c81f
--- example.go  2014-12-29 23:37:42.813320040 +0100
+++ /tmp/go-mutesting-208240643/example.go.3    2014-12-30 00:49:33.573285038 +0100
@@ -16,7 +16,7 @@
        }

        if n < 0 {
-               n = 0
+
        }

        n++
FAIL "/tmp/go-mutesting-208240643/example.go.3" with checksum 82fc14acf7b561598bfce25bf3a162a2
PASS "/tmp/go-mutesting-208240643/example.go.4" with checksum 5720f1bf404abea121feb5a50caf672c
PASS "/tmp/go-mutesting-208240643/example.go.5" with checksum d6c1b5e25241453128f9f3bf1b9e7741
PASS "/tmp/go-mutesting-208240643/example.go.8" with checksum 6928f4458787c7042c8b4505888300a6
The mutation score is 0.857143 (6 passed, 1 failed, 0 skipped, total is 7)
```

By comparing this output to the original output we can state that we now have 7 mutations instead of 8.

## <a name="write-mutation-exec-commands"></a>How do I write my own mutation exec commands?

A mutation exec command is invoked for every mutation which is necessary to test a mutation. Commands should handle at least the following phases.

1. **Setup** the source to include the mutation.
2. **Test** the source by invoking the test suite and possible other test functionality.
3. **Cleanup** all changes and remove all temporary assets.
4. **Report** if the mutation was killed.

It is important to note that each invocation should be isolated and therefore stateless. This means that an invocation must not interfere with other invocations.

A set of environment variables, which define exactly one mutation, is passed on to the command.

| Name            | Description                                                               |
| :-------------- | :------------------------------------------------------------------------ |
| MUTATE_CHANGED  | Defines the filename to the mutation of the original file.                |
| MUTATE_DEBUG    | Defines if debugging output should be printed.                            |
| MUTATE_ORIGINAL | Defines the filename to the original file which was mutated.              |
| MUTATE_PACKAGE  | Defines the import path of the origianl file.                             |
| MUTATE_TIMEOUT  | Defines a timeout which should be taken into account by the exec command. |
| MUTATE_VERBOSE  | Defines if verbose output should be printed.                              |
| TEST_RECURSIVE  | Defines if tests should be run recursively.                               |

A command must exit with an appropriate exit code.

| Exit code | Description                                                                                                   |
| :------   | :--------                                                                                                     |
| 0         | The mutation was killed. Which means that the test led to a failed test after the mutation was applied.       |
| 1         | The mutation is alive. Which means that this could be a flaw in the test suite or even in the implementation. |
| 2         | The mutation was skipped, since there are other problems e.g. compilation errors.                             |
| >2        | The mutation produced an unknown exit code which might be a flaw in the exec command.                         |

Examples for exec commands can be found in the [scripts](/scripts/exec) directory.

## <a name="list-of-mutators"></a>Which mutators are implemented?

### Branch mutators

| Name          | Description                                        |
| :------------ | :------------------------------------------------- |
| branch/case   | Empties case bodies.                               |
| branch/if     | Empties branches of `if` and `else if` statements. |
| branch/else   | Empties branches of `else` statements.             |

### Expression mutators

| Name                | Description                                    |
| :------------------ | :--------------------------------------------- |
| expression/remove   | Searches for `&&` and <code>\|\|</code> operators and makes each term of the operator irrelevant by using `true` or `false` as replacements. |

### Statement mutators

| Name                | Description                                    |
| :------------------ | :--------------------------------------------- |
| statement/remove    | Removes assignment, increment, decrement and expression statements. |

## <a name="write-mutators"></a>How do I write my own mutators?

Each mutator must implement the `Mutator` interface of the [github.com/amyjzhu/mutation-framework/mutator](https://godoc.org/github.com/amyjzhu/mutation-framework/mutator#Mutator) package. The methods of the interface are described in detail in the source code documentation.

Additionally each mutator has to be registered with the `Register` function of the [github.com/amyjzhu/mutation-framework/mutator](https://godoc.org/github.com/amyjzhu/mutation-framework/mutator#Mutator) package to make it usable by the binary.

Examples for mutators can be found in the [github.com/amyjzhu/mutation-framework/mutator](https://godoc.org/github.com/amyjzhu/mutation-framework/mutator) package and its sub-packages.

## <a name="other-projects"></a>Other mutation testing projects and their flaws

go-mutesting is not the first project to implement mutation testing for Go source code. A quick search uncovers the following projects.

- https://github.com/darkhelmet/manbearpig
- https://github.com/kisielk/mutator
- https://github.com/StefanSchroeder/Golang-Mutation-testing

All of them have significant flaws in comparison to go-mutesting:

- Only one type (or even one case) of mutation is implemented.
- Can only be used for one mutator at a time (manbearpig, Golang-Mutation-testing).
- Mutation is done by content which can lead to lots of invalid mutations (Golang-Mutation-testing).
- New mutators are not easily implemented and integrated.
- Can only be used for one package or file at a time.
- Other scenarios as `go test` cannot be applied.
- Do not properly clean up or handle fatal failures.
- No automatic tests to ensure that the algorithms are working at all.
- Uses another language (Golang-Mutation-testing).

## <a name="feature-request"></a>Can I make feature requests and report bugs and problems?

Sure, just submit an [issue via the project tracker](https://github.com/amyjzhu/mutation-framework/issues/new) and I will see what I can do. Please note that I do not guarantee to implement anything soon and bugs and problems are more important to me than new features. If you need something implemented or fixed right away you can contact me via mail <mz@nethead.at> to do contract work for you.
