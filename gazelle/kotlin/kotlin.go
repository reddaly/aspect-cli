package gazelle

import (
	"iter"
	"maps"
	"path"
	"strings"

	"aspect.build/gazelle/gazelle/kotlin/parser"
	jvm_java "github.com/bazel-contrib/rules_jvm/java/gazelle/private/java"
	jvm_types "github.com/bazel-contrib/rules_jvm/java/gazelle/private/types"
)

// IsNativeImport reports if the import literal is a native Kotlin or Java import.
func IsNativeImport(impt string) bool {
	if strings.HasPrefix(impt, "kotlin.") || strings.HasPrefix(impt, "kotlinx.") {
		return true
	}

	// Java native/standard libraries
	if jvm_java.IsStdlib(jvm_types.NewPackageName(impt)) {
		return true
	}

	return false
}

type KotlinTarget struct {
	Imports map[string]*ImportStatement
}

func (t *KotlinTarget) addImport(impt *ImportStatement) {
	t.Imports[impt.ImportHeader.Identifier().Literal()] = impt
}

func (t *KotlinTarget) importsSeq() iter.Seq[*ImportStatement] {
	return maps.Values(t.Imports)
}

/**
 * Information for kotlin library target including:
 * - kotlin files
 * - kotlin import statements from all files
 * - kotlin packages implemented
 */
type KotlinLibTarget struct {
	KotlinTarget

	Packages map[string]*parser.Identifier
	Files    map[string]struct{}
}

func (t *KotlinLibTarget) addFile(file string) {
	t.Files[file] = struct{}{}
}

func (t *KotlinLibTarget) addPackage(pkg *parser.Identifier) {
	t.Packages[pkg.Literal()] = pkg
}

func NewKotlinLibTarget() *KotlinLibTarget {
	return &KotlinLibTarget{
		KotlinTarget: KotlinTarget{
			Imports: make(map[string]*ImportStatement),
		},
		Packages: make(map[string]*parser.Identifier),
		Files:    make(map[string]struct{}),
	}
}

/**
 * Information for kotlin binary (main() method) including:
 * - kotlin import statements from all files
 * - the package
 * - the file
 */
type KotlinBinTarget struct {
	KotlinTarget

	File    string
	Package *parser.Identifier
}

func NewKotlinBinTarget(file string, pkg *parser.Identifier) *KotlinBinTarget {
	return &KotlinBinTarget{
		KotlinTarget: KotlinTarget{
			Imports: make(map[string]*ImportStatement),
		},
		File:    file,
		Package: pkg,
	}
}

// packagesKey is the name of a private attribute set on generated kt_library
// rules. This attribute contains the KotlinTarget for the target.
const packagesKey = "_kotlin_package"

func toBinaryTargetName(mainFile string) string {
	base := strings.ToLower(strings.TrimSuffix(path.Base(mainFile), path.Ext(mainFile)))

	// TODO: move target name template to directive
	return base + "_bin"
}
