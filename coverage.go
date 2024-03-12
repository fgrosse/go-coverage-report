// Package coverage contains code to parse and format code coverage profiles.
package main

import (
	"path"

	"github.com/pkg/errors"
)

type Coverage struct {
	Files        map[string]*Profile
	TotalLines   int64
	CoveredLines int64
	MissedLines  int64
}

func ParseCoverage(filename string) (*Coverage, error) {
	pp, err := ParseProfiles(filename)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse profiles")
	}

	return New(pp), nil
}

func New(profiles []*Profile) *Coverage {
	cov := &Coverage{Files: map[string]*Profile{}}
	for _, p := range profiles {
		cov.add(p)
	}

	return cov
}

func (c *Coverage) add(p *Profile) {
	if p == nil {
		return
	}

	if _, ok := c.Files[p.FileName]; ok {
		// If we actually got here something went very wrong. It should never
		// happen, so it's not worth adding an error return value here.
		panic(errors.Errorf("profile for file %q already exists", p.FileName))
	}

	for _, b := range p.Blocks {
		p.TotalLines += int64(b.NumStmt)
		if b.Count > 0 {
			// If we got at least a single hit in this block we say it was covered
			p.CoveredLines += int64(b.NumStmt)
		}
	}

	p.MissedLines = p.TotalLines - p.CoveredLines

	c.Files[p.FileName] = p
	c.TotalLines += p.TotalLines
	c.CoveredLines += p.CoveredLines
	c.MissedLines += p.MissedLines
}

func (c *Coverage) Percent() float64 {
	if c.TotalLines == 0 {
		return 0
	}

	return float64(c.CoveredLines) / float64(c.TotalLines) * 100
}

func (c *Coverage) ByPackage() map[string]*Coverage {
	packages := map[string][]string{} // maps package paths to files
	for file := range c.Files {
		pkg := path.Dir(file)
		packages[pkg] = append(packages[pkg], file)
	}

	pkgCovs := make(map[string]*Coverage, len(packages))
	for pkg, files := range packages {
		var profiles []*Profile
		for _, file := range files {
			profiles = append(profiles, c.Files[file])
		}

		pkgCovs[pkg] = New(profiles)
	}

	return pkgCovs
}

func (c *Coverage) TrimPrefix(prefix string) {
	for name, cov := range c.Files {
		delete(c.Files, cov.FileName)
		cov.FileName = trimPrefix(name, prefix)
		c.Files[cov.FileName] = cov
	}
}
