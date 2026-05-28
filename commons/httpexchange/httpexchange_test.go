package httpexchange_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	allure "github.com/allure-framework/allure-go/commons/gotest"
	"github.com/allure-framework/allure-go/commons/httpexchange"
	commonswriter "github.com/allure-framework/allure-go/commons/writer"
)

func TestNewExchangeConvertsHTTPValues(t *testing.T) {
	allure.Test(t, "http exchange converts standard http values", func(a *allure.Context) {
		a.Description("Builds an HTTP Exchange payload from standard net/http request and response values. " +
			"The expected result is a schema v1 attachment payload with method, redacted URL/query/header/cookie/form values, status, body, trailers, and timestamps.")

		req := httptest.NewRequest(http.MethodPost, "https://api.example.test/v1/orders/42?dryRun=true&token=secret", strings.NewReader("username=demo&password=pw"))
		req.Header.Set("Authorization", "Bearer secret")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Cookie", "sid=secret; theme=light")
		req.Trailer = http.Header{"Grpc-Status": []string{"0"}}

		resp := &http.Response{
			StatusCode: http.StatusCreated,
			Proto:      "HTTP/2.0",
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"Set-Cookie":   []string{"sid=secret; Path=/; HttpOnly; Secure; SameSite=Lax"},
			},
			Trailer: http.Header{"Grpc-Message": []string{""}},
		}

		var exchange httpexchange.Exchange
		a.Step("construct exchange from request and response", func(a *allure.Context) {
			exchange = httpexchange.NewExchange(
				req,
				[]byte("username=demo&password=pw"),
				resp,
				[]byte(`{"id":42}`),
				httpexchange.WithStartStop(1000, 1050),
			)
			payload, err := httpexchange.Marshal(exchange)
			if err != nil {
				a.T().Fatalf("marshal exchange: %v", err)
			}
			a.Attachment("constructed http exchange", payload, httpexchange.AttachmentContentType)
		})

		a.Step("verify converted exchange fields", func(a *allure.Context) {
			if exchange.SchemaVersion != httpexchange.SchemaVersion || exchange.Start != 1000 || exchange.Stop != 1050 {
				a.T().Fatalf("unexpected root fields: %#v", exchange)
			}
			if exchange.Request.Method != http.MethodPost {
				a.T().Fatalf("unexpected method: %q", exchange.Request.Method)
			}
			if exchange.Request.URL != "https://api.example.test/v1/orders/42?dryRun=true&token=__ALLURE_REDACTED__" {
				a.T().Fatalf("request url was not redacted: %q", exchange.Request.URL)
			}
			requirePair(a.T(), exchange.Request.Headers, "Authorization", httpexchange.RedactedValue)
			requirePair(a.T(), exchange.Request.Query, "token", httpexchange.RedactedValue)
			requirePair(a.T(), exchange.Request.Body.Form, "password", httpexchange.RedactedValue)
			if !strings.Contains(exchange.Request.Body.Value, "password=__ALLURE_REDACTED__") {
				a.T().Fatalf("form body value was not redacted: %#v", exchange.Request.Body)
			}
			if len(exchange.Request.Cookies) != 2 || exchange.Request.Cookies[0].Value != httpexchange.RedactedValue {
				a.T().Fatalf("request cookies were not redacted: %#v", exchange.Request.Cookies)
			}
			requirePair(a.T(), exchange.Request.Trailers, "Grpc-Status", "0")

			if exchange.Response == nil {
				a.T().Fatal("missing response")
			}
			if exchange.Response.Status != http.StatusCreated || exchange.Response.StatusText != "Created" {
				a.T().Fatalf("unexpected response status: %#v", exchange.Response)
			}
			if exchange.Response.Body.Encoding != "utf8" || exchange.Response.Body.Value != `{"id":42}` {
				a.T().Fatalf("unexpected response body: %#v", exchange.Response.Body)
			}
			requirePair(a.T(), exchange.Response.Headers, "Set-Cookie", httpexchange.RedactedValue)
			requirePair(a.T(), exchange.Response.Trailers, "Grpc-Message", "")
			if len(exchange.Response.Cookies) != 1 || exchange.Response.Cookies[0].SameSite != "Lax" || exchange.Response.Cookies[0].Value != httpexchange.RedactedValue {
				a.T().Fatalf("response cookie was not converted: %#v", exchange.Response.Cookies)
			}
		})
	})
}

func TestAttachWritesHTTPExchangeAttachment(t *testing.T) {
	allure.Test(t, "http exchange attach writes typed attachment", func(a *allure.Context) {
		a.Description("Runs a callback inside an isolated reported test context and attaches an HTTP Exchange payload through the package helper. " +
			"The expected result is a typed application/vnd.allure.http+json attachment with the .httpexchange source extension and schema v1 JSON payload.")

		exchange := httpexchange.Exchange{
			Request: httpexchange.Request{
				Method: http.MethodGet,
				URL:    "https://api.example.test/v1/health",
			},
		}

		var results AllureResults
		a.Step("attach exchange in an isolated test context", func(a *allure.Context) {
			results = runWithinTestContext(a.T(), func(ctx *allure.Context) {
				if err := httpexchange.Attach(ctx.Context(), "", exchange); err != nil {
					ctx.T().Fatalf("attach exchange: %v", err)
				}
			}, "attach-test", "attach-payload")
		})

		a.Step("verify attachment metadata and payload", func(a *allure.Context) {
			a.Attachment("generated exchange artifacts", []byte(snapshotEvidence(results)), "text/plain")
			if len(results.Results) != 1 {
				a.T().Fatalf("expected one result, got %d", len(results.Results))
			}
			if len(results.Results[0].Attachments) != 1 {
				a.T().Fatalf("expected one attachment, got %#v", results.Results[0].Attachments)
			}
			attachment := results.Results[0].Attachments[0]
			if attachment.Name != "HTTP GET /v1/health" {
				a.T().Fatalf("unexpected attachment name: %#v", attachment)
			}
			if attachment.Type != httpexchange.AttachmentContentType {
				a.T().Fatalf("unexpected attachment type: %#v", attachment)
			}
			if attachment.Source != "attach-payload-attachment.httpexchange" {
				a.T().Fatalf("unexpected attachment source: %#v", attachment)
			}
			captured := attachHTTPExchangeEvidence(a, results, attachment.Name)
			if captured.SchemaVersion != httpexchange.SchemaVersion || captured.Request.URL != exchange.Request.URL {
				a.T().Fatalf("unexpected captured exchange: %#v", captured)
			}
		})
	})
}

func TestHandlerCapturesHttptestServerExchange(t *testing.T) {
	allure.Test(t, "handler captures httptest server exchange", func(a *allure.Context) {
		a.Description("Runs a real http.Client request against an httptest.Server wrapped with the HTTP Exchange handler middleware. " +
			"The expected result is one server-side exchange attachment containing the received request body and the response returned by the fake service.")

		var results AllureResults
		a.Step("exercise wrapped httptest server in an isolated test context", func(a *allure.Context) {
			results = runWithinTestContext(a.T(), func(ctx *allure.Context) {
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					body, err := io.ReadAll(r.Body)
					if err != nil {
						ctx.T().Fatalf("read request body: %v", err)
					}
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("Trailer", "X-Service-Trailer")
					w.WriteHeader(http.StatusAccepted)
					_, _ = fmt.Fprintf(w, `{"received":%q}`, string(body))
					w.Header().Set("X-Service-Trailer", "done")
				})
				server := httptest.NewServer(httpexchange.NewHandler(ctx.Context(), handler, httpexchange.WithAttachmentName("server exchange")))
				defer server.Close()

				req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/orders?token=secret", strings.NewReader(`{"name":"demo"}`))
				if err != nil {
					ctx.T().Fatalf("create request: %v", err)
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer secret")

				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					ctx.T().Fatalf("send request: %v", err)
				}
				defer resp.Body.Close()
				if _, err := io.ReadAll(resp.Body); err != nil {
					ctx.T().Fatalf("read response body: %v", err)
				}
			}, "server-test", "server-attachment")
		})

		a.Step("verify server-side exchange attachment", func(a *allure.Context) {
			a.Attachment("server exchange artifacts", []byte(snapshotEvidence(results)), "text/plain")
			exchange := attachHTTPExchangeEvidence(a, results, "server exchange")
			if exchange.Request.Method != http.MethodPost {
				a.T().Fatalf("unexpected captured method: %#v", exchange.Request)
			}
			if !strings.Contains(exchange.Request.URL, "token=__ALLURE_REDACTED__") {
				a.T().Fatalf("request URL was not redacted: %q", exchange.Request.URL)
			}
			requirePair(a.T(), exchange.Request.Headers, "Authorization", httpexchange.RedactedValue)
			if exchange.Request.Body == nil || exchange.Request.Body.Value != `{"name":"demo"}` {
				a.T().Fatalf("request body was not captured: %#v", exchange.Request.Body)
			}
			if exchange.Response == nil || exchange.Response.Status != http.StatusAccepted {
				a.T().Fatalf("response was not captured: %#v", exchange.Response)
			}
			if !strings.Contains(exchange.Response.Body.Value, `{"received":"{\"name\":\"demo\"}"}`) {
				a.T().Fatalf("response body was not captured: %#v", exchange.Response.Body)
			}
			requirePair(a.T(), exchange.Response.Trailers, "X-Service-Trailer", "done")
		})
	})
}

func TestTransportCapturesClientExchange(t *testing.T) {
	allure.Test(t, "transport captures client exchange", func(a *allure.Context) {
		a.Description("Runs an http.Client using the HTTP Exchange transport wrapper against an httptest.Server. " +
			"The expected result is one client-side exchange attachment after response body close, including request and response bodies.")

		var results AllureResults
		a.Step("exercise wrapped transport in an isolated test context", func(a *allure.Context) {
			results = runWithinTestContext(a.T(), func(ctx *allure.Context) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					body, err := io.ReadAll(r.Body)
					if err != nil {
						ctx.T().Fatalf("read request body: %v", err)
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = fmt.Fprintf(w, `{"echo":%q}`, string(body))
				}))
				defer server.Close()

				client := &http.Client{
					Transport: httpexchange.NewTransport(ctx.Context(), http.DefaultTransport, httpexchange.WithAttachmentName("client exchange")),
				}
				req, err := http.NewRequest(http.MethodPut, server.URL+"/v1/profile?api_key=secret", strings.NewReader(`{"displayName":"Demo"}`))
				if err != nil {
					ctx.T().Fatalf("create request: %v", err)
				}
				req.Header.Set("Content-Type", "application/json")

				resp, err := client.Do(req)
				if err != nil {
					ctx.T().Fatalf("send request: %v", err)
				}
				if _, err := io.ReadAll(resp.Body); err != nil {
					ctx.T().Fatalf("read response body: %v", err)
				}
				if err := resp.Body.Close(); err != nil {
					ctx.T().Fatalf("close response body: %v", err)
				}
			}, "client-test", "client-attachment")
		})

		a.Step("verify client-side exchange attachment", func(a *allure.Context) {
			a.Attachment("client exchange artifacts", []byte(snapshotEvidence(results)), "text/plain")
			exchange := attachHTTPExchangeEvidence(a, results, "client exchange")
			if exchange.Request.Method != http.MethodPut {
				a.T().Fatalf("unexpected captured method: %#v", exchange.Request)
			}
			if !strings.Contains(exchange.Request.URL, "api_key=__ALLURE_REDACTED__") {
				a.T().Fatalf("request URL was not redacted: %q", exchange.Request.URL)
			}
			if exchange.Request.Body == nil || exchange.Request.Body.Value != `{"displayName":"Demo"}` {
				a.T().Fatalf("request body was not captured: %#v", exchange.Request.Body)
			}
			if exchange.Response == nil || exchange.Response.Status != http.StatusOK {
				a.T().Fatalf("response was not captured: %#v", exchange.Response)
			}
			if !strings.Contains(exchange.Response.Body.Value, `{"echo":"{\"displayName\":\"Demo\"}"}`) {
				a.T().Fatalf("response body was not captured: %#v", exchange.Response.Body)
			}
		})
	})
}

type AllureResults = commonswriter.MemorySnapshot

func runWithinTestContext(t *testing.T, body func(*allure.Context), ids ...string) AllureResults {
	t.Helper()

	memory := commonswriter.NewInMemoryWriter()
	options := []allure.Option{allure.WithWriter(memory)}
	if len(ids) > 0 {
		options = append(options, allure.WithIDGenerator(fixedIDs(ids...)))
	}

	allure.Test(t, "test context", body, options...)
	return memory.Snapshot()
}

func requirePair(t *testing.T, pairs []httpexchange.NameValue, name string, value string) {
	t.Helper()
	for _, pair := range pairs {
		if pair.Name == name && pair.Value == value {
			return
		}
	}
	t.Fatalf("missing pair %s=%s in %#v", name, value, pairs)
}

func attachHTTPExchangeEvidence(a *allure.Context, snapshot AllureResults, name string) httpexchange.Exchange {
	a.Helper()
	payload := attachmentPayload(a.T(), snapshot, name)
	a.Attachment("tested "+name, payload, httpexchange.AttachmentContentType)

	var exchange httpexchange.Exchange
	if err := json.Unmarshal(payload, &exchange); err != nil {
		a.T().Fatalf("unmarshal exchange attachment: %v\n%s", err, payload)
	}
	return exchange
}

func decodeAttachmentExchange(t *testing.T, snapshot AllureResults, name string) httpexchange.Exchange {
	t.Helper()
	payload := attachmentPayload(t, snapshot, name)
	var exchange httpexchange.Exchange
	if err := json.Unmarshal(payload, &exchange); err != nil {
		t.Fatalf("unmarshal exchange attachment: %v\n%s", err, payload)
	}
	return exchange
}

func attachmentPayload(t *testing.T, snapshot AllureResults, name string) []byte {
	t.Helper()
	for _, result := range snapshot.Results {
		for _, attachment := range result.Attachments {
			if attachment.Name != name {
				continue
			}
			payload, ok := snapshot.Attachments[attachment.Source]
			if !ok {
				t.Fatalf("missing attachment payload for %q", attachment.Source)
			}
			return payload
		}
	}
	t.Fatalf("missing attachment %q in %#v", name, snapshot.Results)
	return nil
}

func fixedIDs(values ...string) func() string {
	index := 0
	return func() string {
		if index >= len(values) {
			index++
			return fmt.Sprintf("extra-id-%d", index)
		}
		value := values[index]
		index++
		return value
	}
}

func snapshotEvidence(snapshot AllureResults) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("results=%d attachments=%d globals=%d\n", len(snapshot.Results), len(snapshot.Attachments), len(snapshot.Globals)))
	for _, result := range snapshot.Results {
		builder.WriteString(fmt.Sprintf("- result %s attachments=%d steps=%d\n", result.Name, len(result.Attachments), len(result.Steps)))
		for _, attachment := range result.Attachments {
			builder.WriteString(fmt.Sprintf("  - %s type=%s source=%s size=%d\n", attachment.Name, attachment.Type, attachment.Source, attachment.Size))
		}
	}
	return builder.String()
}
