package httpexchange

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	commons "github.com/allure-framework/allure-go/commons"
)

// Marshal returns a UTF-8 JSON HTTP Exchange attachment payload.
func Marshal(exchange Exchange) ([]byte, error) {
	exchange = normalizeExchange(exchange)
	return json.MarshalIndent(exchange, "", "  ")
}

// Attach writes exchange as an Allure HTTP Exchange attachment.
func Attach(ctx context.Context, name string, exchange Exchange) error {
	exchange = normalizeExchange(exchange)
	if strings.TrimSpace(name) == "" {
		name = defaultNamer(exchange)
	}

	payload, err := Marshal(exchange)
	if err != nil {
		return err
	}

	return commons.Attachment(ctx, name, payload, commons.AttachmentOptions{
		ContentType:   AttachmentContentType,
		FileExtension: AttachmentFileExtension,
	})
}

func attachWithOptions(ctx context.Context, exchange Exchange, options options) error {
	name := defaultAttachmentName
	if options.attachmentNamer != nil {
		if generated := strings.TrimSpace(options.attachmentNamer(exchange)); generated != "" {
			name = generated
		}
	}
	return Attach(ctx, name, exchange)
}

func normalizeExchange(exchange Exchange) Exchange {
	if exchange.SchemaVersion == 0 {
		exchange.SchemaVersion = SchemaVersion
	}
	return exchange
}

func defaultNamer(exchange Exchange) string {
	method := strings.TrimSpace(exchange.Request.Method)
	if method == "" {
		return defaultAttachmentName
	}

	target := requestTarget(exchange.Request.URL)
	if target == "" {
		return "HTTP " + method
	}
	return "HTTP " + method + " " + target
}

func requestTarget(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	target := parsed.EscapedPath()
	if target == "" {
		target = "/"
	}
	if parsed.RawQuery != "" {
		target += "?" + parsed.RawQuery
	}
	return target
}
