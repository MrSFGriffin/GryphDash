package jsonata

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type fixture struct {
	Name       string          `json:"name"`
	Expression string          `json:"expression"`
	Input      json.RawMessage `json:"input"`
	Expected   json.RawMessage `json:"expected"`
}

func TestVersion(t *testing.T) {
	if got := ImplementationVersion(); got != Version && got != "v"+Version {
		t.Fatalf("JSONata version = %q, want %q", got, Version)
	}
}

func TestSharedFixtures(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "jsonata", "expressions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []fixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, tc := range fixtures {
		t.Run(tc.Name, func(t *testing.T) {
			got, err := Evaluate(tc.Expression, tc.Input)
			if err != nil {
				t.Fatal(err)
			}
			var gotValue, expectedValue any
			if err := json.Unmarshal(got, &gotValue); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(tc.Expected, &expectedValue); err != nil {
				t.Fatal(err)
			}
			gotJSON, _ := json.Marshal(gotValue)
			expectedJSON, _ := json.Marshal(expectedValue)
			if !bytes.Equal(gotJSON, expectedJSON) {
				t.Fatalf("result = %s, want %s", gotJSON, expectedJSON)
			}
		})
	}
}
