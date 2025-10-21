package main

const entryPoint string = "main"

// func GenerateCode(prog *Program) *bytes.Buffer {
// 	buffer := bytes.Buffer{}
// 	datBuf := bytes.Buffer{}
// 	strBuf := bytes.Buffer{}
// 	procBuf := bytes.Buffer{}
//
// 	buffer.WriteString("format ELF64 executable 3\n\n")
// 	buffer.WriteString(fmt.Sprintf("entry %s\n\n", entryPoint))
// 	buffer.WriteString("segment readable executable\n\n")
// 	buffer.WriteString(fmt.Sprintf("%s:\n", entryPoint))
//
// 	strCount := 0
//
// 	for _, node := range prog.AllocatedProcs {
// 		if node.VarDecl.Assignment.NodeType == nodeProcDef {
// 			procName := node.VarDecl.Name.Symbol
// 			procBuf.WriteString("; proc_def\n")
// 			procBuf.WriteString(fmt.Sprintf("%s:\n", procName))
// 			for _, nd := range node.VarDecl.Assignment.Proc.Scope {
// 				if nd.NodeType == nodeExit {
// 					procBuf.WriteString("    ; exit\n")
// 					if nd.Exit.Message.NodeType == nodeStringLiteral {
// 						msg := castAssert[string](nd.Exit.Message.Literal.String.Value)
//
// 						strName := fmt.Sprintf("str_%d", strCount)
// 						strSize := fmt.Sprintf("%s_size", strName)
// 						strCount++
//
// 						strBuf.WriteString(fmt.Sprintf("%s db \"%s\", 10\n", strName, msg))
// 						strBuf.WriteString(fmt.Sprintf("%s = $-%s\n", strSize, strName))
//
// 						procBuf.WriteString("    mov rax, 1\n")
// 						procBuf.WriteString("    mov rdi, 1\n")
// 						procBuf.WriteString(fmt.Sprintf("    mov rsi, %s\n", strName))
// 						procBuf.WriteString(fmt.Sprintf("    mov rdx, %s\n", strSize))
// 						procBuf.WriteString("    syscall\n")
// 					}
//
// 					if nd.Exit.Status.NodeType == nodeIdentifier {
// 						procBuf.WriteString("    mov rax, 60\n")
// 						procBuf.WriteString(fmt.Sprintf("    mov rdi, [%s]\n", castAssert[string](nd.Exit.Status.VarDecl.Name.Symbol)))
// 						procBuf.WriteString("    syscall\n")
// 						procBuf.WriteString("    ret\n")
// 					}
// 					if nd.Exit.Status.NodeType == nodeIntLiteral {
// 						status := castAssert[int](nd.Exit.Status.Literal.Int.Value)
// 						procBuf.WriteString("    mov rax, 60\n")
// 						procBuf.WriteString(fmt.Sprintf("    mov rdi, %d\n", status))
// 						procBuf.WriteString("    syscall\n")
// 						procBuf.WriteString("    ret\n")
// 					}
// 					continue
// 				}
// 			}
// 			continue
// 		}
// 	}
//
// 	for _, node := range prog.AllocatedVars {
// 		if node.NodeType == nodeIdentifier {
// 			if node.VarDecl.Assignment.NodeType == nodeBinOp {
// 				varName := node.VarDecl.Name.Symbol
// 				datBuf.WriteString(fmt.Sprintf("    %s dq 0\n", varName))
// 				continue
// 			}
// 			if node.VarDecl.Assignment.NodeType == nodeIntLiteral {
// 				result := castAssert[int](node.VarDecl.Assignment.Literal.Int.Value)
// 				varName := node.VarDecl.Name.Symbol
// 				datBuf.WriteString(fmt.Sprintf("    %s dq %d\n", varName, result))
// 				continue
// 			}
//
// 			if !node.VarDecl.Initialized {
// 				// TODO: Handle different types - right now we only accept Int
// 				assert[any](node.VarDecl.Type.Symbol == "U64", "Right now we only allow U64")
// 				varName := node.VarDecl.Name.Symbol
// 				datBuf.WriteString(fmt.Sprintf("    %s dq 0\n", varName))
// 				continue
// 			}
// 			if node.VarDecl.Assignment.NodeType == nodeIdentifier {
// 				varName := node.VarDecl.Name.Symbol
// 				datBuf.WriteString(fmt.Sprintf("    %s dq 0\n", varName))
// 				continue
// 			}
// 		}
// 	}
//
// 	for opid, node := range prog.Nodes {
// 		if node.NodeType == nodeCall {
// 			name := node.Call.Name.Symbol
// 			buffer.WriteString(fmt.Sprintf("    ; %6d. proc_call\n", opid))
// 			buffer.WriteString(fmt.Sprintf("    call %s\n", name))
// 			continue
// 		}
// 		if node.NodeType == nodeIdentifier {
// 			if node.VarDecl.Assignment.NodeType == nodeIntLiteral ||
// 				node.VarDecl.Assignment.NodeType == nodeFloatLiteral {
// 				continue
// 			}
//
// 			if node.VarDecl.Assignment.NodeType == nodeBinOp {
// 				name := node.VarDecl.Name.Symbol
//
// 				var lhs, rhs Node
// 				var okLhs, okRhs bool = false, false
// 				if node.VarDecl.Assignment.BinOp.Lhs.NodeType != nodeNull {
// 					okLhs = true
// 					lhs = node.VarDecl.Assignment.BinOp.Lhs
// 				}
// 				if node.VarDecl.Assignment.BinOp.Rhs.NodeType != nodeNull {
// 					okRhs = true
// 					rhs = node.VarDecl.Assignment.BinOp.Rhs
// 				}
//
// 				if okLhs && okRhs {
// 					if node.VarDecl.Assignment.BinOp.Operation == opPlus {
// 						buffer.WriteString(fmt.Sprintf("    ; %06d. add\n", opid))
// 						if lhs.NodeType == nodeIntLiteral && rhs.NodeType == nodeIntLiteral {
// 							l := castAssert[int](lhs.Literal.Int.Value)
// 							r := castAssert[int](rhs.Literal.Int.Value)
// 							buffer.WriteString(fmt.Sprintf("    mov rax, %d\n", l))
// 							buffer.WriteString(fmt.Sprintf("    add rax, %d\n", r))
// 							buffer.WriteString(fmt.Sprintf("    mov [%s], rax\n", name))
// 							continue
// 						}
// 						if lhs.NodeType == nodeIntLiteral && rhs.NodeType == nodeIdentifier {
// 							l := castAssert[int](lhs.Literal.Int.Value)
// 							r := rhs.VarDecl.Name.Symbol
// 							buffer.WriteString(fmt.Sprintf("    mov rax, %d\n", l))
// 							buffer.WriteString(fmt.Sprintf("    add rax, [%s]\n", r))
// 							buffer.WriteString(fmt.Sprintf("    mov [%s], rax\n", name))
// 							continue
// 						}
// 						if lhs.NodeType == nodeIdentifier && rhs.NodeType == nodeIntLiteral {
// 							l := lhs.VarDecl.Name.Symbol
// 							r := castAssert[int](rhs.Literal.Int.Value)
// 							buffer.WriteString(fmt.Sprintf("    mov rax, [%s]\n", l))
// 							buffer.WriteString(fmt.Sprintf("    add rax, %d\n", r))
// 							buffer.WriteString(fmt.Sprintf("    mov [%s], rax\n", name))
// 							continue
// 						}
// 						if lhs.NodeType == nodeIdentifier && rhs.NodeType == nodeIdentifier {
// 							l := lhs.VarDecl.Name.Symbol
// 							r := rhs.VarDecl.Name.Symbol
// 							buffer.WriteString(fmt.Sprintf("    mov rax, [%s]\n", l))
// 							buffer.WriteString(fmt.Sprintf("    add rax, [%s]\n", r))
// 							buffer.WriteString(fmt.Sprintf("    mov [%s], rax\n", name))
// 							continue
// 						}
// 						continue
// 					}
//
// 					if node.VarDecl.Assignment.BinOp.Operation == opLessThan {
// 						buffer.WriteString(fmt.Sprintf("    ; %06d. less than\n", opid))
// 						if lhs.NodeType == nodeIntLiteral && rhs.NodeType == nodeIntLiteral {
// 							l := castAssert[int](lhs.Literal.Int.Value)
// 							r := castAssert[int](rhs.Literal.Int.Value)
// 							buffer.WriteString(fmt.Sprintf("    mov rax, %d\n", l))
// 							buffer.WriteString(fmt.Sprintf("    cmp rax, %d\n", r))
// 							buffer.WriteString("    setl r8b\n")
// 							buffer.WriteString(fmt.Sprintf("    mov [%s], r8\n", name))
// 							continue
// 						}
// 						if lhs.NodeType == nodeIntLiteral && rhs.NodeType == nodeIdentifier {
// 							l := castAssert[int](lhs.Literal.Int.Value)
// 							r := rhs.VarDecl.Name.Symbol
// 							buffer.WriteString(fmt.Sprintf("    mov rax, %d\n", l))
// 							buffer.WriteString(fmt.Sprintf("    cmp rax, [%s]\n", r))
// 							buffer.WriteString("    setl r8b\n")
// 							buffer.WriteString(fmt.Sprintf("    mov [%s], r8\n", name))
// 							continue
// 						}
// 						if lhs.NodeType == nodeIdentifier && rhs.NodeType == nodeIntLiteral {
// 							l := lhs.VarDecl.Name.Symbol
// 							r := castAssert[int](rhs.Literal.Int.Value)
// 							buffer.WriteString(fmt.Sprintf("    mov rax, [%s]\n", l))
// 							buffer.WriteString(fmt.Sprintf("    cmp rax, %d\n", r))
// 							buffer.WriteString("    setl r8b\n")
// 							buffer.WriteString(fmt.Sprintf("    mov [%s], r8\n", name))
// 							continue
// 						}
// 						if lhs.NodeType == nodeIdentifier && rhs.NodeType == nodeIdentifier {
// 							l := lhs.VarDecl.Name.Symbol
// 							r := rhs.VarDecl.Name.Symbol
// 							buffer.WriteString(fmt.Sprintf("    mov rax, [%s]\n", l))
// 							buffer.WriteString(fmt.Sprintf("    cmp rax, [%s]\n", r))
// 							buffer.WriteString("    setl r8b\n")
// 							buffer.WriteString(fmt.Sprintf("    mov [%s], r8\n", name))
// 							continue
// 						}
// 						continue
// 					}
// 				}
// 			}
// 			if node.VarDecl.Assignment.NodeType == nodeIdentifier {
// 				varName := node.VarDecl.Name.Symbol
// 				value := castAssert[string](node.VarDecl.Assignment.VarDecl.Name.Symbol)
// 				buffer.WriteString(fmt.Sprintf("    ; %06d. assign\n", opid))
// 				buffer.WriteString(fmt.Sprintf("    mov rax, [%s]\n", value))
// 				buffer.WriteString(fmt.Sprintf("    mov [%s], rax\n", varName))
// 				continue
// 			}
// 		}
//
// 		if node.NodeType == nodeExit {
// 			buffer.WriteString(fmt.Sprintf("    ; %06d. exit\n", opid))
// 			if node.Exit.Message.Literal.String.Value != "" {
// 				msg := castAssert[string](node.Exit.Message.Literal.String.Value)
//
// 				strName := fmt.Sprintf("str_%d", strCount)
// 				strSize := fmt.Sprintf("%s_size", strName)
// 				strCount++
//
// 				strBuf.WriteString(fmt.Sprintf("%s db \"%s\", 10\n", strName, msg))
// 				strBuf.WriteString(fmt.Sprintf("%s = $-%s\n", strSize, strName))
//
// 				buffer.WriteString("    mov rax, 1\n")
// 				buffer.WriteString("    mov rdi, 1\n")
// 				buffer.WriteString(fmt.Sprintf("    mov rsi, %s\n", strName))
// 				buffer.WriteString(fmt.Sprintf("    mov rdx, %s\n", strSize))
// 				buffer.WriteString("    syscall\n")
// 			}
//
// 			if node.Exit.Status.VarDecl.Name.Symbol != "" {
// 				buffer.WriteString("    mov rax, 60\n")
// 				buffer.WriteString(fmt.Sprintf("    mov rdi, [%s]\n", castAssert[string](node.Exit.Status.VarDecl.Name.Symbol)))
// 				buffer.WriteString("    syscall\n")
// 				buffer.WriteString("    ret\n")
// 			} else if status, ok := cast[int](node.Exit.Status.Literal.Int.Value); ok {
// 				buffer.WriteString("    mov rax, 60\n")
// 				buffer.WriteString(fmt.Sprintf("    mov rdi, %d\n", status))
// 				buffer.WriteString("    syscall\n")
// 				buffer.WriteString("    ret\n")
// 			}
// 			continue
// 		}
// 	}
//
// 	buffer.WriteString("\n")
// 	buffer.Write(procBuf.Bytes())
// 	buffer.WriteString("segment readable\n\n")
// 	buffer.Write(strBuf.Bytes())
//
// 	buffer.WriteString("\n")
// 	buffer.WriteString("segment readable writable\n")
// 	buffer.Write(datBuf.Bytes())
//
// 	return &buffer
// }
