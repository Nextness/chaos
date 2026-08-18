package compiler

import (
	"math/big"
	"strconv"
	"strings"
)

// foldConstant evaluates scalar HIR made exclusively from constants. Integer
// text is parsed with big.Int so folding never loses precision before target
// encoding. Aggregate constructors remain constructors but their children are
// folded recursively; materialization happens only when a runtime value is
// required.
func (l *Lowerer) foldConstant(expr HIRExpr) HIRExpr {
	switch n := expr.(type) {
	case *HIRUnary:
		operand := l.foldConstant(n.Operand)
		constant, ok := operand.(*HIRConst)
		if !ok {
			copy := *n
			copy.Operand = operand
			return &copy
		}
		switch n.Op {
		case UnaryOpNeg:
			switch constant.Kind {
			case ConstInt:
				value, ok := exactHIRInteger(constant)
				if ok {
					value.Neg(value)
					return &HIRConst{Span_: n.Span_, Type: n.Type, Kind: ConstInt, Int: value.Int64(), Str: value.String()}
				}
			case ConstFloat:
				value := -constant.Float
				return &HIRConst{Span_: n.Span_, Type: n.Type, Kind: ConstFloat, Float: value, Str: strconv.FormatFloat(value, 'g', -1, 64)}
			}
		case UnaryOpNot:
			if constant.Kind == ConstBool {
				return &HIRConst{Span_: n.Span_, Type: n.Type, Kind: ConstBool, Bool: !constant.Bool}
			}
		}
		copy := *n
		copy.Operand = operand
		return &copy

	case *HIRBinary:
		left := l.foldConstant(n.Left)
		right := l.foldConstant(n.Right)
		leftConst, leftOK := left.(*HIRConst)
		rightConst, rightOK := right.(*HIRConst)
		if leftOK && rightOK {
			if folded := l.foldBinaryConstants(n, leftConst, rightConst); folded != nil {
				return folded
			}
		}
		copy := *n
		copy.Left, copy.Right = left, right
		return &copy

	case *HIRStructInit:
		copy := *n
		copy.Fields = append([]HIRStructInitField(nil), n.Fields...)
		for i := range copy.Fields {
			copy.Fields[i].Value = l.foldConstant(copy.Fields[i].Value)
		}
		return &copy

	case *HIRArrayInit:
		copy := *n
		copy.Items = append([]HIRExpr(nil), n.Items...)
		for i := range copy.Items {
			copy.Items[i] = l.foldConstant(copy.Items[i])
		}
		return &copy

	case *HIRArrayLen:
		array := l.foldConstant(n.Array)
		if init, ok := array.(*HIRArrayInit); ok {
			return &HIRConst{Span_: n.Span_, Type: n.Type, Kind: ConstInt, Int: int64(len(init.Items)), Str: strconv.Itoa(len(init.Items))}
		}
		copy := *n
		copy.Array = array
		return &copy

	case *HIRIndex:
		base := l.foldConstant(n.Base)
		index := l.foldConstant(n.Index)
		if init, ok := base.(*HIRArrayInit); ok {
			if constant, ok := index.(*HIRConst); ok {
				if value, ok := exactHIRInteger(constant); ok && value.IsInt64() {
					i := value.Int64()
					if i >= 0 && i < int64(len(init.Items)) {
						return init.Items[i]
					}
				}
			}
		}
		copy := *n
		copy.Base, copy.Index = base, index
		return &copy

	case *HIRFieldLoad:
		base := l.foldConstant(n.Base)
		if init, ok := base.(*HIRStructInit); ok && n.Field >= 0 && n.Field < len(init.Fields) {
			return init.Fields[n.Field].Value
		}
		copy := *n
		copy.Base = base
		return &copy
	}
	return expr
}

func exactHIRInteger(constant *HIRConst) (*big.Int, bool) {
	text := constant.Str
	if text == "" {
		text = strconv.FormatInt(constant.Int, 10)
	}
	value, ok := new(big.Int).SetString(strings.ReplaceAll(text, "_", ""), 10)
	return value, ok
}

func (l *Lowerer) foldBinaryConstants(binary *HIRBinary, left, right *HIRConst) HIRExpr {
	if left.Kind == ConstInt && right.Kind == ConstInt {
		a, aOK := exactHIRInteger(left)
		b, bOK := exactHIRInteger(right)
		if !aOK || !bOK {
			return nil
		}
		value := new(big.Int)
		switch binary.Op {
		case BinaryOpAdd:
			value.Add(a, b)
		case BinaryOpSub:
			value.Sub(a, b)
		case BinaryOpMul:
			value.Mul(a, b)
		case BinaryOpDiv:
			if b.Sign() == 0 {
				return nil
			}
			value.Quo(a, b)
		case BinaryOpMod:
			if b.Sign() == 0 {
				return nil
			}
			value.Rem(a, b)
		case BinaryOpLt:
			return l.foldedBool(binary, a.Cmp(b) < 0)
		case BinaryOpGt:
			return l.foldedBool(binary, a.Cmp(b) > 0)
		case BinaryOpLe:
			return l.foldedBool(binary, a.Cmp(b) <= 0)
		case BinaryOpGe:
			return l.foldedBool(binary, a.Cmp(b) >= 0)
		case BinaryOpEq:
			return l.foldedBool(binary, a.Cmp(b) == 0)
		case BinaryOpNeq:
			return l.foldedBool(binary, a.Cmp(b) != 0)
		default:
			return nil
		}
		return &HIRConst{Span_: binary.Span_, Type: binary.Type, Kind: ConstInt, Int: value.Int64(), Str: value.String()}
	}

	if left.Kind == ConstFloat && right.Kind == ConstFloat {
		var value float64
		switch binary.Op {
		case BinaryOpAdd:
			value = left.Float + right.Float
		case BinaryOpSub:
			value = left.Float - right.Float
		case BinaryOpMul:
			value = left.Float * right.Float
		case BinaryOpDiv:
			if right.Float == 0 {
				return nil
			}
			value = left.Float / right.Float
		case BinaryOpLt:
			return l.foldedBool(binary, left.Float < right.Float)
		case BinaryOpGt:
			return l.foldedBool(binary, left.Float > right.Float)
		case BinaryOpLe:
			return l.foldedBool(binary, left.Float <= right.Float)
		case BinaryOpGe:
			return l.foldedBool(binary, left.Float >= right.Float)
		case BinaryOpEq:
			return l.foldedBool(binary, left.Float == right.Float)
		case BinaryOpNeq:
			return l.foldedBool(binary, left.Float != right.Float)
		default:
			return nil
		}
		if l.types.Lookup(binary.Type).Name == "F32" {
			value = float64(float32(value))
		}
		return &HIRConst{Span_: binary.Span_, Type: binary.Type, Kind: ConstFloat, Float: value, Str: strconv.FormatFloat(value, 'g', -1, 64)}
	}

	if left.Kind == ConstBool && right.Kind == ConstBool {
		switch binary.Op {
		case BinaryOpAnd:
			return l.foldedBool(binary, left.Bool && right.Bool)
		case BinaryOpOr:
			return l.foldedBool(binary, left.Bool || right.Bool)
		case BinaryOpEq:
			return l.foldedBool(binary, left.Bool == right.Bool)
		case BinaryOpNeq:
			return l.foldedBool(binary, left.Bool != right.Bool)
		}
	}

	if left.Kind == ConstString && right.Kind == ConstString {
		switch binary.Op {
		case BinaryOpLt:
			return l.foldedBool(binary, left.Str < right.Str)
		case BinaryOpGt:
			return l.foldedBool(binary, left.Str > right.Str)
		case BinaryOpLe:
			return l.foldedBool(binary, left.Str <= right.Str)
		case BinaryOpGe:
			return l.foldedBool(binary, left.Str >= right.Str)
		case BinaryOpEq:
			return l.foldedBool(binary, left.Str == right.Str)
		case BinaryOpNeq:
			return l.foldedBool(binary, left.Str != right.Str)
		}
	}

	if left.Kind == ConstError && right.Kind == ConstError {
		switch binary.Op {
		case BinaryOpEq:
			return l.foldedBool(binary, left.Int == right.Int)
		case BinaryOpNeq:
			return l.foldedBool(binary, left.Int != right.Int)
		}
	}
	return nil
}

func (l *Lowerer) foldedBool(binary *HIRBinary, value bool) HIRExpr {
	return &HIRConst{Span_: binary.Span_, Type: l.types.Bool(), Kind: ConstBool, Bool: value}
}
