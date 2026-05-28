// Package assert proxies testify assertions and reports each assertion as an Allure step.
package assert

import upstream "github.com/stretchr/testify/assert"

//go:generate go run ../internal/testifygen/generate.go

// TestingT is an alias for testify's assert TestingT interface.
type TestingT = upstream.TestingT

// ComparisonAssertionFunc is an alias for testify's comparison assertion function type.
type ComparisonAssertionFunc = upstream.ComparisonAssertionFunc

// ValueAssertionFunc is an alias for testify's value assertion function type.
type ValueAssertionFunc = upstream.ValueAssertionFunc

// BoolAssertionFunc is an alias for testify's bool assertion function type.
type BoolAssertionFunc = upstream.BoolAssertionFunc

// ErrorAssertionFunc is an alias for testify's error assertion function type.
type ErrorAssertionFunc = upstream.ErrorAssertionFunc

// PanicAssertionFunc is an alias for testify's panic assertion function type.
type PanicAssertionFunc = upstream.PanicAssertionFunc

// PanicTestFunc is an alias for testify's panic test function type.
type PanicTestFunc = upstream.PanicTestFunc

// Comparison is an alias for testify's custom comparison type.
type Comparison = upstream.Comparison

// CollectT is an alias for testify's EventuallyWithT collector type.
type CollectT = upstream.CollectT

// CompareType is an alias for testify's deprecated comparison result type.
type CompareType = upstream.CompareType

// AnError is testify's sentinel error for testing.
var AnError = upstream.AnError

// Assertions provides assertion methods around the TestingT interface.
type Assertions struct {
	t TestingT
}

// New makes a new Assertions object for the specified TestingT.
func New(t TestingT) *Assertions {
	return &Assertions{t: t}
}
