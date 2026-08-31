// Standard library and module imports for the Chaos compiler.
//
// The standard library lives in chaos-stdlib/core.chaos on disk. Programs opt
// in with '#import «core»;' (flat import) or 'c :: #import «core»;'
// (namespaced import). Unlike a C #include, an import only makes declarations
// available; the backend emits only the functions a program actually reaches
// from its entry point, so unused imports are discarded.
//
// Module resolution searches the stdlib directory first, then the directory
// of the importing file. The stdlib directory is configured by the driver
// through StdlibDir (chaosc sets it from --stdlib); it defaults to
// ./chaos-stdlib.
package compiler

import (
	"os"
	"path/filepath"
	"strings"
)

// StdlibDir is the directory searched for standard-library modules. The
// chaosc CLI sets it from --stdlib; it defaults to ./chaos-stdlib.
var StdlibDir = "chaos-stdlib"

// ResolveImports resolves every ImportDecl in program.Decls. Flat imports
// splice the module's declarations into the program (each module once);
// namespaced imports keep an ImportDecl whose Decls the type checker and
// lowerer consult for 'namespace.member' access. Nested imports inside a
// module are resolved recursively, and a module imported more than once is
// parsed only once. The returned program shares the input's declarations
// except for the resolved imports, and its Sources map includes every module
// file so diagnostics render against the right source.
func ResolveImports(program *Program, stdlibDir string) (*Program, DiagnosticList) {
	return resolveImports(program, stdlibDir, nil)
}

// ResolveImportsWithOverlay resolves imports while preferring the supplied
// canonical-path source bytes. Editor tooling uses this to analyze unsaved
// changes in open imported documents without modifying files on disk.
func ResolveImportsWithOverlay(program *Program, stdlibDir string, overlay map[string][]byte) (*Program, DiagnosticList) {
	return resolveImports(program, stdlibDir, overlay)
}

func resolveImports(program *Program, stdlibDir string, overlay map[string][]byte) (*Program, DiagnosticList) {
	if program == nil {
		return program, nil
	}
	sources := program.Sources
	if sources == nil {
		sources = make(map[FileID]SourceFile)
	}
	r := &importResolver{
		stdlibDir: stdlibDir,
		loaded:    make(map[string]*Program),
		loading:   make(map[string]bool),
		overlay:   overlay,
		sources:   sources,
		nextFile:  nextFileID(sources),
	}
	decls := r.resolveDecls(program.Decls, sourceDir(program))
	merged := &Program{
		Decls:     decls,
		Entry:     program.Entry,
		EntrySpan: program.EntrySpan,
		EntryDecl: program.EntryDecl,
		Sources:   sources,
	}
	return merged, r.diags
}

// importResolver carries the state of one import-resolution pass.
type importResolver struct {
	stdlibDir string
	loaded    map[string]*Program // canonical path -> resolved declarations
	loading   map[string]bool     // canonical paths currently being resolved
	overlay   map[string][]byte   // canonical path -> unsaved source
	sources   map[FileID]SourceFile
	nextFile  FileID
	diags     DiagnosticList
}

// resolveDecls resolves the imports in a declaration list, splicing flat
// imports and keeping namespaced imports as resolved ImportDecl nodes.
func (r *importResolver) resolveDecls(decls []Decl, fromDir string) []Decl {
	out := make([]Decl, 0, len(decls))
	seen := make(map[Decl]bool)
	appendDecl := func(decl Decl) {
		if !seen[decl] {
			seen[decl] = true
			out = append(out, decl)
		}
	}
	for _, decl := range decls {
		imp, ok := decl.(*ImportDecl)
		if !ok {
			appendDecl(decl)
			continue
		}
		mod := r.resolveModule(imp.Module, fromDir, imp.Span_)
		if mod == nil {
			continue // a diagnostic was already reported
		}
		if imp.Namespace == "" {
			// Flat import: splice each declaration once in this declaration
			// list. Cached modules share declaration pointers, so this also
			// removes transitive duplicates without hiding declarations from
			// the parent module that imports this list.
			for _, d := range mod.Decls {
				appendDecl(d)
			}
		} else {
			appendDecl(&ImportDecl{
				Span_:     imp.Span_,
				Module:    imp.Module,
				Namespace: imp.Namespace,
				Decls:     mod.Decls,
			})
		}
	}
	return out
}

// resolveModule loads, parses, and recursively resolves one module. The
// result is cached by canonical path so repeated path aliases share one parse
// while distinct modules with the same basename remain distinct.
func (r *importResolver) resolveModule(name, fromDir string, span Span) *Program {
	path := r.findModule(name, fromDir)
	if path == "" {
		r.diags.Error(span, "cannot find module '"+name+"'", "place '"+name+".chaos' in the stdlib directory or next to the importing file")
		return nil
	}
	canonical, err := canonicalModulePath(path)
	if err != nil {
		r.diags.Error(span, "cannot resolve module '"+name+"': "+err.Error(), "use a valid module path")
		return nil
	}
	if mod, ok := r.loaded[canonical]; ok {
		return mod
	}
	if r.loading[canonical] {
		r.diags.Error(span, "cyclic import of module '"+name+"'", "break the import cycle")
		return nil
	}
	r.loading[canonical] = true
	defer delete(r.loading, canonical)
	source, ok := r.overlay[canonical]
	if !ok {
		var err error
		source, err = os.ReadFile(canonical)
		if err != nil {
			r.diags.Error(span, "cannot read module '"+name+"': "+err.Error(), "check file permissions")
			return nil
		}
	}
	fileID := r.nextFile
	r.nextFile++
	r.sources[fileID] = SourceFile{ID: fileID, Path: canonical, Source: source, LineOffsets: BuildLineOffsets(source)}
	tokens, tokDiags := Tokenize(source, fileID)
	r.diags = append(r.diags, tokDiags...)
	if tokDiags.HasErrors() {
		return nil
	}
	result := ParseProgram(tokens)
	r.diags = append(r.diags, result.Diags...)
	if result.Program == nil {
		return nil
	}
	mod := &Program{Decls: r.resolveDecls(result.Program.Decls, filepath.Dir(canonical))}
	r.loaded[canonical] = mod
	return mod
}

// findModule searches the stdlib directory first, then the importing file's
// directory, for "<module>.chaos".
func (r *importResolver) findModule(name, fromDir string) string {
	for _, dir := range []string{r.stdlibDir, fromDir} {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, name+".chaos")
		canonical, err := canonicalModulePath(candidate)
		if err == nil {
			if _, ok := r.overlay[canonical]; ok {
				return canonical
			}
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func canonicalModulePath(path string) (string, error) {
	canonical, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	canonical = filepath.Clean(canonical)
	if evaluated, evalErr := filepath.EvalSymlinks(canonical); evalErr == nil {
		canonical = evaluated
	}
	return canonical, nil
}

// sourceDir returns the directory of the program's main source file, derived
// from the entry span's file. It returns "" when the program carries no
// sources.
func sourceDir(program *Program) string {
	if sf, ok := program.Sources[program.EntrySpan.File]; ok {
		return filepath.Dir(stripFileScheme(sf.Path))
	}
	return ""
}

// stripFileScheme removes a "file://" prefix from a path so LSP document URIs
// resolve like ordinary filesystem paths.
func stripFileScheme(path string) string {
	if strings.HasPrefix(path, "file://") {
		return strings.TrimPrefix(path, "file://")
	}
	return path
}

// nextFileID returns the first FileID above every ID already present in
// sources, so module files never collide with the driver's main file.
func nextFileID(sources map[FileID]SourceFile) FileID {
	var max FileID = -1
	for id := range sources {
		if id > max {
			max = id
		}
	}
	return max + 1
}
