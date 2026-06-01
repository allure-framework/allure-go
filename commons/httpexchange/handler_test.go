package httpexchange_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	allure "github.com/allure-framework/allure-go/commons/gotest"
	"github.com/allure-framework/allure-go/commons/httpexchange"
)

func TestHandlerCoversFallbackAndPanicBranches(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Exercises server middleware branches for a nil handler and a panicking handler. " +
			"The expected result is a 404 exchange for the fallback handler and an exchange with panic details before the panic is rethrown.")

		var results AllureResults
		a.Step("exercise fallback and panic handlers in an isolated test context", func(a *allure.Context) {
			results = runWithinTestContext(a.T(), func(ctx *allure.Context) {
				fallback := httpexchange.NewHandler(ctx.Context(), nil, httpexchange.WithAttachmentName("fallback exchange"))
				recorder := httptest.NewRecorder()
				fallback.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/missing", nil))
				if recorder.Code != http.StatusNotFound {
					ctx.T().Fatalf("fallback handler returned status %d", recorder.Code)
				}

				panicHandler := httpexchange.NewHandler(ctx.Context(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					panic("boom")
				}), httpexchange.WithAttachmentName("panic exchange"))

				recovered := recoverHandlerPanic(func() {
					panicHandler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/panic", nil))
				})
				if recovered != "boom" {
					ctx.T().Fatalf("expected panic to be rethrown, got %#v", recovered)
				}
			}, "handler-branch-test", "fallback-attachment", "panic-attachment")
		})

		a.Step("verify fallback and panic attachments", func(a *allure.Context) {
			a.Attachment("handler branch artifacts", []byte(snapshotEvidence(results)), "text/plain")

			exchange := attachHTTPExchangeEvidence(a, results, "fallback exchange")
			if exchange.Response == nil || exchange.Response.Status != http.StatusNotFound {
				a.T().Fatalf("fallback exchange did not capture 404 response: %#v", exchange)
			}
			if exchange.Request.Body != nil {
				a.T().Fatalf("fallback request without body should not record a body: %#v", exchange.Request.Body)
			}

			exchange = attachHTTPExchangeEvidence(a, results, "panic exchange")
			if exchange.Error == nil || exchange.Error.Name != "panic" || exchange.Error.Message != "boom" {
				a.T().Fatalf("panic exchange did not capture panic details: %#v", exchange)
			}
		})
	})
}

func recoverHandlerPanic(fn func()) (recovered any) {
	defer func() {
		recovered = recover()
	}()
	fn()
	return nil
}
