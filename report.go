package main

import (
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
)

type Report struct {
	Old, New        *Coverage
	ChangedFiles    []string
	ChangedPackages []string
}

func NewReport(oldCov, newCov *Coverage, changedFiles []string) *Report {
	return &Report{
		Old:             oldCov,
		New:             newCov,
		ChangedFiles:    changedFiles,
		ChangedPackages: changedPackages(changedFiles),
	}
}

func changedPackages(changedFiles []string) []string {
	packages := map[string]bool{}
	for _, file := range changedFiles {
		pkg := filepath.Dir(file)
		packages[pkg] = true
	}

	result := make([]string, 0, len(packages))
	for pkg := range packages {
		result = append(result, pkg)
	}

	sort.Strings(result)

	return result
}

func (r *Report) Title() string {
	oldCovPkgs := r.Old.ByPackage()
	newCovPkgs := r.New.ByPackage()

	var numDecrease, numIncrease int
	for _, pkg := range r.ChangedPackages {
		var oldPercent, newPercent float64

		if cov, ok := oldCovPkgs[pkg]; ok {
			oldPercent = cov.Percent()
		}

		if cov, ok := newCovPkgs[pkg]; ok {
			newPercent = cov.Percent()
		}

		newP := round(newPercent, 2) // do rounding here so the diff is not affected by confusing rounding errors in the third decimal place
		oldP := round(oldPercent, 2)
		switch {
		case newP > oldP:
			numIncrease++
		case newP < oldP:
			numDecrease++
		}

	}

	switch {
	case numIncrease == 0 && numDecrease == 0:
		return fmt.Sprintln("### Merging this branch will **not change** overall coverage")
	case numIncrease > 0 && numDecrease == 0:
		return fmt.Sprintln("### Merging this branch will **increase** overall coverage")
	case numIncrease == 0 && numDecrease > 0:
		return fmt.Sprintln("### Merging this branch will **decrease** overall coverage")
	default:
		return fmt.Sprintf("### Merging this branch changes the coverage (%d decrease, %d increase)\n", numDecrease, numIncrease)
	}
}

func (r *Report) Markdown() string {
	report := new(strings.Builder)

	fmt.Fprintln(report, r.Title())
	fmt.Fprintln(report, "| Impacted Packages | Coverage Δ | :robot: |")
	fmt.Fprintln(report, "|-------------------|------------|---------|")

	oldCovPkgs := r.Old.ByPackage()
	newCovPkgs := r.New.ByPackage()
	for _, pkg := range r.ChangedPackages {
		var oldPercent, newPercent float64

		if cov, ok := oldCovPkgs[pkg]; ok {
			oldPercent = cov.Percent()
		}

		if cov, ok := newCovPkgs[pkg]; ok {
			newPercent = cov.Percent()
		}

		emoji, diffStr := emojiScore(newPercent, oldPercent)
		fmt.Fprintf(report, "| %s | %.2f%% (%s) | %s |\n",
			pkg,
			newPercent,
			diffStr,
			emoji,
		)
	}

	return report.String()
}

func (r *Report) JSON() string {
	data, err := json.MarshalIndent(r, "", "    ")
	if err != nil {
		panic(err) // should never happen
	}

	return string(data)
}

func (r *Report) TrimPrefix(prefix string) {
	for i, name := range r.ChangedPackages {
		r.ChangedPackages[i] = trimPrefix(name, prefix)
	}
	for i, name := range r.ChangedFiles {
		r.ChangedFiles[i] = trimPrefix(name, prefix)
	}

	r.Old.TrimPrefix(prefix)
	r.New.TrimPrefix(prefix)
}

func trimPrefix(name, prefix string) string {
	trimmed := strings.TrimPrefix(name, prefix)
	trimmed = strings.TrimPrefix(trimmed, "/")
	if trimmed == "" {
		trimmed = "."
	}

	return trimmed
}

func round(val float64, places int) float64 {
	if val == 0 {
		return 0
	}

	pow := math.Pow10(places)
	digit := math.Round(pow * val)
	return digit / pow
}

func emojiScore(newPercent, oldPercent float64) (emoji, diffStr string) {
	diff := newPercent - oldPercent
	switch {
	case diff < -50:
		emoji = strings.Repeat(":skull: ", 5)
		diffStr = fmt.Sprintf("**%+.2f%%**", diff)
	case diff < -10:
		emoji = strings.Repeat(":skull: ", int(-diff/10))
		diffStr = fmt.Sprintf("**%+.2f%%**", diff)
	case diff < 0:
		emoji = ":thumbsdown:"
		diffStr = fmt.Sprintf("**%+.2f%%**", diff)
	case diff == 0:
		emoji = ""
		diffStr = "ø"
	case diff > 20:
		emoji = ":star2:"
		diffStr = fmt.Sprintf("**%+.2f%%**", diff)
	case diff > 10:
		emoji = ":tada:"
		diffStr = fmt.Sprintf("**%+.2f%%**", diff)
	case diff > 0:
		emoji = ":thumbsup:"
		diffStr = fmt.Sprintf("**%+.2f%%**", diff)
	}

	return emoji, diffStr
}
