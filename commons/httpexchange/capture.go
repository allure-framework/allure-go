package httpexchange

import (
	"bytes"
	"io"
	"sync"
)

type bodyRecorder struct {
	mu        sync.Mutex
	limit     int64
	buffer    bytes.Buffer
	size      int64
	truncated bool
}

func newBodyRecorder(limit int64) *bodyRecorder {
	return &bodyRecorder{limit: limit}
}

func (r *bodyRecorder) record(content []byte) {
	if len(content) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.size += int64(len(content))
	if r.limit >= 0 && r.size > r.limit {
		r.truncated = true
	}

	if r.limit == 0 {
		return
	}
	if r.limit < 0 {
		_, _ = r.buffer.Write(content)
		return
	}

	remaining := r.limit - int64(r.buffer.Len())
	if remaining <= 0 {
		return
	}
	if int64(len(content)) > remaining {
		content = content[:remaining]
	}
	_, _ = r.buffer.Write(content)
}

func (r *bodyRecorder) snapshot(knownSize int64) bodyCapture {
	if r == nil {
		return bodyCapture{}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	size := r.size
	truncated := r.truncated
	if knownSize >= 0 {
		size = knownSize
		if knownSize < r.size {
			size = r.size
		}
		if knownSize > int64(r.buffer.Len()) {
			truncated = true
		}
	}

	return bodyCapture{
		present:   true,
		content:   append([]byte(nil), r.buffer.Bytes()...),
		size:      size,
		truncated: truncated,
	}
}

type captureReadCloser struct {
	source   io.ReadCloser
	recorder *bodyRecorder
	known    int64
}

func newCaptureReadCloser(source io.ReadCloser, limit int64, knownSize int64) *captureReadCloser {
	return &captureReadCloser{
		source:   source,
		recorder: newBodyRecorder(limit),
		known:    knownSize,
	}
}

func (c *captureReadCloser) Read(p []byte) (int, error) {
	n, err := c.source.Read(p)
	if n > 0 {
		c.recorder.record(p[:n])
	}
	return n, err
}

func (c *captureReadCloser) Close() error {
	return c.source.Close()
}

func (c *captureReadCloser) snapshot() bodyCapture {
	if c == nil {
		return bodyCapture{}
	}
	return c.recorder.snapshot(c.known)
}
