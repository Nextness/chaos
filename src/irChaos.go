package main

import (
	"bytes"
	"fmt"
)

func irChaos(ls *LexerState) *bytes.Buffer {
	buffer := bytes.Buffer{}

	irVarTypes := bytes.Buffer{}
	typeId := 0
	typePrefix := "typ"
	mapVars := map[string]string{}

	irIdentifiers := bytes.Buffer{}
	identId := 0
	identPrefix := "var"
	mapIdent := map[string]string{}

	buffer.WriteString("chaosIntermediateRepresentation\n")
	irVarTypes.WriteString("\ntypes\n")
	irIdentifiers.WriteString("\nvars\n")
	for ls.cursor < ls.count {
		val := ls.data[ls.cursor]
		switch v := val.(type) {
		default:
			fmt.Printf("IR Not implemented for %T\n", v)
			ls.cursor++
		case *Node:
			if v.nodeType == identifierNode {
				irType := fmt.Sprintf("%s%d", typePrefix, typeId)
				if _, ok := mapVars[v.identifier.tpe]; !ok {
					mapVars[v.identifier.tpe] = irType
					irVarTypes.WriteString(fmt.Sprintf("    %s = %s\n", irType, v.identifier.tpe))
					typeId++
				}

				irIdent := fmt.Sprintf("%s%d", identPrefix, identId)
				if val, ok := mapIdent[v.identifier.val]; ok {
					mapIdent[v.identifier.sym] = val
					irIdentifiers.WriteString(fmt.Sprintf("    (c) %s %s = %s\n", irIdent, mapVars[v.identifier.tpe], val))
				} else if _, ok := mapIdent[v.identifier.sym]; !ok {
					mapIdent[v.identifier.sym] = irIdent
					if v.identifier.stt != "not-initialized" {
						irIdentifiers.WriteString(fmt.Sprintf("    (p) %s %s = %s\n", irIdent, mapVars[v.identifier.tpe], v.identifier.val))
					} else {
						irIdentifiers.WriteString(fmt.Sprintf("    (p) %s %s\n", irIdent, mapVars[v.identifier.tpe]))
					}
				}

				identId++
				ls.cursor++
				continue
			}
		}
	}

	buffer.Write(irVarTypes.Bytes())
	buffer.Write(irIdentifiers.Bytes())
	return &buffer
}
