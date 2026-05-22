// Package ids provides identifier helpers for Allure result files.
package ids

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/allure-framework/allure-go/commons/model"
)

// New returns a random RFC 4122 version 4 UUID string.
func New() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("allure ids: generate uuid: %v", err))
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// MD5 returns the lowercase hex MD5 hash of value.
func MD5(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

// TestCaseID returns the stable Allure test case id for a full test name.
func TestCaseID(fullName string) string {
	if fullName == "" {
		return ""
	}

	return MD5(fullName)
}

// HistoryID returns the stable Allure history id for a test case id and
// non-excluded parameters.
func HistoryID(testCaseID string, parameters []model.Parameter) string {
	if testCaseID == "" {
		return ""
	}

	included := make([]model.Parameter, 0, len(parameters))
	for _, parameter := range parameters {
		if !parameter.Excluded {
			included = append(included, parameter)
		}
	}

	sort.Slice(included, func(i, j int) bool {
		if included[i].Name == included[j].Name {
			return included[i].Value < included[j].Value
		}

		return included[i].Name < included[j].Name
	})

	parts := make([]string, 0, len(included))
	for _, parameter := range included {
		parts = append(parts, parameter.Name+":"+parameter.Value)
	}

	return testCaseID + ":" + MD5(strings.Join(parts, ","))
}

// HistoryIDFromFullName derives a test case id from fullName and then returns
// the corresponding Allure history id.
func HistoryIDFromFullName(fullName string, parameters []model.Parameter) string {
	return HistoryID(TestCaseID(fullName), parameters)
}
