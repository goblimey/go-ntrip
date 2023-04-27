#! /bin/bash

# This script runs a complete test with coverage of the Go code
# within the current directory and its subdirectories.  It scans
# the files, descending recursively into any directories.  If 
# it finds a file with a name ending ".go" it runs a test with
# coverage.  If there are test files in the directory, they
# will be run.  If not, a coverage report will be produced
# showing zero coverage, even if the code is covered by tests
# in other directories.
#
# The result is a report of the tests including any any errors on
# the standard output channel plus a set of test coverage reports,
# one per directory, each displayed in a separate tab in your
# default browser.
#
# Note: the Go compiler copes with a directory containing many
# .go files - it merges them together to form one package.
# However, I've noticed that the resulting test coverage report 
# only displays one of the source files, so if the local tests
# don't cover all of the code, you can't always see the lines
# which are not covered.  (This appears to be the case with 
# Go 1.20.)


# Remove any coverage files from previous runs.  Later we
# use the existence of a coverage file to avoid multiple test
# runs.

find . -name 'coverage.out' -exec rm {} \;


# Find all directories containing .go files and run tester on
# each.  if there are files foo.go and foo_test.go, tester will
# be run twice but the second run will see the test coverage 
# file from the first run and do nothing.

find . -name '*.go' -exec bash -c 'dir=`dirname {}`; cd $dir; pwd; if test ! -f coverage.out; then echo `basename {}`; go test -coverprofile=coverage.out; go tool cover -html=coverage.out; fi' \;

# descend foo descends into directory foo, runs tester, then descends into any 
# subdirectories.
descend() {

echo cd $1
	cd $1

	tester

echo descend  *
	for i in *
	do
		if test -d $i
		then
echo descend $i
			descend $i
		fi
	done

	cd ..
	echo descend done `pwd`
}


# tester runs any tests and produces the test coverage report.
tester() {

pwd
		# If there is already a coverage report, do nothing.
		if test ! -f coverage.out
		then
			echo go test -coverprofile=coverage.out
			echo go tool cover -html=coverage.out
		fi
}
