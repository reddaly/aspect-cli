package parser

import (
	"fmt"
	"iter"
	"regexp"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	treeutils "aspect.build/gazelle/gazelle/common/treesitter"
)

// ParseResult holds the result of parsing a Kotlin file.
type ParseResult struct {
	// The path of the file as it was parsed to the Parse function.
	File string

	// The list of parsed import statements.
	Imports []*ImportStatement

	// Identifier for the package name as it appears in the [packageHeader]
	// of the Kotlin file.
	//
	// [packageHeader]: https://kotlinlang.org/spec/syntax-and-grammar.html#grammar-rule-packageHeader
	Package *Identifier

	// True if the file defines a main function.
	//
	// TODO: there are different ways a main function can appear in Kotlin
	//   source code. The parser should support different types and extract
	//   details the identifier within the file corresponding to any
	//   main functions that appear.
	HasMain bool
}

// ImportStatement corresponds to a single [importHeader] in a Kotlin file.
//
// [importHeader]: https://kotlinlang.org/spec/syntax-and-grammar.html#grammar-rule-importHeader
type ImportStatement struct {
	// identifier corresponds to the identifier part of the [importHeader
	// in the Kotlin spec].
	//
	// [importHeader in the Kotlin spec]: https://kotlinlang.org/spec/syntax-and-grammar.html#grammar-rule-importHeader
	identifier *Identifier

	// True if the import is a [star-import] per the Kotlin spec. That is,
	// it is an import of everything in a package.
	//
	// [star-import]: https://kotlinlang.org/spec/packages-and-imports.html#importing.
	isStarImport bool

	// The import alias, if the import contains an [importAlias], or nil if not.
	//
	// [importAlias]: https://kotlinlang.org/spec/syntax-and-grammar.html#grammar-rule-importAlias
	alias *SimpleIdentiifer
}

// Identifier returns the [Identifier] that corresponds to the identifier part
// of the [importHeader in the Kotlin spec].
//
// [importHeader in the Kotlin spec]: https://kotlinlang.org/spec/syntax-and-grammar.html#grammar-rule-importHeader
func (i *ImportStatement) Identifier() *Identifier {
	return i.identifier
}

// IsStarImport returns true if the import is a [star-import] per the Kotlin spec. That is,
// it is an import of everything in a package.
//
// [star-import]: https://kotlinlang.org/spec/packages-and-imports.html#importing.
func (i *ImportStatement) IsStarImport() bool {
	return i.isStarImport
}

// Alias returns the import alias, if the import contains an [importAlias], or nil if not.
//
// [importAlias]: https://kotlinlang.org/spec/syntax-and-grammar.html#grammar-rule-importAlias
func (i *ImportStatement) Alias() *SimpleIdentiifer {
	return i.alias
}

// Alias returns the import alias, if the import contains an [importAlias], or nil if not.
//
// [importAlias]: https://kotlinlang.org/spec/syntax-and-grammar.html#grammar-rule-importAlias
func (i *ImportStatement) String() string {
	switch {
	case i.Alias() != nil:
		return i.Identifier().Literal() + " as " + i.Alias().Literal()
	case i.IsStarImport():
		return i.Identifier().Literal() + ".*"
	default:
		return i.Identifier().Literal()
	}
}

// Identifiers is a parsed [Kotlin identifier]. Identifiers are used in
// import statements and elsewhere in Kotlin source code.
//
// [Kotlin identifier]:
// https://kotlinlang.org/spec/syntax-and-grammar.html#grammar-rule-identifier
type Identifier struct {
	parts []*SimpleIdentiifer
}

// Parent returns this identifier with the last dot-delimited name component
// removed.
//
// For an identifier like "foo.bar.baz", returns an Identifer for "foo.baz". For
// an identifier without a dot, returns nil.
func (i *Identifier) Parent() *Identifier {
	if len(i.parts) <= 1 {
		return nil
	}
	return &Identifier{i.parts[0 : len(i.parts)-1]}
}

// Literal returns the form the the [SimpleIdentiifer] as it would appears in
// Kotlin source code.
func (i *Identifier) Literal() string {
	strs := []string{}
	for _, part := range i.parts {
		strs = append(strs, part.Literal())
	}
	return strings.Join(strs, ".")
}

// Child returns this identifier with an additional component.
//
// For an identifier like "foo.bar.baz" an an argument like `NewSimpleIdentifier("zee")`,
// returns an Identifer like "foo.bar.baz.zee".
func (i *Identifier) Child(childComponent *SimpleIdentiifer) *Identifier {
	childId := &Identifier{}
	childId.parts = append(childId.parts, i.parts...)
	childId.parts = append(childId.parts, childComponent)

	return childId
}

// SimpleIdentiifer corresonds to the [simpleIdentifier] grammar rule in the Kotlin
// language specificaiton. An [Identifier] is made up of dot-delimited
// [SimpleIdentifier] instances.
//
// [simpleIdentifier]: https://kotlinlang.org/spec/syntax-and-grammar.html#grammar-rule-simpleIdentifier
type SimpleIdentiifer struct {
	literal string
}

// NewSimpleIdentifier returns a [SimpleIdentiifer] from an identifier literal.
func NewSimpleIdentifier(value string) (*SimpleIdentiifer, error) {
	if kotlinUnquotedIdentifierRegexp.MatchString(value) {
		return &SimpleIdentiifer{value}, nil
	}
	return nil, fmt.Errorf("NewSimpleIdentifier only supports identifiers that match %s; %q doesn't match", kotlinUnquotedIdentifierRegexp, value)
}

// Literal returns the form the the [SimpleIdentiifer] as it would appears in
// Kotlin source code.
func (si *SimpleIdentiifer) Literal() string {
	return si.literal
}

// Normalize returns the version of the identifier without backticks if backticks are
// included in the identifier unnecessarily.
func (si *SimpleIdentiifer) Normalize() *SimpleIdentiifer {
	if !strings.HasPrefix(si.literal, "`") {
		return si
	}
	betweenQuoteMarks := si.literal[1 : len(si.literal)-1]
	if kotlinUnquotedIdentifierRegexp.MatchString(betweenQuoteMarks) {
		return &SimpleIdentiifer{betweenQuoteMarks}
	}
	return si
}

// kotlinUnquotedIdentifierRegexp corresponds to the the [Identifier Kotlin grammar rule]
// excluding the backtick-quoted syntax.
//
// [Identifier Kotlin grammar rule]: https://kotlinlang.org/spec/syntax-and-grammar.html#grammar-rule-Identifier
var kotlinUnquotedIdentifierRegexp = regexp.MustCompile(
	//(Letter | '_') {Letter | '_' | UnicodeDigit}
	`[\p{L}_][\p{L}_\d]*`,
)

type Parser interface {
	Parse(filePath, source string) (*ParseResult, []error)
}

type treeSitterParser struct {
	Parser
}

func NewParser() Parser {
	p := treeSitterParser{}

	return &p
}

func (p *treeSitterParser) Parse(filePath, source string) (*ParseResult, []error) {
	result := &ParseResult{
		File: filePath,
	}

	errs := make([]error, 0)

	sourceCode := []byte(source)

	tree, err := treeutils.ParseSourceCode(treeutils.Kotlin, filePath, sourceCode)
	if err != nil {
		errs = append(errs, err)
	}

	if tree != nil {
		rootNode := tree.(treeutils.TreeAst).SitterTree.RootNode()

		// Extract imports from the root nodes
		for _, nodeI := range namedChildren(rootNode) {
			if nodeI.Type() == "import_list" {
				for j := 0; j < int(nodeI.NamedChildCount()); j++ {
					nodeJ := nodeI.NamedChild(j)
					if nodeJ.Type() == "import_header" {
						result.Imports = append(result.Imports, must(readImportHeader(nodeJ, result, sourceCode)))
					}
				}
			} else if nodeI.Type() == "package_header" {
				if result.Package != nil {
					// TODO - check if this error is even possible in a unit test.
					errs = append(errs, fmt.Errorf("multiple package declarations found in %q", filePath))
				} else {
					result.Package = must(readIdentifier(
						onlyNamedChildWithType(nodeI, sourceCode, "identifier"),
						sourceCode, false))
				}
			} else if nodeI.Type() == "function_declaration" {
				nodeJ := onlyNamedChildWithType(nodeI, sourceCode, "simple_identifier")
				if nodeJ.Content(sourceCode) == "main" {
					result.HasMain = true
				}
			}
		}

		treeErrors := tree.QueryErrors()
		if treeErrors != nil {
			errs = append(errs, treeErrors...)
		}
	}

	return result, errs
}

func onlyNamedChildWithType(node *sitter.Node, sourceCode []byte, typeName string) *sitter.Node {
	childrenMatching := namedChildrenWithType(node, typeName)

	switch count := len(childrenMatching); count {
	case 1:
		return childrenMatching[0]
	case 0:
		panic(fmt.Errorf("no named children of node with type %q: %s", typeName, node.Content(sourceCode)))
	default:
		panic(fmt.Errorf("%s childen of node with type %q, wanted 1: %s", count, node.Content(sourceCode)))
	}
}

func onlyChildWithType(node *sitter.Node, sourceCode []byte, typeName string) *sitter.Node {
	childrenMatching := namedChildrenWithType(node, typeName)

	switch count := len(childrenMatching); count {
	case 1:
		return childrenMatching[0]
	case 0:
		panic(fmt.Errorf("no named children of node with type %q: %s", typeName, node.Content(sourceCode)))
	default:
		panic(fmt.Errorf("%s childen of node with type %q, wanted 1: %s", count, node.Content(sourceCode)))
	}
}

// optionalOnlyChildWithType returns the first node of type typeName in
// the named children of node. Nil is returned if there is no such node.
// The function panics if multiple children exist of the given type.
func optionalOnlyChildWithType(node *sitter.Node, sourceCode []byte, typeName string) *sitter.Node {
	childrenMatching := filter(allChildren(node), func(node *sitter.Node) bool {
		return node.Type() == typeName
	})

	switch count := len(childrenMatching); count {
	case 1:
		return childrenMatching[0]
	case 0:
		return nil
	default:
		panic(fmt.Errorf("%s childen of node with type %q, wanted 1: %s", count, node.Content(sourceCode)))
	}
}

func namedChildrenWithType(node *sitter.Node, typeName string) []*sitter.Node {
	return filter(namedChildren(node), func(node *sitter.Node) bool {
		return node.Type() == typeName
	})
}

func readImportHeader(importHeaderNode *sitter.Node, result *ParseResult, sourceCode []byte) (*ImportStatement, error) {
	// identifier [.* | alias ]
	identifierNode := onlyNamedChildWithType(importHeaderNode, sourceCode, "identifier")
	identifier, err := readIdentifier(identifierNode, sourceCode, false)
	if err != nil {
		return nil, fmt.Errorf("error parsing import: failed to read identifier - %w", err)
	}

	isStar := false
	var alias *SimpleIdentiifer

	/*
		Structure of import_header for "import x.y.z.*":

		import_header: Named=true; Symbol: 158; Content: "import  x.y.z.*"
		import_header/0:import: Named=false; Symbol: 10; Content: "import"
		import_header/1:identifier: Named=true; Symbol: 309; Content: "x.y.z"
		import_header/1:identifier/0:simple_identifier: Named=true; Symbol: 308; Content: "x"
		import_header/1:identifier/1:.: Named=false; Symbol: 34; Content: "."
		import_header/1:identifier/2:simple_identifier: Named=true; Symbol: 308; Content: "y"
		import_header/1:identifier/3:.: Named=false; Symbol: 34; Content: "."
		import_header/1:identifier/4:simple_identifier: Named=true; Symbol: 308; Content: "z"
		import_header/2:.*: Named=false; Symbol: 11; Content: ".*"
	*/
	if aliasNode := optionalOnlyChildWithType(importHeaderNode, sourceCode, "import_alias"); aliasNode != nil {
		alias = (&SimpleIdentiifer{onlyNamedChildWithType(aliasNode, sourceCode, "type_identifier").Content(sourceCode)}).Normalize()
	} else if
	/*
		Structure of import_header for import com.example.foo.Bar as MyBar

		import_header: Content: "import com.example.foo.Bar as MyBar"
		import_header/0:import: Content: "import"
		import_header/1:identifier: Content: "com.example.foo.Bar"
		import_header/1:identifier/0:simple_identifier: Content: "com"
		import_header/1:identifier/1:.: Content: "."
		import_header/1:identifier/2:simple_identifier: Content: "example"
		import_header/1:identifier/3:.: Content: "."
		import_header/1:identifier/4:simple_identifier: Content: "foo"
		import_header/1:identifier/5:.: Content: "."
		import_header/1:identifier/6:simple_identifier: Content: "Bar"
		import_header/2:import_alias: Content: "as MyBar"
		import_header/2:import_alias/0:as: Content: "as"
		import_header/2:import_alias/1:type_identifier: Content: "MyBar"
	*/importStar := optionalOnlyChildWithType(importHeaderNode, sourceCode, ".*"); importStar != nil {
		isStar = true
	}

	return &ImportStatement{
		identifier,
		isStar,
		alias,
	}, nil
}

func readIdentifier(node *sitter.Node, sourceCode []byte, ignoreLast bool) (*Identifier, error) {
	if node.Type() != "identifier" {
		return nil, fmt.Errorf("readIdentifier must be passed an 'identifier' treesitter Node, got node type %q: %s", node.Type(), node.Content(sourceCode))
	}

	var parts []*SimpleIdentiifer

	var s strings.Builder

	total := int(node.NamedChildCount())
	if ignoreLast {
		total = total - 1
	}

	for c := 0; c < total; c++ {
		nodeC := node.NamedChild(c)

		switch nodeC.Type() {
		case "comment": // ignore
		case "simple_identifier":
			parts = append(parts, readSimpleIdentifier(nodeC, sourceCode))
			if s.Len() > 0 {
				s.WriteString(".")
			}
			s.WriteString(nodeC.Content(sourceCode))
		default:
			return nil, fmt.Errorf("unexpected node type %q within identifier node: %s", nodeC.Type(), node.Content(sourceCode))
		}
	}

	return &Identifier{parts}, nil
}

func readSimpleIdentifier(node *sitter.Node, sourceCode []byte) *SimpleIdentiifer {
	if node.Type() != "simple_identifier" {
		panic(fmt.Errorf("readIdentifier must be passed an 'simple_identifier' treesitter Node, got node type %q: %s", node.Type(), node.Content(sourceCode)))
	}
	return (&SimpleIdentiifer{node.Content(sourceCode)}).Normalize()
}

func must[T any](obj T, err error) T {
	if err != nil {
		panic(err)
	}
	return obj
}

func allChildren(node *sitter.Node) []*sitter.Node {
	var seq iter.Seq[*sitter.Node] = func(yield func(*sitter.Node) bool) {
		for i := 0; i < int(node.ChildCount()); i++ {
			yield(node.Child(i))
		}
	}
	return sequenceToSlice(seq)
}

func namedChildren(node *sitter.Node) []*sitter.Node {
	var seq iter.Seq[*sitter.Node] = func(yield func(*sitter.Node) bool) {
		for i := 0; i < int(node.NamedChildCount()); i++ {
			yield(node.NamedChild(i))
		}
	}
	return sequenceToSlice(seq)
}

// nodeDebugString returns the debug representation of the entire tree as a string.
func nodeDebugString(node *sitter.Node, sourceCode []byte) string {
	return nodeDebugStringRecursive(node, node.Type(), sourceCode)
}

// nodeDebugStringRecursive recursively builds the debug representation of a node and its children.
func nodeDebugStringRecursive(node *sitter.Node, path string, sourceCode []byte) string {
	var sb strings.Builder

	// Add node information to the string builder
	sb.WriteString(fmt.Sprintf("%s: Named=%v; Symbol: %v; Content: %q\n",
		path,
		node.IsNamed(),
		node.Symbol(),
		node.Content(sourceCode)))

	// Iterate over children and call debugNode recursively
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		childPath := fmt.Sprintf("%s/%d:%s", path, i, child.Type()) // Append child index and type to path.
		sb.WriteString(nodeDebugStringRecursive(child, childPath, sourceCode))
	}

	return sb.String()
}

func sequenceToSlice[T any](seq iter.Seq[T]) []T {
	var result []T
	for item := range seq {
		result = append(result, item)
	}
	return result
}

func filter[T any](slice []T, f func(T) bool) []T {
	result := []T{}
	for _, v := range slice {
		if f(v) {
			result = append(result, v)
		}
	}
	return result
}
