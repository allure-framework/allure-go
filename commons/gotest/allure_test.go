package allure

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/allure-framework/allure-go/commons/model"
	allureruntime "github.com/allure-framework/allure-go/commons/runtime"
	"github.com/allure-framework/allure-go/commons/testplan"
	commonswriter "github.com/allure-framework/allure-go/commons/writer"
)

func TestHelperWritesResultAndStep(t *testing.T) {
	t.Setenv("ALLURE_LABEL_MODULE", "commons")

	Test(t, "gotest helper writes result and step", func(a *Context) {
		a.Description("Runs the gotest helper with an isolated in-memory writer and deterministic clocks and ids, then verifies the generated child Allure result. " +
			"The expected result is one passed child result with the requested display metadata, owner/module labels, link and parameter metadata, step descriptions, step attachments, global attachments, and isolated global error forwarding.")

		memory := commonswriter.NewInMemoryWriter()
		ids := fixedIDs("test-1", "metadata-1", "path-1", "payload-1", "global-content-attachment", "global-content", "global-path-attachment", "global-path")
		pathAttachment := filepath.Join(a.T().TempDir(), "payload.txt")
		childDescription := "Verifies that the gotest helper creates a reported subtest, forwards metadata, records steps, and writes attachments through the configured writer. " +
			"The expected result is a passed Allure result with owner and parameter metadata, markdown and HTML descriptions, and separate evidence steps for metadata and payload attachment recording."
		childDescriptionHTML := "<p>Verifies that the gotest helper creates a reported subtest, forwards metadata, records steps, and writes attachments through the configured writer. " +
			"The expected result is a passed Allure result with owner and parameter metadata, markdown and HTML descriptions, and separate evidence steps for metadata and payload attachment recording.</p>"
		var isolatedMessages []allureruntime.Message

		a.Step("prepare isolated writer and path attachment", func(a *Context) {
			if err := os.WriteFile(pathAttachment, []byte("from path"), 0o644); err != nil {
				a.T().Fatalf("write path attachment: %v", err)
			}
			a.Attachment("deterministic child ids", []byte("test-1\nmetadata-1\npath-1\npayload-1\nglobal-content-attachment\nglobal-content\nglobal-path-attachment\nglobal-path"), "text/plain")
			a.AttachmentPath(pathAttachment, pathAttachment, "text/plain")
		})

		a.Step("run child reported test with isolated writer", func(a *Context) {
			Test(a.T(), "reported child", func(a *Context) {
				a.Step("record metadata and descriptions", func(a *Context) {
					state := allureruntime.Current(a.Context())
					if state.TestID == "" {
						a.T().Fatalf("context did not expose active test id")
					}
					a.StepParameter("phase", "metadata")
					a.StepDisplayName("renamed metadata step")
					a.StepDescription("metadata step markdown")
					a.StepDescriptionHTML("<p>metadata step html</p>")
					a.Label("owner", "qa")
					a.Link("https://example.test/doc", "doc", string(model.LinkTypeLink))
					a.Parameter("case", "happy")
					a.Description(childDescription)
					a.DescriptionHTML(childDescriptionHTML)
					a.DisplayName("display reported test")
					a.TestCaseName("logical reported test")
					a.TestCaseID("case-reported")
					a.HistoryID("history-reported")
					a.Attachment("metadata evidence", []byte("owner=qa\ncase=happy\ndescription=markdown and html"), "text/plain")
					a.AttachmentPath(pathAttachment, pathAttachment, "text/plain")
				})
				a.Step("record payload attachment", func(a *Context) {
					a.Attachment("payload", []byte("hello"), "text/plain")
				})
				a.GlobalAttachment("global content", []byte("global"), "text/plain")
				a.GlobalAttachmentPath("global path", pathAttachment, "text/plain")
				isolated := &Context{
					t: a.T(),
					ctx: allureruntime.WithRuntime(context.Background(), allureruntime.RuntimeFunc(func(_ context.Context, message allureruntime.Message) error {
						isolatedMessages = append(isolatedMessages, message)
						return nil
					})),
				}
				isolated.GlobalError(model.StatusDetails{Message: "setup failed"})
			}, WithWriter(memory), WithClock(fixedClock(1000, 1010, 1020)), WithIDGenerator(ids))
		})

		a.Step("verify generated child result and artifacts", func(a *Context) {
			snapshot := memory.Snapshot()
			a.Attachment("generated child artifacts", []byte(snapshotEvidence(snapshot, isolatedMessages)), "text/plain")
			if len(snapshot.Results) != 1 {
				a.T().Fatalf("expected one result, got %d", len(snapshot.Results))
			}

			result := snapshot.Results[0]
			if result.UUID != "test-1" {
				a.T().Fatalf("unexpected uuid: %q", result.UUID)
			}
			if result.Name != "display reported test" {
				a.T().Fatalf("unexpected display name: %q", result.Name)
			}
			if result.TestCaseName != "logical reported test" || result.TestCaseID != "case-reported" || result.HistoryID != "history-reported" {
				a.T().Fatalf("unexpected explicit ids: %#v", result)
			}
			if result.Status != "passed" {
				a.T().Fatalf("unexpected status: %q", result.Status)
			}
			if len(result.Steps) != 2 {
				a.T().Fatalf("unexpected steps: %#v", result.Steps)
			}
			if result.Steps[0].Name != "renamed metadata step" {
				a.T().Fatalf("unexpected first step: %#v", result.Steps[0])
			}
			if result.Steps[0].Description != "metadata step markdown" || result.Steps[0].DescriptionHTML != "<p>metadata step html</p>" {
				a.T().Fatalf("unexpected first step descriptions: %#v", result.Steps[0])
			}
			if len(result.Steps[0].Parameters) != 1 || result.Steps[0].Parameters[0].Name != "phase" || result.Steps[0].Parameters[0].Value != "metadata" {
				a.T().Fatalf("unexpected first step parameters: %#v", result.Steps[0].Parameters)
			}
			if result.Steps[1].Name != "record payload attachment" {
				a.T().Fatalf("unexpected second step: %#v", result.Steps[1])
			}
			if len(result.Steps[0].Attachments) != 2 || len(result.Steps[1].Attachments) != 1 {
				a.T().Fatalf("expected step attachments, got %#v", result.Steps)
			}
			if len(result.Labels) < 3 {
				a.T().Fatalf("expected framework labels plus owner, got %#v", result.Labels)
			}
			if !hasLabel(result.Labels, "module", "commons") {
				a.T().Fatalf("expected module label from env, got %#v", result.Labels)
			}
			if !hasStaticLink(result.Links, "https://example.test/doc", "doc", string(model.LinkTypeLink)) {
				a.T().Fatalf("expected link, got %#v", result.Links)
			}
			if result.Description != childDescription {
				a.T().Fatalf("unexpected description: %q", result.Description)
			}
			if result.DescriptionHTML != childDescriptionHTML {
				a.T().Fatalf("unexpected html description: %q", result.DescriptionHTML)
			}
			if len(snapshot.Globals) != 2 {
				a.T().Fatalf("expected global attachment and path attachment, got %#v", snapshot.Globals)
			}
			if len(isolatedMessages) != 1 || isolatedMessages[0].Type != allureruntime.MessageGlobalError {
				a.T().Fatalf("expected isolated global error message, got %#v", isolatedMessages)
			}
		})
	})
}

func TestStepReturnsTypedValue(t *testing.T) {
	Test(t, "typed step helper returns body value", func(a *Context) {
		a.Description("Uses the package-level generic Step helper to return a typed value from a reported step. " +
			"The expected result is that the step body can still add step metadata while the caller receives the produced value.")

		a.Step("return value from generic step helper", func(a *Context) {
			value := Step(a, "build client token", func(a *Context) typedStepValue {
				a.StepParameter("kind", "token")
				return typedStepValue{Token: "token-123"}
			})
			a.Attachment("returned typed value", []byte(value.Token), "text/plain")
			if value.Token != "token-123" {
				a.T().Fatalf("unexpected typed step value: %#v", value)
			}
		})

		a.Step("return zero value when body is nil", func(a *Context) {
			value := Step[int](a, "nil typed body", nil)
			if value != 0 {
				a.T().Fatalf("expected zero value from nil body, got %d", value)
			}
		})
	})
}

type typedStepValue struct {
	Token string
}

func TestStaticOptionsWriteInitialMetadata(t *testing.T) {
	Test(t, "static options write initial metadata", func(a *Context) {
		a.Description("Runs a gotest child test with static options and a test plan loaded before the test body, then verifies the metadata written to the generated child result. " +
			"The expected result is that the test plan includes the Allure ID, the body runs, and the result contains static labels, links, parameters, descriptions, display name, logical case name, test case id, and history id.")

		memory := commonswriter.NewInMemoryWriter()
		planPath := filepath.Join(a.T().TempDir(), "testplan.json")
		childDescription := "Runs a gotest child test with static options and a test plan loaded before the test body, then verifies the metadata written to the generated child result. " +
			"The expected result is that the test plan includes the Allure ID, the body runs, and the result contains static labels, links, parameters, descriptions, display name, logical case name, test case id, and history id."
		childDescriptionHTML := "<p>Runs a gotest child test with static options and a test plan loaded before the test body, then verifies the metadata written to the generated child result. " +
			"The expected result is that the test plan includes the Allure ID, the body runs, and the result contains static labels, links, parameters, descriptions, display name, logical case name, test case id, and history id.</p>"

		a.Step("prepare test plan and isolated writer", func(a *Context) {
			if err := os.WriteFile(planPath, []byte(`{"version":"1.0","tests":[{"id":"A-1"}]}`), 0o644); err != nil {
				a.T().Fatalf("write test plan: %v", err)
			}
			a.AttachmentPath(planPath, planPath, "application/json")
		})

		a.Step("run child test with static options and test plan", func(a *Context) {
			Test(a.T(), "static options child", func(a *Context) {
				a.Step("run body after static metadata is applied", func(a *Context) {
					a.Attachment("body evidence", []byte("executed"), "text/plain")
				})
			},
				WithWriter(memory),
				WithClock(fixedClock(2000, 2010, 2020)),
				WithIDGenerator(fixedIDs("static-1", "body-attachment")),
				WithTestPlan(&testplan.Plan{}),
				WithTestPlanPath(planPath),
				WithAllureID("A-1"),
				WithLabels(model.Label{Name: "layer", Value: "unit"}),
				WithOwner("qa"),
				WithTag("smoke"),
				WithSeverity("critical"),
				WithEpic("Accounts"),
				WithFeature("Authentication"),
				WithStory("Login"),
				WithParentSuite("commons"),
				WithSuite("gotest"),
				WithSubSuite("options"),
				WithPackage("github.com/allure-framework/allure-go/commons/gotest"),
				WithLink("https://example.test/doc", "doc", string(model.LinkTypeLink)),
				WithLinks(model.Link{URL: "https://example.test/ref", Name: "ref", Type: string(model.LinkTypeLink)}),
				WithIssue("AUTH-1", "https://example.test/AUTH-1"),
				WithTMS("TMS-1", "https://example.test/TMS-1"),
				WithParameter("browser", "chrome"),
				WithParameterOptions("password", "***", ParameterOptions{Excluded: true, Mode: model.ParameterModeMasked}),
				WithDescription(childDescription),
				WithDescriptionHTML(childDescriptionHTML),
				WithDisplayName("display static options"),
				WithTestCaseName("logical static options"),
				WithTestCaseID("case-static"),
				WithHistoryID("history-static"),
			)
		})

		a.Step("verify generated static metadata result", func(a *Context) {
			snapshot := memory.Snapshot()
			a.Attachment("generated static metadata artifacts", []byte(snapshotEvidence(snapshot, nil)), "text/plain")
			if len(snapshot.Results) != 1 {
				a.T().Fatalf("expected one result, got %d", len(snapshot.Results))
			}

			result := snapshot.Results[0]
			if result.Name != "display static options" {
				a.T().Fatalf("unexpected display name: %q", result.Name)
			}
			if result.TestCaseName != "logical static options" {
				a.T().Fatalf("unexpected test case name: %q", result.TestCaseName)
			}
			if result.TestCaseID != "case-static" {
				a.T().Fatalf("unexpected test case id: %q", result.TestCaseID)
			}
			if result.HistoryID != "history-static" {
				a.T().Fatalf("unexpected history id: %q", result.HistoryID)
			}
			if result.Description != childDescription || result.DescriptionHTML != childDescriptionHTML {
				a.T().Fatalf("unexpected descriptions: markdown=%q html=%q", result.Description, result.DescriptionHTML)
			}
			for _, label := range []model.Label{
				{Name: "ALLURE_ID", Value: "A-1"},
				{Name: "owner", Value: "qa"},
				{Name: "tag", Value: "smoke"},
				{Name: "severity", Value: "critical"},
				{Name: "epic", Value: "Accounts"},
				{Name: "feature", Value: "Authentication"},
				{Name: "story", Value: "Login"},
				{Name: "parentSuite", Value: "commons"},
				{Name: "suite", Value: "gotest"},
				{Name: "subSuite", Value: "options"},
				{Name: "package", Value: "github.com/allure-framework/allure-go/commons/gotest"},
				{Name: "layer", Value: "unit"},
			} {
				if !hasLabel(result.Labels, label.Name, label.Value) {
					a.T().Fatalf("missing static label %s=%s: %#v", label.Name, label.Value, result.Labels)
				}
			}
			if !hasStaticLink(result.Links, "https://example.test/doc", "doc", string(model.LinkTypeLink)) {
				a.T().Fatalf("missing static link: %#v", result.Links)
			}
			if !hasStaticLink(result.Links, "https://example.test/ref", "ref", string(model.LinkTypeLink)) {
				a.T().Fatalf("missing static bulk link: %#v", result.Links)
			}
			if !hasStaticLink(result.Links, "https://example.test/AUTH-1", "AUTH-1", string(model.LinkTypeIssue)) {
				a.T().Fatalf("missing static issue: %#v", result.Links)
			}
			if !hasStaticLink(result.Links, "https://example.test/TMS-1", "TMS-1", string(model.LinkTypeTMS)) {
				a.T().Fatalf("missing static tms: %#v", result.Links)
			}
			if len(result.Parameters) != 2 || result.Parameters[0].Name != "browser" || result.Parameters[1].Name != "password" || !result.Parameters[1].Excluded || result.Parameters[1].Mode != model.ParameterModeMasked {
				a.T().Fatalf("unexpected static parameters: %#v", result.Parameters)
			}
		})
	})
}

func TestContentAttachmentsUseAllureJSExtensions(t *testing.T) {
	Test(t, "content attachments use Allure JS content type extensions", func(a *Context) {
		a.Description("Runs a child gotest report through an isolated in-memory writer and records content-backed attachments whose MIME types are representative entries from the Allure JS extension table. " +
			"The expected result is that gotest gives each attachment a deterministic source name with the same extension Allure JS would use, including common text and JSON types, Allure-specific payloads, binary payloads, and parameterized content types.")

		memory := commonswriter.NewInMemoryWriter()
		cases := []struct {
			name        string
			contentType string
			payload     string
			id          string
			wantSource  string
		}{
			{name: "json payload", contentType: "application/json", payload: `{"ok":true}`, id: "json-source", wantSource: "json-source-attachment.json"},
			{name: "text payload", contentType: "text/plain", payload: "ok=true", id: "text-source", wantSource: "text-source-attachment.txt"},
			{name: "image diff payload", contentType: "application/vnd.allure.image.diff", payload: "imagediff", id: "diff-source", wantSource: "diff-source-attachment.imagediff"},
			{name: "http exchange payload", contentType: "application/vnd.allure.http+json", payload: `{"request":{},"response":{}}`, id: "http-source", wantSource: "http-source-attachment.httpexchange"},
			{name: "metadata payload", contentType: "application/vnd.allure.metadata+json", payload: `{"name":"value"}`, id: "metadata-source", wantSource: "metadata-source-attachment.metadata"},
			{name: "playwright trace payload", contentType: "application/vnd.allure.playwright-trace", payload: "zip-bytes", id: "trace-source", wantSource: "trace-source-attachment.zip"},
			{name: "binary payload", contentType: "application/octet-stream", payload: "raw", id: "binary-source", wantSource: "binary-source-attachment.bin"},
			{name: "dita map payload", contentType: "application/dita+xml; format=map", payload: "<map/>", id: "dita-source", wantSource: "dita-source-attachment.ditamap"},
			{name: "json charset payload", contentType: "application/json; charset=utf-8", payload: `{"charset":"utf-8"}`, id: "json-charset-source", wantSource: "json-charset-source-attachment.json"},
		}

		a.Step("run child test with content-backed attachments", func(a *Context) {
			Test(a.T(), "content attachment child", func(a *Context) {
				a.Step("record typed content attachments", func(a *Context) {
					for _, tc := range cases {
						a.Attachment(tc.name, []byte(tc.payload), tc.contentType)
					}
				})
			}, WithWriter(memory), WithIDGenerator(fixedIDs("typed-content",
				"json-source",
				"text-source",
				"diff-source",
				"http-source",
				"metadata-source",
				"trace-source",
				"binary-source",
				"dita-source",
				"json-charset-source",
			)))
		})

		a.Step("verify generated attachment source extensions match Allure JS", func(a *Context) {
			snapshot := memory.Snapshot()
			a.Attachment("generated attachment sources", []byte(snapshotEvidence(snapshot, nil)), "text/plain")
			if len(snapshot.Results) != 1 {
				a.T().Fatalf("expected one child result, got %d", len(snapshot.Results))
			}
			attachments := snapshot.Results[0].Steps[0].Attachments
			if len(attachments) != len(cases) {
				a.T().Fatalf("expected %d step attachments, got %#v", len(cases), attachments)
			}
			for index, tc := range cases {
				if attachments[index].Source != tc.wantSource {
					a.T().Fatalf("%s attachment source did not use Allure JS extension: want %q got %#v", tc.name, tc.wantSource, attachments[index])
				}
			}
		})
	})
}

func TestTitlePathIncludesPackageFolders(t *testing.T) {
	Test(t, "title path includes package folders", func(a *Context) {
		a.Description("Runs a child gotest report through an isolated in-memory writer from the commons/gotest package. " +
			"The expected result is that the generated Allure title path starts with package folder names relative to the current module before the Go test and subtest hierarchy, and the default suite labels follow the same path.")

		memory := commonswriter.NewInMemoryWriter()

		a.Step("run child test from commons gotest package", func(a *Context) {
			Test(a.T(), "package path child", func(a *Context) {
				a.Step("record child evidence", func(a *Context) {
					a.Attachment("child package context", []byte("module=commons\npackage=gotest"), "text/plain")
				})
			}, WithWriter(memory), WithIDGenerator(fixedIDs("title-path-child", "title-path-attachment")))
		})

		a.Step("verify title path includes package folders", func(a *Context) {
			snapshot := memory.Snapshot()
			a.Attachment("generated title path artifacts", []byte(snapshotEvidence(snapshot, nil)), "text/plain")
			if len(snapshot.Results) != 1 {
				a.T().Fatalf("expected one child result, got %d", len(snapshot.Results))
			}

			got := snapshot.Results[0].TitlePath
			want := []string{"gotest", "TestTitlePathIncludesPackageFolders", "title_path_includes_package_folders", "package_path_child"}
			a.Attachment("observed title path", []byte(strings.Join(got, "\n")), "text/plain")
			a.Attachment("expected title path", []byte(strings.Join(want, "\n")), "text/plain")
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				a.T().Fatalf("unexpected title path\nwant: %#v\n got: %#v", want, got)
			}
			if !hasLabel(snapshot.Results[0].Labels, labelParentSuite, "gotest") {
				a.T().Fatalf("missing generated parentSuite label: %#v", snapshot.Results[0].Labels)
			}
			if !hasLabel(snapshot.Results[0].Labels, labelSuite, "TestTitlePathIncludesPackageFolders") {
				a.T().Fatalf("missing generated suite label: %#v", snapshot.Results[0].Labels)
			}
			if !hasLabel(snapshot.Results[0].Labels, labelSubSuite, "title_path_includes_package_folders > package_path_child") {
				a.T().Fatalf("missing generated subSuite label: %#v", snapshot.Results[0].Labels)
			}
		})
	})
}

func TestSuiteLabelsFromSingleElementTitlePath(t *testing.T) {
	Test(t, "single-element title path becomes suite label", func(a *Context) {
		a.Description("Verifies the suite label rule for a title path containing only one segment. " +
			"The expected result is that the segment is reported as the suite label and no parentSuite or subSuite label is generated.")

		var labels []model.Label
		titlePath := []string{"lonely"}

		a.Step("derive suite labels from one title path segment", func(a *Context) {
			labels = suiteLabelsFromTitlePath(titlePath, nil)
			a.Attachment("title path", []byte(strings.Join(titlePath, "\n")), "text/plain")
			a.Attachment("derived suite labels", []byte(labelsEvidence(labels)), "text/plain")
		})

		a.Step("verify only suite label is generated", func(a *Context) {
			assertLabelsExact(a, labels, []model.Label{{Name: labelSuite, Value: "lonely"}})
		})
	})
}

func TestSuiteLabelsFromTwoElementTitlePath(t *testing.T) {
	Test(t, "two-element title path becomes parent suite and suite labels", func(a *Context) {
		a.Description("Verifies the suite label rule for a title path containing two segments. " +
			"The expected result is that the first segment becomes parentSuite, the second segment becomes suite, and no subSuite label is generated.")

		var labels []model.Label
		titlePath := []string{"model", "TestName"}

		a.Step("derive suite labels from two title path segments", func(a *Context) {
			labels = suiteLabelsFromTitlePath(titlePath, nil)
			a.Attachment("title path", []byte(strings.Join(titlePath, "\n")), "text/plain")
			a.Attachment("derived suite labels", []byte(labelsEvidence(labels)), "text/plain")
		})

		a.Step("verify parentSuite and suite labels are generated", func(a *Context) {
			assertLabelsExact(a, labels, []model.Label{
				{Name: labelParentSuite, Value: "model"},
				{Name: labelSuite, Value: "TestName"},
			})
		})
	})
}

func TestSuiteLabelsFromLongTitlePath(t *testing.T) {
	Test(t, "long title path becomes parent suite suite and joined sub suite", func(a *Context) {
		a.Description("Verifies the suite label rule for a title path containing more than two segments. " +
			"The expected result is that the first two segments become parentSuite and suite, while the remaining path is joined with the > separator and reported as subSuite.")

		var labels []model.Label
		titlePath := []string{"model", "TestTestResultJSONShape", "test result serializes to Allure JSON shape"}

		a.Step("derive suite labels from long title path", func(a *Context) {
			labels = suiteLabelsFromTitlePath(titlePath, nil)
			a.Attachment("title path", []byte(strings.Join(titlePath, "\n")), "text/plain")
			a.Attachment("derived suite labels", []byte(labelsEvidence(labels)), "text/plain")
		})

		a.Step("verify joined subSuite label is generated", func(a *Context) {
			assertLabelsExact(a, labels, []model.Label{
				{Name: labelParentSuite, Value: "model"},
				{Name: labelSuite, Value: "TestTestResultJSONShape"},
				{Name: labelSubSuite, Value: "test result serializes to Allure JSON shape"},
			})
		})
	})
}

func TestExplicitSuiteLabelsOverrideGeneratedTitlePathLabels(t *testing.T) {
	Test(t, "explicit suite labels override generated title path labels", func(a *Context) {
		a.Description("Verifies that static or environment-provided suite labels can override the defaults derived from titlePath. " +
			"The expected result is that no generated parentSuite, suite, or subSuite label is added when all three suite hierarchy labels are already explicit.")

		explicit := []model.Label{
			{Name: labelParentSuite, Value: "custom parent"},
			{Name: labelSuite, Value: "custom suite"},
			{Name: labelSubSuite, Value: "custom sub"},
		}
		titlePath := []string{"commons", "model", "TestName"}
		var generated []model.Label

		a.Step("derive suite labels with explicit overrides present", func(a *Context) {
			generated = suiteLabelsFromTitlePath(titlePath, explicit)
			a.Attachment("title path", []byte(strings.Join(titlePath, "\n")), "text/plain")
			a.Attachment("explicit suite labels", []byte(labelsEvidence(explicit)), "text/plain")
			a.Attachment("generated suite labels", []byte(labelsEvidence(generated)), "text/plain")
		})

		a.Step("verify no generated labels replace explicit suite labels", func(a *Context) {
			assertLabelsExact(a, generated, nil)
		})
	})
}

func TestLabelsFromEnv(t *testing.T) {
	Test(t, "environment labels map to Allure labels", func(a *Context) {
		a.Description("Verifies that gotest can read module-level labels from environment variables. " +
			"The expected result is that ALLURE_LABEL_MODULE becomes module and ALLURE_LABEL_PARENT_SUITE becomes parentSuite while empty and unrelated variables are ignored.")

		env := []string{
			"ALLURE_LABEL_MODULE=commons",
			"ALLURE_LABEL_PARENT_SUITE=runtime",
			"ALLURE_LABEL_EMPTY=",
			"OTHER=value",
		}

		var labels []string
		a.Step("parse label environment variables", func(a *Context) {
			parsed := labelsFromEnv(env)
			for _, label := range parsed {
				labels = append(labels, label.Name+"="+label.Value)
			}
			a.Attachment("parsed labels", []byte(strings.Join(labels, "\n")), "text/plain")
		})

		a.Step("verify canonical label names", func(a *Context) {
			want := []string{"module=commons", "parentSuite=runtime"}
			a.Attachment("expected labels", []byte(strings.Join(want, "\n")), "text/plain")
			if strings.Join(labels, "\n") != strings.Join(want, "\n") {
				a.T().Fatalf("unexpected labels\nwant: %v\n got: %v", want, labels)
			}
		})
	})
}

func hasLabel(labels []model.Label, name string, value string) bool {
	for _, label := range labels {
		if label.Name == name && label.Value == value {
			return true
		}
	}
	return false
}

func assertLabelsExact(a *Context, got []model.Label, want []model.Label) {
	a.T().Helper()

	a.Attachment("observed labels", []byte(labelsEvidence(got)), "text/plain")
	a.Attachment("expected labels", []byte(labelsEvidence(want)), "text/plain")
	if labelsEvidence(got) != labelsEvidence(want) {
		a.T().Fatalf("unexpected labels\nwant: %#v\n got: %#v", want, got)
	}
}

func labelsEvidence(labels []model.Label) string {
	if len(labels) == 0 {
		return "<none>"
	}

	lines := make([]string, 0, len(labels))
	for _, label := range labels {
		lines = append(lines, label.Name+"="+label.Value)
	}

	return strings.Join(lines, "\n")
}

func hasStaticLink(links []model.Link, url string, name string, linkType string) bool {
	for _, link := range links {
		if link.URL == url && link.Name == name && link.Type == linkType {
			return true
		}
	}
	return false
}

func snapshotEvidence(snapshot commonswriter.MemorySnapshot, isolatedMessages []allureruntime.Message) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("results=%d attachments=%d globals=%d isolatedMessages=%d\n", len(snapshot.Results), len(snapshot.Attachments), len(snapshot.Globals), len(isolatedMessages)))

	for index, result := range snapshot.Results {
		builder.WriteString(fmt.Sprintf("\nresult[%d]\n", index))
		builder.WriteString(fmt.Sprintf("uuid=%s\nname=%s\nstatus=%s\nfullName=%s\n", result.UUID, result.Name, result.Status, result.FullName))
		builder.WriteString(fmt.Sprintf("testCaseName=%s\ntestCaseID=%s\nhistoryID=%s\n", result.TestCaseName, result.TestCaseID, result.HistoryID))
		builder.WriteString("labels:\n")
		for _, label := range result.Labels {
			builder.WriteString(fmt.Sprintf("- %s=%s\n", label.Name, label.Value))
		}
		builder.WriteString("links:\n")
		for _, link := range result.Links {
			builder.WriteString(fmt.Sprintf("- %s %s %s\n", link.Type, link.Name, link.URL))
		}
		builder.WriteString("parameters:\n")
		for _, parameter := range result.Parameters {
			builder.WriteString(fmt.Sprintf("- %s=%s excluded=%t mode=%s\n", parameter.Name, parameter.Value, parameter.Excluded, parameter.Mode))
		}
		builder.WriteString("steps:\n")
		for _, step := range result.Steps {
			builder.WriteString(fmt.Sprintf("- %s status=%s attachments=%d parameters=%d\n", step.Name, step.Status, len(step.Attachments), len(step.Parameters)))
		}
	}

	sources := make([]string, 0, len(snapshot.Attachments))
	for source := range snapshot.Attachments {
		sources = append(sources, source)
	}
	sort.Strings(sources)
	builder.WriteString("\nattachment sources:\n")
	for _, source := range sources {
		builder.WriteString(fmt.Sprintf("- %s bytes=%d\n", source, len(snapshot.Attachments[source])))
	}

	builder.WriteString("\nglobals:\n")
	for _, globals := range snapshot.Globals {
		builder.WriteString(fmt.Sprintf("- uuid=%s attachments=%d errors=%d\n", globals.UUID, len(globals.Attachments), len(globals.Errors)))
	}

	builder.WriteString("\nisolated message types:\n")
	for _, message := range isolatedMessages {
		builder.WriteString(fmt.Sprintf("- %s\n", message.Type))
	}

	return builder.String()
}

func fixedClock(values ...int64) func() int64 {
	index := 0
	return func() int64 {
		if index >= len(values) {
			return values[len(values)-1]
		}
		value := values[index]
		index++
		return value
	}
}

func fixedIDs(values ...string) func() string {
	index := 0
	return func() string {
		if index >= len(values) {
			return values[len(values)-1]
		}
		value := values[index]
		index++
		return value
	}
}
