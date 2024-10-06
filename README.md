A Kotlin plugin for [bazel's gazelle
tool](https://github.com/bazelbuild/bazel-gazelle/tree/master) for automatically
generating BUILD.bazel files from Kotlin source code.

### Fork status
This code was extracted from https://github.com/aspect-build/aspect-cli. I would
like to merge it upstream depending on how the conversation goes with the
original authors. See https://github.com/aspect-build/aspect-cli/issues/750 for
discussion.


# Usage


# Dependency resolution

Gazelle must resolve what Bazel labels (like `@foo//bar/baz`) a Kotlin file depends on.
The rules followed by this plugin are as follows:

1. The dependencies of a Kotlin file are determined from the explicit imports in
   the file. Fully-qualified identifiers present elsewhere in the file are not
   considered when determining dependencies.

2. The resolution algorithm (as implemented in `gazelle/kotlin/resolver.go`)
   
3. An identifier matches a [Maven
   artifact](https://maven.apache.org/repositories/artifacts.html) if one of the
   packages declared as an export of that artifact is a package prefix

4. If the import to be resolved is in the library index, the import will be resolved to that library. If `-index=true`, Gazelle builds an index of library rules in the current repository before starting dependency resolution, and this is how most dependencies are resolved.

   1. For Kotlin, the match is based on the importpath attribute.

   2. For proto, the match is based on the srcs attribute.


# Terminology

**identifier** is used to refer to what the Kotlin spec calls an
[identifier](https://kotlinlang.org/spec/syntax-and-grammar.html#grammar-rule-identifier).
This is a fully-qualified or partially-qualified name. Fully-qualified
identifiers are used in `package` and `import` statements in Kotlin files.

**parent identifier**: In this document and in the code base, "parent identifier" means
an identifier with the last dot-delimited component removed. Sometimes it may be loosely
used to refer to all the ancestor identifiers as well (parent, parent of parent, etc.).

# Directives

Many [gazelle directives](https://github.com/bazelbuild/bazel-gazelle#directives) are generic
and will apply the the behavior of the Kotlin plugin.

The Kotlin plugin has additional directives for configuring behavior:

## gazelle:java_maven_install_file

Specifies where the `maven_install.json` file is located.

This directive is defined by the [rules_jvm gazelle plugin](), and the Kotlin
plugins hares logic with the Java plugin to parse it.