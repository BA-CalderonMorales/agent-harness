// Command complexity-report produces a repeatable complexity snapshot of
// the repository: per-file size, per-function length and cyclomatic
// complexity, and per-package aggregates.
//
// It is a reporting tool, not a gate. Run it from the repository root:
//
//	cd scripts/verify/complexity-report && go run . -root ../../..
//
// or `make complexity`. With -strict it exits non-zero when a source file
// exceeds -budget lines or a function exceeds -max-complexity, which is
// how a future change can ratchet the thresholds without a bespoke script.
//
// Tests are excluded from the source metrics and counted separately: the
// question this answers is how hard the shipping code is to change.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type funcStat struct {
	File       string `json:"file"`
	Package    string `json:"package"`
	Name       string `json:"name"`
	Lines      int    `json:"lines"`
	Complexity int    `json:"complexity"`
	Exported   bool   `json:"exported"`
}

type pkgStat struct {
	Files         int `json:"files"`
	Lines         int `json:"lines"`
	Funcs         int `json:"funcs"`
	MaxComplexity int `json:"max_complexity"`
	SumComplexity int `json:"sum_complexity"`
	OverBudget    int `json:"over_budget"`
	TestFiles     int `json:"test_files"`
	TestLines     int `json:"test_lines"`
	ExportedFuncs int `json:"exported_funcs"`
}

type report struct {
	TotalSourceFiles  int                 `json:"total_source_files"`
	TotalSourceLines  int                 `json:"total_source_lines"`
	TotalTestLines    int                 `json:"total_test_lines"`
	Functions         int                 `json:"functions"`
	Packages          map[string]*pkgStat `json:"packages"`
	TopComplexity     []funcStat          `json:"top_complexity"`
	TopLines          []funcStat          `json:"top_lines"`
	OverBudgetFiles   []fileStat          `json:"over_budget_files"`
	ComplexityBuckets map[string]int      `json:"complexity_buckets"`
}

type fileStat struct {
	File  string `json:"file"`
	Lines int    `json:"lines"`
}

func main() {
	root := flag.String("root", ".", "repository root")
	top := flag.Int("top", 20, "rows in the ranked lists")
	budget := flag.Int("budget", 400, "soft line budget for a source file")
	maxComplexity := flag.Int("max-complexity", 0, "fail when a function exceeds this complexity (0 disables)")
	strict := flag.Bool("strict", false, "fail on over-budget files as well as over-complex functions")
	asJSON := flag.Bool("json", false, "emit JSON instead of a text report")
	flag.Parse()

	rep, err := analyze(*root, *top, *budget)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else {
		printReport(rep, *top)
	}

	failed := false
	if *maxComplexity > 0 {
		for _, f := range rep.TopComplexity {
			if f.Complexity > *maxComplexity {
				fmt.Fprintf(os.Stderr, "complexity gate: %s has complexity %d (limit %d)\n", f.Name, f.Complexity, *maxComplexity)
				failed = true
			}
		}
	}
	if *strict && len(rep.OverBudgetFiles) > 0 {
		for _, f := range rep.OverBudgetFiles {
			fmt.Fprintf(os.Stderr, "size gate: %s has %d lines (budget %d)\n", f.File, f.Lines, *budget)
		}
		failed = true
	}
	if failed {
		os.Exit(1)
	}
}

func analyze(root string, top, budget int) (*report, error) {
	fset := token.NewFileSet()
	rep := &report{Packages: map[string]*pkgStat{}}
	var funcs []funcStat

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "build", "node_modules", ".gitnexus", "vendor":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		lines := strings.Count(string(src), "\n")
		pkg := filepath.Dir(rel)
		ps := rep.Packages[pkg]
		if ps == nil {
			ps = &pkgStat{}
			rep.Packages[pkg] = ps
		}

		if strings.HasSuffix(path, "_test.go") {
			ps.TestFiles++
			ps.TestLines += lines
			rep.TotalTestLines += lines
			return nil
		}

		rep.TotalSourceFiles++
		rep.TotalSourceLines += lines
		ps.Files++
		ps.Lines += lines
		if lines > budget {
			ps.OverBudget++
			rep.OverBudgetFiles = append(rep.OverBudgetFiles, fileStat{File: rel, Lines: lines})
		}

		f, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			return err
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			start := fset.Position(fn.Pos()).Line
			end := fset.Position(fn.End()).Line
			cx := cyclomatic(fn.Body)
			funcs = append(funcs, funcStat{
				File:       rel,
				Package:    pkg,
				Name:       fn.Name.Name,
				Lines:      end - start + 1,
				Complexity: cx,
				Exported:   fn.Name.IsExported(),
			})
			ps.Funcs++
			ps.SumComplexity += cx
			if cx > ps.MaxComplexity {
				ps.MaxComplexity = cx
			}
			if fn.Name.IsExported() {
				ps.ExportedFuncs++
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	rep.Functions = len(funcs)
	rep.ComplexityBuckets = map[string]int{"1-5": 0, "6-10": 0, "11-15": 0, "16-20": 0, "21+": 0}
	for _, f := range funcs {
		switch {
		case f.Complexity <= 5:
			rep.ComplexityBuckets["1-5"]++
		case f.Complexity <= 10:
			rep.ComplexityBuckets["6-10"]++
		case f.Complexity <= 15:
			rep.ComplexityBuckets["11-15"]++
		case f.Complexity <= 20:
			rep.ComplexityBuckets["16-20"]++
		default:
			rep.ComplexityBuckets["21+"]++
		}
	}
	rep.TopComplexity = append([]funcStat(nil), funcs...)
	sort.Slice(rep.TopComplexity, func(i, j int) bool {
		if rep.TopComplexity[i].Complexity != rep.TopComplexity[j].Complexity {
			return rep.TopComplexity[i].Complexity > rep.TopComplexity[j].Complexity
		}
		return rep.TopComplexity[i].Lines > rep.TopComplexity[j].Lines
	})
	rep.TopComplexity = head(rep.TopComplexity, top)

	rep.TopLines = append([]funcStat(nil), funcs...)
	sort.Slice(rep.TopLines, func(i, j int) bool { return rep.TopLines[i].Lines > rep.TopLines[j].Lines })
	rep.TopLines = head(rep.TopLines, top)

	sort.Slice(rep.OverBudgetFiles, func(i, j int) bool {
		return rep.OverBudgetFiles[i].Lines > rep.OverBudgetFiles[j].Lines
	})
	return rep, nil
}

func printReport(rep *report, top int) {
	fmt.Printf("source: %d files, %d lines, %d functions, %d packages\n", rep.TotalSourceFiles, rep.TotalSourceLines, rep.Functions, len(rep.Packages))
	fmt.Printf("tests:  %d lines (%.2fx source)\n\n", rep.TotalTestLines, ratio(rep.TotalTestLines, rep.TotalSourceLines))

	fmt.Printf("== top %d by cyclomatic complexity ==\n", top)
	for _, f := range rep.TopComplexity {
		fmt.Printf("%4d  %4d lines  %-38s %s\n", f.Complexity, f.Lines, f.Name, f.File)
	}

	fmt.Printf("\n== top %d by function length ==\n", top)
	for _, f := range rep.TopLines {
		fmt.Printf("%4d lines  cx=%-4d %-38s %s\n", f.Lines, f.Complexity, f.Name, f.File)
	}

	pkgs := make([]string, 0, len(rep.Packages))
	for name := range rep.Packages {
		pkgs = append(pkgs, name)
	}
	sort.Slice(pkgs, func(i, j int) bool { return rep.Packages[pkgs[i]].Lines > rep.Packages[pkgs[j]].Lines })

	fmt.Printf("\n== complexity distribution ==\n")
	for _, band := range []string{"1-5", "6-10", "11-15", "16-20", "21+"} {
		fmt.Printf("%-6s %d\n", band, rep.ComplexityBuckets[band])
	}

	fmt.Printf("\n== packages by source lines ==\n")
	fmt.Printf("%-58s %5s %7s %6s %6s %5s\n", "package", "files", "lines", "funcs", "maxcx", "over")
	for _, name := range pkgs {
		p := rep.Packages[name]
		fmt.Printf("%-58s %5d %7d %6d %6d %5d\n", name, p.Files, p.Lines, p.Funcs, p.MaxComplexity, p.OverBudget)
	}

	if len(rep.OverBudgetFiles) > 0 {
		fmt.Printf("\n== source files over 400 lines ==\n")
		for _, f := range rep.OverBudgetFiles {
			fmt.Printf("%5d  %s\n", f.Lines, f.File)
		}
	}
}

func cyclomatic(body *ast.BlockStmt) int {
	cx := 1
	ast.Inspect(body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause:
			cx++
		case *ast.BinaryExpr:
			if v.Op == token.LAND || v.Op == token.LOR {
				cx++
			}
		}
		return true
	})
	return cx
}

func head[T any](items []T, n int) []T {
	if n > len(items) {
		n = len(items)
	}
	return items[:n]
}

func ratio(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}
