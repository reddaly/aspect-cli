package gazelle

import (
	"aspect.build/gazelle/gazelle/kotlin/parser"
	"github.com/bazelbuild/bazel-gazelle/resolve"
)

// ImportStatement corresponds to a single Kotlin import.
type ImportStatement struct {
	// The path of the file containing the import
	SourcePath string

	// All of the parsed import information.
	ImportHeader *parser.ImportStatement
}

// importSpecForIdentifier returns the gazelle [resolve.ImportSpec]
// for the given Kotlin [parser.Identifier], which should
// be a package path or the prefix of a package path.
func importSpecForIdentifier(id *parser.Identifier) resolve.ImportSpec {
	return resolve.ImportSpec{
		Lang: LanguageName,
		Imp:  id.Literal(),
	}
}
