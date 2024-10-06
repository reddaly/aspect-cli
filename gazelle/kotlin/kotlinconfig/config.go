package kotlinconfig

import (
	"path/filepath"
	"strings"

	"github.com/bazel-contrib/rules_jvm/java/gazelle/javaconfig"
)

const Directive_KotlinExtension = "kotlin"

type KotlinConfig struct {
	javaConfig *javaconfig.Config

	parent *KotlinConfig
	rel    string

	testFileSuffixes []string

	generationEnabled bool
}

type Configs = map[string]*KotlinConfig

func New(repoRoot string) *KotlinConfig {
	return &KotlinConfig{
		javaConfig:        javaconfig.New(repoRoot),
		generationEnabled: true,
		parent:            nil,
		testFileSuffixes:  []string{"Test.kt"},
	}
}

// NewChild creates a new child Config. It inherits desired values from the
// current Config and sets itself as the parent to the child.
func (c *KotlinConfig) NewChild(childPath string) *KotlinConfig {
	cCopy := *c
	cCopy.javaConfig = c.javaConfig.NewChild()
	cCopy.rel = childPath
	cCopy.parent = c
	cCopy.testFileSuffixes = append([]string(nil), c.testFileSuffixes...)
	return &cCopy
}

// SetGenerationEnabled sets whether the extension is enabled or not.
func (c *KotlinConfig) SetGenerationEnabled(enabled bool) {
	c.generationEnabled = enabled
}

// GenerationEnabled returns whether the extension is enabled or not.
func (c *KotlinConfig) GenerationEnabled() bool {
	return c.generationEnabled
}

// JavaConfig returns the [javaconfig.Config] used as part of the Kotlin config.
func (c *KotlinConfig) JavaConfig() *javaconfig.Config {
	return c.javaConfig
}

// IsTestBaseName reports if the given basename within the same bazel package
// as the config should be considered a test.
func (c *KotlinConfig) IsTestBaseName(baseName string) bool {
	for _, suffix := range c.testFileSuffixes {
		if strings.HasSuffix(baseName, suffix) {
			return true
		}
	}
	return false
}

// ParentForPackage returns the parent Config for the given Bazel package.
func ParentForPackage(c Configs, pkg string) *KotlinConfig {
	dir := filepath.Dir(pkg)
	if dir == "." {
		dir = ""
	}
	parent := (map[string]*KotlinConfig)(c)[dir]
	return parent
}
