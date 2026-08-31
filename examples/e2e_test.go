// End-to-end tests for the Chaos compiler.
//
// The suite drives the full compilation pipeline over the example programs in
// this directory and validates every step:
//
//   - examples/valid/   must compile cleanly through every stage. Their dumps
//     (tokens, AST, HIR, MIR, and fasm assembly) are compared against golden
//     files, and the emitted binary is assembled and executed to check the
//     exit status, stdout, and stderr output.
//   - examples/invalid/ must fail. Each file declares the stage where the
//     failure must occur (tokenize, parse, type, target, lower, mir, verify,
//     emit)
//     and a substring that must appear in the diagnostic message. The first
//     stage with errors must match the declared stage, and the rendered
//     diagnostics are compared against golden files.
//
// Example metadata lives in the leading comment block of each file:
//
//	// expect-exit: 42        (valid) expected exit status of the binary
//	// expect-stdout: <empty> (valid) exact stdout content
//	// expect-stderr: boom    (valid) exact stderr content
//	// expect-stage: target   (invalid) stage that must fail
//	// expect-error: substring (invalid) substring of the diagnostic message
//
// Run with -update to regenerate the golden files, for example:
//
//	cd examples && go test -count=1 -update ./...
//
// This is not a unit test: it exercises the tokenizer, parser, type checker,
// both IRs, the verifier, the fasm backend, the fasm assembler, and the
// produced executable against real source files.
package examples

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"chaos_compiler/compiler"
)

var update = flag.Bool("update", false, "regenerate golden dump files")

// init points the import resolver at the repository's stdlib directory. The
// e2e tests run from examples/, so the stdlib lives one level up.
func init() {
	compiler.StdlibDir = "../chaos-stdlib"
}

// stage names in pipeline order.
var stageNames = []string{"tokenize", "parse", "type", "target", "lower", "mir", "verify", "emit"}

// pipeline holds the output of every compilation stage.
type pipeline struct {
	tokens  []compiler.Token
	parse   compiler.ParseResult
	hir     *compiler.HIR
	mir     *compiler.MIRProgram
	asm     string
	sf      *compiler.SourceFile
	sources map[compiler.FileID]compiler.SourceFile // all files (main + imports)
	diags   map[string]compiler.DiagnosticList      // per-stage diagnostics
	failed  string                                  // first stage with errors, "" if clean
}

// runPipeline drives the full pipeline exactly like the chaosc CLI does and
// records the first stage that reports errors.
func runPipeline(t *testing.T, name string, source []byte) *pipeline {
	t.Helper()
	sm := &compiler.SourceManager{}
	fileID := sm.Register(name, source)
	p := &pipeline{sf: sm.Lookup(fileID), diags: make(map[string]compiler.DiagnosticList)}
	p.sources = map[compiler.FileID]compiler.SourceFile{fileID: *p.sf}

	tokens, diags := compiler.Tokenize(source, fileID)
	p.tokens = tokens
	p.diags["tokenize"] = diags
	if diags.HasErrors() {
		p.failed = "tokenize"
		return p
	}

	result := compiler.ParseProgram(tokens)
	p.parse = result
	p.diags["parse"] = result.Diags
	if result.Program != nil {
		result.Program.Sources = p.sources
	}
	if result.Diags.HasErrors() {
		p.failed = "parse"
		return p
	}

	analysis, diags := compiler.AnalyzeProgram(result.Program)
	if analysis != nil && analysis.Program != nil {
		p.sources = analysis.Program.Sources
	}
	if diags.HasErrors() {
		p.diags["type"] = diags
		p.failed = "type"
		return p
	}

	if diags := compiler.ValidateTarget(result.Program, analysis, "fasm"); diags.HasErrors() {
		p.diags["target"] = diags
		p.failed = "target"
		return p
	}

	hir, diags := compiler.LowerAnalyzedProgram(result.Program, analysis)
	p.hir = hir
	p.diags["lower"] = diags
	if diags.HasErrors() {
		p.failed = "lower"
		return p
	}

	mir, diags := compiler.LowerToMIR(hir)
	p.mir = mir
	p.diags["mir"] = diags
	if diags.HasErrors() {
		p.failed = "mir"
		return p
	}

	if diags := compiler.VerifyMIR(mir); diags.HasErrors() {
		p.diags["verify"] = diags
		p.failed = "verify"
		return p
	}

	backend := compiler.NewBackend("fasm")
	if backend == nil {
		t.Fatalf("fasm backend not registered")
	}
	asm, diags := backend.Emit(mir)
	p.asm = asm
	p.diags["emit"] = diags
	if diags.HasErrors() {
		p.failed = "emit"
		return p
	}
	return p
}

// dumps returns the golden dumps for a clean pipeline, one per compilation
// step.
func (p *pipeline) dumps() map[string]string {
	return map[string]string{
		"tokens": compiler.DumpTokens(p.tokens),
		"ast":    compiler.DumpParseResult(p.parse),
		"hir":    compiler.DumpHIR(p.hir),
		"mir":    compiler.DumpMIR(p.mir),
		"asm":    p.asm,
	}
}

// checkGolden compares content against the golden file at relPath, or writes
// it when -update is set.
func checkGolden(t *testing.T, relPath, content string) {
	t.Helper()
	path := filepath.Join("golden", relPath)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir golden: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden file %s; run with -update to generate it: %v", path, err)
	}
	if string(want) != content {
		t.Errorf("golden mismatch for %s:\n--- want ---\n%s\n--- got ---\n%s", path, want, content)
	}
}

// renderDiags renders a diagnostic list with source context for golden files.
func renderDiags(diags compiler.DiagnosticList, p *pipeline) string {
	var buf bytes.Buffer
	if p.sources != nil {
		compiler.RenderAllSources(&buf, diags, p.sources)
	} else {
		compiler.RenderAll(&buf, diags, p.sf)
	}
	return buf.String()
}

// exampleMeta is the parsed metadata from a file's leading comment block.
type exampleMeta struct {
	hasExit   bool
	exit      int
	hasStdout bool
	stdout    string
	hasStderr bool
	stderr    string
	stage     string
	errorSub  string
}

var metaRe = regexp.MustCompile(`^\s*//\s*expect-([a-z-]+):\s*(.+)\s*$`)

func parseMeta(t *testing.T, source []byte) exampleMeta {
	t.Helper()
	var m exampleMeta
	for _, line := range strings.Split(string(source), "\n") {
		match := metaRe.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		value := strings.TrimSpace(match[2])
		switch match[1] {
		case "exit":
			n, err := strconv.Atoi(value)
			if err != nil {
				t.Fatalf("bad expect-exit %q: %v", value, err)
			}
			m.exit = n
			m.hasExit = true
		case "stderr":
			m.stderr = parseExpectedStream(t, "stderr", value)
			m.hasStderr = true
		case "stdout":
			m.stdout = parseExpectedStream(t, "stdout", value)
			m.hasStdout = true
		case "stage":
			m.stage = value
		case "error":
			m.errorSub = value
		}
	}
	return m
}

func parseExpectedStream(t *testing.T, name, value string) string {
	t.Helper()
	if value == "<empty>" {
		return ""
	}
	if strings.HasPrefix(value, "\"") {
		decoded, err := strconv.Unquote(value)
		if err != nil {
			t.Fatalf("bad expect-%s %q: %v", name, value, err)
		}
		return decoded
	}
	return value
}

func TestValidExamples(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("valid", "*.chaos"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no valid examples found: %v", err)
	}
	fasmPath, _ := exec.LookPath("fasm")
	for _, file := range files {
		file := file
		name := strings.TrimSuffix(filepath.Base(file), ".chaos")
		t.Run("valid/"+name, func(t *testing.T) {
			source, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("read %s: %v", file, err)
			}
			meta := parseMeta(t, source)

			p := runPipeline(t, filepath.Base(file), source)
			if p.failed != "" {
				t.Fatalf("pipeline failed at stage %s:\n%s", p.failed, renderDiags(p.diags[p.failed], p))
			}

			// Validate the dump of every compilation step against goldens.
			for step, dump := range p.dumps() {
				checkGolden(t, filepath.Join("valid", name, step+".txt"), dump)
			}

			// Assemble and run the binary, checking exit status and both
			// output streams, including intentional empty output.
			if fasmPath == "" {
				t.Skip("fasm not available; compilation and dumps validated")
			}
			dir := t.TempDir()
			asmPath := filepath.Join(dir, "out.asm")
			binPath := filepath.Join(dir, "out.bin")
			if err := os.WriteFile(asmPath, []byte(p.asm), 0o600); err != nil {
				t.Fatalf("write asm: %v", err)
			}
			if out, err := exec.Command(fasmPath, asmPath, binPath).CombinedOutput(); err != nil {
				t.Fatalf("fasm failed: %v\n%s", err, out)
			}
			if err := os.Chmod(binPath, 0o700); err != nil {
				t.Fatalf("chmod: %v", err)
			}
			var stdout, stderr bytes.Buffer
			cmd := exec.Command(binPath)
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err = cmd.Run()
			exitCode := 0
			if err != nil {
				var ee *exec.ExitError
				if !errors.As(err, &ee) {
					t.Fatalf("run failed: %v", err)
				}
				exitCode = ee.ExitCode()
			}
			if exitCode != meta.exit {
				t.Errorf("exit code = %d, want %d", exitCode, meta.exit)
			}
			if stdout.String() != meta.stdout {
				t.Errorf("stdout = %q, want %q", stdout.String(), meta.stdout)
			}
			if stderr.String() != meta.stderr {
				t.Errorf("stderr = %q, want %q", stderr.String(), meta.stderr)
			}
		})
	}
}

func TestInvalidExamples(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("invalid", "*.chaos"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no invalid examples found: %v", err)
	}
	for _, file := range files {
		file := file
		name := strings.TrimSuffix(filepath.Base(file), ".chaos")
		t.Run("invalid/"+name, func(t *testing.T) {
			source, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("read %s: %v", file, err)
			}
			meta := parseMeta(t, source)
			if meta.stage == "" || meta.errorSub == "" {
				t.Fatal("invalid example must declare expect-stage and expect-error")
			}

			p := runPipeline(t, filepath.Base(file), source)
			if p.failed == "" {
				t.Fatalf("example compiled cleanly, want failure at stage %s", meta.stage)
			}
			if p.failed != meta.stage {
				t.Errorf("failed at stage %s, want %s\n%s", p.failed, meta.stage, renderDiags(p.diags[p.failed], p))
			}

			// The expected error must appear in the failing stage's diagnostics.
			rendered := renderDiags(p.diags[p.failed], p)
			if !strings.Contains(rendered, meta.errorSub) {
				t.Errorf("diagnostics do not contain %q:\n%s", meta.errorSub, rendered)
			}

			// Track the exact diagnostics as golden output.
			checkGolden(t, filepath.Join("invalid", name, "diagnostics.txt"), rendered)
		})
	}
}

// TestExampleMetadata validates the harness itself: every example must carry
// the metadata its directory requires, so a typo in a header comment fails
// loudly instead of silently skipping a check.
func TestExampleMetadata(t *testing.T) {
	for _, dir := range []string{"valid", "invalid"} {
		files, _ := filepath.Glob(filepath.Join(dir, "*.chaos"))
		for _, file := range files {
			source, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("read %s: %v", file, err)
			}
			meta := parseMeta(t, source)
			switch dir {
			case "valid":
				if meta.stage != "" || meta.errorSub != "" {
					t.Errorf("%s: valid example must not declare expect-stage/expect-error", file)
				}
				if !meta.hasExit {
					t.Errorf("%s: valid example must declare expect-exit", file)
				}
				if !meta.hasStdout {
					t.Errorf("%s: valid example must declare expect-stdout", file)
				}
				if !meta.hasStderr {
					t.Errorf("%s: valid example must declare expect-stderr", file)
				}
			case "invalid":
				if meta.hasExit {
					t.Errorf("%s: invalid example must not declare expect-exit", file)
				}
				if meta.stage == "" {
					t.Errorf("%s: invalid example must declare expect-stage", file)
				}
				if meta.errorSub == "" {
					t.Errorf("%s: invalid example must declare expect-error", file)
				}
				valid := false
				for _, s := range stageNames {
					if s == meta.stage {
						valid = true
					}
				}
				if !valid {
					t.Errorf("%s: unknown stage %q (want one of %v)", file, meta.stage, stageNames)
				}
			}
		}
	}
}
