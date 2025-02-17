package golinters

import (
	"bufio"
	"fmt"
	"go/token"
	"os"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/tools/go/analysis"

	"github.com/anduril/golangci-lint/pkg/golinters/goanalysis"
	"github.com/anduril/golangci-lint/pkg/lint/linter"
	"github.com/anduril/golangci-lint/pkg/result"
)

func getLLLIssuesForFile(filename string, maxLineLen int, tabSpaces string) ([]result.Issue, error) {
	var res []result.Issue

	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("can't open file %s: %s", filename, err)
	}
	defer f.Close()

	lineNumber := 1
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.Replace(line, "\t", tabSpaces, -1)
		lineLen := utf8.RuneCountInString(line)
		if lineLen > maxLineLen {
			res = append(res, result.Issue{
				Pos: token.Position{
					Filename: filename,
					Line:     lineNumber,
				},
				Text:       fmt.Sprintf("line is %d characters", lineLen),
				FromLinter: lllName,
			})
		}
		lineNumber++
	}

	if err := scanner.Err(); err != nil {
		if err == bufio.ErrTooLong && maxLineLen < bufio.MaxScanTokenSize {
			// scanner.Scan() might fail if the line is longer than bufio.MaxScanTokenSize
			// In the case where the specified maxLineLen is smaller than bufio.MaxScanTokenSize
			// we can return this line as a long line instead of returning an error.
			// The reason for this change is that this case might happen with autogenerated files
			// The go-bindata tool for instance might generate a file with a very long line.
			// In this case, as it's an auto generated file, the warning returned by lll will
			// be ignored.
			// But if we return a linter error here, and this error happens for an autogenerated
			// file the error will be discarded (fine), but all the subsequent errors for lll will
			// be discarded for other files, and we'll miss legit error.
			res = append(res, result.Issue{
				Pos: token.Position{
					Filename: filename,
					Line:     lineNumber,
					Column:   1,
				},
				Text:       fmt.Sprintf("line is more than %d characters", bufio.MaxScanTokenSize),
				FromLinter: lllName,
			})
		} else {
			return nil, fmt.Errorf("can't scan file %s: %s", filename, err)
		}
	}

	return res, nil
}

const lllName = "lll"

func NewLLL() *goanalysis.Linter {
	var mu sync.Mutex
	var resIssues []goanalysis.Issue

	analyzer := &analysis.Analyzer{
		Name: lllName,
		Doc:  goanalysis.TheOnlyanalyzerDoc,
	}
	return goanalysis.NewLinter(
		lllName,
		"Reports long lines",
		[]*analysis.Analyzer{analyzer},
		nil,
	).WithContextSetter(func(lintCtx *linter.Context) {
		analyzer.Run = func(pass *analysis.Pass) (interface{}, error) {
			var fileNames []string
			for _, f := range pass.Files {
				pos := pass.Fset.PositionFor(f.Pos(), false)
				fileNames = append(fileNames, pos.Filename)
			}

			var res []goanalysis.Issue
			spaces := strings.Repeat(" ", lintCtx.Settings().Lll.TabWidth)
			for _, f := range fileNames {
				issues, err := getLLLIssuesForFile(f, lintCtx.Settings().Lll.LineLength, spaces)
				if err != nil {
					return nil, err
				}
				for i := range issues {
					res = append(res, goanalysis.NewIssue(&issues[i], pass))
				}
			}

			if len(res) == 0 {
				return nil, nil
			}

			mu.Lock()
			resIssues = append(resIssues, res...)
			mu.Unlock()

			return nil, nil
		}
	}).WithIssuesReporter(func(*linter.Context) []goanalysis.Issue {
		return resIssues
	}).WithLoadMode(goanalysis.LoadModeSyntax)
}
