# Based on https://bazel.build/external/migration#fetch-deps-module-extensions

load("//:repositories.bzl", "tree_sitter_kotlin_dep")

def _non_module_dependencies_impl(_ctx):
    tree_sitter_kotlin_dep()

non_module_dependencies = module_extension(
    implementation = _non_module_dependencies_impl,
)