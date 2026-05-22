package writer

import (
	"context"

	"github.com/allure-framework/allure-go/commons/model"
)

// MultiWriter fans out every write call to multiple writers.
type MultiWriter struct {
	writers []Writer
}

// NewMultiWriter returns a writer that forwards calls to all non-nil writers.
func NewMultiWriter(writers ...Writer) *MultiWriter {
	filtered := make([]Writer, 0, len(writers))
	for _, writer := range writers {
		if writer != nil {
			filtered = append(filtered, writer)
		}
	}

	return &MultiWriter{writers: filtered}
}

// WriteResult forwards a test result to each writer.
func (w *MultiWriter) WriteResult(ctx context.Context, result model.TestResult) error {
	for _, writer := range w.writers {
		if err := writer.WriteResult(ctx, result); err != nil {
			return err
		}
	}

	return nil
}

// WriteContainer forwards a container to each writer.
func (w *MultiWriter) WriteContainer(ctx context.Context, container model.TestResultContainer) error {
	for _, writer := range w.writers {
		if err := writer.WriteContainer(ctx, container); err != nil {
			return err
		}
	}

	return nil
}

// WriteAttachment forwards an in-memory attachment to each writer.
func (w *MultiWriter) WriteAttachment(ctx context.Context, source string, content []byte) error {
	for _, writer := range w.writers {
		if err := writer.WriteAttachment(ctx, source, content); err != nil {
			return err
		}
	}

	return nil
}

// CopyAttachment forwards a file attachment copy request to each writer.
func (w *MultiWriter) CopyAttachment(ctx context.Context, source string, path string) error {
	for _, writer := range w.writers {
		if err := writer.CopyAttachment(ctx, source, path); err != nil {
			return err
		}
	}

	return nil
}

// WriteEnvironmentInfo forwards environment properties to each writer.
func (w *MultiWriter) WriteEnvironmentInfo(ctx context.Context, environment map[string]string) error {
	for _, writer := range w.writers {
		if err := writer.WriteEnvironmentInfo(ctx, environment); err != nil {
			return err
		}
	}

	return nil
}

// WriteCategories forwards categories to each writer.
func (w *MultiWriter) WriteCategories(ctx context.Context, categories []model.Category) error {
	for _, writer := range w.writers {
		if err := writer.WriteCategories(ctx, categories); err != nil {
			return err
		}
	}

	return nil
}

// WriteExecutorInfo forwards executor information to each writer.
func (w *MultiWriter) WriteExecutorInfo(ctx context.Context, executor model.Executor) error {
	for _, writer := range w.writers {
		if err := writer.WriteExecutorInfo(ctx, executor); err != nil {
			return err
		}
	}

	return nil
}

// WriteGlobals forwards run-level globals to each writer.
func (w *MultiWriter) WriteGlobals(ctx context.Context, globals model.Globals) error {
	for _, writer := range w.writers {
		if err := writer.WriteGlobals(ctx, globals); err != nil {
			return err
		}
	}

	return nil
}
