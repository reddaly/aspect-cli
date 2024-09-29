# Based on https://bazel.build/external/migration#fetch-deps-module-extensions

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
def tree_sitter_kotlin_dep():
    http_archive(
        name = "tree-sitter-kotlin",
        sha256 = "f8d6f766ff2da1bd411e6d55f4394abbeab808163d5ea6df9daa75ad48eb0834",
        strip_prefix = "tree-sitter-kotlin-0.3.5",
        urls = ["https://github.com/fwcd/tree-sitter-kotlin/archive/0.3.5.tar.gz"],
        build_file_content = """
filegroup(
    name = "srcs",
    srcs = glob(["src/**/*.c", "src/**/*.h"]),
    visibility = ["//visibility:public"],
)
""",
    )
