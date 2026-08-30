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
		seen:      make(map[Decl]bool),
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
	loaded    map[string]*Program // module name -> resolved declarations
	loading   map[string]bool     // modules currently being resolved (cycle guard)
	seen      map[Decl]bool       // declarations already spliced (pointer identity)
	sources   map[FileID]SourceFile
	nextFile  FileID
	diags     DiagnosticList
}

// resolveDecls resolves the imports in a declaration list, splicing flat
// imports and keeping namespaced imports as resolved ImportDecl nodes.
func (r *importResolver) resolveDecls(decls []Decl, fromDir string) []Decl {
	out := make([]Decl, 0, len(decls))
	for _, decl := range decls {
		imp, ok := decl.(*ImportDecl)
		if !ok {
			out = append(out, decl)
			continue
		}
		mod := r.resolveModule(imp.Module, fromDir, imp.Span_)
		if mod == nil {
			continue // a diagnostic was already reported
		}
		if imp.Namespace == "" {
			// Flat import: splice the module's declarations once.
			for _, d := range mod.Decls {
				if !r.seen[d] {
					r.seen[d] = true
					out = append(out, d)
				}
			}
		} else {
			out = append(out, &ImportDecl{
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
// result is cached by module name so repeated imports share one parse.
func (r *importResolver) resolveModule(name, fromDir string, span Span) *Program {
	if mod, ok := r.loaded[name]; ok {
		return mod
	}
	if r.loading[name] {
		r.diags.Error(span, "cyclic import of module '"+name+"'", "break the import cycle")
		return nil
	}
	path := r.findModule(name, fromDir)
	if path == "" {
		r.diags.Error(span, "cannot find module '"+name+"'", "place '"+name+".chaos' in the stdlib directory or next to the importing file")
		return nil
	}
	source, err := os.ReadFile(path)
	if err != nil {
		r.diags.Error(span, "cannot read module '"+name+"': "+err.Error(), "check file permissions")
		return nil
	}
	fileID := r.nextFile
	r.nextFile++
	r.sources[fileID] = SourceFile{ID: fileID, Path: path, Source: source, LineOffsets: BuildLineOffsets(source)}
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
	r.loading[name] = true
	mod := &Program{Decls: r.resolveDecls(result.Program.Decls, filepath.Dir(path))}
	r.loading[name] = false
	r.loaded[name] = mod
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
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
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
