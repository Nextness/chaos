package compiler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func parseGoFileForDrift(t *testing.T, path string) (*ast.File, string) {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, source, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return file, string(source)
}

func receiverTypeName(expr ast.Expr) string {
	switch n := expr.(type) {
	case *ast.Ident:
		return n.Name
	case *ast.StarExpr:
		return receiverTypeName(n.X)
	}
	return ""
}

func receiverTypesWithMethod(file *ast.File, methods ...string) map[string]bool {
	wanted := make(map[string]bool, len(methods))
	for _, method := range methods {
		wanted[method] = true
	}
	types := make(map[string]bool)
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 || !wanted[fn.Name.Name] {
			continue
		}
		if name := receiverTypeName(fn.Recv.List[0].Type); name != "" {
			types[name] = true
		}
	}
	return types
}

func TestASTWalkerCoversEveryNodeType(t *testing.T) {
	astFile, _ := parseGoFileForDrift(t, "ast.go")
	_, walker := parseGoFileForDrift(t, "ast_walk.go")
	for name := range receiverTypesWithMethod(astFile, "nodeSpan") {
		if !strings.Contains(walker, "*"+name) {
			t.Errorf("AST node %s is not handled by WalkAST", name)
		}
	}
}

func TestMIRLowererCoversEveryHIRNodeType(t *testing.T) {
	hirFile, _ := parseGoFileForDrift(t, "hir.go")
	_, lowerer := parseGoFileForDrift(t, "lower_mir.go")
	for name := range receiverTypesWithMethod(hirFile, "hirStmtNode", "hirExprNode") {
		if !strings.Contains(lowerer, "*"+name) {
			t.Errorf("HIR node %s is not handled by MIR lowering", name)
		}
	}
}

func TestVerifierAndBackendMentionEveryMIROpcode(t *testing.T) {
	mirFile, _ := parseGoFileForDrift(t, "mir.go")
	_, verifier := parseGoFileForDrift(t, "ir_verify.go")
	_, backend := parseGoFileForDrift(t, "fasm.go")
	var opcodes []string
	for _, decl := range mirFile.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST || len(gen.Specs) == 0 {
			continue
		}
		first, ok := gen.Specs[0].(*ast.ValueSpec)
		if !ok {
			continue
		}
		typeName, ok := first.Type.(*ast.Ident)
		if !ok || typeName.Name != "MIROpcode" {
			continue
		}
		for _, spec := range gen.Specs {
			values := spec.(*ast.ValueSpec)
			for _, name := range values.Names {
				opcodes = append(opcodes, name.Name)
			}
		}
	}
	if len(opcodes) == 0 {
		t.Fatal("did not discover the MIROpcode declaration")
	}
	for _, opcode := range opcodes {
		if !strings.Contains(verifier, opcode) {
			t.Errorf("MIR opcode %s is not handled by the verifier", opcode)
		}
		if !strings.Contains(backend, opcode) {
			t.Errorf("MIR opcode %s is not handled by the fasm backend", opcode)
		}
	}
}
