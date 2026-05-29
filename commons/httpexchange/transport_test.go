package httpexchange_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	allure "github.com/allure-framework/allure-go/commons/gotest"
	"github.com/allure-framework/allure-go/commons/httpexchange"
)

func TestTransportCoversImmediateAndErrorBranches(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Exercises transport branches that attach before a response body is closed, plus response body read errors. " +
			"The expected result is typed exchange attachments for transport errors, nil responses, no-body responses, dynamic names, and read failures.")

		readErr := errors.New("read failed")

		var results AllureResults
		a.Step("exercise transport branches in an isolated test context", func(a *allure.Context) {
			results = runWithinTestContext(a.T(), func(ctx *allure.Context) {
				if httpexchange.NewTransport(ctx.Context(), nil) == nil {
					ctx.T().Fatal("default transport wrapper should not be nil")
				}

				if _, err := httpexchange.NewTransport(ctx.Context(), roundTripFunc(func(req *http.Request) (*http.Response, error) {
					return nil, nil
				})).RoundTrip(nil); err == nil {
					ctx.T().Fatal("nil request should return an error")
				}

				req := httptest.NewRequest(http.MethodGet, "https://api.example.test/v1/error", nil)
				_, err := httpexchange.NewTransport(ctx.Context(), roundTripFunc(func(req *http.Request) (*http.Response, error) {
					return nil, timeoutErr{}
				}), httpexchange.WithAttachmentName("transport error")).RoundTrip(req)
				if err == nil {
					ctx.T().Fatal("transport error branch should return the base error")
				}

				req = httptest.NewRequest(http.MethodGet, "https://api.example.test/v1/nil-response", nil)
				resp, err := httpexchange.NewTransport(ctx.Context(), roundTripFunc(func(req *http.Request) (*http.Response, error) {
					return nil, nil
				}), httpexchange.WithAttachmentName("nil response")).RoundTrip(req)
				if err != nil || resp != nil {
					ctx.T().Fatalf("nil response branch should return nil response and nil error, got resp=%#v err=%v", resp, err)
				}

				req = httptest.NewRequest(http.MethodHead, "https://api.example.test/v1/no-body", nil)
				resp, err = httpexchange.NewTransport(ctx.Context(), roundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusNoContent,
						Proto:      "HTTP/1.1",
						Header:     http.Header{"X-Trace": []string{"trace-1"}},
						Request:    req,
					}, nil
				}), httpexchange.WithAttachmentName("no body response")).RoundTrip(req)
				if err != nil || resp == nil || resp.StatusCode != http.StatusNoContent {
					ctx.T().Fatalf("unexpected no-body response: resp=%#v err=%v", resp, err)
				}

				req = httptest.NewRequest(http.MethodDelete, "https://api.example.test/v1/dynamic", nil)
				resp, err = httpexchange.NewTransport(ctx.Context(), roundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusNoContent,
						Proto:      "HTTP/1.1",
						Body:       http.NoBody,
						Request:    req,
					}, nil
				}), httpexchange.WithAttachmentNamer(func(exchange httpexchange.Exchange) string {
					return "dynamic " + exchange.Request.Method
				})).RoundTrip(req)
				if err != nil || resp == nil || resp.StatusCode != http.StatusNoContent {
					ctx.T().Fatalf("unexpected dynamic-name response: resp=%#v err=%v", resp, err)
				}

				req = httptest.NewRequest(http.MethodGet, "https://api.example.test/v1/read-error", nil)
				resp, err = httpexchange.NewTransport(ctx.Context(), roundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode:    http.StatusOK,
						Proto:         "HTTP/1.1",
						ContentLength: 10,
						Header:        http.Header{"Content-Type": []string{"text/plain"}},
						Body:          &failingReadCloser{content: []byte("partial"), err: readErr},
						Request:       req,
					}, nil
				}), httpexchange.WithAttachmentName("read error response")).RoundTrip(req)
				if err != nil {
					ctx.T().Fatalf("unexpected round trip error: %v", err)
				}
				buffer := make([]byte, 16)
				n, err := resp.Body.Read(buffer)
				if n != len("partial") || !errors.Is(err, readErr) {
					ctx.T().Fatalf("expected read failure after partial body, got n=%d err=%v", n, err)
				}
				if err := resp.Body.Close(); err != nil {
					ctx.T().Fatalf("close response body: %v", err)
				}
			},
				"transport-branch-test",
				"transport-error-attachment",
				"nil-response-attachment",
				"no-body-attachment",
				"dynamic-attachment",
				"read-error-attachment",
			)
		})

		a.Step("verify branch attachments", func(a *allure.Context) {
			a.Attachment("transport branch artifacts", []byte(snapshotEvidence(results)), "text/plain")

			exchange := attachHTTPExchangeEvidence(a, results, "transport error")
			if exchange.Error == nil || exchange.Error.Name != "TimeoutError" || exchange.Response != nil {
				a.T().Fatalf("unexpected transport error exchange: %#v", exchange)
			}

			exchange = attachHTTPExchangeEvidence(a, results, "nil response")
			if exchange.Response != nil || exchange.Error != nil {
				a.T().Fatalf("unexpected nil response exchange: %#v", exchange)
			}

			exchange = attachHTTPExchangeEvidence(a, results, "no body response")
			if exchange.Response == nil || exchange.Response.Status != http.StatusNoContent || exchange.Response.Body != nil {
				a.T().Fatalf("unexpected no-body exchange: %#v", exchange)
			}
			requirePair(a.T(), exchange.Response.Headers, "X-Trace", "trace-1")

			exchange = attachHTTPExchangeEvidence(a, results, "dynamic DELETE")
			if exchange.Request.Method != http.MethodDelete {
				a.T().Fatalf("unexpected dynamic exchange: %#v", exchange)
			}

			exchange = attachHTTPExchangeEvidence(a, results, "read error response")
			if exchange.Error == nil || exchange.Error.Message != readErr.Error() {
				a.T().Fatalf("read error was not captured: %#v", exchange)
			}
			if exchange.Response == nil || exchange.Response.Body == nil || exchange.Response.Body.Value != "partial" || !exchange.Response.Body.Truncated {
				a.T().Fatalf("partial response body was not captured: %#v", exchange.Response)
			}
		})
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type failingReadCloser struct {
	content []byte
	err     error
}

func (r *failingReadCloser) Read(p []byte) (int, error) {
	if len(r.content) == 0 {
		return 0, io.EOF
	}

	n := copy(p, r.content)
	r.content = r.content[n:]
	return n, r.err
}

func (r *failingReadCloser) Close() error {
	return nil
}
