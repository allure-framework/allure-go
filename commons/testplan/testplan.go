// Package testplan parses and matches Allure test plan files.
package testplan

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

// EnvPath is the environment variable that points to an Allure test plan file.
const EnvPath = "ALLURE_TESTPLAN_PATH"

// SkipLabelName is the label name adapters may use to mark plan-filtered tests.
const SkipLabelName = "ALLURE_TESTPLAN_SKIP"

// Plan is the supported Allure test plan JSON document.
type Plan struct {
	Version string `json:"version,omitempty"`
	Tests   []Test `json:"tests,omitempty"`
}

// Test is one entry in an Allure test plan.
type Test struct {
	ID       any    `json:"id,omitempty"`
	Selector string `json:"selector,omitempty"`
}

// Subject describes a discovered test candidate that can be matched against a
// test plan.
type Subject struct {
	AllureID       string
	FullName       string
	NativeSelector string
	Tags           []string
}

// LoadFromEnv loads the test plan referenced by EnvPath.
func LoadFromEnv() (*Plan, error) {
	path := os.Getenv(EnvPath)
	if path == "" {
		return nil, nil
	}

	return LoadFile(path)
}

// LoadFile reads and parses an Allure test plan from path.
func LoadFile(path string) (*Plan, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read test plan: %w", err)
	}

	return Parse(content)
}

// Parse decodes an Allure test plan JSON document.
func Parse(content []byte) (*Plan, error) {
	var plan Plan
	if err := json.Unmarshal(content, &plan); err != nil {
		return nil, fmt.Errorf("parse test plan: %w", err)
	}

	if plan.Version != "" && plan.Version != "1.0" {
		return nil, fmt.Errorf("unsupported test plan version %q", plan.Version)
	}
	if len(plan.Tests) == 0 {
		return nil, nil
	}

	return &plan, nil
}

// Includes reports whether subject is selected by plan.
func Includes(plan *Plan, subject Subject) bool {
	if plan == nil || len(plan.Tests) == 0 {
		return true
	}

	effectiveID := subject.AllureID
	if effectiveID == "" {
		effectiveID = AllureIDFromTags(subject.Tags)
	}

	for _, test := range plan.Tests {
		if effectiveID != "" && test.ID != nil && fmt.Sprint(test.ID) == effectiveID {
			return true
		}
		if subject.FullName != "" && test.Selector == subject.FullName {
			return true
		}
		if subject.NativeSelector != "" && test.Selector == subject.NativeSelector {
			return true
		}
	}

	return false
}

var allureIDTagPattern = regexp.MustCompile(`^@allure\.id[:=](.+)$`)

// AllureIDFromTags returns the first Allure id encoded as @allure.id=<id> or
// @allure.id:<id> in tags.
func AllureIDFromTags(tags []string) string {
	for _, tag := range tags {
		matches := allureIDTagPattern.FindStringSubmatch(tag)
		if len(matches) == 2 {
			return matches[1]
		}
	}

	return ""
}
