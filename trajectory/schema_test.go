package trajectory

import (
	"encoding/json"
	"os"
	"sort"
	"testing"
)

// schemaDoc captures only the parts of trace.schema.json this test asserts on:
// the step type enum and the per-type required-field rules encoded as if/then.
type schemaDoc struct {
	Defs struct {
		Step struct {
			Properties struct {
				Type struct {
					Enum []string `json:"enum"`
				} `json:"type"`
			} `json:"properties"`
			AllOf []struct {
				If struct {
					Properties struct {
						Type struct {
							Const string `json:"const"`
						} `json:"type"`
					} `json:"properties"`
				} `json:"if"`
				Then struct {
					Required []string `json:"required"`
				} `json:"then"`
			} `json:"allOf"`
		} `json:"step"`
	} `json:"$defs"`
}

func loadSchemaDoc(t *testing.T) schemaDoc {
	t.Helper()
	data, err := os.ReadFile("trace.schema.json")
	if err != nil {
		t.Fatalf("read trace.schema.json: %v", err)
	}
	var doc schemaDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse trace.schema.json: %v", err)
	}
	return doc
}

// TestSchemaTypeEnumMatchesConstants guards against the published schema drifting
// away from the Go StepType constants (the source of truth). If a StepType is
// added or renamed without touching the schema, this fails.
func TestSchemaTypeEnumMatchesConstants(t *testing.T) {
	doc := loadSchemaDoc(t)

	want := []string{
		string(StepTypeCallLLM),
		string(StepTypeToolCall),
		string(StepTypeToolResult),
		string(StepTypeNodeTransition),
	}
	got := append([]string(nil), doc.Defs.Step.Properties.Type.Enum...)

	sort.Strings(want)
	sort.Strings(got)

	if len(got) != len(want) {
		t.Fatalf("schema type enum has %d values %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("schema type enum = %v, want %v", got, want)
		}
	}
}

// TestSchemaRequiredFieldsMatchValidate guards the per-type required-field rules:
// the schema's if/then blocks must encode the same "type -> required field" map
// that Run.Validate enforces. Keep this in sync with the switch in Validate.
func TestSchemaRequiredFieldsMatchValidate(t *testing.T) {
	doc := loadSchemaDoc(t)

	want := map[string]string{
		string(StepTypeCallLLM):        "llm",
		string(StepTypeToolCall):       "tool",
		string(StepTypeToolResult):     "tool",
		string(StepTypeNodeTransition): "node",
	}

	got := map[string]string{}
	for _, rule := range doc.Defs.Step.AllOf {
		typ := rule.If.Properties.Type.Const
		if typ == "" {
			t.Fatalf("schema allOf entry with no type const: %+v", rule)
		}
		if len(rule.Then.Required) != 1 {
			t.Fatalf("schema allOf for %q must require exactly one field, got %v", typ, rule.Then.Required)
		}
		got[typ] = rule.Then.Required[0]
	}

	if len(got) != len(want) {
		t.Fatalf("schema encodes %d per-type rules %v, want %d %v", len(got), got, len(want), want)
	}
	for typ, field := range want {
		if got[typ] != field {
			t.Fatalf("schema requires %q for %q, want %q", got[typ], typ, field)
		}
	}
}
