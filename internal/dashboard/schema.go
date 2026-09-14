package dashboard

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// ProtocolVersion is the provider contract consumed by the customizable
// dashboard model. It is deliberately independent of the legacy widget API.
const ProtocolVersion = 2

var schemaID = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._/-]*[a-z0-9])?$`)

const maxGridColumns = 12

type SourceDescription struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema"`
	URL         string         `json:"url,omitempty"`
	Method      string         `json:"method,omitempty"`
}

type ProviderDescription struct {
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	Description     string              `json:"description"`
	ProtocolVersion int                 `json:"protocolVersion"`
	Sources         []SourceDescription `json:"sources"`
	TemplatePacks   []TemplatePack      `json:"templatePacks,omitempty"`
	Definitions     []WidgetDefinition  `json:"definitions,omitempty"`
}

type TemplatePack struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Templates   []WidgetTemplate `json:"templates"`
}

type WidgetTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Origin      string            `json:"origin"`
	Revision    int               `json:"revision"`
	Inputs      []string          `json:"inputs,omitempty"`
	Project     json.RawMessage   `json:"project"`
	HTML        string            `json:"html"`
	CSS         string            `json:"css"`
	Bindings    map[string]string `json:"bindings,omitempty"`
	TUIFallback string            `json:"tuiFallback"`
	References  []string          `json:"references,omitempty"`
	Width       int               `json:"width"`
	Height      int               `json:"height"`
}

type WidgetInput struct {
	Source     string `json:"source"`
	Expression string `json:"expression"`
}

type WidgetDefinition struct {
	ID          string         `json:"id"`
	TemplateID  string         `json:"templateId"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Group       string         `json:"group"`
	Inputs      map[string]any `json:"inputs,omitempty"`
	Props       map[string]any `json:"props,omitempty"`
	Width       int            `json:"width"`
	Height      int            `json:"height"`
	Default     bool           `json:"default"`
	Repeat      string         `json:"repeat,omitempty"`
}

type WidgetInstance struct {
	ID           string                 `json:"id"`
	DefinitionID string                 `json:"definitionId"`
	TemplateID   string                 `json:"templateId"`
	Inputs       map[string]WidgetInput `json:"inputs,omitempty"`
	Props        map[string]any         `json:"props,omitempty"`
}

type GridPosition struct {
	ID string `json:"id"`
	X  int    `json:"x"`
	Y  int    `json:"y"`
	W  int    `json:"w"`
	H  int    `json:"h"`
}

type DashboardDocument struct {
	Version   int              `json:"version"`
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Revision  int              `json:"revision"`
	Instances []WidgetInstance `json:"instances"`
	Grid      []GridPosition   `json:"grid"`
	TUIOrder  []string         `json:"tuiOrder"`
}

func ParseStrict(data []byte, dst any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}

func ParseProviderDescription(data []byte) (ProviderDescription, error) {
	var v ProviderDescription
	if err := ParseStrict(data, &v); err != nil {
		return v, err
	}
	return v, ValidateProviderDescription(v)
}

func ValidateProviderDescription(v ProviderDescription) error {
	if err := validateID("provider ID", v.ID); err != nil {
		return err
	}
	if err := text("provider name", v.Name); err != nil {
		return err
	}
	if err := text("provider description", v.Description); err != nil {
		return err
	}
	if v.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("unsupported provider protocol version %d", v.ProtocolVersion)
	}
	sources := map[string]bool{}
	for i, source := range v.Sources {
		if err := validateSource(source); err != nil {
			return fmt.Errorf("source %d: %w", i, err)
		}
		if sources[source.ID] {
			return fmt.Errorf("duplicate source ID %q", source.ID)
		}
		sources[source.ID] = true
	}
	seenTemplates := map[string]bool{}
	seenDefinitions := map[string]bool{}
	references := map[string][]string{}
	seenPacks := map[string]bool{}
	for _, pack := range v.TemplatePacks {
		if err := validateID("template pack ID", pack.ID); err != nil {
			return err
		}
		if seenPacks[pack.ID] {
			return fmt.Errorf("duplicate template pack ID %q", pack.ID)
		}
		seenPacks[pack.ID] = true
		for _, template := range pack.Templates {
			if err := ValidateWidgetTemplate(template); err != nil {
				return err
			}
			if seenTemplates[template.ID] {
				return fmt.Errorf("duplicate template ID %q", template.ID)
			}
			seenTemplates[template.ID] = true
			references[template.ID] = append([]string(nil), template.References...)
		}
	}
	for _, definition := range v.Definitions {
		if err := ValidateWidgetDefinition(definition, seenTemplates, sources); err != nil {
			return err
		}
		if seenDefinitions[definition.ID] {
			return fmt.Errorf("duplicate definition ID %q", definition.ID)
		}
		seenDefinitions[definition.ID] = true
	}
	return validateTemplateReferences(seenTemplates, references)
}

func ValidateWidgetTemplate(v WidgetTemplate) error {
	if err := validateID("template ID", v.ID); err != nil {
		return err
	}
	for _, field := range []struct{ name, value string }{{"template name", v.Name}, {"template description", v.Description}, {"template origin", v.Origin}, {"template HTML", v.HTML}, {"template CSS", v.CSS}, {"template TUI fallback", v.TUIFallback}} {
		if err := text(field.name, field.value); err != nil {
			return err
		}
	}
	if v.Revision < 1 || v.Width < 1 || v.Height < 1 {
		return fmt.Errorf("template %q has invalid revision or dimensions", v.ID)
	}
	if len(v.Project) == 0 || !json.Valid(v.Project) {
		return fmt.Errorf("template %q has invalid project JSON", v.ID)
	}
	for name, expression := range v.Bindings {
		if err := ValidateExpression(expression); err != nil {
			return fmt.Errorf("template %q binding %q: %w", v.ID, name, err)
		}
	}
	for _, reference := range v.References {
		if err := validateID("template reference", reference); err != nil {
			return err
		}
	}
	return nil
}

func ValidateWidgetDefinition(v WidgetDefinition, templates, sources map[string]bool) error {
	if err := validateID("definition ID", v.ID); err != nil {
		return err
	}
	if err := validateID("definition template ID", v.TemplateID); err != nil {
		return err
	}
	if !templates[v.TemplateID] {
		return fmt.Errorf("definition %q references unknown template %q", v.ID, v.TemplateID)
	}
	for _, field := range []struct{ name, value string }{{"definition name", v.Name}, {"definition description", v.Description}, {"definition group", v.Group}} {
		if err := text(field.name, field.value); err != nil {
			return err
		}
	}
	if v.Width < 1 || v.Height < 1 {
		return fmt.Errorf("definition %q has invalid dimensions", v.ID)
	}
	if v.Repeat != "" {
		if err := ValidateExpression(v.Repeat); err != nil {
			return fmt.Errorf("definition %q repeat: %w", v.ID, err)
		}
	}
	for name, raw := range v.Inputs {
		if strings.TrimSpace(name) == "" {
			return errors.New("definition input name is empty")
		}
		if input, ok := raw.(map[string]any); ok {
			if source, _ := input["source"].(string); source != "" && !sources[source] {
				return fmt.Errorf("definition %q references unknown source %q", v.ID, source)
			}
			if expression, _ := input["expression"].(string); expression != "" {
				if err := ValidateExpression(expression); err != nil {
					return fmt.Errorf("definition %q input %q: %w", v.ID, name, err)
				}
			}
		}
	}
	return nil
}

func ParseDashboardDocument(data []byte, definitions, templates map[string]bool) (DashboardDocument, error) {
	var v DashboardDocument
	if err := ParseStrict(data, &v); err != nil {
		return v, err
	}
	return v, ValidateDashboardDocument(v, definitions, templates)
}

func ValidateDashboardDocument(v DashboardDocument, definitions, templates map[string]bool) error {
	if v.Version != 1 {
		return fmt.Errorf("unsupported dashboard version %d", v.Version)
	}
	if err := validateID("dashboard ID", v.ID); err != nil {
		return err
	}
	if err := text("dashboard name", v.Name); err != nil {
		return err
	}
	if v.Revision < 1 {
		return errors.New("dashboard revision must be positive")
	}
	instances := map[string]bool{}
	grid := map[string]bool{}
	ordered := map[string]bool{}
	for _, instance := range v.Instances {
		if err := validateID("instance ID", instance.ID); err != nil {
			return err
		}
		if instances[instance.ID] {
			return fmt.Errorf("duplicate instance ID %q", instance.ID)
		}
		instances[instance.ID] = true
		if !definitions[instance.DefinitionID] {
			return fmt.Errorf("instance %q references unknown definition %q", instance.ID, instance.DefinitionID)
		}
		if !templates[instance.TemplateID] {
			return fmt.Errorf("instance %q references unknown template %q", instance.ID, instance.TemplateID)
		}
		for inputName, input := range instance.Inputs {
			if strings.TrimSpace(inputName) == "" || strings.TrimSpace(input.Source) == "" {
				return fmt.Errorf("instance %q has invalid input %q", instance.ID, inputName)
			}
			if err := ValidateExpression(input.Expression); err != nil {
				return fmt.Errorf("instance %q input %q: %w", instance.ID, inputName, err)
			}
		}
	}
	for _, p := range v.Grid {
		if !instances[p.ID] {
			return fmt.Errorf("grid references unknown instance %q", p.ID)
		}
		if grid[p.ID] {
			return fmt.Errorf("duplicate grid instance %q", p.ID)
		}
		grid[p.ID] = true
		if p.X < 0 || p.Y < 0 || p.W < 1 || p.H < 1 || p.X+p.W > maxGridColumns {
			return fmt.Errorf("invalid layout bounds for %q", p.ID)
		}
	}
	for _, id := range v.TUIOrder {
		if !instances[id] {
			return fmt.Errorf("TUI order references unknown instance %q", id)
		}
		if ordered[id] {
			return fmt.Errorf("duplicate TUI order instance %q", id)
		}
		ordered[id] = true
	}
	return nil
}

func ValidateExpression(expression string) error {
	if strings.TrimSpace(expression) == "" {
		return errors.New("expression is empty")
	}
	depth := 0
	quote := byte(0)
	escaped := false
	for i := 0; i < len(expression); i++ {
		c := expression[i]
		if quote != 0 {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		switch c {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth < 0 {
				return fmt.Errorf("unexpected %q at position %d", c, i)
			}
		}
	}
	if quote != 0 || depth != 0 {
		return errors.New("unbalanced expression")
	}
	return nil
}

func validateSource(v SourceDescription) error {
	if err := validateID("source ID", v.ID); err != nil {
		return err
	}
	if err := text("source name", v.Name); err != nil {
		return err
	}
	if err := text("source description", v.Description); err != nil {
		return err
	}
	if v.Schema == nil {
		return errors.New("source schema is required")
	}
	if v.URL != "" {
		u, err := url.Parse(v.URL)
		if err != nil || u.Scheme != "https" {
			return errors.New("source URL must be HTTPS")
		}
	}
	return nil
}
func validateID(field, value string) error {
	if !schemaID.MatchString(value) || strings.Contains(value, "//") {
		return fmt.Errorf("invalid %s %q", field, value)
	}
	return nil
}
func text(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", field)
	}
	return nil
}
func validateTemplateReferences(templates map[string]bool, references map[string][]string) error {
	for id, refs := range references {
		for _, ref := range refs {
			if !templates[ref] {
				return fmt.Errorf("template %q references unknown template %q", id, ref)
			}
		}
	}
	visiting, visited := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("template reference cycle includes %q", id)
		}
		if visited[id] {
			return nil
		}
		visiting[id] = true
		for _, ref := range references[id] {
			if err := visit(ref); err != nil {
				return err
			}
		}
		delete(visiting, id)
		visited[id] = true
		return nil
	}
	for id := range templates {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

// TemplateIDs returns a stable ordering for callers building reference indexes.
func TemplateIDs(packs []TemplatePack) []string {
	var ids []string
	for _, p := range packs {
		for _, t := range p.Templates {
			ids = append(ids, t.ID)
		}
	}
	sort.Strings(ids)
	return ids
}
