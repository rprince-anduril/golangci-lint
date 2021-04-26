package golinters

import (
	"fmt"
	"sync"

	"golang.org/x/tools/go/analysis"
	"honnef.co/go/tools/unused"

	"github.com/anduril/golangci-lint/pkg/golinters/goanalysis"
	"github.com/anduril/golangci-lint/pkg/lint/linter"
	"github.com/anduril/golangci-lint/pkg/result"
)

func NewUnused() *goanalysis.Linter {
	const name = "unused"

	var mu sync.Mutex
	var resIssues []goanalysis.Issue

	analyzer := &analysis.Analyzer{
		Name:     name,
		Doc:      unused.Analyzer.Doc,
		Requires: unused.Analyzer.Requires,
		Run: func(pass *analysis.Pass) (interface{}, error) {
			res, err := unused.Analyzer.Run(pass)
			if err != nil {
				return nil, err
			}

			sr := unused.Serialize(pass, res.(unused.Result), pass.Fset)

			var issues []goanalysis.Issue
			for _, object := range sr.Unused {
				issue := goanalysis.NewIssue(&result.Issue{
					FromLinter: name,
					Text:       fmt.Sprintf("%s %s is unused", object.Kind, object.Name),
					Pos:        object.Position,
				}, pass)

				issues = append(issues, issue)
			}

			mu.Lock()
			resIssues = append(resIssues, issues...)
			mu.Unlock()

			return nil, nil
		},
	}

	analyzers := []*analysis.Analyzer{analyzer}
	setAnalyzersGoVersion(analyzers)

	lnt := goanalysis.NewLinter(
		name,
		"Checks Go code for unused constants, variables, functions and types",
		analyzers,
		nil,
	).WithIssuesReporter(func(lintCtx *linter.Context) []goanalysis.Issue {
		return resIssues
	}).WithLoadMode(goanalysis.LoadModeTypesInfo)

	return lnt
}
