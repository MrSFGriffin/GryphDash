package dashboard

import "testing"

func validSchemaProvider() ProviderDescription {
	return ProviderDescription{
		ID: "codex", Name: "Codex", Description: "Account data", ProtocolVersion: ProtocolVersion,
		Sources:       []SourceDescription{{ID: "codex/limits", Name: "Limits", Description: "Limit windows", Schema: map[string]any{"type": "object"}}},
		TemplatePacks: []TemplatePack{{ID: "core", Name: "Core", Description: "Built-ins", Templates: []WidgetTemplate{{ID: "core/card", Name: "Card", Description: "A card", Origin: "core", Revision: 1, Project: []byte(`{}`), HTML: `<span></span>`, CSS: `.x{}`, TUIFallback: "card", Width: 4, Height: 2}}}},
		Definitions:   []WidgetDefinition{{ID: "codex/card", TemplateID: "core/card", Name: "Card", Description: "A card", Group: "Codex", Width: 4, Height: 2}},
	}
}

func TestValidateProviderDescription(t *testing.T) {
	if err := ValidateProviderDescription(validSchemaProvider()); err != nil {
		t.Fatal(err)
	}
	bad := validSchemaProvider()
	bad.Definitions = append(bad.Definitions, bad.Definitions[0])
	if err := ValidateProviderDescription(bad); err == nil {
		t.Fatal("expected duplicate definition rejection")
	}
	bad = validSchemaProvider()
	bad.TemplatePacks[0].Templates[0].References = []string{"core/missing"}
	if err := ValidateProviderDescription(bad); err == nil {
		t.Fatal("expected unknown template rejection")
	}
	bad = validSchemaProvider()
	bad.TemplatePacks[0].Templates[0].References = []string{"core/card"}
	if err := ValidateProviderDescription(bad); err == nil {
		t.Fatal("expected template cycle rejection")
	}
}

func TestValidateDashboardDocument(t *testing.T) {
	if err := ValidateDashboardDocument(DashboardDocument{Version: 1, ID: "default", Name: "Default", Revision: 1, Instances: []WidgetInstance{{ID: "instance-1", DefinitionID: "codex/card", TemplateID: "core/card"}}, Grid: []GridPosition{{ID: "instance-1", X: 0, Y: 0, W: 4, H: 2}}, TUIOrder: []string{"instance-1"}}, map[string]bool{"codex/card": true}, map[string]bool{"core/card": true}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDashboardDocument(DashboardDocument{Version: 1, ID: "default", Name: "Default", Revision: 1, Grid: []GridPosition{{ID: "missing", X: -1, Y: 0, W: 0, H: 1}}}, nil, nil); err == nil {
		t.Fatal("expected invalid layout rejection")
	}
}

func TestValidateExpression(t *testing.T) {
	for _, expression := range []string{"account.name", "items[price > 2].price", "$fromMillis(timestamp)"} {
		if err := ValidateExpression(expression); err != nil {
			t.Fatal(err)
		}
	}
	for _, expression := range []string{"", "items[", "'unterminated"} {
		if err := ValidateExpression(expression); err == nil {
			t.Fatalf("expected invalid expression %q", expression)
		}
	}
}
