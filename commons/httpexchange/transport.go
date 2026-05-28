package httpexchange

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/allure-framework/allure-go/commons/clock"
)

// NewTransport wraps base and records every client-side HTTP round trip as an
// Allure HTTP Exchange attachment. Response exchanges are attached when the
// response body is closed so the body can be captured without changing client
// behavior.
func NewTransport(ctx context.Context, base http.RoundTripper, opts ...Option) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &transport{
		ctx:     ctx,
		base:    base,
		options: applyOptions(opts),
	}
}

type transport struct {
	ctx     context.Context
	base    http.RoundTripper
	options options
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("httpexchange: nil request")
	}

	start := clock.NowMillis()
	capturedReq := req.Clone(req.Context())
	requestBody := wrapClientRequestBody(capturedReq, t.options)

	resp, err := t.base.RoundTrip(capturedReq)
	if err != nil {
		stop := clock.NowMillis()
		exchange := exchangeFromClient(capturedReq, requestBody, resp, nil, start, stop, t.options)
		exchange.Error = errorFromError(err)
		_ = attachWithOptions(t.ctx, exchange, t.options)
		return resp, err
	}
	if resp == nil {
		stop := clock.NowMillis()
		exchange := exchangeFromClient(capturedReq, requestBody, nil, nil, start, stop, t.options)
		_ = attachWithOptions(t.ctx, exchange, t.options)
		return nil, nil
	}
	if resp.Body == nil || resp.Body == http.NoBody {
		stop := clock.NowMillis()
		exchange := exchangeFromClient(capturedReq, requestBody, resp, nil, start, stop, t.options)
		_ = attachWithOptions(t.ctx, exchange, t.options)
		return resp, nil
	}

	responseBody := newCaptureResponseBody(resp.Body, t.options.bodyLimit, resp.ContentLength, func(captured bodyCapture, readErr error) error {
		stop := clock.NowMillis()
		exchange := exchangeFromClient(capturedReq, requestBody, resp, &captured, start, stop, t.options)
		if readErr != nil {
			exchange.Error = errorFromError(readErr)
		}
		return attachWithOptions(t.ctx, exchange, t.options)
	})
	resp.Body = responseBody
	return resp, nil
}

func wrapClientRequestBody(req *http.Request, options options) *captureReadCloser {
	if req == nil || req.Body == nil || req.Body == http.NoBody {
		return nil
	}

	body := newCaptureReadCloser(req.Body, options.bodyLimit, req.ContentLength)
	req.Body = body
	return body
}

func exchangeFromClient(req *http.Request, requestBody *captureReadCloser, resp *http.Response, responseBody *bodyCapture, start int64, stop int64, options options) Exchange {
	requestCapture := bodyCapture{}
	if requestBody != nil {
		requestCapture = requestBody.snapshot()
	}

	exchange := Exchange{
		SchemaVersion: SchemaVersion,
		Start:         start,
		Stop:          stop,
		Request:       fromRequestCapture(req, requestCapture, options),
	}
	if resp != nil {
		captured := bodyCapture{}
		if responseBody != nil {
			captured = *responseBody
		}
		response := fromResponseCapture(resp, captured, options)
		exchange.Response = &response
	}
	return exchange
}

type captureResponseBody struct {
	source   io.ReadCloser
	recorder *bodyRecorder
	known    int64
	once     sync.Once
	readErr  error
	closeErr error
	onClose  func(bodyCapture, error) error
}

func newCaptureResponseBody(source io.ReadCloser, limit int64, knownSize int64, onClose func(bodyCapture, error) error) *captureResponseBody {
	return &captureResponseBody{
		source:   source,
		recorder: newBodyRecorder(limit),
		known:    knownSize,
		onClose:  onClose,
	}
}

func (b *captureResponseBody) Read(p []byte) (int, error) {
	n, err := b.source.Read(p)
	if n > 0 {
		b.recorder.record(p[:n])
	}
	if err != nil && err != io.EOF {
		b.readErr = err
	}
	return n, err
}

func (b *captureResponseBody) Close() error {
	b.once.Do(func() {
		sourceErr := b.source.Close()
		attachErr := b.onClose(b.recorder.snapshot(b.known), b.readErr)
		if sourceErr != nil {
			b.closeErr = sourceErr
			return
		}
		b.closeErr = attachErr
	})
	return b.closeErr
}
