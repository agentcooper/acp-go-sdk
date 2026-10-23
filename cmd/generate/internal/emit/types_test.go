package emit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"github.com/coder/acp-go-sdk/cmd/generate/internal/load"
)

func TestFlattenedUnionProperties_Scope(t *testing.T) {
	session := &load.Definition{AllOf: []*load.Definition{{Ref: "#/$defs/SessionId"}}, Description: "Session scope"}
	tool := &load.Definition{AnyOf: []*load.Definition{{Ref: "#/$defs/ToolCallId"}, {Type: "null"}}}
	schema := &load.Schema{Defs: map[string]*load.Definition{
		"SessionScope": {Properties: map[string]*load.Definition{"sessionId": session, "toolCallId": tool}, Required: []string{"sessionId"}},
		"RequestScope": {Properties: map[string]*load.Definition{"requestId": {Ref: "#/$defs/RequestId"}}, Required: []string{"requestId"}},
	}}
	d := &load.Definition{AnyOf: []*load.Definition{
		{AllOf: []*load.Definition{{Ref: "#/$defs/SessionScope"}}},
		{Ref: "#/$defs/RequestScope"},
	}}
	props := flattenedUnionProperties(schema, d)
	if len(props) != 3 {
		t.Fatalf("expected three scope fields, got %v", props)
	}
	if props["sessionId"].AnyOf[0].Ref != "#/$defs/SessionId" || props["requestId"].AnyOf[0].Ref != "#/$defs/RequestId" {
		t.Fatal("lost referenced field types")
	}
	if props["sessionId"].AnyOf[1].Type != "null" {
		t.Fatal("alternative field must be optional")
	}
	if props["toolCallId"] != tool {
		t.Fatal("already nullable field should be preserved")
	}
	if len(session.AnyOf) != 0 || len(session.AllOf) != 1 {
		t.Fatal("input schema was mutated")
	}
}

// Exercise the actual schema-to-Go path so a scope fix cannot be lost when
// regenerating bindings, even if the helper still works in isolation.
func TestWriteTypesJen_ElicitationScope(t *testing.T) {
	schemaDir := filepath.Join("..", "..", "..", "..", "schema")
	schema, err := load.ReadSchema(schemaDir)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := load.ReadMeta(schemaDir)
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := WriteTypesJen(out, schema, meta); err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(out, "types_gen.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"CreateElicitationForm": false, "CreateElicitationUrl": false, "CreateElicitationOther": false}
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		if _, ok := want[spec.Name.Name]; !ok {
			return true
		}
		want[spec.Name.Name] = true
		fields := map[string]string{"SessionId": "sessionId", "ToolCallId": "toolCallId", "RequestId": "requestId"}
		for _, field := range spec.Type.(*ast.StructType).Fields.List {
			for _, name := range field.Names {
				wire, ok := fields[name.Name]
				if !ok {
					continue
				}
				ptr, ok := field.Type.(*ast.StarExpr)
				if !ok {
					t.Fatalf("%s.%s must be optional", spec.Name, name)
				}
				if id, ok := ptr.X.(*ast.Ident); !ok || id.Name != name.Name {
					t.Fatalf("wrong scope field type: %v", ptr.X)
				}
				if field.Tag == nil || field.Tag.Value != "`json:\""+wire+",omitempty\"`" {
					t.Fatalf("wrong scope field tag: %v", field.Tag)
				}
				delete(fields, name.Name)
			}
		}
		if len(fields) != 0 {
			t.Fatalf("%s missing fields: %v", spec.Name, fields)
		}
		return false
	})
	for name, found := range want {
		if !found {
			t.Errorf("missing generated type %s", name)
		}
	}
}
