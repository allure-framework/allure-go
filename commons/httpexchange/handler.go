package httpexchange

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"sort"
	"strings"

	"github.com/allure-framework/allure-go/commons/clock"
)

// NewHandler wraps next and records every handled server-side HTTP exchange as
// an Allure HTTP Exchange attachment.
func NewHandler(ctx context.Context, next http.Handler, opts ...Option) http.Handler {
	options := applyOptions(opts)
	if next == nil {
		next = http.NotFoundHandler()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := clock.NowMillis()
		requestBody := wrapRequestBody(r, options)
		responseWriter := newCaptureResponseWriter(w, options.bodyLimit)

		var recovered any
		defer func() {
			stop := clock.NowMillis()
			exchange := exchangeFromServer(r, requestBody, responseWriter, start, stop, options)
			if recovered != nil {
				exchange.Error = &Error{Name: "panic", Message: fmt.Sprint(recovered)}
			}
			_ = attachWithOptions(ctx, exchange, options)
			if recovered != nil {
				panic(recovered)
			}
		}()

		defer func() {
			recovered = recover()
		}()

		next.ServeHTTP(responseWriter, r)
	})
}

func wrapRequestBody(req *http.Request, options options) *captureReadCloser {
	if req == nil || req.Body == nil || req.Body == http.NoBody {
		return nil
	}

	body := newCaptureReadCloser(req.Body, options.bodyLimit, req.ContentLength)
	req.Body = body
	return body
}

func exchangeFromServer(req *http.Request, requestBody *captureReadCloser, responseWriter *captureResponseWriter, start int64, stop int64, options options) Exchange {
	requestCapture := bodyCapture{}
	if requestBody != nil {
		requestCapture = requestBody.snapshot()
	}

	status, headers, trailers, responseCapture := responseWriter.snapshot()
	response := &http.Response{
		StatusCode: status,
		Proto:      req.Proto,
		Header:     headers,
		Trailer:    trailers,
	}

	return Exchange{
		SchemaVersion: SchemaVersion,
		Start:         start,
		Stop:          stop,
		Request:       fromRequestCapture(req, requestCapture, options),
		Response:      responsePtr(fromResponseCapture(response, responseCapture, options)),
	}
}

func responsePtr(response Response) *Response {
	return &response
}

type captureResponseWriter struct {
	w          http.ResponseWriter
	recorder   *bodyRecorder
	status     int
	sentHeader http.Header
}

func newCaptureResponseWriter(w http.ResponseWriter, limit int64) *captureResponseWriter {
	return &captureResponseWriter{
		w:        w,
		recorder: newBodyRecorder(limit),
	}
}

func (w *captureResponseWriter) Header() http.Header {
	return w.w.Header()
}

func (w *captureResponseWriter) Write(content []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.w.Write(content)
	if n > 0 {
		w.recorder.record(content[:n])
	}
	return n, err
}

func (w *captureResponseWriter) WriteHeader(statusCode int) {
	if w.status != 0 {
		return
	}
	w.status = statusCode
	w.sentHeader = w.w.Header().Clone()
	w.w.WriteHeader(statusCode)
}

func (w *captureResponseWriter) Unwrap() http.ResponseWriter {
	return w.w
}

func (w *captureResponseWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *captureResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.w.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer %T does not implement http.Hijacker", w.w)
	}
	return hijacker.Hijack()
}

func (w *captureResponseWriter) Push(target string, opts *http.PushOptions) error {
	pusher, ok := w.w.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (w *captureResponseWriter) ReadFrom(reader io.Reader) (int64, error) {
	return io.Copy(w, reader)
}

func (w *captureResponseWriter) CloseNotify() <-chan bool {
	closeNotifier, ok := w.w.(http.CloseNotifier)
	if !ok {
		return make(chan bool)
	}
	return closeNotifier.CloseNotify()
}

func (w *captureResponseWriter) snapshot() (int, http.Header, http.Header, bodyCapture) {
	status := w.status
	if status == 0 {
		status = http.StatusOK
	}

	finalHeader := w.w.Header().Clone()
	header := w.sentHeader
	if header == nil {
		header = finalHeader.Clone()
	}

	trailers := extractTrailers(header, finalHeader)
	header = headerWithoutTrailerValues(header, trailers)
	return status, header, trailers, w.recorder.snapshot(-1)
}

func extractTrailers(header http.Header, finalHeader http.Header) http.Header {
	names := trailerNames(header, finalHeader)
	if len(names) == 0 {
		return nil
	}

	trailers := http.Header{}
	for _, name := range names {
		values := finalHeader.Values(name)
		values = append(values, finalHeader.Values(http.TrailerPrefix+name)...)
		if len(values) > 0 {
			trailers[textproto.CanonicalMIMEHeaderKey(name)] = append([]string(nil), values...)
		}
	}
	if len(trailers) == 0 {
		return nil
	}
	return trailers
}

func trailerNames(header http.Header, finalHeader http.Header) []string {
	seen := map[string]struct{}{}
	var names []string
	add := func(name string) {
		name = textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(name))
		if name == "" {
			return
		}
		normalized := strings.ToLower(name)
		if _, ok := seen[normalized]; ok {
			return
		}
		seen[normalized] = struct{}{}
		names = append(names, name)
	}

	for _, value := range header.Values("Trailer") {
		for _, name := range strings.Split(value, ",") {
			add(name)
		}
	}
	for key := range finalHeader {
		if strings.HasPrefix(key, http.TrailerPrefix) {
			add(strings.TrimPrefix(key, http.TrailerPrefix))
		}
	}

	sort.Strings(names)
	return names
}

func headerWithoutTrailerValues(header http.Header, trailers http.Header) http.Header {
	filtered := header.Clone()
	for key := range filtered {
		if strings.HasPrefix(key, http.TrailerPrefix) {
			delete(filtered, key)
		}
	}
	for key := range trailers {
		delete(filtered, key)
	}
	return filtered
}
