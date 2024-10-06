package parser

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var testCases = []struct {
	desc, kt string
	want     parseResultComparable
}{
	{
		desc: "import star",
		kt: `package a.b.c

import  x.y.z.* 
		`,
		want: parseResultComparable{
			File:    "stars.kt",
			Package: "a.b.c",
			Imports: []importComparable{
				{Identifier: "x.y.z", IsStar: true},
			},
		},
	},
	{
		desc: "aliased",
		kt: `package hey.there

import com.example.foo.Bar as MyBar
import com.example.foo.Bar as /*x*/MyBar2
`,
		want: parseResultComparable{
			File:    "aliased.kt",
			Package: "hey.there",
			Imports: []importComparable{
				{Identifier: "com.example.foo.Bar", Alias: "MyBar"},
				{Identifier: "com.example.foo.Bar", Alias: "MyBar2"},
			},
		},
	},
	{
		desc: "empty",
		kt:   "",
		want: parseResultComparable{
			File:    "empty.kt",
			Package: "",
			Imports: []importComparable{},
		},
	},
	{
		desc: "simple",
		kt: `
import a.B
import c.D as E
	`,
		want: parseResultComparable{
			File:    "simple.kt",
			Package: "",
			Imports: []importComparable{
				{Identifier: "a.B"},
				{Identifier: "c.D", Alias: "E"},
			},
		},
	},
	{
		desc: "stars",
		kt: `package a.b.c

import  d.y.* 
		`,
		want: parseResultComparable{
			File:    "stars.kt",
			Package: "a.b.c",
			Imports: []importComparable{
				{Identifier: "d.y", IsStar: true},
			},
		},
	},
	{
		desc: "comments",
		kt: `
/*dlfkj*/package /*dlfkj*/ x // x
//z
import a.B // y
//z

/* asdf */ import /* asdf */ c.D // w
import /* fdsa */ d/* asdf */.* // w
				`,
		want: parseResultComparable{
			File:    "comments.kt",
			Package: "x",
			Imports: []importComparable{
				{Identifier: "a.B"},
				{Identifier: "c.D"},
				{Identifier: "d", IsStar: true},
			},
		},
	},
	{
		desc: "value-classes",
		kt: `
@JvmInline
value class Password(private val s: String)
	`,
		want: parseResultComparable{
			File:    "simple.kt",
			Package: "",
			Imports: []importComparable{},
		},
	},
}

func TestTreesitterParser(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			res, _ := NewParser().Parse(tc.want.File, tc.kt)

			if diff := cmp.Diff(tc.want, makeComparable(res), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("unexpected diff (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestMainDetection(t *testing.T) {
	t.Run("main detection", func(t *testing.T) {
		res, _ := NewParser().Parse("main.kt", "fun main() {}")
		if !res.HasMain {
			t.Errorf("main method should be detected")
		}

		res, _ = NewParser().Parse("x.kt", `
package my.demo
fun main() {}
		`)
		if !res.HasMain {
			t.Errorf("main method should be detected with package")
		}

		res, _ = NewParser().Parse("x.kt", `
package my.demo
import kotlin.text.*
fun main() {}
		`)
		if !res.HasMain {
			t.Errorf("main method should be detected with imports")
		}
	})
}

type parseResultComparable struct {
	File    string
	Imports []importComparable
	Package string
	HasMain bool
}

type importComparable struct {
	Identifier string
	IsStar     bool
	Alias      string
}

func makeComparable(result *ParseResult) parseResultComparable {
	comparable := parseResultComparable{
		File:    result.File,
		Package: packageString(result),
		HasMain: result.HasMain,
	}
	for _, imp := range result.Imports {
		alias := ""
		if imp.Alias() != nil {
			alias = imp.Alias().Literal()
		}
		comparable.Imports = append(comparable.Imports, importComparable{
			Identifier: imp.Identifier().Literal(),
			IsStar:     imp.IsStarImport(),
			Alias:      alias,
		})
	}
	return comparable
}

func packageString(pr *ParseResult) string {
	if pr.Package == nil {
		return ""
	}
	return pr.Package.Literal()
}
