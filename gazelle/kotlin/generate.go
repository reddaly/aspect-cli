package gazelle

import (
	"fmt"
	"iter"
	"maps"
	"math"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/emirpasic/gods/sets/treeset"

	gazelle "aspect.build/gazelle/gazelle/common"
	"aspect.build/gazelle/gazelle/common/git"
	"aspect.build/gazelle/gazelle/kotlin/kotlinconfig"
	"aspect.build/gazelle/gazelle/kotlin/parser"
	BazelLog "aspect.build/gazelle/internal/logger"
)

const (
	// TODO: move to common
	MaxWorkerCount = 12
)

func (kt *kotlinLang) GenerateRules(args language.GenerateArgs) language.GenerateResult {
	// TODO: record args.GenFiles labels?

	cfg := args.Config.Exts[LanguageName].(kotlinconfig.Configs)[args.Rel]

	// When we return empty, we mean that we don't generate anything, but this
	// still triggers the indexing for all the TypeScript targets in this package.
	if !cfg.GenerationEnabled() {
		BazelLog.Tracef("GenerateRules(%s) disabled: %s", LanguageName, args.Rel)
		return language.GenerateResult{}
	}

	BazelLog.Tracef("GenerateRules(%s): %s; config = %s", LanguageName, args.Rel, cfg)

	// Collect all source files.
	sourceFiles := kt.collectSourceFiles(cfg, args)

	// TODO: multiple library targets (lib, test, ...)
	libTarget := NewKotlinLibTarget()
	binTargets := map[string]*KotlinBinTarget{}
	var testTargets []*KotlinTestTarget

	// Parse all source files and group information into target(s)
	for p := range kt.parseFiles(args, sourceFiles) {
		var target *KotlinTarget

		if cfg.IsTestBaseName(filepath.Base(p.File)) {
			testTarget := NewKotlinTestTarget([]string{p.File}, p.Package, guessClassName(p))
			testTargets = append(testTargets, testTarget)
			target = &testTarget.KotlinTarget
		} else if p.HasMain {
			binTarget := NewKotlinBinTarget(p.File, p.Package)
			binTargets[p.File] = binTarget

			target = &binTarget.KotlinTarget
		} else {
			libTarget.addFile(p.File)
			if p.Package != nil {
				libTarget.addPackage(p.Package)
			}

			target = &libTarget.KotlinTarget
		}

		for _, impt := range p.Imports {
			target.addImport(&ImportStatement{
				SourcePath:   p.File,
				ImportHeader: impt,
			})
		}
	}

	var result language.GenerateResult

	if len(libTarget.Files) != 0 {
		libTargetName := gazelle.ToDefaultTargetName(args, "root")
		srcGenErr := kt.addLibraryRule(libTargetName, libTarget, args, false, &result)
		if srcGenErr != nil {
			fmt.Fprintf(os.Stderr, "Library rule generation error: %v\n", srcGenErr)
			os.Exit(1)
		}
	}

	sortedBinTargets := slices.SortedFunc(maps.Values(binTargets), func(a, b *KotlinBinTarget) int {
		return strings.Compare(toBinaryTargetName(a.File), toBinaryTargetName(b.File))
	})

	for _, binTarget := range sortedBinTargets {
		binTargetName := toBinaryTargetName(binTarget.File)
		if err := kt.addBinaryRule(binTargetName, binTarget, args, &result); err != nil {
			fmt.Fprintf(os.Stderr, "Binary rule generation error: %v\n", err)
			os.Exit(1)
		}
	}

	sort.Slice(testTargets, func(i, j int) bool {
		return testTargets[i].Files[0] < testTargets[j].Files[0]
	})

	for _, target := range testTargets {
		binTargetName := toTestTargetName(target.Files[0])
		if err := kt.addTestRule(binTargetName, target, args, &result); err != nil {
			fmt.Fprintf(os.Stderr, "Test rule generation error: %v\n", err)
			os.Exit(1)
		}
	}

	return result
}

func (kt *kotlinLang) addLibraryRule(targetName string, target *KotlinLibTarget, args language.GenerateArgs, isTestRule bool, result *language.GenerateResult) error {
	// Check for name-collisions with the rule being generated.
	colError := gazelle.CheckCollisionErrors(targetName, KtJvmLibrary, sourceRuleKinds, args)
	if colError != nil {
		return colError
	}

	// Generate nothing if there are no source files. Remove any existing rules.
	if len(target.Files) == 0 {
		if args.File == nil {
			return nil
		}

		for _, r := range args.File.Rules {
			if r.Name() == targetName && r.Kind() == KtJvmLibrary {
				emptyRule := rule.NewRule(KtJvmLibrary, targetName)
				result.Empty = append(result.Empty, emptyRule)
				return nil
			}
		}

		return nil
	}

	ktLibrary := rule.NewRule(KtJvmLibrary, targetName)
	ktLibrary.SetAttr("srcs", sequenceToSlice(maps.Keys(target.Files)))
	ktLibrary.SetPrivateAttr(packagesKey, target)

	if isTestRule {
		ktLibrary.SetAttr("testonly", true)
	}

	result.Gen = append(result.Gen, ktLibrary)
	result.Imports = append(result.Imports, target)

	BazelLog.Infof("add rule '%s' '%s:%s'", ktLibrary.Kind(), args.Rel, ktLibrary.Name())
	return nil
}

func (kt *kotlinLang) addBinaryRule(targetName string, target *KotlinBinTarget, args language.GenerateArgs, result *language.GenerateResult) error {
	// Check for name-collisions with the rule being generated.
	colError := gazelle.CheckCollisionErrors(targetName, KtJvmBinary, treeset.NewWithStringComparator(KtJvmBinary), args)
	if colError != nil {
		return colError
	}
	// TODO: Rely on the parser to determine the main class. The main function could be
	// elsewhere in the file.
	main_class := strings.TrimSuffix(target.File, ".kt")
	if target.Package != nil {
		main_class = target.Package.Literal() + "." + main_class
	}

	ktBinary := rule.NewRule(KtJvmBinary, targetName)
	ktBinary.SetAttr("srcs", []string{target.File})
	ktBinary.SetAttr("main_class", main_class)
	ktBinary.SetPrivateAttr(packagesKey, target)

	result.Gen = append(result.Gen, ktBinary)
	result.Imports = append(result.Imports, target)

	BazelLog.Infof("add rule '%s' '%s:%s'", ktBinary.Kind(), args.Rel, ktBinary.Name())
	return nil
}

func (kt *kotlinLang) addTestRule(targetName string, target *KotlinTestTarget, args language.GenerateArgs, result *language.GenerateResult) error {
	// Check for name-collisions with the rule being generated.
	colError := gazelle.CheckCollisionErrors(targetName, KtJvmTest, treeset.NewWithStringComparator(KtJvmTest), args)
	if colError != nil {
		return colError
	}

	// TODO - is this necessary? It was copied from the addLibRule function.
	// Generate nothing if there are no source files. Remove any existing rules.
	if len(target.Files) == 0 {
		if args.File == nil {
			return nil
		}

		for _, r := range args.File.Rules {
			if r.Name() == targetName && r.Kind() == KtJvmTest {
				emptyRule := rule.NewRule(KtJvmTest, targetName)
				result.Empty = append(result.Empty, emptyRule)
				return nil
			}
		}

		return nil
	}

	ktLibrary := rule.NewRule(KtJvmTest, targetName)
	ktLibrary.SetAttr("srcs", target.Files)
	ktLibrary.SetAttr("test_class", target.TestClass.Literal())
	ktLibrary.SetPrivateAttr(packagesKey, target)

	result.Gen = append(result.Gen, ktLibrary)
	result.Imports = append(result.Imports, target)

	BazelLog.Infof("add rule %q %q:%q", ktLibrary.Kind(), args.Rel, ktLibrary.Name())
	return nil
}

// TODO: put in common?
func (kt *kotlinLang) parseFiles(args language.GenerateArgs, sources *treeset.Set) chan *parser.ParseResult {
	// The channel of all files to parse.
	sourcePathChannel := make(chan string)

	// The channel of parse results.
	resultsChannel := make(chan *parser.ParseResult)

	// The number of workers. Don't create more workers than necessary.
	workerCount := int(math.Min(MaxWorkerCount, float64(1+sources.Size()/2)))

	// Start the worker goroutines.
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for sourcePath := range sourcePathChannel {
				r, errs := parseFile(path.Join(args.Config.RepoRoot, args.Rel), sourcePath)

				// Output errors to stdout
				if len(errs) > 0 {
					fmt.Println(sourcePath, "parse error(s):")
					for _, err := range errs {
						fmt.Println(err)
					}
				}

				resultsChannel <- r
			}
		}()
	}

	// Send files to the workers.
	go func() {
		sourceFileChannelIt := sources.Iterator()
		for sourceFileChannelIt.Next() {
			sourcePathChannel <- sourceFileChannelIt.Value().(string)
		}

		close(sourcePathChannel)
	}()

	// Wait for all workers to finish.
	go func() {
		wg.Wait()
		close(resultsChannel)
	}()

	return resultsChannel
}

// Parse the passed file for import statements.
func parseFile(rootDir, filePath string) (*parser.ParseResult, []error) {
	BazelLog.Tracef("ParseImports(%s): %s", LanguageName, filePath)

	content, err := os.ReadFile(path.Join(rootDir, filePath))
	if err != nil {
		return nil, []error{err}
	}

	p := parser.NewParser()
	return p.Parse(filePath, string(content))
}

func (kt *kotlinLang) collectSourceFiles(cfg *kotlinconfig.KotlinConfig, args language.GenerateArgs) *treeset.Set {
	sourceFiles := treeset.NewWithStringComparator()

	// TODO: "module" targets similar to java?

	isIgnored := git.GetIgnoreFunction(args.Config)

	gazelle.GazelleWalkDir(args, isIgnored, func(f string) error {
		// Otherwise the file is either source or potentially importable.
		if isSourceFileType(f) {
			BazelLog.Tracef("SourceFile: %s", f)

			sourceFiles.Add(f)
		}

		return nil
	})

	return sourceFiles
}

func isSourceFileType(f string) bool {
	ext := path.Ext(f)
	return ext == ".kt" || ext == ".kts"
}

func sequenceToSlice[T any](seq iter.Seq[T]) []T {
	var result []T
	for item := range seq {
		result = append(result, item)
	}
	return result
}

// guessClassName returns a Kotlin identifier for the class within the provided file, or nil
// if there is no class within the file.
func guessClassName(p *parser.ParseResult) *parser.Identifier {
	id, err := parser.NewSimpleIdentifier(strings.TrimSuffix(filepath.Base(p.File), ".kt"))
	if err != nil {
		return nil
	}
	return p.Package.Child(id)
}
