package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/fgrosse/go-coverage-report/pkg"
)

var usage = fmt.Sprintf(`Usage: %s [OPTIONS] <OLD_COVERAGE_FILE> <NEW_COVERAGE_FILE> <CHANGED_FILES_FILE>

Parse the OLD_COVERAGE_FILE and NEW_COVERAGE_FILE and compare the coverage of the files listed in CHANGED_FILES_FILE.
The result is printed to stdout as a simple Markdown table with emojis indicating the coverage change per package.

ARGUMENTS:
  OLD_COVERAGE_FILE   The path to the old coverage file in the format produced by go test -coverprofile
  NEW_COVERAGE_FILE   The path to the new coverage file in the same format as OLD_COVERAGE_FILE
  CHANGED_FILES_FILE  The path to the file containing the list of changed files encoded as JSON string array

OPTIONS:
`, filepath.Base(os.Args[0]))

func main() {
	log.SetFlags(0)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage)
		flag.PrintDefaults()
	}

	flag.String("format", "markdown", "output format (currently only 'markdown' is supported)")

	oldCov, newCov, changed := programArgs()
	err := run(oldCov, newCov, changed)
	if err != nil {
		log.Fatalln("ERROR: ", err)
	}
}

func programArgs() (oldCov, newCov, changedFile string) {
	flag.Parse()

	args := flag.Args()
	if len(args) != 3 {
		if len(args) > 0 {
			log.Printf("ERROR: Expected exactly 3 arguments but got %d\n\n", len(args))
		}
		flag.Usage()
		os.Exit(1)
	}

	return args[0], args[1], args[2]
}

func run(oldCovPath, newCovPath, changedFilesPath string) error {
	oldCov, err := coverage.Parse(oldCovPath)
	if err != nil {
		return fmt.Errorf("failed to parse old coverage: %w", err)
	}

	newCov, err := coverage.Parse(newCovPath)
	if err != nil {
		return fmt.Errorf("failed to parse new coverage: %w", err)
	}

	changedFiles, err := coverage.ParseChangedFiles("github.com/fgrosse/prioqueue", changedFilesPath)
	if err != nil {
		return fmt.Errorf("failed to load changed files: %w", err)
	}

	report := coverage.NewReport(oldCov, newCov, changedFiles)
	fmt.Fprintln(os.Stdout, report.Markdown())

	return nil
}
