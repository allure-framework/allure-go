package httpexchange_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	allure "github.com/allure-framework/allure-go/commons/gotest"
	"github.com/allure-framework/allure-go/commons/httpexchange"
)

func TestPublicConversionHelpersAndOptions(t *testing.T) {
	allure.Test(t, "public conversion helpers and options", func(a *allure.Context) {
		a.Description("Exercises the direct request/response conversion helpers and the public conversion options. " +
			"The expected result is custom redaction, body truncation, binary body encoding, and nil input handling.")

		req := httptest.NewRequest(http.MethodPatch, "https://api.example.test/v1/items?visible=yes&custom_token=secret", strings.NewReader("pin=1234&name=demo"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
		req.Header.Set("X-Secret", "header-secret")
		req.Header.Set("Cookie", "custom_cookie=secret")

		a.Step("convert request with custom redactions", func(a *allure.Context) {
			request := httpexchange.FromRequest(
				req,
				[]byte("pin=1234&name=demo"),
				httpexchange.WithRedactedHeaders("x-secret"),
				httpexchange.WithRedactedQueryParameters("custom_token"),
				httpexchange.WithRedactedCookies("custom_cookie"),
				httpexchange.WithRedactedFormFields("pin"),
			)
			payload, err := json.MarshalIndent(request, "", "  ")
			if err != nil {
				a.T().Fatalf("marshal converted request: %v", err)
			}
			a.Attachment("converted request", payload, "application/json")

			if !strings.Contains(request.URL, "custom_token=__ALLURE_REDACTED__") {
				a.T().Fatalf("query parameter was not redacted in URL: %q", request.URL)
			}
			requirePair(a.T(), request.Headers, "X-Secret", httpexchange.RedactedValue)
			requirePair(a.T(), request.Query, "custom_token", httpexchange.RedactedValue)
			requirePair(a.T(), request.Body.Form, "pin", httpexchange.RedactedValue)
			if request.Body.Encoding != "utf8" || !strings.Contains(request.Body.Value, "pin=__ALLURE_REDACTED__") {
				a.T().Fatalf("unexpected request body: %#v", request.Body)
			}
			if len(request.Cookies) != 1 || request.Cookies[0].Value != httpexchange.RedactedValue {
				a.T().Fatalf("cookie was not redacted: %#v", request.Cookies)
			}
		})

		a.Step("convert response with body limit and binary encoding", func(a *allure.Context) {
			resp := &http.Response{
				StatusCode:    http.StatusTeapot,
				Proto:         "HTTP/1.1",
				ContentLength: 4,
				Header: http.Header{
					"Content-Type": []string{"application/octet-stream"},
					"X-Secret":     []string{"response-secret"},
				},
			}
			response := httpexchange.FromResponse(
				resp,
				[]byte{0x00, 0x01, 0x02, 0x03},
				httpexchange.WithBodyLimit(2),
				httpexchange.WithRedactedHeaders("x-secret"),
			)
			payload, err := json.MarshalIndent(response, "", "  ")
			if err != nil {
				a.T().Fatalf("marshal converted response: %v", err)
			}
			a.Attachment("converted response", payload, "application/json")

			if response.Status != http.StatusTeapot || response.StatusText != "I'm a teapot" {
				a.T().Fatalf("unexpected response status: %#v", response)
			}
			requirePair(a.T(), response.Headers, "X-Secret", httpexchange.RedactedValue)
			if response.Body.Encoding != "base64" || response.Body.Value != base64.StdEncoding.EncodeToString([]byte{0x00, 0x01}) {
				a.T().Fatalf("unexpected binary body: %#v", response.Body)
			}
			if response.Body.Size != 4 || !response.Body.Truncated {
				a.T().Fatalf("unexpected body size metadata: %#v", response.Body)
			}
		})

		a.Step("handle nil conversion inputs", func(a *allure.Context) {
			if request := httpexchange.FromRequest(nil, nil); request.Method != "" || request.Body != nil {
				a.T().Fatalf("nil request should convert to zero request: %#v", request)
			}
			if response := httpexchange.FromResponse(nil, nil); response.Status != 0 || response.Body != nil {
				a.T().Fatalf("nil response should convert to zero response: %#v", response)
			}
		})
	})
}

func TestExchangeErrorOptionsAndMarshal(t *testing.T) {
	allure.Test(t, "exchange error options and marshal", func(a *allure.Context) {
		a.Description("Covers explicit and derived exchange errors plus direct JSON marshaling. " +
			"The expected result is normalized schema v1 JSON and stable error fields.")

		req := httptest.NewRequest(http.MethodGet, "https://api.example.test/v1/fail", nil)

		a.Step("derive timeout error details", func(a *allure.Context) {
			exchange := httpexchange.NewExchange(req, nil, nil, nil, httpexchange.WithError(timeoutErr{}))
			if exchange.Error == nil || exchange.Error.Name != "TimeoutError" || exchange.Error.Message != "deadline exceeded" {
				a.T().Fatalf("unexpected derived error: %#v", exchange.Error)
			}
		})

		a.Step("record explicit error details", func(a *allure.Context) {
			exchange := httpexchange.NewExchange(req, nil, nil, nil, httpexchange.WithErrorDetails("TLSError", "bad certificate", "stack"))
			if exchange.Error == nil || exchange.Error.Name != "TLSError" || exchange.Error.Message != "bad certificate" || exchange.Error.Stack != "stack" {
				a.T().Fatalf("unexpected explicit error: %#v", exchange.Error)
			}
		})

		a.Step("marshal normalizes schema version", func(a *allure.Context) {
			payload, err := httpexchange.Marshal(httpexchange.Exchange{
				Request: httpexchange.Request{
					Method: http.MethodPost,
					URL:    "https://api.example.test/v1/items",
				},
			})
			if err != nil {
				a.T().Fatalf("marshal exchange: %v", err)
			}
			a.Attachment("marshaled exchange", payload, httpexchange.AttachmentContentType)

			var exchange httpexchange.Exchange
			if err := json.Unmarshal(payload, &exchange); err != nil {
				a.T().Fatalf("unmarshal exchange: %v", err)
			}
			if exchange.SchemaVersion != httpexchange.SchemaVersion {
				a.T().Fatalf("schema version was not normalized: %#v", exchange)
			}
		})
	})
}

func TestAttachDefaultNames(t *testing.T) {
	allure.Test(t, "attach default names", func(a *allure.Context) {
		a.Description("Writes attachments without explicit names to cover default naming branches. " +
			"The expected result is the generic name for missing methods and request-target names for root or unparsable URLs.")

		var results AllureResults
		a.Step("attach exchanges in an isolated test context", func(a *allure.Context) {
			results = runWithinTestContext(a.T(), func(ctx *allure.Context) {
				cases := []httpexchange.Exchange{
					{Request: httpexchange.Request{URL: "https://api.example.test/v1/items"}},
					{Request: httpexchange.Request{Method: http.MethodGet, URL: "https://api.example.test?x=1"}},
					{Request: httpexchange.Request{Method: http.MethodGet, URL: "://bad-url"}},
				}
				for _, exchange := range cases {
					if err := httpexchange.Attach(ctx.Context(), "", exchange); err != nil {
						ctx.T().Fatalf("attach exchange: %v", err)
					}
				}
			}, "default-name-test", "generic-attachment", "root-attachment", "invalid-attachment")
		})

		a.Step("verify generated names", func(a *allure.Context) {
			a.Attachment("default name artifacts", []byte(snapshotEvidence(results)), "text/plain")
			attachHTTPExchangeEvidence(a, results, "HTTP Exchange")
			attachHTTPExchangeEvidence(a, results, "HTTP GET /?x=1")
			attachHTTPExchangeEvidence(a, results, "HTTP GET ://bad-url")
		})
	})
}

type timeoutErr struct{}

func (timeoutErr) Error() string {
	return "deadline exceeded"
}

func (timeoutErr) Timeout() bool {
	return true
}

var _ error = timeoutErr{}
var _ interface{ Timeout() bool } = timeoutErr{}

func TestWithErrorIgnoresNil(t *testing.T) {
	allure.Test(t, "nil error option is ignored", func(a *allure.Context) {
		exchange := httpexchange.NewExchange(
			httptest.NewRequest(http.MethodGet, "https://api.example.test/v1/ok", nil),
			nil,
			nil,
			nil,
			httpexchange.WithError(nil),
		)
		if exchange.Error != nil {
			a.T().Fatalf("nil error should not be recorded: %#v", exchange.Error)
		}

		exchange = httpexchange.NewExchange(
			httptest.NewRequest(http.MethodGet, "https://api.example.test/v1/ok", nil),
			nil,
			nil,
			nil,
			httpexchange.WithError(errors.New("plain error")),
		)
		if exchange.Error == nil || exchange.Error.Name == "TimeoutError" || exchange.Error.Message != "plain error" {
			a.T().Fatalf("plain error should be recorded without timeout classification: %#v", exchange.Error)
		}
	})
}
