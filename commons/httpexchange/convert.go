package httpexchange

import (
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type bodyCapture struct {
	present   bool
	content   []byte
	size      int64
	truncated bool
}

// NewExchange builds an Allure HTTP Exchange payload from standard net/http
// request and response values. Request and response body arguments must be the
// bytes already captured by the caller; nil means the body was not captured.
func NewExchange(req *http.Request, requestBody []byte, resp *http.Response, responseBody []byte, opts ...Option) Exchange {
	options := applyOptions(opts)
	exchange := Exchange{
		SchemaVersion: SchemaVersion,
		Start:         options.start,
		Stop:          options.stop,
		Request:       FromRequest(req, requestBody, opts...),
		Error:         options.err,
	}
	if resp != nil {
		response := FromResponse(resp, responseBody, opts...)
		exchange.Response = &response
	}
	return exchange
}

// FromRequest builds the request portion of an Allure HTTP Exchange payload.
// The body argument must be the bytes already captured by the caller; nil means
// the body was not captured.
func FromRequest(req *http.Request, body []byte, opts ...Option) Request {
	options := applyOptions(opts)
	if req == nil {
		return Request{}
	}

	captured := captureFromBytes(body, req.ContentLength, options)
	return fromRequestCapture(req, captured, options)
}

// FromResponse builds the response portion of an Allure HTTP Exchange payload.
// The body argument must be the bytes already captured by the caller; nil means
// the body was not captured.
func FromResponse(resp *http.Response, body []byte, opts ...Option) Response {
	options := applyOptions(opts)
	if resp == nil {
		return Response{}
	}

	captured := captureFromBytes(body, resp.ContentLength, options)
	return fromResponseCapture(resp, captured, options)
}

func fromRequestCapture(req *http.Request, body bodyCapture, options options) Request {
	if req == nil {
		return Request{}
	}

	return Request{
		Method:      req.Method,
		URL:         redactedURL(requestURL(req), options),
		HTTPVersion: req.Proto,
		Cookies:     cookiesFromRequest(req, options),
		Headers:     nameValuesFromHeader(req.Header, options.redactedHeaders),
		Query:       queryFromURL(req.URL, options),
		Body:        bodyFromCapture(req.Header.Get("Content-Type"), body, options),
		Trailers:    nameValuesFromHeader(req.Trailer, options.redactedHeaders),
	}
}

func fromResponseCapture(resp *http.Response, body bodyCapture, options options) Response {
	if resp == nil {
		return Response{}
	}

	status := resp.StatusCode
	return Response{
		Status:      status,
		StatusText:  statusText(status),
		HTTPVersion: resp.Proto,
		Cookies:     cookiesFromResponse(resp, options),
		Headers:     nameValuesFromHeader(resp.Header, options.redactedHeaders),
		Body:        bodyFromCapture(resp.Header.Get("Content-Type"), body, options),
		Trailers:    nameValuesFromHeader(resp.Trailer, options.redactedHeaders),
	}
}

func captureFromBytes(content []byte, knownSize int64, options options) bodyCapture {
	if content == nil {
		return bodyCapture{}
	}

	size := int64(len(content))
	truncated := false
	if knownSize >= 0 {
		size = knownSize
		if knownSize < int64(len(content)) {
			size = int64(len(content))
		}
		truncated = knownSize > int64(len(content))
	}

	if options.bodyLimit >= 0 && int64(len(content)) > options.bodyLimit {
		content = content[:options.bodyLimit]
		truncated = true
	}

	return bodyCapture{
		present:   true,
		content:   append([]byte(nil), content...),
		size:      size,
		truncated: truncated,
	}
}

func bodyFromCapture(contentType string, captured bodyCapture, options options) *Body {
	if !captured.present {
		return nil
	}

	content := append([]byte(nil), captured.content...)
	size := captured.size
	if size == 0 && len(content) > 0 {
		size = int64(len(content))
	}

	body := &Body{
		ContentType: contentType,
		Size:        size,
		Truncated:   captured.truncated,
	}

	mediaType := mediaType(contentType)
	if mediaType == "application/x-www-form-urlencoded" && utf8.Valid(content) {
		form, redactedValue, ok := formFromBody(content, options)
		if ok {
			body.Form = form
			content = []byte(redactedValue)
		}
	}

	if len(content) == 0 {
		body.Encoding = "utf8"
		body.Value = ""
		return body
	}

	if isTextBody(contentType, content) {
		body.Encoding = "utf8"
		body.Value = string(content)
		return body
	}

	body.Encoding = "base64"
	body.Value = base64.StdEncoding.EncodeToString(content)
	return body
}

func requestURL(req *http.Request) *url.URL {
	if req == nil || req.URL == nil {
		return nil
	}

	copied := *req.URL
	if copied.IsAbs() {
		return &copied
	}

	if req.Host != "" {
		if copied.Scheme == "" {
			if req.TLS != nil {
				copied.Scheme = "https"
			} else {
				copied.Scheme = "http"
			}
		}
		if copied.Host == "" {
			copied.Host = req.Host
		}
	}

	return &copied
}

func redactedURL(value *url.URL, options options) string {
	if value == nil {
		return ""
	}

	copied := *value
	query := copied.Query()
	if redactValues(query, options.redactedQueries) {
		copied.RawQuery = query.Encode()
	}
	return copied.String()
}

func queryFromURL(value *url.URL, options options) []NameValue {
	if value == nil {
		return nil
	}

	query := value.Query()
	if len(query) == 0 {
		return nil
	}
	redactValues(query, options.redactedQueries)
	return nameValuesFromValues(query)
}

func nameValuesFromHeader(header http.Header, redactions map[string]struct{}) []NameValue {
	if len(header) == 0 {
		return nil
	}

	keys := make([]string, 0, len(header))
	for key := range header {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return strings.ToLower(keys[i]) < strings.ToLower(keys[j])
	})

	values := make([]NameValue, 0, len(header))
	for _, key := range keys {
		headerValues := append([]string(nil), header[key]...)
		sort.Strings(headerValues)
		if len(headerValues) == 0 {
			values = append(values, NameValue{Name: key, Value: redactedValue(key, "", redactions)})
			continue
		}
		for _, value := range headerValues {
			values = append(values, NameValue{Name: key, Value: redactedValue(key, value, redactions)})
		}
	}
	return values
}

func nameValuesFromValues(values url.Values) []NameValue {
	if len(values) == 0 {
		return nil
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	pairs := make([]NameValue, 0, len(values))
	for _, key := range keys {
		keyValues := append([]string(nil), values[key]...)
		sort.Strings(keyValues)
		if len(keyValues) == 0 {
			pairs = append(pairs, NameValue{Name: key, Value: ""})
			continue
		}
		for _, value := range keyValues {
			pairs = append(pairs, NameValue{Name: key, Value: value})
		}
	}
	return pairs
}

func cookiesFromRequest(req *http.Request, options options) []Cookie {
	if req == nil {
		return nil
	}

	return cookiesFromHTTPCookies(req.Cookies(), options)
}

func cookiesFromResponse(resp *http.Response, options options) []Cookie {
	if resp == nil {
		return nil
	}

	return cookiesFromHTTPCookies(resp.Cookies(), options)
}

func cookiesFromHTTPCookies(cookies []*http.Cookie, options options) []Cookie {
	if len(cookies) == 0 {
		return nil
	}

	sort.Slice(cookies, func(i, j int) bool {
		return cookies[i].Name < cookies[j].Name
	})

	result := make([]Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie == nil {
			continue
		}
		result = append(result, cookieFromHTTP(cookie, options))
	}
	return result
}

func cookieFromHTTP(cookie *http.Cookie, options options) Cookie {
	result := Cookie{
		Name:     cookie.Name,
		Value:    redactedValue(cookie.Name, cookie.Value, options.redactedCookies),
		Domain:   cookie.Domain,
		HTTPOnly: cookie.HttpOnly,
		MaxAge:   cookie.MaxAge,
		Path:     cookie.Path,
		SameSite: sameSite(cookie.SameSite),
		Secure:   cookie.Secure,
	}
	if !cookie.Expires.IsZero() {
		result.Expires = cookie.Expires.UTC().Format(time.RFC3339)
	}
	return result
}

func formFromBody(content []byte, options options) ([]NameValue, string, bool) {
	values, err := url.ParseQuery(string(content))
	if err != nil {
		return nil, "", false
	}

	redacted := redactValues(values, options.redactedFormFields)
	if redacted {
		return nameValuesFromValues(values), values.Encode(), true
	}
	return nameValuesFromValues(values), string(content), true
}

func redactValues(values url.Values, redactions map[string]struct{}) bool {
	redacted := false
	for key, keyValues := range values {
		if !isRedacted(key, redactions) {
			continue
		}
		for index := range keyValues {
			keyValues[index] = RedactedValue
		}
		values[key] = keyValues
		redacted = true
	}
	return redacted
}

func redactedValue(name string, value string, redactions map[string]struct{}) string {
	if isRedacted(name, redactions) {
		return RedactedValue
	}
	return value
}

func isRedacted(name string, redactions map[string]struct{}) bool {
	if _, ok := redactions["*"]; ok {
		return true
	}
	_, ok := redactions[normalizeName(name)]
	return ok
}

func addRedactions(redactions map[string]struct{}, names ...string) {
	for _, name := range names {
		normalized := normalizeName(name)
		if normalized != "" {
			redactions[normalized] = struct{}{}
		}
	}
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func mediaType(contentType string) string {
	contentType = strings.TrimSpace(strings.ToLower(contentType))
	if contentType == "" {
		return ""
	}
	parsed, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		parsed, _, _ = strings.Cut(contentType, ";")
	}
	return strings.TrimSpace(strings.ToLower(parsed))
}

func isTextBody(contentType string, content []byte) bool {
	mediaType := mediaType(contentType)
	if mediaType == "" {
		return utf8.Valid(content)
	}
	if strings.HasPrefix(mediaType, "text/") {
		return utf8.Valid(content)
	}
	if strings.HasSuffix(mediaType, "+json") || strings.HasSuffix(mediaType, "+xml") {
		return utf8.Valid(content)
	}
	switch mediaType {
	case "application/graphql",
		"application/javascript",
		"application/json",
		"application/x-www-form-urlencoded",
		"application/xml":
		return utf8.Valid(content)
	default:
		return false
	}
}

func sameSite(sameSite http.SameSite) string {
	switch sameSite {
	case http.SameSiteDefaultMode:
		return "Default"
	case http.SameSiteLaxMode:
		return "Lax"
	case http.SameSiteStrictMode:
		return "Strict"
	case http.SameSiteNoneMode:
		return "None"
	default:
		return ""
	}
}

func errorFromError(err error) *Error {
	if err == nil {
		return nil
	}

	var netErr interface{ Timeout() bool }
	name := fmt.Sprintf("%T", err)
	if errors.As(err, &netErr) && netErr.Timeout() {
		name = "TimeoutError"
	}
	return &Error{Name: name, Message: err.Error()}
}
