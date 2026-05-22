package allure

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strings"
	"testing"

	commons "github.com/allure-framework/allure-go/commons"
	"github.com/allure-framework/allure-go/commons/model"
	"github.com/allure-framework/allure-go/commons/testplan"
)

type probeRun struct {
	commandLine   string
	workingDir    string
	artifactDir   string
	envParameters map[string]string
	output        []byte
	err           error
	producedFiles []producedFile
	results       []model.TestResult
	globals       []model.Globals
	attachments   map[string][]byte
}

type producedFile struct {
	path    string
	content []byte
}

func TestGotestRunContractStatusesAndMetadata(t *testing.T) {
	Test(t, "gotest run contract statuses and metadata", func(a *Context) {
		a.Description("Creates temporary Go projects and runs them through the gotest adapter in separate Go processes for passed, failed, broken, and skipped outcomes. " +
			"The expected result is that each child process writes a real Allure result with the correct status, full name, title path, explicit identifiers, metadata, step status, attachments, and global artifacts.")

		cases := []struct {
			mode        string
			status      model.Status
			wantSuccess bool
		}{
			{mode: "passed", status: model.StatusPassed, wantSuccess: true},
			{mode: "failed", status: model.StatusFailed, wantSuccess: false},
			{mode: "broken", status: model.StatusBroken, wantSuccess: false},
			{mode: "skipped", status: model.StatusSkipped, wantSuccess: true},
		}

		for _, tc := range cases {
			tc := tc
			a.Step("validate "+tc.mode+" subprocess probe", func(a *Context) {
				dir := a.T().TempDir()
				run := runStatusProbe(a, tc.mode, dir, "", true)

				a.Step("verify "+tc.mode+" process exit status", func(a *Context) {
					assertProbeExit(a.T(), tc.mode, run.err, tc.wantSuccess, run.output)
				})
				a.Step("verify "+tc.mode+" result status and metadata", func(a *Context) {
					result := requireOneResult(a.T(), run)
					assertStatusProbeResult(a.T(), result, tc.mode, tc.status)
				})
				a.Step("verify "+tc.mode+" global error and global attachment results", func(a *Context) {
					assertProbeGlobals(a.T(), run, tc.mode)
				})
				a.Step("verify "+tc.mode+" produced attachment contents", func(a *Context) {
					assertProbeAttachments(a.T(), run, tc.mode)
				})
			})
		}
	})
}

func TestGotestConfigurationResultsDirectory(t *testing.T) {
	Test(t, "gotest configuration resolves result directories", func(a *Context) {
		a.Description("Verifies the result directory behavior used by command-line Go test runs. " +
			"The expected result is that an absolute ALLURE_RESULTS_DIR is honored and, when it is unset, the adapter writes to an allure-results directory below the test process working directory.")

		a.Step("write to an explicit absolute results directory", func(a *Context) {
			dir := filepath.Join(a.T().TempDir(), "custom-results")
			run := runStatusProbe(a, "passed", dir, "", true)

			a.Step("verify explicit results directory process exit status", func(a *Context) {
				assertProbeExit(a.T(), "explicit results dir", run.err, true, run.output)
			})
			a.Step("verify explicit results directory contains passed result files", func(a *Context) {
				assertProducedFilesUnderDir(a.T(), run, dir)
				result := requireOneResult(a.T(), run)
				if result.Status != model.StatusPassed {
					a.T().Fatalf("unexpected status from explicit results dir probe: %s", result.Status)
				}
			})
		})

		a.Step("write to the default results directory under the child working directory", func(a *Context) {
			workDir := a.T().TempDir()
			resultDir := filepath.Join(workDir, defaultResultsDir)
			run := runStatusProbe(a, "passed", resultDir, workDir, false)

			a.Step("verify default results directory process exit status", func(a *Context) {
				assertProbeExit(a.T(), "default results dir", run.err, true, run.output)
			})
			a.Step("verify default results directory is below the child working directory", func(a *Context) {
				assertProducedFilesUnderDir(a.T(), run, resultDir)
				result := requireOneResult(a.T(), run)
				if result.Status != model.StatusPassed {
					a.T().Fatalf("unexpected status from default results dir probe: %s", result.Status)
				}
			})
		})
	})
}

func TestGotestSubtestsAndParallelIsolation(t *testing.T) {
	Test(t, "gotest subtests and parallel runs isolate results", func(a *Context) {
		a.Description("Creates temporary Go projects whose tests emit multiple reported subtests and parallel reported tests with shared result directories. " +
			"The expected result is that every Allure result keeps its own label, step, and attachment evidence without leaking metadata across sibling or parallel tests.")

		a.Step("verify sibling subtests produce distinct results", func(a *Context) {
			run := runProbe(a, "^TestNestedSubtests$", "", "", true, nil)

			a.Step("verify sibling subtests process exit status", func(a *Context) {
				assertProbeExit(a.T(), "nested subtests", run.err, true, run.output)
			})
			a.Step("verify sibling subtests keep distinct scenario labels and attachments", func(a *Context) {
				assertScenarioResults(a.T(), run, "scenario", []string{"valid credentials", "locked account"})
			})
		})

		a.Step("verify parallel subtests keep isolated metadata and attachments", func(a *Context) {
			run := runProbe(a, "^TestParallelIsolation$", "", "", true, nil)

			a.Step("verify parallel subtests process exit status", func(a *Context) {
				assertProbeExit(a.T(), "parallel subtests", run.err, true, run.output)
			})
			a.Step("verify parallel subtests do not leak labels or attachments", func(a *Context) {
				assertScenarioResults(a.T(), run, "parallelCase", []string{"parallel alpha", "parallel beta"})
			})
		})
	})
}

func TestGotestTestPlanFiltering(t *testing.T) {
	Test(t, "gotest test plan filtering", func(a *Context) {
		a.Description("Creates temporary Go projects and runs them with ALLURE_TESTPLAN_PATH configured for static metadata known before the test body executes. " +
			"The expected result is that gotest executes and reports selected tests while deselected tests are skipped before their body can write evidence.")

		a.Step("select child test by static Allure ID", func(a *Context) {
			planPath := writeProbeTestPlan(a, `{"version":"1.0","tests":[{"id":"PLAN-1"}]}`)
			run := runProbe(a, "^TestPlanAllureIDSelection$", "", "", true, map[string]string{
				testplan.EnvPath: planPath,
			}, planPath)

			a.Step("verify Allure ID test plan process exit status", func(a *Context) {
				assertProbeExit(a.T(), "allure id test plan", run.err, true, run.output)
			})
			a.Step("verify only the selected Allure ID result is reported", func(a *Context) {
				assertScenarioResults(a.T(), run, "planCase", []string{"selected-id"})
				result := requireOneResult(a.T(), run)
				if !hasLabel(result.Labels, "ALLURE_ID", "PLAN-1") {
					a.T().Fatalf("missing selected Allure ID label: %#v", result.Labels)
				}
			})
		})

		a.Step("select child test by full name", func(a *Context) {
			planPath := writeProbeTestPlan(a, `{"version":"1.0","tests":[{"selector":"TestPlanFullNameSelection/selected-by-full-name"}]}`)
			run := runProbe(a, "^TestPlanFullNameSelection$", "", "", true, map[string]string{
				testplan.EnvPath: planPath,
			}, planPath)

			a.Step("verify full-name test plan process exit status", func(a *Context) {
				assertProbeExit(a.T(), "full-name test plan", run.err, true, run.output)
			})
			a.Step("verify only the selected full-name result is reported", func(a *Context) {
				assertScenarioResults(a.T(), run, "planCase", []string{"selected-full-name"})
			})
		})
	})
}

func writeProbeTestPlan(a *Context, content string) string {
	a.T().Helper()

	path := filepath.Join(a.T().TempDir(), "testplan.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		a.T().Fatalf("write test plan: %v", err)
	}

	return path
}

func runStatusProbe(a *Context, mode string, resultDir string, chdir string, setResultsDir bool) probeRun {
	a.T().Helper()

	extraEnv := map[string]string{
		"ALLURE_GOTEST_PROBE":       mode,
		"ALLURE_LABEL_MODULE":       "gotest-probe",
		"ALLURE_LABEL_PARENT_SUITE": "acceptance",
		"ALLURE_LABEL_SUITE":        "runtime",
		"ALLURE_LABEL_SUB_SUITE":    "status",
	}
	if chdir != "" {
		extraEnv["ALLURE_GOTEST_CHDIR"] = chdir
	}

	return runProbe(a, "^TestProbeStatus$", resultDir, chdir, setResultsDir, extraEnv)
}

func runProbe(a *Context, pattern string, resultDir string, chdir string, setResultsDir bool, extraEnv map[string]string, preparedFiles ...string) probeRun {
	a.T().Helper()

	var project probeProject
	commandArgs := []string{"go", "test", "-count=1", "-run", pattern, "."}
	commandLine := strings.Join(commandArgs, " ")
	var artifactDir string

	a.Step("prepare test project", func(a *Context) {
		project = prepareProbeProject(a)
		files := append([]string{}, project.files...)
		files = append(files, preparedFiles...)
		for _, path := range files {
			attachFileEvidence(a, preparedAttachmentName(project.dir, path), path)
		}
	})

	if setResultsDir {
		artifactDir = resultDir
		if artifactDir == "" {
			artifactDir = filepath.Join(project.dir, defaultResultsDir)
		}
	} else {
		artifactDir = filepath.Join(chdir, defaultResultsDir)
	}
	envParameters := probeEnvParameters(artifactDir, setResultsDir, extraEnv)

	var run probeRun
	a.Step("run "+commandLine, func(a *Context) {
		a.StepParameter("working directory", project.dir)
		a.StepParameter("results directory", artifactDir)
		for _, name := range sortedKeys(envParameters) {
			a.StepParameter("env "+name, envParameters[name])
		}

		command := exec.Command(commandArgs[0], commandArgs[1:]...)
		command.Dir = project.dir
		command.Env = probeEnv(artifactDir, setResultsDir, extraEnv)
		output, err := command.CombinedOutput()

		run = collectProbeArtifacts(a.T(), artifactDir)
		run.commandLine = commandLine
		run.workingDir = project.dir
		run.artifactDir = artifactDir
		run.envParameters = envParameters
		run.output = output
		run.err = err

		a.Attachment("process output", output, "text/plain")
		for _, file := range run.producedFiles {
			attachContentEvidence(a, producedAttachmentName(project.dir, artifactDir, file.path), file.content, file.path)
		}
	})

	return run
}

func attachFileEvidence(a *Context, name string, path string) {
	a.T().Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		a.T().Fatalf("read evidence file %s: %v", path, err)
	}
	attachContentEvidence(a, name, content, path)
}

func attachContentEvidence(a *Context, name string, content []byte, path string) {
	a.T().Helper()

	a.report(commons.Attachment(a.ctx, name, content, commons.AttachmentOptions{
		ContentType:   contentTypeForPath(path),
		FileExtension: filepath.Ext(path),
	}))
}

func preparedAttachmentName(projectDir string, path string) string {
	if name, ok := relativeName(projectDir, path); ok {
		return name
	}

	return filepath.ToSlash(filepath.Base(path))
}

func producedAttachmentName(projectDir string, artifactDir string, path string) string {
	if name, ok := relativeName(projectDir, path); ok {
		return name
	}
	if name, ok := relativeName(artifactDir, path); ok {
		return filepath.ToSlash(filepath.Join(defaultResultsDir, filepath.FromSlash(name)))
	}

	return filepath.ToSlash(filepath.Base(path))
}

func relativeName(base string, path string) (string, bool) {
	if base == "" {
		return "", false
	}

	relative, err := filepath.Rel(base, path)
	if err != nil || relative == "." || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}

	return filepath.ToSlash(relative), true
}

func probeEnvParameters(resultDir string, setResultsDir bool, extra map[string]string) map[string]string {
	parameters := map[string]string{
		"GOWORK": "off",
	}
	for key, value := range extra {
		parameters[key] = value
	}
	if setResultsDir {
		parameters["ALLURE_RESULTS_DIR"] = resultDir
	} else {
		parameters["ALLURE_RESULTS_DIR"] = "<unset>"
	}

	return parameters
}

func probeEnv(resultDir string, setResultsDir bool, extra map[string]string) []string {
	overrides := map[string]string{
		"GOWORK": "off",
	}
	for key, value := range extra {
		overrides[key] = value
	}
	if setResultsDir {
		overrides["ALLURE_RESULTS_DIR"] = resultDir
	}

	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if name == "ALLURE_RESULTS_DIR" && !setResultsDir {
			continue
		}
		if name == testplan.EnvPath {
			continue
		}
		if strings.HasPrefix(name, "ALLURE_LABEL_") {
			continue
		}
		if _, ok := overrides[name]; ok {
			continue
		}
		env = append(env, entry)
	}
	for name, value := range overrides {
		env = append(env, name+"="+value)
	}
	sort.Strings(env)

	return env
}

type probeProject struct {
	dir   string
	files []string
}

func prepareProbeProject(a *Context) probeProject {
	a.T().Helper()

	dir := a.T().TempDir()
	files := []preparedProjectFile{
		{
			name:    "go.mod",
			content: []byte(probeGoMod(a.T())),
		},
		{
			name:    "probe_test.go",
			content: readProbeFixture(a.T()),
		},
	}

	written := make([]string, 0, len(files))
	for _, file := range files {
		path := filepath.Join(dir, file.name)
		if err := os.WriteFile(path, file.content, 0o644); err != nil {
			a.T().Fatalf("write probe project file %s: %v", path, err)
		}
		written = append(written, path)
	}

	return probeProject{dir: dir, files: written}
}

type preparedProjectFile struct {
	name    string
	content []byte
}

func probeGoMod(t *testing.T) string {
	t.Helper()

	return strings.Join([]string{
		"module alluregotestprobe",
		"",
		"go 1.25.0",
		"",
		"require github.com/allure-framework/allure-go/commons v0.0.0",
		"",
		"replace github.com/allure-framework/allure-go/commons => " + filepath.ToSlash(commonsModuleDir(t)),
		"",
	}, "\n")
}

func readProbeFixture(t *testing.T) []byte {
	t.Helper()

	content, err := os.ReadFile(probeFixturePath(t))
	if err != nil {
		t.Fatalf("read probe fixture: %v", err)
	}

	return content
}

func probeFixturePath(t *testing.T) string {
	t.Helper()

	return filepath.Join(gotestDir(t), "testdata", "statusprobe", "probe_test.go")
}

func commonsModuleDir(t *testing.T) string {
	t.Helper()

	return filepath.Clean(filepath.Join(gotestDir(t), ".."))
}

func gotestDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := goruntime.Caller(0)
	if !ok {
		t.Fatal("resolve gotest directory")
	}

	return filepath.Dir(file)
}

func collectProbeArtifacts(t *testing.T, dir string) probeRun {
	t.Helper()

	run := probeRun{
		artifactDir:   dir,
		attachments:   map[string][]byte{},
		producedFiles: collectProducedFiles(t, dir),
	}
	for _, file := range run.producedFiles {
		run.attachments[filepath.Base(file.path)] = file.content
		switch {
		case strings.HasSuffix(file.path, "-result.json"):
			var result model.TestResult
			unmarshalArtifact(t, file.path, file.content, &result)
			run.results = append(run.results, result)
		case strings.HasSuffix(file.path, "-globals.json"):
			var globals model.Globals
			unmarshalArtifact(t, file.path, file.content, &globals)
			run.globals = append(run.globals, globals)
		}
	}

	return run
}

func collectProducedFiles(t *testing.T, dir string) []producedFile {
	t.Helper()

	var files []producedFile
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files = append(files, producedFile{path: path, content: content})
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("collect produced files in %s: %v", dir, err)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].path < files[j].path
	})

	return files
}

func unmarshalArtifact(t *testing.T, path string, content []byte, target any) {
	t.Helper()

	if err := json.Unmarshal(content, target); err != nil {
		t.Fatalf("unmarshal %s: %v\n%s", path, err, content)
	}
}

func assertProbeExit(t *testing.T, name string, err error, wantSuccess bool, output []byte) {
	t.Helper()

	if wantSuccess && err != nil {
		t.Fatalf("%s probe failed: %v\n%s", name, err, output)
	}
	if !wantSuccess && err == nil {
		t.Fatalf("%s probe passed unexpectedly\n%s", name, output)
	}
}

func assertProducedFilesUnderDir(t *testing.T, run probeRun, dir string) {
	t.Helper()

	if len(run.producedFiles) == 0 {
		t.Fatalf("expected produced files under %s, got none", dir)
	}

	for _, file := range run.producedFiles {
		relative, err := filepath.Rel(dir, file.path)
		if err != nil {
			t.Fatalf("resolve produced file %s relative to %s: %v", file.path, dir, err)
		}
		if relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			t.Fatalf("produced file %s is outside expected directory %s", file.path, dir)
		}
	}
}

func requireOneResult(t *testing.T, run probeRun) model.TestResult {
	t.Helper()

	if len(run.results) != 1 {
		t.Fatalf("expected one result, got %d: %#v\n%s", len(run.results), run.results, run.output)
	}

	return run.results[0]
}

func assertStatusProbeResult(t *testing.T, result model.TestResult, mode string, status model.Status) {
	t.Helper()

	if result.Status != status {
		t.Fatalf("unexpected status for %s: %s", mode, result.Status)
	}
	if result.Name != "display "+mode {
		t.Fatalf("unexpected display name for %s: %q", mode, result.Name)
	}
	if result.TestCaseName != "logical "+mode {
		t.Fatalf("unexpected test case name for %s: %q", mode, result.TestCaseName)
	}
	if !strings.Contains(result.FullName, "TestProbeStatus/probe_"+mode) {
		t.Fatalf("unexpected full name for %s: %q", mode, result.FullName)
	}
	if len(result.TitlePath) < 2 || result.TitlePath[0] != "TestProbeStatus" || !strings.Contains(result.TitlePath[len(result.TitlePath)-1], mode) {
		t.Fatalf("unexpected title path for %s: %#v", mode, result.TitlePath)
	}
	if result.TestCaseID != "case-"+mode {
		t.Fatalf("unexpected test case id for %s: %q", mode, result.TestCaseID)
	}
	if result.HistoryID != "history-"+mode {
		t.Fatalf("unexpected history id for %s: %q", mode, result.HistoryID)
	}
	if result.Description == "" {
		t.Fatalf("expected markdown description for %s", mode)
	}
	if result.DescriptionHTML != "" {
		t.Fatalf("expected no html description for actual probe test %s, got %q", mode, result.DescriptionHTML)
	}
	if !hasLabel(result.Labels, "framework", "go-test") || !hasLabel(result.Labels, "module", "gotest-probe") || !hasLabel(result.Labels, "probe", mode) {
		t.Fatalf("missing expected labels for %s: %#v", mode, result.Labels)
	}
	if !hasLink(result.Links, "https://example.test/"+mode, "probe "+mode, string(model.LinkTypeLink)) {
		t.Fatalf("missing expected link for %s: %#v", mode, result.Links)
	}
	if !hasParameter(result.Parameters, "mode", mode) {
		t.Fatalf("missing expected parameter for %s: %#v", mode, result.Parameters)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected two steps for %s, got %#v", mode, result.Steps)
	}
	if result.Steps[1].Status != status {
		t.Fatalf("unexpected final step status for %s: %s", mode, result.Steps[1].Status)
	}
}

func assertProbeGlobals(t *testing.T, run probeRun, mode string) {
	t.Helper()

	var hasGlobalError bool
	var hasGlobalAttachment bool
	for _, globals := range run.globals {
		for _, details := range globals.Errors {
			if details.Message == "global error "+mode {
				hasGlobalError = true
			}
		}
		for _, attachment := range globals.Attachments {
			if attachment.Name == "global content "+mode {
				hasGlobalAttachment = true
			}
		}
	}
	if !hasGlobalError || !hasGlobalAttachment {
		t.Fatalf("missing global artifacts for %s: %#v", mode, run.globals)
	}
}

func assertProbeAttachments(t *testing.T, run probeRun, mode string) {
	t.Helper()

	expected := map[string]string{
		"global content " + mode:  "mode=" + mode,
		"path attachment " + mode: "path=" + mode,
		"status evidence":         statusEvidence(mode),
	}
	for name, want := range expected {
		got, ok := findAttachmentContent(run, name)
		if !ok {
			t.Fatalf("missing attachment %q for %s", name, mode)
		}
		if string(got) != want {
			t.Fatalf("unexpected attachment %q for %s: want %q got %q", name, mode, want, got)
		}
	}
}

func statusEvidence(mode string) string {
	switch mode {
	case "failed":
		return "failed by Errorf"
	case "broken":
		return "broken by panic"
	case "skipped":
		return "skipped by Skip"
	default:
		return mode
	}
}

func assertScenarioResults(t *testing.T, run probeRun, labelName string, scenarios []string) {
	t.Helper()

	if len(run.results) != len(scenarios) {
		t.Fatalf("expected %d results, got %d: %#v\n%s", len(scenarios), len(run.results), run.results, run.output)
	}

	for _, scenario := range scenarios {
		result, ok := findResultByLabel(run.results, labelName, scenario)
		if !ok {
			t.Fatalf("missing result with %s=%s: %#v", labelName, scenario, run.results)
		}
		if result.Status != model.StatusPassed {
			t.Fatalf("unexpected status for %s: %s", scenario, result.Status)
		}
		if len(result.Steps) != 1 || len(result.Steps[0].Attachments) != 1 {
			t.Fatalf("unexpected step evidence for %s: %#v", scenario, result.Steps)
		}
		attachment := result.Steps[0].Attachments[0]
		if got := string(run.attachments[attachment.Source]); got != scenario {
			t.Fatalf("unexpected attachment for %s: %q", scenario, got)
		}
		for _, other := range scenarios {
			if other != scenario && hasLabel(result.Labels, labelName, other) {
				t.Fatalf("result for %s leaked label for %s: %#v", scenario, other, result.Labels)
			}
		}
	}
}

func findResultByLabel(results []model.TestResult, name string, value string) (model.TestResult, bool) {
	for _, result := range results {
		if hasLabel(result.Labels, name, value) {
			return result, true
		}
	}

	return model.TestResult{}, false
}

func findAttachmentContent(run probeRun, name string) ([]byte, bool) {
	for _, result := range run.results {
		if content, ok := findAttachmentContentInAttachments(run, result.Attachments, name); ok {
			return content, true
		}
		if content, ok := findAttachmentContentInSteps(run, result.Steps, name); ok {
			return content, true
		}
	}
	for _, globals := range run.globals {
		if content, ok := findAttachmentContentInGlobalAttachments(run, globals.Attachments, name); ok {
			return content, true
		}
	}

	return nil, false
}

func findAttachmentContentInSteps(run probeRun, steps []model.StepResult, name string) ([]byte, bool) {
	for _, step := range steps {
		if content, ok := findAttachmentContentInAttachments(run, step.Attachments, name); ok {
			return content, true
		}
		if content, ok := findAttachmentContentInSteps(run, step.Steps, name); ok {
			return content, true
		}
	}

	return nil, false
}

func findAttachmentContentInAttachments(run probeRun, attachments []model.Attachment, name string) ([]byte, bool) {
	for _, attachment := range attachments {
		if attachment.Name == name {
			content, ok := run.attachments[attachment.Source]
			return content, ok
		}
	}

	return nil, false
}

func findAttachmentContentInGlobalAttachments(run probeRun, attachments []model.GlobalAttachment, name string) ([]byte, bool) {
	for _, attachment := range attachments {
		if attachment.Name == name {
			content, ok := run.attachments[attachment.Source]
			return content, ok
		}
	}

	return nil, false
}

func hasLink(links []model.Link, url string, name string, linkType string) bool {
	for _, link := range links {
		if link.URL == url && link.Name == name && link.Type == linkType {
			return true
		}
	}

	return false
}

func hasParameter(parameters []model.Parameter, name string, value string) bool {
	for _, parameter := range parameters {
		if parameter.Name == name && parameter.Value == value {
			return true
		}
	}

	return false
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	return keys
}

func contentTypeForPath(path string) string {
	switch filepath.Ext(path) {
	case ".go":
		return "text/x-go"
	case ".json":
		return "application/json"
	case ".mod", ".txt", ".properties":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}
