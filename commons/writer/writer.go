// Package writer provides output backends for Allure result artifacts.
package writer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/allure-framework/allure-go/commons/model"
)

// Writer persists Allure result artifacts.
type Writer interface {
	// WriteResult writes a *-result.json test result artifact.
	WriteResult(context.Context, model.TestResult) error
	// WriteContainer writes a *-container.json container artifact.
	WriteContainer(context.Context, model.TestResultContainer) error
	// WriteAttachment writes an attachment artifact from memory.
	WriteAttachment(context.Context, string, []byte) error
	// CopyAttachment writes an attachment artifact by copying an existing file.
	CopyAttachment(context.Context, string, string) error
	// WriteEnvironmentInfo writes environment.properties.
	WriteEnvironmentInfo(context.Context, map[string]string) error
	// WriteCategories writes categories.json.
	WriteCategories(context.Context, []model.Category) error
	// WriteExecutorInfo writes executor.json.
	WriteExecutorInfo(context.Context, model.Executor) error
	// WriteGlobals writes a *-globals.json artifact.
	WriteGlobals(context.Context, model.Globals) error
}

// FileSystemWriter writes Allure artifacts to a directory on disk.
type FileSystemWriter struct {
	Dir string
}

// NewFileSystemWriter returns a filesystem writer for dir.
func NewFileSystemWriter(dir string) *FileSystemWriter {
	if dir == "" {
		dir = "allure-results"
	}

	return &FileSystemWriter{Dir: dir}
}

// WriteResult writes result as <uuid>-result.json.
func (w *FileSystemWriter) WriteResult(ctx context.Context, result model.TestResult) error {
	if result.UUID == "" {
		return errors.New("write result: uuid is required")
	}

	return w.writeJSON(ctx, result.UUID+"-result.json", result)
}

// WriteContainer writes container as <uuid>-container.json.
func (w *FileSystemWriter) WriteContainer(ctx context.Context, container model.TestResultContainer) error {
	if container.UUID == "" {
		return errors.New("write container: uuid is required")
	}

	return w.writeJSON(ctx, container.UUID+"-container.json", container)
}

// WriteAttachment writes content to source in the results directory.
func (w *FileSystemWriter) WriteAttachment(ctx context.Context, source string, content []byte) error {
	if source == "" {
		return errors.New("write attachment: source is required")
	}

	return w.writeFile(ctx, source, bytes.NewReader(content))
}

// CopyAttachment copies path to source in the results directory.
func (w *FileSystemWriter) CopyAttachment(ctx context.Context, source string, path string) error {
	if source == "" {
		return errors.New("copy attachment: source is required")
	}
	if path == "" {
		return errors.New("copy attachment: path is required")
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("copy attachment: open source: %w", err)
	}
	defer file.Close()

	return w.writeFile(ctx, source, file)
}

// WriteEnvironmentInfo writes sorted environment properties.
func (w *FileSystemWriter) WriteEnvironmentInfo(ctx context.Context, environment map[string]string) error {
	return w.writeBytes(ctx, "environment.properties", encodeEnvironment(environment))
}

// WriteCategories writes categories.json.
func (w *FileSystemWriter) WriteCategories(ctx context.Context, categories []model.Category) error {
	return w.writeJSON(ctx, "categories.json", categories)
}

// WriteExecutorInfo writes executor.json.
func (w *FileSystemWriter) WriteExecutorInfo(ctx context.Context, executor model.Executor) error {
	return w.writeJSON(ctx, "executor.json", executor)
}

// WriteGlobals writes globals as <uuid>-globals.json.
func (w *FileSystemWriter) WriteGlobals(ctx context.Context, globals model.Globals) error {
	if globals.UUID == "" {
		return errors.New("write globals: uuid is required")
	}

	return w.writeJSON(ctx, globals.UUID+"-globals.json", globals)
}

func (w *FileSystemWriter) writeJSON(ctx context.Context, name string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", name, err)
	}
	payload = append(payload, '\n')

	return w.writeBytes(ctx, name, payload)
}

func (w *FileSystemWriter) writeBytes(ctx context.Context, name string, payload []byte) error {
	return w.writeFile(ctx, name, bytes.NewReader(payload))
}

func (w *FileSystemWriter) writeFile(ctx context.Context, name string, reader io.Reader) error {
	if err := checkContext(ctx); err != nil {
		return err
	}

	if err := os.MkdirAll(w.Dir, 0o755); err != nil {
		return fmt.Errorf("create results dir: %w", err)
	}

	target, err := w.targetPath(name)
	if err != nil {
		return err
	}

	temp, err := os.CreateTemp(w.Dir, "."+filepath.Base(target)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	tempName := temp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempName)
		}
	}()

	if _, err := io.Copy(temp, reader); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := checkContext(ctx); err != nil {
		return err
	}

	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove existing target: %w", err)
	}
	if err := os.Rename(tempName, target); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	removeTemp = false
	return nil
}

func (w *FileSystemWriter) targetPath(name string) (string, error) {
	base := filepath.Base(name)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return "", fmt.Errorf("invalid artifact name: %q", name)
	}

	return filepath.Join(w.Dir, base), nil
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func encodeEnvironment(environment map[string]string) []byte {
	keys := make([]string, 0, len(environment))
	for key := range environment {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(escapeProperty(key))
		builder.WriteByte('=')
		builder.WriteString(escapeProperty(environment[key]))
		builder.WriteByte('\n')
	}

	return []byte(builder.String())
}

func escapeProperty(value string) string {
	var builder strings.Builder
	for _, r := range value {
		switch r {
		case '\\':
			builder.WriteString(`\\`)
		case '\n':
			builder.WriteString(`\n`)
		case '\r':
			builder.WriteString(`\r`)
		case '\t':
			builder.WriteString(`\t`)
		case '=', ':':
			builder.WriteByte('\\')
			builder.WriteRune(r)
		default:
			builder.WriteRune(r)
		}
	}

	return builder.String()
}
