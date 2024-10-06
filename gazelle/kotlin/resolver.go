package gazelle

import (
	"fmt"
	"iter"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"

	common "aspect.build/gazelle/gazelle/common"
	"aspect.build/gazelle/gazelle/kotlin/kotlinconfig"
	"aspect.build/gazelle/gazelle/kotlin/parser"
	BazelLog "aspect.build/gazelle/internal/logger"

	jvm_types "github.com/bazel-contrib/rules_jvm/java/gazelle/private/types"
)

var _ resolve.Resolver = (*kotlinLang)(nil)

const (
	Resolution_Error        = -1
	Resolution_None         = 0
	Resolution_NotFound     = 1
	Resolution_Label        = 2
	Resolution_NativeKotlin = 3
)

type ResolutionType int

func (*kotlinLang) Name() string {
	return LanguageName
}

// Determine what rule (r) outputs which can be imported.
func (kt *kotlinLang) Imports(c *config.Config, r *rule.Rule, f *rule.File) []resolve.ImportSpec {
	BazelLog.Debugf("Imports(%s): '%s:%s'", LanguageName, f.Pkg, r.Name())

	if r.PrivateAttr(packagesKey) == nil {
		return nil
	}
	target, isLib := r.PrivateAttr(packagesKey).(*KotlinLibTarget)
	if !isLib {
		return nil
	}
	var provides []resolve.ImportSpec

	for _, pkg := range target.Packages {
		provides = append(provides, resolve.ImportSpec{
			Lang: LanguageName,
			Imp:  pkg.Literal(),
		})
	}
	sort.Slice(provides, func(i, j int) bool {
		return provides[i].Imp < provides[j].Imp
	})
	return provides
}

func (kt *kotlinLang) Embeds(r *rule.Rule, from label.Label) []label.Label {
	return []label.Label{}
}

// Resolve implements the [github.com/bazelbuild/bazel-gazelle/language.Language.Resolve] function for Kotlin.
func (kt *kotlinLang) Resolve(c *config.Config, ix *resolve.RuleIndex, rc *repo.RemoteCache, r *rule.Rule, importData interface{}, from label.Label) {
	start := time.Now()
	BazelLog.Infof("Resolve(%s): //%s:%s", LanguageName, from.Pkg, r.Name())

	if r.Kind() == KtJvmLibrary || r.Kind() == KtJvmBinary || r.Kind() == KtJvmTest {
		var target KotlinTarget
		var extraDeps []*label.Label

		switch r.Kind() {
		case KtJvmLibrary:
			target = importData.(*KotlinLibTarget).KotlinTarget
		case KtJvmBinary:
			target = importData.(*KotlinBinTarget).KotlinTarget
		case KtJvmTest:
			testTarget := importData.(*KotlinTestTarget)
			target = testTarget.KotlinTarget

			importContext := func() string {
				return fmt.Sprintf("implicit test dependency on library that provides the Kotlin/Java package %q", testTarget.Package.Literal())
			}
			if resolutionType, depLabel, err := kt.resolveImport(c, ix, testTarget.Package, from, importContext); err != nil {
				log.Fatalf("error resolving library dependency of test: %v", err)
			} else if resolutionType == Resolution_Label {
				extraDeps = append(extraDeps, depLabel)
			}

		default:
			log.Fatalf("Resolve called on unknown rule kind %q", r.Kind())
		}

		deps, err := kt.resolveImports(c, ix, target.importsSeq(), from)
		if err != nil {
			log.Fatalf("Resolution Error: %v", err)
			os.Exit(1)
		}
		for _, extraDep := range extraDeps {
			deps.Add(extraDep)
		}

		if !deps.Empty() {
			r.SetAttr("deps", deps.Labels())
		}
	}

	BazelLog.Infof("Resolve(%s): //%s:%s DONE in %s", LanguageName, from.Pkg, r.Name(), time.Since(start).String())
}

// importContext is a function that describes the context of the import for
// error reporting purposes.
func (kt *kotlinLang) resolveImports(
	c *config.Config,
	ix *resolve.RuleIndex,
	imports iter.Seq[*ImportStatement],
	from label.Label,
) (*common.LabelSet, error) {
	deps := common.NewLabelSet(from)

	for impt := range imports {

		importContext := func() string {
			return fmt.Sprintf("the %q import statement in %q", impt.ImportHeader.String(), impt.SourcePath)
		}

		resolutionType, dep, err := kt.resolveImport(c, ix, impt.ImportHeader.Identifier(), from, importContext)
		if err != nil {
			return nil, err
		}

		if resolutionType == Resolution_NotFound {
			BazelLog.Debugf("import %s for target %q not found", impt.ImportHeader.String(), from.String())

			notFound := fmt.Errorf(
				"Import %[1]q from %[2]q is an unknown dependency. Possible solutions:\n"+
					"\t1. Instruct Gazelle to resolve to a known dependency using a directive:\n"+
					"\t\t# gazelle:resolve [src-lang] kotlin import-string label\n",
				impt.ImportHeader.String(), impt.SourcePath,
			)

			fmt.Printf("Resolution error %v\n", notFound)
			continue
		}

		if resolutionType == Resolution_NativeKotlin || resolutionType == Resolution_None {
			continue
		}

		if dep != nil {
			deps.Add(dep)
		}
	}

	return deps, nil
}

// resolveImport resolves the import indicated by the [*parser.Identifier] argument.
//
// fromStatement indicates the import statement from which the identifier was derived, but
// the identifier of the fromStatement may be different from identifier if this function
// is being called recursively for a parent [*parser.Identifier].
//
// importContext is a function that describes the context of the import for
// error reporting purposes.
func (kt *kotlinLang) resolveImport(
	c *config.Config,
	ix *resolve.RuleIndex,
	identifier *parser.Identifier,
	fromLabel label.Label,
	importContext func() string,
) (ResolutionType, *label.Label, error) {
	// Gazelle overrides
	// TODO: generalize into gazelle/common
	if override, ok := resolve.FindRuleWithOverride(c, importSpecForIdentifier(identifier), LanguageName); ok {
		return Resolution_Label, &override, nil
	}

	// TODO: generalize into gazelle/common
	if matches := ix.FindRulesByImportWithConfig(c, importSpecForIdentifier(identifier), LanguageName); len(matches) > 0 {
		filteredMatches := make([]label.Label, 0, len(matches))
		for _, match := range matches {
			// Prevent from adding itself as a dependency.
			if !match.IsSelfImport(fromLabel) {
				filteredMatches = append(filteredMatches, match.Label)
			}
		}

		// Too many results, don't know which is correct
		if len(filteredMatches) > 1 {
			return Resolution_Error, nil, fmt.Errorf(
				"Importing identifier %q (from %s) resolved to multiple targets (%s)"+
					" - this must be fixed using the \"gazelle:resolve\" directive",
				identifier.Literal(),
				importContext(),
				targetListFromResults(matches))
		}

		// The matches were self imports, no dependency is needed
		if len(filteredMatches) == 0 {
			return Resolution_None, nil, nil
		}

		match := filteredMatches[0]

		return Resolution_Label, &match, nil
	}

	// Native kotlin imports
	if IsNativeImport(identifier.Literal()) {
		return Resolution_NativeKotlin, nil, nil
	}

	cfgs := c.Exts[LanguageName].(kotlinconfig.Configs)
	cfg, _ := cfgs[fromLabel.Pkg]

	// Maven imports
	if mavenResolver := kt.mavenResolver; mavenResolver != nil {
		if l, mavenError := (*mavenResolver).Resolve(jvm_types.NewPackageName(identifier.Literal()), cfg.JavaConfig().ExcludedArtifacts(), cfg.JavaConfig().MavenRepositoryName()); mavenError == nil {
			return Resolution_Label, &l, nil
		} else {
			BazelLog.Debugf("Maven resolution failed for identifier %q: %v", identifier.Literal(), mavenError)
		}
	}

	// The original import, like "x.y.z" might be a subpackage within a package that resolves,
	// so try to resolve the original identifer, then try to resolve the parent
	// identifier, etc.
	importParent := identifier.Parent()
	if importParent == nil {
		return Resolution_NotFound, nil, nil
	}
	return kt.resolveImport(c, ix, importParent, fromLabel, importContext)
}

// targetListFromResults returns a string with the human-readable list of
// targets contained in the given results.
// TODO: move to gazelle/common
func targetListFromResults(results []resolve.FindResult) string {
	list := make([]string, len(results))
	for i, result := range results {
		list[i] = result.Label.String()
	}
	return strings.Join(list, ", ")
}
