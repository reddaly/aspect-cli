module aspect.build/gazelle

go 1.23.0

require (
	github.com/bazel-contrib/rules_jvm v0.27.0
	github.com/bazelbuild/bazel-gazelle v0.39.0
	github.com/bazelbuild/buildtools v0.0.0-20240918101019-be1c24cc9a44
	github.com/emirpasic/gods v1.18.1
	github.com/go-git/go-git/v5 v5.12.0
	github.com/rs/zerolog v1.33.0
	github.com/smacker/go-tree-sitter v0.0.0-20240827094217-dd81d9e9be82
	github.com/yargevad/filepathx v1.0.0
	go.starlark.net v0.0.0-20240925182052-1207426daebd
)

require (
	github.com/chzyer/readline v0.0.0-20180603132655-2972be24d48e // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.5.0 // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	golang.org/x/mod v0.20.0 // indirect
	golang.org/x/net v0.22.0 // indirect
	golang.org/x/sys v0.25.0 // indirect
	golang.org/x/tools/go/vcs v0.1.0-deprecated // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
)

// DO NOT SUBMIT - local dev only
//replace github.com/bazel-contrib/rules_jvm => "/home/red/code/rules_jvm"
