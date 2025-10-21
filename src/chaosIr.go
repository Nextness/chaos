package main

type OpType int

const (
	irNull OpType = iota
	irNoOp
	// TO-DO: exit should probably be removed as an intrinsic and be just a function
	// with comp time logic to decide OS.
	irExit
	irOpCall
	irAutoVar
	irProcDef
)

type IROp struct {
	OpType      OpType
	Symbol      string
	VarType     string
	StringValue string
	IntValue    int
	FloatValue  float64
	IdentValue  string
	ProcInputs  []IROp
	ProcExec    []IROp
	ExitMsg     *IROp
	ExitStatus  *IROp
}

func ProgramToIR(node *Node) IROp {

	if node.NodeType == nodeNoOp {
		return IROp{OpType: irNoOp}
	}

	if node.NodeType == nodeCall {
		symbol := node.Call.Name.Symbol
		return IROp{
			OpType: irOpCall,
			Symbol: symbol,
		}
	}

	if node.NodeType == nodeIdentifier {
		symbol := node.VarDecl.Name.Symbol
		varType := node.VarDecl.Type.Symbol

		if node.VarDecl.Assignment.NodeType == nodeProcDef {
			irOp := IROp{
				Symbol: symbol,
				OpType: irProcDef,
			}

			if inputLen := len(node.VarDecl.Assignment.Proc.Inputs); inputLen > 0 {
				var procInputs []IROp
				for _, n := range node.VarDecl.Assignment.Proc.Inputs {
					op := ProgramToIR(&n)
					if op.OpType != irNoOp {
						procInputs = append(procInputs, op)
					}
				}
				irOp.ProcInputs = procInputs
			}

			var procScope []IROp
			for _, n := range node.VarDecl.Assignment.Proc.Scope {
				op := ProgramToIR(&n)
				procScope = append(procScope, op)
			}
			irOp.ProcExec = procScope
			return irOp
		}

		if !node.VarDecl.Initialized {
			return IROp{
				OpType:  irAutoVar,
				Symbol:  symbol,
				VarType: varType,
			}
		}

		if node.VarDecl.Assignment.NodeType == nodeIntLiteral {
			intValue := castAssert[int](node.VarDecl.Assignment.Literal.Int.Value)
			return IROp{
				OpType:   irAutoVar,
				Symbol:   symbol,
				VarType:  varType,
				IntValue: intValue,
			}
		}

		if node.VarDecl.Assignment.NodeType == nodeFloatLiteral {
			// TO-DO: create a new field for Float instead of using Int
			floatValue := castAssert[float64](node.VarDecl.Assignment.Literal.Int.Value)
			return IROp{
				OpType:     irAutoVar,
				Symbol:     symbol,
				VarType:    varType,
				FloatValue: floatValue,
			}
		}

		if node.VarDecl.Assignment.NodeType == nodeStringLiteral {
			stringValue := castAssert[string](node.VarDecl.Assignment.Literal.String.Value)
			return IROp{
				OpType:      irAutoVar,
				Symbol:      symbol,
				VarType:     varType,
				StringValue: stringValue,
			}
		}

		if node.VarDecl.Assignment.NodeType == nodeIdentifier {
			identValue := node.VarDecl.Assignment.VarDecl.Name.Symbol
			return IROp{
				OpType:     irAutoVar,
				Symbol:     symbol,
				VarType:    varType,
				IdentValue: identValue,
			}
		}
	}

	if node.NodeType == nodeExit {
		exitMsg := IROp{
			OpType:      irAutoVar,
			VarType:     "String",
			StringValue: "",
		}
		if node.Exit.Message.NodeType == nodeIdentifier {
			exitMsg.IdentValue = node.Exit.Message.VarDecl.Name.Symbol
		} else if node.Exit.Message.NodeType == nodeStringLiteral {
			exitMsg.StringValue = castAssert[string](node.Exit.Message.Literal.String.Value)
		}

		exitSts := IROp{
			OpType:   irAutoVar,
			VarType:  "S64",
			IntValue: 0,
		}
		if node.Exit.Status.NodeType == nodeIdentifier {
			exitMsg.IdentValue = node.Exit.Status.VarDecl.Name.Symbol
		} else if node.Exit.Message.NodeType == nodeIntLiteral {
			exitMsg.IntValue = castAssert[int](node.Exit.Message.Literal.Int.Value)
		}

		return IROp{
			OpType:     irExit,
			ExitMsg:    &exitMsg,
			ExitStatus: &exitSts,
		}
	}

	return assert[IROp](false, "unrecheable")
}
