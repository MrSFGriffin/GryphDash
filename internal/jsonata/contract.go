// Package jsonata provides the JSONata contract used by dashboard documents.
package jsonata

import (
	"fmt"

	jsonata206 "github.com/jsonata-go/jsonata/v206"
)

// Version is the JSONata language version shared with the browser runtime.
const Version = "2.0.6"

// Evaluate compiles and evaluates one expression using the pinned JSONata
// implementation. JSON input and output are used deliberately so Go and
// JavaScript compare the same JSON values and missing results remain distinct
// from explicit null values in the implementation.
func Evaluate(expression string, input []byte) ([]byte, error) {
	compiled, err := jsonata206.Compile(expression, false)
	if err != nil {
		return nil, fmt.Errorf("compile JSONata expression: %w", err)
	}
	result, err := compiled.Evaluate(input, nil)
	if err != nil {
		return nil, fmt.Errorf("evaluate JSONata expression: %w", err)
	}
	return result, nil
}

// Validate checks syntax using the same pinned compiler used for evaluation.
func Validate(expression string) error {
	if expression == "" {
		return fmt.Errorf("expression is empty")
	}
	if _, err := jsonata206.Compile(expression, false); err != nil {
		return fmt.Errorf("compile JSONata expression: %w", err)
	}
	return nil
}

// ImplementationVersion reports the version implemented by the pinned Go
// package, allowing callers and tests to verify the language contract.
func ImplementationVersion() string {
	return jsonata206.Version()
}
