// Package require proxies testify requirements and reports each requirement as an Allure step.
package require

import upstream "github.com/stretchr/testify/require"

// TestingT is an alias for testify's require TestingT interface.
type TestingT = upstream.TestingT

// ComparisonAssertionFunc is an alias for testify's comparison requirement function type.
type ComparisonAssertionFunc = upstream.ComparisonAssertionFunc

// ValueAssertionFunc is an alias for testify's value requirement function type.
type ValueAssertionFunc = upstream.ValueAssertionFunc

// BoolAssertionFunc is an alias for testify's bool requirement function type.
type BoolAssertionFunc = upstream.BoolAssertionFunc

// ErrorAssertionFunc is an alias for testify's error requirement function type.
type ErrorAssertionFunc = upstream.ErrorAssertionFunc

// Assertions provides requirement methods around the TestingT interface.
type Assertions struct {
	t TestingT
}

// New makes a new Assertions object for the specified TestingT.
func New(t TestingT) *Assertions {
	return &Assertions{t: t}
}
