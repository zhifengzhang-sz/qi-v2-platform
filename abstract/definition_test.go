package abstract

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// definitionFiles are the files of the definition whose declarations README.md's table holds, one table a file.
var definitionFiles = []string{
	"Kafka.lean", "Acquisition.lean", "Warehousing.lean", "Visualization.lean", "MarketModelEngine.lean",
	"Workspace.lean", "Wiring.lean",
}

// The table of README.md against the pinned definition (QI_DEFINITION, the shell's) and this package: every
// declaration of the definition's files has a row, in its file's table; every row names a declaration its file
// makes; and every row names Go that exists here, or says why the declaration is not in code.
func TestDefinition(t *testing.T) {
	dir := os.Getenv("QI_DEFINITION")
	if dir == "" {
		t.Fatal("QI_DEFINITION is not set: the project's shell gives it, the definition's package (flake.nix)")
	}
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	tables, err := readmeRows(string(readme))
	if err != nil {
		t.Fatalf("README.md: %v", err)
	}
	names, err := goNames(".")
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range definitionFiles {
		src, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			t.Fatalf("the definition at QI_DEFINITION: %v", err)
		}
		declared, err := leanDeclarations(string(src))
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if len(declared) == 0 {
			t.Errorf("%s: no declaration read", file)
		}
		rows, ok := tables[file]
		if !ok {
			t.Errorf("README.md: no table for %s", file)
		}
		for _, d := range declared {
			if _, ok := rows[d]; !ok {
				t.Errorf("%s declares %s, which has no row in README.md", file, d)
			}
		}
		for _, d := range slices.Sorted(maps.Keys(rows)) {
			if !slices.Contains(declared, d) {
				t.Errorf("README.md: the table of %s has a row for %s, which the file does not declare", file, d)
			}
			row := rows[d]
			if len(row.goNames) == 0 && row.note == "" {
				t.Errorf("README.md: the row of %s names no Go and does not say why it is not in code", d)
			}
			for _, name := range row.goNames {
				if !names[name] {
					t.Errorf("README.md: the row of %s names %s, which this package does not have", d, name)
				}
			}
		}
	}
	for _, file := range slices.Sorted(maps.Keys(tables)) {
		if !slices.Contains(definitionFiles, file) {
			t.Errorf("README.md: a table for %s, which is not one of the definition's files read here", file)
		}
	}
}

// row is one row of README.md's table: the Go names it gives a declaration, and its note.
type row struct {
	goNames []string
	note    string
}

var (
	tableHeading = "## The definition, declaration by declaration"
	fileHeading  = regexp.MustCompile("^### `([^`]+)`$")
	codeSpan     = regexp.MustCompile("`([^`]+)`")
)

// readmeRows reads the tables under tableHeading, to the next heading of its level: for each file (a heading
// "### `File.lean`"), each row's declaration with its Go names and its note. A row is a line that begins with
// "| `"; its cells are the declaration, one name in backticks; the Go names, each in backticks, with commas
// between them, or nothing; and the note.
func readmeRows(readme string) (map[string]map[string]row, error) {
	tables := map[string]map[string]row{}
	inside, file := false, ""
	for n, line := range strings.Split(readme, "\n") {
		switch {
		case line == tableHeading:
			inside = true
			continue
		case strings.HasPrefix(line, "## "):
			inside = false
		}
		if !inside {
			continue
		}
		if m := fileHeading.FindStringSubmatch(line); m != nil {
			file = m[1]
			if _, twice := tables[file]; twice {
				return nil, fmt.Errorf("line %d: a second table for %s", n+1, file)
			}
			tables[file] = map[string]row{}
			continue
		}
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		if file == "" {
			return nil, fmt.Errorf("line %d: a row before any file's heading", n+1)
		}
		cells := strings.Split(line, "|")
		if len(cells) != 5 || cells[0] != "" || strings.TrimSpace(cells[4]) != "" {
			return nil, fmt.Errorf("line %d: a row is three cells, the declaration, the Go and the note: %q", n+1, line)
		}
		declaration := codeSpan.FindStringSubmatch(strings.TrimSpace(cells[1]))
		if declaration == nil || declaration[0] != strings.TrimSpace(cells[1]) {
			return nil, fmt.Errorf("line %d: the first cell is one declaration's name in backticks: %q", n+1, cells[1])
		}
		r := row{note: strings.TrimSpace(cells[3])}
		for _, m := range codeSpan.FindAllStringSubmatch(cells[2], -1) {
			r.goNames = append(r.goNames, m[1])
		}
		if rest := strings.Trim(codeSpan.ReplaceAllString(cells[2], ""), " ,"); rest != "" {
			return nil, fmt.Errorf("line %d: the second cell is Go names in backticks, with commas between them: %q", n+1, cells[2])
		}
		if _, twice := tables[file][declaration[1]]; twice {
			return nil, fmt.Errorf("line %d: a second row for %s", n+1, declaration[1])
		}
		tables[file][declaration[1]] = r
	}
	if len(tables) == 0 {
		return nil, fmt.Errorf("no table under %q", tableHeading)
	}
	return tables, nil
}

// goNames gives the names this package has, in its files and its tests: a function, a type, a constant or a
// variable by its name; a method, a struct's field and an interface's method as Type.Name; a type's parameter as
// Type[Name].
func goNames(dir string) (map[string]bool, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	fset := token.NewFileSet()
	for _, path := range files {
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil {
					names[d.Name.Name] = true
					continue
				}
				if recv := receiver(d.Recv.List[0].Type); recv != "" {
					names[recv+"."+d.Name.Name] = true
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.ValueSpec:
						for _, name := range s.Names {
							names[name.Name] = true
						}
					case *ast.TypeSpec:
						typeNames(names, s)
					}
				}
			}
		}
	}
	return names, nil
}

// receiver gives the name of a method's receiver type: T of T, *T, T[A] and T[A, B].
func receiver(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return receiver(e.X)
	case *ast.IndexExpr:
		return receiver(e.X)
	case *ast.IndexListExpr:
		return receiver(e.X)
	}
	return ""
}

func typeNames(names map[string]bool, s *ast.TypeSpec) {
	name := s.Name.Name
	names[name] = true
	if s.TypeParams != nil {
		for _, param := range s.TypeParams.List {
			for _, p := range param.Names {
				names[name+"["+p.Name+"]"] = true
			}
		}
	}
	var members *ast.FieldList
	switch t := s.Type.(type) {
	case *ast.StructType:
		members = t.Fields
	case *ast.InterfaceType:
		members = t.Methods
	}
	if members == nil {
		return
	}
	for _, member := range members.List {
		for _, m := range member.Names {
			names[name+"."+m.Name] = true
		}
	}
}

var (
	leanName        = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_'.]*$`)
	leanField       = regexp.MustCompile(`^  ([A-Za-z_][A-Za-z0-9_']*) :`)
	leanConstructor = regexp.MustCompile(`^  \| ([A-Za-z_][A-Za-z0-9_']*)$`)
)

// leanDeclarations gives the names of what a file of the definition declares, in the file's order, each in full,
// under the namespaces it is in: every structure, inductive, def, abbrev and theorem; a structure's fields, as
// Structure.field; an inductive's constructors, as Inductive.constructor.
//
// It reads the Lean the definition's files are written in, and no more: commands begin at a line's first column,
// a structure's fields and an inductive's constructors are indented by two, one a line, and what is indented
// further continues the line above. A line it does not know is an error, never passed over, so that a declaration
// written another way is not missed.
func leanDeclarations(src string) ([]string, error) {
	var (
		declared []string
		scopes   []string // the namespaces and sections open; a section is ""
		owner    string   // the structure or inductive whose body the indented lines are; "" for none
		body     string   // "structure", "inductive", or "" for a body that declares nothing
	)
	declare := func(name string) error {
		if slices.Contains(declared, name) {
			return fmt.Errorf("%s is declared twice", name)
		}
		declared = append(declared, name)
		return nil
	}
	for n, line := range strings.Split(leanCode(src), "\n") {
		line = strings.TrimRight(line, " ")
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, " ") {
			if strings.HasPrefix(line, "   ") || body == "" {
				continue
			}
			var m []string
			if body == "structure" {
				m = leanField.FindStringSubmatch(line)
			} else {
				m = leanConstructor.FindStringSubmatch(line)
			}
			if m == nil {
				return nil, fmt.Errorf("line %d: in the %s %s, a line that is none of its own: %q", n+1, body, owner, line)
			}
			if err := declare(owner + "." + m[1]); err != nil {
				return nil, fmt.Errorf("line %d: %w", n+1, err)
			}
			continue
		}
		owner, body = "", ""
		words := strings.Fields(line)
		for len(words) > 1 && slices.Contains([]string{"noncomputable", "private", "protected"}, words[0]) {
			words = words[1:]
		}
		switch words[0] {
		case "import", "open", "variable":
		case "namespace":
			if len(words) != 2 {
				return nil, fmt.Errorf("line %d: a namespace with no name, or more: %q", n+1, line)
			}
			scopes = append(scopes, words[1])
		case "section":
			scopes = append(scopes, "")
		case "end":
			if len(scopes) == 0 {
				return nil, fmt.Errorf("line %d: an end with nothing open", n+1)
			}
			if open := scopes[len(scopes)-1]; len(words) > 2 || (len(words) == 2 && words[1] != open) || (len(words) == 1 && open != "") {
				return nil, fmt.Errorf("line %d: %q ends what is not the last opened, %q", n+1, line, open)
			}
			scopes = scopes[:len(scopes)-1]
		case "structure", "inductive", "def", "abbrev", "theorem":
			if len(words) < 2 || !leanName.MatchString(words[1]) {
				return nil, fmt.Errorf("line %d: a %s whose name is not read: %q", n+1, words[0], line)
			}
			name := words[1]
			for i := len(scopes) - 1; i >= 0; i-- {
				if scopes[i] != "" {
					name = scopes[i] + "." + name
				}
			}
			if err := declare(name); err != nil {
				return nil, fmt.Errorf("line %d: %w", n+1, err)
			}
			if words[0] == "structure" || words[0] == "inductive" {
				if words[len(words)-1] != "where" {
					return nil, fmt.Errorf("line %d: a %s whose first line does not end with where: %q", n+1, words[0], line)
				}
				owner, body = name, words[0]
			}
		default:
			return nil, fmt.Errorf("line %d: a command this test does not know: %q", n+1, line)
		}
	}
	if len(scopes) != 0 {
		return nil, fmt.Errorf("at the end, still open: %q", scopes)
	}
	return declared, nil
}

// leanCode gives a Lean file without its comments, each put out as spaces, its lines kept: block comments
// (/- … -/, a doc comment being one), which may hold one another, and line comments (-- …). A string is kept as it
// is.
func leanCode(src string) string {
	var b strings.Builder
	depth := 0
	for i := 0; i < len(src); {
		rest := src[i:]
		switch {
		case strings.HasPrefix(rest, "/-"):
			depth++
			b.WriteString("  ")
			i += 2
		case depth > 0 && strings.HasPrefix(rest, "-/"):
			depth--
			b.WriteString("  ")
			i += 2
		case depth > 0:
			if src[i] == '\n' {
				b.WriteByte('\n')
			} else {
				b.WriteByte(' ')
			}
			i++
		case strings.HasPrefix(rest, "--"):
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case src[i] == '"':
			j := i + 1
			for j < len(src) && src[j] != '"' {
				if src[j] == '\\' {
					j++
				}
				j++
			}
			j = min(j+1, len(src))
			b.WriteString(src[i:j])
			i = j
		default:
			b.WriteByte(src[i])
			i++
		}
	}
	return b.String()
}
