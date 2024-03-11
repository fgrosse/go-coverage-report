package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var usage = strings.TrimSpace(fmt.Sprintf(`
Usage: %s [OPTIONS] <OLD_COVERAGE_FILE> <NEW_COVERAGE_FILE> <CHANGED_FILES_FILE>

Parse the OLD_COVERAGE_FILE and NEW_COVERAGE_FILE and compare the coverage of the
files listed in CHANGED_FILES_FILE. The result is printed to stdout as a simple
Markdown table with emojis indicating the coverage change per package.

You can use the -prefix flag to add a prefix to all paths in the list of changed
files. This is useful to map the changed files (e.g., ["foo/my_file.go"] to their
coverage profile which uses the full package name to identify the files
(e.g., "github.com/fgrosse/example/foo/my_file.go"). Note that currently,
packages with a different name than their directory are not supported.

ARGUMENTS:
  OLD_COVERAGE_FILE   The path to the old coverage file in the format produced by go test -coverprofile
  NEW_COVERAGE_FILE   The path to the new coverage file in the same format as OLD_COVERAGE_FILE
  CHANGED_FILES_FILE  The path to the file containing the list of changed files encoded as JSON string array

OPTIONS:
`, filepath.Base(os.Args[0])))

type options struct {
	prefix string
	format string
}

func main() {
	log.SetFlags(0)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage)
		flag.PrintDefaults()
	}

	flag.String("prefix", "", "prefix to add to all paths in the JSON file of changed files")
	flag.String("format", "markdown", "output format (currently only 'markdown' is supported)")

	err := run(programArgs())
	if err != nil {
		log.Fatalln("ERROR: ", err)
	}
}

func programArgs() (oldCov, newCov, changedFile string, opts options) {
	flag.Parse()

	args := flag.Args()
	if len(args) != 3 {
		if len(args) > 0 {
			log.Printf("ERROR: Expected exactly 3 arguments but got %d\n\n", len(args))
		}
		flag.Usage()
		os.Exit(1)
	}

	opts = options{
		prefix: flag.Lookup("prefix").Value.String(),
		format: flag.Lookup("format").Value.String(),
	}

	return args[0], args[1], args[2], opts
}

func run(oldCovPath, newCovPath, changedFilesPath string, opts options) error {
	oldCov, err := ParseCoverage(oldCovPath)
	if err != nil {
		return fmt.Errorf("failed to parse old coverage: %w", err)
	}

	newCov, err := ParseCoverage(newCovPath)
	if err != nil {
		return fmt.Errorf("failed to parse new coverage: %w", err)
	}

	changedFiles, err := ParseChangedFiles(changedFilesPath, opts.prefix)
	if err != nil {
		return fmt.Errorf("failed to load changed files: %w", err)
	}

	report := NewReport(oldCov, newCov, changedFiles)
	fmt.Fprintln(os.Stdout, report.Markdown())

	return nil
}
