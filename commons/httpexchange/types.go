// Package httpexchange builds Allure HTTP Exchange attachments from Go's
// standard net/http types.
package httpexchange

import "net/http"

const (
	// SchemaVersion is the Allure HTTP Exchange schema version emitted by this package.
	SchemaVersion = 1
	// AttachmentContentType is the MIME type for Allure HTTP Exchange attachments.
	AttachmentContentType = "application/vnd.allure.http+json"
	// AttachmentFileExtension is the recommended extension for HTTP Exchange attachments.
	AttachmentFileExtension = ".httpexchange"
	// RedactedValue is the v1 sentinel for redacted header, query, form, and cookie values.
	RedactedValue = "__ALLURE_REDACTED__"
)

const (
	defaultAttachmentName = "HTTP Exchange"
	defaultBodyLimit      = int64(64 * 1024)
)

// NameValue is the common shape used for headers, query parameters, and form fields.
type NameValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Cookie is the HTTP Exchange cookie shape.
type Cookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain,omitempty"`
	Expires  string `json:"expires,omitempty"`
	HTTPOnly bool   `json:"httpOnly,omitempty"`
	MaxAge   int    `json:"maxAge,omitempty"`
	Path     string `json:"path,omitempty"`
	SameSite string `json:"sameSite,omitempty"`
	Secure   bool   `json:"secure,omitempty"`
}

// Stream describes captured streaming body metadata.
type Stream struct {
	Type       string `json:"type,omitempty"`
	Complete   *bool  `json:"complete,omitempty"`
	ChunkCount int    `json:"chunkCount,omitempty"`
}

// BodyPart describes one captured multipart body part.
type BodyPart struct {
	Name        string      `json:"name,omitempty"`
	FileName    string      `json:"fileName,omitempty"`
	Headers     []NameValue `json:"headers,omitempty"`
	ContentType string      `json:"contentType,omitempty"`
	Encoding    string      `json:"encoding,omitempty"`
	Value       string      `json:"value,omitempty"`
	Size        int64       `json:"size,omitempty"`
	Truncated   bool        `json:"truncated,omitempty"`
}

// Body describes a captured request or response body.
type Body struct {
	ContentType string      `json:"contentType,omitempty"`
	Encoding    string      `json:"encoding,omitempty"`
	Value       string      `json:"value,omitempty"`
	Size        int64       `json:"size,omitempty"`
	Truncated   bool        `json:"truncated,omitempty"`
	Form        []NameValue `json:"form,omitempty"`
	Parts       []BodyPart  `json:"parts,omitempty"`
	Stream      *Stream     `json:"stream,omitempty"`
}

// Request is the HTTP Exchange request object.
type Request struct {
	Method      string      `json:"method"`
	URL         string      `json:"url"`
	HTTPVersion string      `json:"httpVersion,omitempty"`
	Cookies     []Cookie    `json:"cookies,omitempty"`
	Headers     []NameValue `json:"headers,omitempty"`
	Query       []NameValue `json:"query,omitempty"`
	Body        *Body       `json:"body,omitempty"`
	Trailers    []NameValue `json:"trailers,omitempty"`
}

// InformationalResponse describes a captured 1xx response.
type InformationalResponse struct {
	Status     int         `json:"status,omitempty"`
	StatusText string      `json:"statusText,omitempty"`
	Headers    []NameValue `json:"headers,omitempty"`
}

// Response is the HTTP Exchange response object.
type Response struct {
	Status                 int                     `json:"status,omitempty"`
	StatusText             string                  `json:"statusText,omitempty"`
	HTTPVersion            string                  `json:"httpVersion,omitempty"`
	Cookies                []Cookie                `json:"cookies,omitempty"`
	Headers                []NameValue             `json:"headers,omitempty"`
	Body                   *Body                   `json:"body,omitempty"`
	Trailers               []NameValue             `json:"trailers,omitempty"`
	InformationalResponses []InformationalResponse `json:"informationalResponses,omitempty"`
}

// Error describes a transport, timeout, cancellation, TLS, DNS, or protocol error.
type Error struct {
	Name    string `json:"name,omitempty"`
	Message string `json:"message,omitempty"`
	Stack   string `json:"stack,omitempty"`
}

// Exchange is the root Allure HTTP Exchange payload.
type Exchange struct {
	SchemaVersion int       `json:"schemaVersion"`
	Start         int64     `json:"start,omitempty"`
	Stop          int64     `json:"stop,omitempty"`
	Request       Request   `json:"request"`
	Response      *Response `json:"response,omitempty"`
	Error         *Error    `json:"error,omitempty"`
}

// AttachmentNamer returns the attachment name for a captured exchange.
type AttachmentNamer func(Exchange) string

type options struct {
	bodyLimit          int64
	redactedHeaders    map[string]struct{}
	redactedQueries    map[string]struct{}
	redactedCookies    map[string]struct{}
	redactedFormFields map[string]struct{}
	start              int64
	stop               int64
	err                *Error
	attachmentNamer    AttachmentNamer
}

// Option customizes conversion, redaction, and capture helpers.
type Option func(*options)

// WithBodyLimit limits captured body values to limit bytes. A negative limit
// captures all bytes. A zero limit records body metadata without a value.
func WithBodyLimit(limit int64) Option {
	return func(options *options) {
		options.bodyLimit = limit
	}
}

// WithRedactedHeaders adds case-insensitive header names that should use RedactedValue.
func WithRedactedHeaders(names ...string) Option {
	return func(options *options) {
		addRedactions(options.redactedHeaders, names...)
	}
}

// WithRedactedQueryParameters adds case-insensitive query parameter names that
// should use RedactedValue in both request.url and request.query.
func WithRedactedQueryParameters(names ...string) Option {
	return func(options *options) {
		addRedactions(options.redactedQueries, names...)
	}
}

// WithRedactedCookies adds case-insensitive cookie names that should use RedactedValue.
func WithRedactedCookies(names ...string) Option {
	return func(options *options) {
		addRedactions(options.redactedCookies, names...)
	}
}

// WithRedactedFormFields adds case-insensitive URL-encoded form field names
// that should use RedactedValue in both body.value and body.form.
func WithRedactedFormFields(names ...string) Option {
	return func(options *options) {
		addRedactions(options.redactedFormFields, names...)
	}
}

// WithStartStop sets exchange start and stop timestamps as Unix epoch milliseconds.
func WithStartStop(start int64, stop int64) Option {
	return func(options *options) {
		options.start = start
		options.stop = stop
	}
}

// WithError records err as the exchange transport error.
func WithError(err error) Option {
	return func(options *options) {
		options.err = errorFromError(err)
	}
}

// WithErrorDetails records explicit error details on the exchange.
func WithErrorDetails(name string, message string, stack string) Option {
	return func(options *options) {
		options.err = &Error{Name: name, Message: message, Stack: stack}
	}
}

// WithAttachmentName sets a fixed attachment name for handler and transport captures.
func WithAttachmentName(name string) Option {
	return func(options *options) {
		options.attachmentNamer = func(Exchange) string {
			return name
		}
	}
}

// WithAttachmentNamer sets a dynamic attachment name for handler and transport captures.
func WithAttachmentNamer(namer AttachmentNamer) Option {
	return func(options *options) {
		if namer != nil {
			options.attachmentNamer = namer
		}
	}
}

func defaultOptions() options {
	options := options{
		bodyLimit:          defaultBodyLimit,
		redactedHeaders:    map[string]struct{}{},
		redactedQueries:    map[string]struct{}{},
		redactedCookies:    map[string]struct{}{},
		redactedFormFields: map[string]struct{}{},
		attachmentNamer:    defaultNamer,
	}
	addRedactions(options.redactedHeaders,
		"authorization",
		"proxy-authorization",
		"cookie",
		"set-cookie",
		"x-api-key",
		"x-auth-token",
	)
	addRedactions(options.redactedQueries,
		"access_token",
		"api_key",
		"apikey",
		"client_secret",
		"id_token",
		"password",
		"refresh_token",
		"secret",
		"token",
	)
	addRedactions(options.redactedFormFields,
		"access_token",
		"api_key",
		"apikey",
		"client_secret",
		"id_token",
		"password",
		"refresh_token",
		"secret",
		"token",
	)
	options.redactedCookies["*"] = struct{}{}
	return options
}

func applyOptions(opts []Option) options {
	options := defaultOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	return options
}

func statusText(status int) string {
	if status == 0 {
		return ""
	}
	return http.StatusText(status)
}
