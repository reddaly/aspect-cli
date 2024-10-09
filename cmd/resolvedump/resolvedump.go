package main

import (
	"flag"
	"fmt"
	"iter"
	"os"
	"strings"

	"aspect.build/gazelle/gazelle/common/treesitter"
	"aspect.build/gazelle/gazelle/common/treesitter/grammars/kotlin"
	"aspect.build/gazelle/internal/logger"
	"github.com/goreleaser/fileglob"
	sitter "github.com/smacker/go-tree-sitter"
)

var (
	pattern = flag.String("pattern", "/foo/bar/*", "Glob pattern to match files")
)

func main() {
	flag.Parse()
	if err := mainErr(); err != nil {
		logger.Errorf("failed with error %v", err)
		os.Exit(1)
	}
}

func mainErr() error {
	files, err := fileglob.Glob(*pattern, fileglob.MaybeRootFS)
	if err != nil {
		return err
	}

	logger.Infof("%q matched %d files", *pattern, len(files))

	errCount := 0
	for _, f := range files {
		report, err := analyzeFile(f)
		if err != nil {
			logger.Errorf("error analyzing file %s: %v", f, err)
			errCount++
			continue
		}
		if report.ParseError != nil {
			errCount++
			logger.Errorf("Parse error for %s:\n%v", f, report.ParseError)
		}

		if len(report.TopLevelIdentifiers) != 0 {
			//logger.Infof("%s has top level identifiers: %v; package %s", f, report.TopLevelIdentifiers, report.Package)
		}
	}
	logger.Infof("%d total files, %d errors", len(files), errCount)

	return nil
}

type analysis struct {
	ParseError          error
	Package             string
	TopLevelIdentifiers []string
}

func (a *analysis) gazelleResolveComments() []string {
	if a.Package == "" {
		return nil
	}
	var out []string
	for _, id := range a.TopLevelIdentifiers {
		// # gazelle:resolve source-lang import-lang import-string label
		out = append(out, fmt.Sprintf("# gazelle:resolve kotlin kotlin %s.%s TODO_LABEL", a.Package, id))
	}
	return out
}

func analyzeFile(path string) (*analysis, error) {
	sourceCode, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", err)
	}

	tree, err := treesitter.ParseSourceCode(treesitter.Kotlin, path, sourceCode)
	if err != nil {
		return &analysis{
			ParseError: err,
		}, nil
	}

	rootNode := tree.(treesitter.TreeAst).SitterTree.RootNode()

	topLevelIdentifiers := []string{}

	for m := range matches(topLevelIdentifierQuery, rootNode) {
		//logger.Infof("CAPTURES = %v", m.Captures)
		topLevelIdentifiers = append(topLevelIdentifiers, m.Captures[0].Node.Content(sourceCode))
	}

	var parseErr error
	parseErrs := collectParseErrors(rootNode, sourceCode)

	pkg, err := extractPackage(rootNode, sourceCode)
	if err != nil {
		parseErrs = append(parseErrs, err.Error())
	}

	if len(parseErrs) > 0 {
		parseErr = fmt.Errorf("%d parse error(s):\n%s", len(parseErrs), strings.Join(parseErrs, "\n"))
	}

	return &analysis{
		ParseError:          parseErr,
		TopLevelIdentifiers: topLevelIdentifiers,
		Package:             pkg,
	}, nil
}

func extractPackage(rootNode *sitter.Node, sourceCode []byte) (string, error) {
	parts := []string{}
	for m := range matches(packageIdentifierQuery, rootNode) {
		parts = append(parts, m.Captures[0].Node.Content(sourceCode))
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("source file doesn't have a package statement")
	}
	return strings.Join(parts, "."), nil
}

func collectParseErrors(rootNode *sitter.Node, sourceCode []byte) []string {
	var errs []string
	for m := range matches(errQuery, rootNode) {
		at := m.Captures[0].Node
		atStart := at.StartPoint()
		show := at

		// Navigate up the AST to include the full source line
		if atStart.Column > 0 {
			for show.StartPoint().Row > 0 && show.StartPoint().Row == atStart.Row {
				show = show.Parent()
			}
		}

		// Extract only that line from the parent Node
		lineI := int(atStart.Row - show.StartPoint().Row)
		colI := int(atStart.Column)
		line := strings.Split(show.Content(sourceCode), "\n")[lineI]

		pre := fmt.Sprintf("     %d: ", atStart.Row+1)
		msg := pre + line
		arw := strings.Repeat(" ", len(pre)+colI) + "^"

		errs = append(errs, fmt.Sprintf(msg+"\n"+arw))
	}
	return errs
}

// Queries created with the help of running
// ./node_modules/.bin/tree-sitter build-wasm and
// ./node_modules/.bin/tree-sitter playground
var (
	errQuery = mustNewQuery("(ERROR) @error")

	packageIdentifierQuery = mustNewQuery(`
(source_file
	(package_header (identifier (simple_identifier) @id)))
`)

	// https://kotlinlang.org/spec/syntax-and-grammar.html#grammar-rule-topLevelObject
	topLevelIdentifierQuery = mustNewQuery(`
	(source_file
		(property_declaration
			(variable_declaration) @topLevelIdentifier))

	(source_file
		(function_declaration
			(simple_identifier) @functionId))

	(source_file
		(class_declaration
			(type_identifier) @classId))

	(source_file
		(type_alias
			(type_identifier) @typeId))

		 (source_file
			(object_declaration
				(type_identifier) @typeId))
	`)
)

func mustNewQuery(query string) *sitter.Query {
	q, err := sitter.NewQuery([]byte(query), kotlin.GetLanguage())
	if err != nil {
		panic(err)
	}
	return q
}

func matches(query *sitter.Query, node *sitter.Node) iter.Seq[*sitter.QueryMatch] {
	return func(yield func(*sitter.QueryMatch) bool) {
		qc := sitter.NewQueryCursor()
		defer qc.Close()
		qc.Exec(query, node)
		for {
			m, ok := qc.NextMatch()
			if !ok {
				break
			}
			if !yield(m) {
				break
			}
		}
	}
}
