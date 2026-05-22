package writer

import (
	"context"
	"os"
	"sync"

	"github.com/allure-framework/allure-go/commons/model"
)

// InMemoryWriter stores Allure artifacts in memory for tests and adapters that
// need to inspect generated output.
type InMemoryWriter struct {
	mu          sync.Mutex
	results     []model.TestResult
	containers  []model.TestResultContainer
	attachments map[string][]byte
	environment map[string]string
	categories  []model.Category
	executor    model.Executor
	globals     []model.Globals
}

// MemorySnapshot is a copy of the artifacts recorded by an InMemoryWriter.
type MemorySnapshot struct {
	Results     []model.TestResult
	Containers  []model.TestResultContainer
	Attachments map[string][]byte
	Environment map[string]string
	Categories  []model.Category
	Executor    model.Executor
	Globals     []model.Globals
}

// NewInMemoryWriter returns an empty in-memory writer.
func NewInMemoryWriter() *InMemoryWriter {
	return &InMemoryWriter{
		attachments: map[string][]byte{},
		environment: map[string]string{},
	}
}

// WriteResult records a test result.
func (w *InMemoryWriter) WriteResult(_ context.Context, result model.TestResult) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.results = append(w.results, result)
	return nil
}

// WriteContainer records a test result container.
func (w *InMemoryWriter) WriteContainer(_ context.Context, container model.TestResultContainer) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.containers = append(w.containers, container)
	return nil
}

// WriteAttachment records an attachment payload under source.
func (w *InMemoryWriter) WriteAttachment(_ context.Context, source string, content []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.attachments[source] = append([]byte(nil), content...)
	return nil
}

// CopyAttachment reads path and records its content under source.
func (w *InMemoryWriter) CopyAttachment(ctx context.Context, source string, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return w.WriteAttachment(ctx, source, content)
}

// WriteEnvironmentInfo records environment properties.
func (w *InMemoryWriter) WriteEnvironmentInfo(_ context.Context, environment map[string]string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.environment = copyStringMap(environment)
	return nil
}

// WriteCategories records categories.
func (w *InMemoryWriter) WriteCategories(_ context.Context, categories []model.Category) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.categories = append([]model.Category(nil), categories...)
	return nil
}

// WriteExecutorInfo records executor information.
func (w *InMemoryWriter) WriteExecutorInfo(_ context.Context, executor model.Executor) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.executor = executor
	return nil
}

// WriteGlobals records run-level globals.
func (w *InMemoryWriter) WriteGlobals(_ context.Context, globals model.Globals) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.globals = append(w.globals, globals)
	return nil
}

// Snapshot returns a copy of all artifacts recorded so far.
func (w *InMemoryWriter) Snapshot() MemorySnapshot {
	w.mu.Lock()
	defer w.mu.Unlock()

	attachments := make(map[string][]byte, len(w.attachments))
	for source, content := range w.attachments {
		attachments[source] = append([]byte(nil), content...)
	}

	return MemorySnapshot{
		Results:     append([]model.TestResult(nil), w.results...),
		Containers:  append([]model.TestResultContainer(nil), w.containers...),
		Attachments: attachments,
		Environment: copyStringMap(w.environment),
		Categories:  append([]model.Category(nil), w.categories...),
		Executor:    w.executor,
		Globals:     append([]model.Globals(nil), w.globals...),
	}
}

func copyStringMap(values map[string]string) map[string]string {
	copied := make(map[string]string, len(values))
	for key, value := range values {
		copied[key] = value
	}

	return copied
}
