package writer_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	allure "github.com/allure-framework/allure-go/commons/gotest"
	"github.com/allure-framework/allure-go/commons/model"
	"github.com/allure-framework/allure-go/commons/writer"
)

var errWriterStopped = errors.New("writer stopped")

func TestFileSystemWriterWritesArtifacts(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Verifies that FileSystemWriter writes every supported Allure artifact type to disk. " +
			"The expected result is deterministic files for test results, containers, attachments, environment info, categories, executor info, and global errors with the expected content.")

		dir := a.T().TempDir()
		w := writer.NewFileSystemWriter(dir)
		ctx := context.Background()
		expectedFiles := []string{
			"test-1-result.json",
			"container-1-container.json",
			"payload.txt",
			"environment.properties",
			"categories.json",
			"executor.json",
			"global-1-globals.json",
		}

		a.Step("write every supported filesystem artifact", func(a *allure.Context) {
			a.Attachment("expected artifact files", []byte(strings.Join(expectedFiles, "\n")), "text/plain")

			if err := w.WriteResult(ctx, model.TestResult{UUID: "test-1", Name: "works", Status: model.StatusPassed}); err != nil {
				a.T().Fatalf("write result: %v", err)
			}
			if err := w.WriteContainer(ctx, model.TestResultContainer{UUID: "container-1", Children: []string{"test-1"}}); err != nil {
				a.T().Fatalf("write container: %v", err)
			}
			if err := w.WriteAttachment(ctx, "payload.txt", []byte("hello")); err != nil {
				a.T().Fatalf("write attachment: %v", err)
			}
			if err := w.WriteEnvironmentInfo(ctx, map[string]string{"b": "2", "a": "1"}); err != nil {
				a.T().Fatalf("write environment: %v", err)
			}
			if err := w.WriteCategories(ctx, []model.Category{{Name: "Product defects", MatchedStatuses: []model.Status{model.StatusFailed}}}); err != nil {
				a.T().Fatalf("write categories: %v", err)
			}
			if err := w.WriteExecutorInfo(ctx, model.Executor{Name: "GitHub Actions", Type: "github", BuildName: "build-1"}); err != nil {
				a.T().Fatalf("write executor: %v", err)
			}
			if err := w.WriteGlobals(ctx, model.Globals{UUID: "global-1", Errors: []model.GlobalError{{StatusDetails: model.StatusDetails{Message: "setup failed"}, Timestamp: 1000}}}); err != nil {
				a.T().Fatalf("write globals: %v", err)
			}
		})

		a.Step("verify persisted artifact contents", func(a *allure.Context) {
			var evidence strings.Builder
			for _, name := range expectedFiles {
				content, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil {
					a.T().Fatalf("read %s: %v", name, err)
				}
				fmt.Fprintf(&evidence, "== %s ==\n%s\n", name, content)
			}
			a.Attachment("persisted artifact contents", []byte(evidence.String()), "text/plain")

			assertFileContains(a.T(), filepath.Join(dir, "test-1-result.json"), `"uuid": "test-1"`)
			assertFileContains(a.T(), filepath.Join(dir, "container-1-container.json"), `"children": [`)
			assertFileContains(a.T(), filepath.Join(dir, "payload.txt"), `hello`)
			assertFileContains(a.T(), filepath.Join(dir, "environment.properties"), "a=1\nb=2\n")
			assertFileContains(a.T(), filepath.Join(dir, "categories.json"), `"Product defects"`)
			assertFileContains(a.T(), filepath.Join(dir, "executor.json"), `"GitHub Actions"`)
			assertFileContains(a.T(), filepath.Join(dir, "global-1-globals.json"), `"setup failed"`)
		})
	})
}

func TestFileSystemWriterUsesTargetTmpPath(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Verifies that an attachment is staged at <target>.tmp in the results directory, so consumers ignore it until publication. A blocked staging path must preserve the published attachment, and a successful retry must leave only the completed target.")

		dir := a.T().TempDir()
		w := writer.NewFileSystemWriter(dir)
		target := filepath.Join(dir, "payload.txt")
		temporary := target + ".tmp"

		a.Step("publish an attachment and block its staging path", func(a *allure.Context) {
			if err := w.WriteAttachment(context.Background(), "payload.txt", []byte("published")); err != nil {
				a.T().Fatalf("write initial attachment: %v", err)
			}
			if err := os.Mkdir(temporary, 0o755); err != nil {
				a.T().Fatalf("block staging path: %v", err)
			}
		})

		a.Step("verify a blocked target.tmp preserves the published attachment", func(a *allure.Context) {
			err := w.WriteAttachment(context.Background(), "payload.txt", []byte("replacement"))
			if err == nil {
				a.T().Fatal("expected write to fail while payload.txt.tmp is blocked")
			}
			content, readErr := os.ReadFile(target)
			if readErr != nil {
				a.T().Fatalf("read published attachment: %v", readErr)
			}
			a.Attachment("blocked staging attempt", []byte(fmt.Sprintf("staging path: %s\nerror: %v\npublished content: %q", temporary, err, content)), "text/plain")
			if string(content) != "published" {
				a.T().Fatalf("published content changed after staging failure: %q", content)
			}
			info, err := os.Stat(temporary)
			if err != nil || !info.IsDir() {
				a.T().Fatalf("staging blocker was changed: info=%v, error=%v", info, err)
			}
		})

		a.Step("unblock staging and verify publication removes the temporary file", func(a *allure.Context) {
			if err := os.Remove(temporary); err != nil {
				a.T().Fatalf("remove staging blocker: %v", err)
			}
			if err := w.WriteAttachment(context.Background(), "payload.txt", []byte("replacement")); err != nil {
				a.T().Fatalf("retry attachment write: %v", err)
			}
			content, err := os.ReadFile(target)
			if err != nil {
				a.T().Fatalf("read replacement attachment: %v", err)
			}
			a.Attachment("published payload.txt", content, "text/plain")
			if string(content) != "replacement" {
				a.T().Fatalf("unexpected replacement content: %q", content)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				a.T().Fatalf("read results directory: %v", err)
			}
			if len(entries) != 1 || entries[0].Name() != "payload.txt" {
				a.T().Fatalf("expected only payload.txt after publication, got %v", entries)
			}
		})
	})
}

func TestFileSystemWriterPreservesExistingStagingFile(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Verifies that a write cannot truncate or publish another write's unfinished <target>.tmp file when the exact staging path is already occupied.")

		dir := a.T().TempDir()
		w := writer.NewFileSystemWriter(dir)
		temporary := filepath.Join(dir, "payload.txt.tmp")

		a.Step("prepare an unfinished attachment in the staging file", func(a *allure.Context) {
			if err := os.WriteFile(temporary, []byte("unfinished attachment"), 0o600); err != nil {
				a.T().Fatalf("write staging fixture: %v", err)
			}
		})

		a.Step("verify a competing write leaves the staging file untouched", func(a *allure.Context) {
			err := w.WriteAttachment(context.Background(), "payload.txt", []byte("competing attachment"))
			if !errors.Is(err, os.ErrExist) {
				a.T().Fatalf("expected an occupied staging path error, got %v", err)
			}
			content, err := os.ReadFile(temporary)
			if err != nil {
				a.T().Fatalf("read unfinished staging file: %v", err)
			}
			a.Attachment("preserved payload.txt.tmp", content, "text/plain")
			if string(content) != "unfinished attachment" {
				a.T().Fatalf("unfinished staging content changed: %q", content)
			}
			if _, err := os.Stat(filepath.Join(dir, "payload.txt")); !errors.Is(err, os.ErrNotExist) {
				a.T().Fatalf("target must not be published by the competing write: %v", err)
			}
		})
	})
}

func TestFileSystemWriterSyncErrors(t *testing.T) {
	cases := []struct {
		name    string
		syncErr error
		publish bool
	}{
		{name: "successful_sync", publish: true},
		{name: "unsupported_sync", syncErr: errors.ErrUnsupported, publish: true},
		{name: "wrapped_unsupported_sync", syncErr: fmt.Errorf("filesystem: %w", &os.PathError{Op: "sync", Path: "payload.txt.tmp", Err: errors.ErrUnsupported}), publish: true},
		{name: "unsupported_file_sync", syncErr: &os.PathError{Op: "sync", Path: "payload.txt.tmp", Err: syscall.EINVAL}, publish: true},
		{name: "io_failure", syncErr: &os.PathError{Op: "sync", Path: "payload.txt.tmp", Err: syscall.EIO}},
		{name: "permission_failure", syncErr: os.ErrPermission},
		{name: "closed_file", syncErr: os.ErrClosed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			allure.Wrap(t, func(a *allure.Context) {
				a.Description("Verifies that synchronization is attempted on the complete, open temporary file before publication. Unsupported sync permits publication; other sync errors preserve the previous target. Both outcomes remove the temporary file.")
				a.Parameter("sync error", fmt.Sprint(tc.syncErr))
				a.Parameter("publish replacement", fmt.Sprint(tc.publish))

				dir := a.T().TempDir()
				target := filepath.Join(dir, "payload.txt")
				w := writer.NewFileSystemWriter(dir)

				a.Step("prepare an existing published attachment", func(a *allure.Context) {
					if err := os.WriteFile(target, []byte("published"), 0o600); err != nil {
						a.T().Fatalf("prepare target: %v", err)
					}
				})

				var writeErr error
				syncCalls := 0
				var stagedPath string
				var stagedContent, previousContent []byte
				var statErr, stagedReadErr, previousReadErr error
				a.Step("write the replacement and simulate the filesystem sync result", func(a *allure.Context) {
					writeErr = writer.WriteFileWithSync(context.Background(), w, "payload.txt", strings.NewReader("replacement"), func(file *os.File) error {
						syncCalls++
						stagedPath = file.Name()
						_, statErr = file.Stat()
						stagedContent, stagedReadErr = os.ReadFile(stagedPath)
						previousContent, previousReadErr = os.ReadFile(target)
						return tc.syncErr
					})
					a.Attachment("sync observation", []byte(fmt.Sprintf("sync calls: %d\nstaging path: %s\nstaged content: %q\ntarget content during sync: %q\nopen file error: %v\nstaged read error: %v\ntarget read error: %v\nwrite error: %v", syncCalls, stagedPath, stagedContent, previousContent, statErr, stagedReadErr, previousReadErr, writeErr)), "text/plain")
				})

				a.Step("verify sync ordering, publication outcome, and staging cleanup", func(a *allure.Context) {
					if syncCalls != 1 || stagedPath != target+".tmp" {
						a.T().Fatalf("expected one sync of %s.tmp, got %d calls at %q", target, syncCalls, stagedPath)
					}
					if statErr != nil || stagedReadErr != nil || previousReadErr != nil {
						a.T().Fatalf("sync must happen on an open staged file before replacing the target: stat=%v, staged read=%v, target read=%v", statErr, stagedReadErr, previousReadErr)
					}
					if string(stagedContent) != "replacement" || string(previousContent) != "published" {
						a.T().Fatalf("unexpected content during sync: staged=%q, target=%q", stagedContent, previousContent)
					}
					expectedContent := "published"
					if tc.publish {
						expectedContent = "replacement"
						if writeErr != nil {
							a.T().Fatalf("expected publication after successful or unsupported sync, got %v", writeErr)
						}
					} else if !errors.Is(writeErr, tc.syncErr) {
						a.T().Fatalf("expected sync error %v, got %v", tc.syncErr, writeErr)
					}
					content, err := os.ReadFile(target)
					if err != nil {
						a.T().Fatalf("read final target: %v", err)
					}
					a.Attachment("final payload.txt", content, "text/plain")
					if string(content) != expectedContent {
						a.T().Fatalf("expected target content %q, got %q", expectedContent, content)
					}
					if _, err := os.Stat(target + ".tmp"); !errors.Is(err, os.ErrNotExist) {
						a.T().Fatalf("temporary file must be removed after the write: %v", err)
					}
				})
			})
		})
	}
}

func TestInMemoryWriterSnapshotsArtifacts(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Verifies that InMemoryWriter records every supported Allure artifact type without touching the filesystem. " +
			"The expected result is a snapshot containing durable copies of results, containers, attachments, environment info, categories, executor info, and global artifacts.")

		w := writer.NewInMemoryWriter()
		environment := map[string]string{"os": "linux"}
		categories := []model.Category{{Name: "Product defects", MatchedStatuses: []model.Status{model.StatusFailed}}}
		attachment := []byte("hello")

		a.Step("write every artifact type to memory", func(a *allure.Context) {
			a.Attachment("memory write operations", []byte(strings.Join([]string{
				"WriteResult uuid=test-1 name=works",
				"WriteContainer uuid=container-1 child=test-1",
				"WriteAttachment source=payload.txt content=hello",
				"WriteEnvironmentInfo os=linux",
				"WriteCategories Product defects",
				"WriteExecutorInfo GitHub Actions",
				"WriteGlobals global-1",
			}, "\n")), "text/plain")

			if err := w.WriteResult(context.Background(), model.TestResult{UUID: "test-1", Name: "works"}); err != nil {
				a.T().Fatalf("write result: %v", err)
			}
			if err := w.WriteContainer(context.Background(), model.TestResultContainer{UUID: "container-1", Children: []string{"test-1"}}); err != nil {
				a.T().Fatalf("write container: %v", err)
			}
			if err := w.WriteAttachment(context.Background(), "payload.txt", attachment); err != nil {
				a.T().Fatalf("write attachment: %v", err)
			}
			if err := w.WriteEnvironmentInfo(context.Background(), environment); err != nil {
				a.T().Fatalf("write environment: %v", err)
			}
			if err := w.WriteCategories(context.Background(), categories); err != nil {
				a.T().Fatalf("write categories: %v", err)
			}
			if err := w.WriteExecutorInfo(context.Background(), model.Executor{Name: "GitHub Actions", Type: "github"}); err != nil {
				a.T().Fatalf("write executor: %v", err)
			}
			if err := w.WriteGlobals(context.Background(), model.Globals{UUID: "global-1", Errors: []model.GlobalError{{StatusDetails: model.StatusDetails{Message: "setup failed"}, Timestamp: 1000}}}); err != nil {
				a.T().Fatalf("write globals: %v", err)
			}
			attachment[0] = 'H'
			environment["os"] = "mutated"
			categories[0].Name = "Mutated"
		})

		a.Step("snapshot and verify in-memory artifacts", func(a *allure.Context) {
			snapshot := w.Snapshot()
			payload := string(snapshot.Attachments["payload.txt"])
			a.Attachment("memory snapshot", []byte(fmt.Sprintf("results: %d\ncontainers: %d\nattachments: %d\nenvironment: %v\ncategories: %d\nexecutor: %s\nglobals: %d\npayload.txt: %q",
				len(snapshot.Results), len(snapshot.Containers), len(snapshot.Attachments), snapshot.Environment, len(snapshot.Categories), snapshot.Executor.Name, len(snapshot.Globals), payload)), "text/plain")

			if len(snapshot.Results) != 1 {
				a.T().Fatalf("expected one result, got %d", len(snapshot.Results))
			}
			if len(snapshot.Containers) != 1 {
				a.T().Fatalf("expected one container, got %d", len(snapshot.Containers))
			}
			if payload != "hello" {
				a.T().Fatalf("unexpected attachment payload: %q", snapshot.Attachments["payload.txt"])
			}
			if snapshot.Environment["os"] != "linux" {
				a.T().Fatalf("environment was not copied: %#v", snapshot.Environment)
			}
			if len(snapshot.Categories) != 1 || snapshot.Categories[0].Name != "Product defects" {
				a.T().Fatalf("categories were not copied: %#v", snapshot.Categories)
			}
			if snapshot.Executor.Name != "GitHub Actions" {
				a.T().Fatalf("executor was not stored: %#v", snapshot.Executor)
			}
			if len(snapshot.Globals) != 1 || len(snapshot.Globals[0].Errors) != 1 {
				a.T().Fatalf("expected one global error, got %#v", snapshot.Globals)
			}
		})
	})
}

func TestMultiWriterFansOutAndStopsOnError(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Verifies that MultiWriter implements the full Writer interface by forwarding every public write method to each configured writer and stopping at the first error. " +
			"The expected result is that successful calls reach both writers in order, nil writers are ignored, and a failing writer prevents later writers from receiving the failed call.")

		ctx := context.Background()
		first := &recordingWriter{name: "first"}
		second := &recordingWriter{name: "second"}
		multi := writer.NewMultiWriter(first, nil, second)

		a.Step("forward every writer method to all writers", func(a *allure.Context) {
			path := filepath.Join(a.T().TempDir(), "payload.txt")
			if err := os.WriteFile(path, []byte("from path"), 0o644); err != nil {
				a.T().Fatalf("write attachment source: %v", err)
			}

			calls := []error{
				multi.WriteResult(ctx, model.TestResult{UUID: "test-1", Name: "works"}),
				multi.WriteContainer(ctx, model.TestResultContainer{UUID: "container-1"}),
				multi.WriteAttachment(ctx, "payload.txt", []byte("hello")),
				multi.CopyAttachment(ctx, "path.txt", path),
				multi.WriteEnvironmentInfo(ctx, map[string]string{"os": "linux"}),
				multi.WriteCategories(ctx, []model.Category{{Name: "Product defects"}}),
				multi.WriteExecutorInfo(ctx, model.Executor{Name: "CI"}),
				multi.WriteGlobals(ctx, model.Globals{UUID: "global-1"}),
			}
			for _, err := range calls {
				if err != nil {
					a.T().Fatalf("multi writer call failed: %v", err)
				}
			}
			a.Attachment("first writer calls", []byte(strings.Join(first.calls, "\n")), "text/plain")
			a.Attachment("second writer calls", []byte(strings.Join(second.calls, "\n")), "text/plain")
			if strings.Join(first.calls, "\n") != strings.Join(second.calls, "\n") {
				a.T().Fatalf("writers saw different calls\nfirst: %v\nsecond: %v", first.calls, second.calls)
			}
			if len(first.calls) != 8 {
				a.T().Fatalf("expected eight forwarded calls, got %d: %v", len(first.calls), first.calls)
			}
		})

		a.Step("stop forwarding after first writer error", func(a *allure.Context) {
			failing := &recordingWriter{name: "failing", err: errWriterStopped}
			afterFailure := &recordingWriter{name: "after"}
			multi := writer.NewMultiWriter(failing, afterFailure)
			err := multi.WriteResult(ctx, model.TestResult{UUID: "test-2"})
			a.Attachment("error fanout calls", []byte(fmt.Sprintf("error: %v\nfailing calls: %v\nafter calls: %v", err, failing.calls, afterFailure.calls)), "text/plain")
			if !errors.Is(err, errWriterStopped) {
				a.T().Fatalf("expected writer error, got %v", err)
			}
			if len(afterFailure.calls) != 0 {
				a.T().Fatalf("writer after failure should not be called: %v", afterFailure.calls)
			}
		})
	})
}

type recordingWriter struct {
	name  string
	err   error
	calls []string
}

func (w *recordingWriter) record(call string) error {
	w.calls = append(w.calls, call)
	return w.err
}

func (w *recordingWriter) WriteResult(context.Context, model.TestResult) error {
	return w.record("WriteResult")
}

func (w *recordingWriter) WriteContainer(context.Context, model.TestResultContainer) error {
	return w.record("WriteContainer")
}

func (w *recordingWriter) WriteAttachment(context.Context, string, []byte) error {
	return w.record("WriteAttachment")
}

func (w *recordingWriter) CopyAttachment(context.Context, string, string) error {
	return w.record("CopyAttachment")
}

func (w *recordingWriter) WriteEnvironmentInfo(context.Context, map[string]string) error {
	return w.record("WriteEnvironmentInfo")
}

func (w *recordingWriter) WriteCategories(context.Context, []model.Category) error {
	return w.record("WriteCategories")
}

func (w *recordingWriter) WriteExecutorInfo(context.Context, model.Executor) error {
	return w.record("WriteExecutorInfo")
}

func (w *recordingWriter) WriteGlobals(context.Context, model.Globals) error {
	return w.record("WriteGlobals")
}

func assertFileContains(t *testing.T, path string, needle string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(content), needle) {
		t.Fatalf("%s does not contain %q\n%s", path, needle, content)
	}
}
