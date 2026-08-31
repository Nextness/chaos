package compiler

// FormatTypeExpr renders a parsed type expression using Chaos source syntax.
// It is shared by compiler-facing dumps and editor tooling so symbolic array
// sizes and nested pointer/array forms are represented consistently.
func FormatTypeExpr(expr Expr) string {
	switch value := expr.(type) {
	case *IdentExpr:
		return value.Name
	case *ArrayTypeExpr:
		prefix := "[]"
		switch value.Kind {
		case ArrayFixed:
			prefix = "[" + formatSyntaxExpr(value.Size) + "]"
		case ArrayDynamic:
			prefix = "[dyn]"
		}
		return prefix + FormatTypeExpr(value.Elem)
	case *PointerTypeExpr:
		text := "*" + FormatTypeExpr(value.Elem)
		if value.Nullable {
			text += "?"
		}
		return text
	case *ParenExpr:
		return "(" + FormatTypeExpr(value.Inner) + ")"
	}
	return ""
}

func formatSyntaxExpr(expr Expr) string {
	switch value := expr.(type) {
	case *IdentExpr:
		return value.Name
	case *IntExpr:
		return value.Value
	case *UnaryExpr:
		return value.Op.String() + formatSyntaxExpr(value.Operand)
	case *BinaryExpr:
		return formatSyntaxExpr(value.Left) + " " + value.Op.String() + " " + formatSyntaxExpr(value.Right)
	case *ParenExpr:
		return "(" + formatSyntaxExpr(value.Inner) + ")"
	}
	return "?"
}
