package gazelle

import (
	"fmt"

	jvm_maven "github.com/bazel-contrib/rules_jvm/java/gazelle/private/maven"
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/emirpasic/gods/sets/treeset"
)

const LanguageName = "kotlin"

const (
	KtJvmLibrary = "kt_jvm_library"
	KtJvmBinary  = "kt_jvm_binary"
	// rulesKotlinWorkspaceBasedRepositoryName is the canonical repository name of the
	// rules_kotlin repository for WORKSPACE-based projects.
	rulesKotlinWorkspaceBasedRepositoryName = "io_bazel_rules_kotlin"

	// rulesKotlinModuleName is the name of [rules_kotlin bzlmod module].
	//
	// [rules_kotlin bzlmod module]: https://registry.bazel.build/modules/rules_kotlin.
	rulesKotlinModuleName = "rules_kotlin"
)

var sourceRuleKinds = treeset.NewWithStringComparator(KtJvmLibrary)

var (
	_ language.Language            = (*kotlinLang)(nil)
	_ language.ModuleAwareLanguage = (*kotlinLang)(nil)
)

// The Gazelle extension for TypeScript rules.
// TypeScript satisfies the language.Language interface including the
// Configurer and Resolver types.
type kotlinLang struct {
	// TODO: extend rules_jvm extension instead of duplicating?
	mavenResolver    *jvm_maven.Resolver
	mavenInstallFile string
}

// NewLanguage initializes a new TypeScript that satisfies the language.Language
// interface. This is the entrypoint for the extension initialization.
func NewLanguage() language.Language {
	return &kotlinLang{}
}

var kotlinKinds = map[string]rule.KindInfo{
	KtJvmLibrary: {
		MatchAny: false,
		NonEmptyAttrs: map[string]bool{
			"srcs": true,
		},
		SubstituteAttrs: map[string]bool{},
		MergeableAttrs: map[string]bool{
			"srcs": true,
		},
		ResolveAttrs: map[string]bool{
			"deps": true,
		},
	},

	KtJvmBinary: {
		MatchAny: false,
		NonEmptyAttrs: map[string]bool{
			"srcs":       true,
			"main_class": true,
		},
		SubstituteAttrs: map[string]bool{},
		MergeableAttrs:  map[string]bool{},
		ResolveAttrs:    map[string]bool{},
	},
}

func (*kotlinLang) Kinds() map[string]rule.KindInfo {
	return kotlinKinds
}

func (l *kotlinLang) Loads() []rule.LoadInfo {
	return l.ApparentLoads(func(moduleName string) string {
		switch moduleName {
		case rulesKotlinModuleName:
			return rulesKotlinWorkspaceBasedRepositoryName
		default:
			panic(fmt.Errorf("unexpected module name %q", moduleName))
		}
	})
}

// ApparentLoads implements [language.ModuleAwareLanguage].
func (kt *kotlinLang) ApparentLoads(moduleToApparentName func(string) string) []rule.LoadInfo {
	// Note from [language.ModuleAwareLanguage]:
	//
	// The moduleToApparentName argument is a function that resolves a given
	// Bazel module name to the apparent repository name configured for this
	// module in the MODULE.bazel file, or the empty string if there is no such
	// module or the MODULE.bazel file doesn't exist. Languages should use the
	// non-empty value returned by this function to form the repository part of
	// the load statements they return and fall back to using the legacy
	// WORKSPACE name otherwise.
	rulesKotlinRepo := moduleToApparentName(rulesKotlinModuleName)
	if rulesKotlinRepo == "" {
		rulesKotlinRepo = rulesKotlinWorkspaceBasedRepositoryName
	}
	return []rule.LoadInfo{
		{
			Name: "@" + rulesKotlinRepo + "//kotlin:jvm.bzl",
			Symbols: []string{
				KtJvmLibrary,
				KtJvmBinary,
			},
		},
	}
}

func (*kotlinLang) Fix(c *config.Config, f *rule.File) {}
