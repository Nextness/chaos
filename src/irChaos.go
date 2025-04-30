package main

import (
	"bytes"
	"fmt"
)

// TODO: Improve how I'm handling IR in general. This code looks like shit...
func irChaos(ls *LexerState) *bytes.Buffer {
	buffer := bytes.Buffer{}

	irVarTypes := bytes.Buffer{}
	typeId := 0
	typePrefix := "typ"
	mapVars := map[string]string{}

	irSymbols := bytes.Buffer{}
	symbolsId := 0
	symbolsPrefix := "sym"
	mapSymbols := map[string]string{}

	irIdentifiers := bytes.Buffer{}
	identId := 0
	identPrefix := "var"
	mapIdent := map[string]string{}

	procTable := bytes.Buffer{}
	procId := 0
	procPrefix := "proc"
	mapProc := map[string]string{}

	argTable := bytes.Buffer{}
	argId := 0
	argPrefix := "arg"
	mapArg := map[string]string{}

	mainLoop := bytes.Buffer{}

	buffer.WriteString("chaosIntermediateRepresentation\n")
	irVarTypes.WriteString("\ntypes\n")
	irSymbols.WriteString("\nsymbols\n")
	irIdentifiers.WriteString("\nvars\n")
	mainLoop.WriteString("\nmain\n")

	// TODO: Better handle proc/builtins in IR
	procTable.WriteString("\nproc\n")
	procType := fmt.Sprintf("%s%d", procPrefix, procId)
	procTable.WriteString(fmt.Sprintf("    %s print\n", procType))
	mapProc["print"] = procType
	procId++

	argTable.WriteString("\nargs\n")

	for ls.cursor < ls.count {
		val := ls.data[ls.cursor]
		switch v := val.(type) {
		case *Node:
			if v.nodeType == identifierNode {
				irType := fmt.Sprintf("%s%d", typePrefix, typeId)
				if _, ok := mapVars[v.identifier.tpe]; !ok {
					mapVars[v.identifier.tpe] = irType
					irVarTypes.WriteString(fmt.Sprintf("    %s %s\n", irType, v.identifier.tpe))
					typeId++
				}

				irIdent := fmt.Sprintf("%s%d", identPrefix, identId)
				if val, ok := mapIdent[v.identifier.val]; ok {
					// already existing in vars
					mapIdent[v.identifier.sym] = val
					irIdentifiers.WriteString(fmt.Sprintf("    %s %s %s\n", irIdent, mapVars[v.identifier.tpe], val))
				} else if _, ok := mapIdent[v.identifier.sym]; !ok {
					mapIdent[v.identifier.sym] = irIdent
					symbolFmt := fmt.Sprintf("%s%d", symbolsPrefix, symbolsId)
					mapSymbols[v.identifier.sym] = symbolFmt
					// non existing variables
					var formattedStr string
					if v.identifier.stt != "not-initialized" {
						formattedStr = fmt.Sprintf("    %s %s %s\n", irIdent, mapVars[v.identifier.tpe], v.identifier.val)
						irSymbols.WriteString(fmt.Sprintf("    %s %s %s\n", symbolFmt, irIdent, v.identifier.sym))
						symbolsId++
					} else {
						formattedStr = fmt.Sprintf("    %s %s\n", irIdent, mapVars[v.identifier.tpe])
					}
					irIdentifiers.WriteString(formattedStr)
				}

				identId++
				ls.cursor++
				continue
			}
		case *NodeExitWith:
			if v.msg != "" {
				prnt := mapProc["print"]
				assert(prnt != "", "Expected value for print intrinsic")
				arg, ok := mapArg[v.msg]
				if !ok {
					a := fmt.Sprintf("%s%d", argPrefix, argId)
					mapArg[v.msg] = a
					arg = a
					argId++

					argVal := fmt.Sprintf("    %s %s '%s'\n", prnt, a, v.msg)
					argTable.WriteString(argVal)
				}

				out := fmt.Sprintf("    call %s %s\n", prnt, arg)
				mainLoop.WriteString(out)
			}
			var out string
			out = fmt.Sprintf("    call exit %d\n", v.val)
			if v.sym != "" {
				out = fmt.Sprintf("    call exit %s\n", v.sym)
			}
			mainLoop.WriteString(out)
			ls.cursor++
			continue
		default:
			fmt.Printf("IR Not implemented for %T\n", v)
			panic("not implemented")
		}
	}

	buffer.Write(procTable.Bytes())
	buffer.WriteString(irSymbols.String())
	buffer.Write(argTable.Bytes())
	buffer.Write(irVarTypes.Bytes())
	buffer.Write(irIdentifiers.Bytes())
	buffer.WriteString(mainLoop.String())
	return &buffer
}
