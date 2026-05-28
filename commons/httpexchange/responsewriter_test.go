package httpexchange

import (
	"bufio"
	"bytes"
	"net"
	"net/http"
	"strings"
	"testing"

	allure "github.com/allure-framework/allure-go/commons/gotest"
)

func TestCaptureResponseWriterOptionalInterfaces(t *testing.T) {
	allure.Test(t, "capture response writer optional interfaces", func(a *allure.Context) {
		a.Description("Covers the optional response writer interfaces preserved by the HTTP Exchange middleware. " +
			"The expected result is delegated flush, push, hijack, read-from, close notification, and trailer capture behavior.")

		base := newOptionalResponseWriter()
		writer := newCaptureResponseWriter(base, 64)

		writer.Header().Set("Content-Type", "text/plain")
		writer.Header().Set("Trailer", "X-Trailer")

		a.Step("delegate optional response writer methods", func(a *allure.Context) {
			writer.Flush()

			n, err := writer.ReadFrom(strings.NewReader("streamed response"))
			if err != nil || n != int64(len("streamed response")) {
				a.T().Fatalf("ReadFrom returned n=%d err=%v", n, err)
			}

			if unwrapped := writer.Unwrap(); unwrapped != base {
				a.T().Fatalf("unexpected unwrapped writer: %#v", unwrapped)
			}

			if err := writer.Push("/assets/app.js", nil); err != nil {
				a.T().Fatalf("push: %v", err)
			}

			conn, rw, err := writer.Hijack()
			if err != nil {
				a.T().Fatalf("hijack: %v", err)
			}
			if conn == nil || rw == nil {
				a.T().Fatalf("hijack returned conn=%#v rw=%#v", conn, rw)
			}
			_ = conn.Close()
			_ = base.hijackPeer.Close()

			if closeNotify := writer.CloseNotify(); closeNotify != base.closeCh {
				a.T().Fatal("CloseNotify did not delegate to the wrapped writer")
			}
		})

		a.Step("verify captured writer state", func(a *allure.Context) {
			writer.Header().Set("X-Trailer", "done")

			status, header, trailers, captured := writer.snapshot()
			if status != http.StatusOK || base.status != http.StatusOK {
				a.T().Fatalf("unexpected status wrapper=%d base=%d", status, base.status)
			}
			if !base.flushed || !base.hijacked || len(base.pushed) != 1 || base.pushed[0] != "/assets/app.js" {
				a.T().Fatalf("optional methods were not delegated: %#v", base)
			}
			if string(captured.content) != "streamed response" || captured.truncated {
				a.T().Fatalf("unexpected captured body: %#v", captured)
			}
			if header.Get("X-Trailer") != "" {
				a.T().Fatalf("trailer value leaked into headers: %#v", header)
			}
			if trailers.Get("X-Trailer") != "done" {
				a.T().Fatalf("trailer was not captured: %#v", trailers)
			}
		})
	})
}

type optionalResponseWriter struct {
	header     http.Header
	body       bytes.Buffer
	status     int
	flushed    bool
	hijacked   bool
	hijackPeer net.Conn
	pushed     []string
	closeCh    chan bool
}

func newOptionalResponseWriter() *optionalResponseWriter {
	return &optionalResponseWriter{
		header:  http.Header{},
		closeCh: make(chan bool),
	}
}

func (w *optionalResponseWriter) Header() http.Header {
	return w.header
}

func (w *optionalResponseWriter) Write(content []byte) (int, error) {
	return w.body.Write(content)
}

func (w *optionalResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
}

func (w *optionalResponseWriter) Flush() {
	w.flushed = true
}

func (w *optionalResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.hijacked = true
	server, client := net.Pipe()
	w.hijackPeer = client
	return server, bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server)), nil
}

func (w *optionalResponseWriter) Push(target string, opts *http.PushOptions) error {
	w.pushed = append(w.pushed, target)
	return nil
}

func (w *optionalResponseWriter) CloseNotify() <-chan bool {
	return w.closeCh
}
