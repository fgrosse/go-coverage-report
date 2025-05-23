package main

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	cov, err := ParseCoverage("testdata/01-new-coverage.txt", nil)
	require.NoError(t, err)

	assert.EqualValues(t, 102, cov.TotalStmt)
	assert.EqualValues(t, 92, cov.CoveredStmt)
	assert.EqualValues(t, 10, cov.MissedStmt)
	assert.InDelta(t, 90.196, cov.Percent(), 0.001)
}

func TestCoverage_ByPackage(t *testing.T) {
	cov, err := ParseCoverage("testdata/01-new-coverage.txt", nil)
	require.NoError(t, err)

	pkgs := cov.ByPackage()
	assert.Len(t, pkgs, 1)

	pkgCov := pkgs["github.com/fgrosse/prioqueue"]
	assert.NotNil(t, pkgCov)
	assert.EqualValues(t, 102, pkgCov.TotalStmt)
	assert.EqualValues(t, 92, pkgCov.CoveredStmt)
	assert.EqualValues(t, 10, pkgCov.MissedStmt)
}

func TestCoverage_ByPackageFiltered(t *testing.T) {

	regex := regexp.MustCompile(".*max_.*.go")
	cov, err := ParseCoverage("testdata/01-new-coverage.txt", regex)
	require.NoError(t, err)

	pkgs := cov.ByPackage()
	assert.Len(t, pkgs, 1)

	pkgCov := pkgs["github.com/fgrosse/prioqueue"]
	assert.NotNil(t, pkgCov)
	assert.EqualValues(t, 52, pkgCov.TotalStmt)
	assert.EqualValues(t, 42, pkgCov.CoveredStmt)
	assert.EqualValues(t, 10, pkgCov.MissedStmt)
}
