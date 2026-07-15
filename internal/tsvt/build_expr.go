package tsvt

import (
	grammar "github.com/uplang/tsvsheet.go/src/grammar/tsvsheet"
)

// buildExpr converts one expression parse node into the typed Expr AST. The
// operator and reference forms are handled here; the leaf literals fall through
// to buildLeaf.
func buildExpr(ctx grammar.IExpressionContext) (Expr, error) {
	switch c := ctx.(type) {
	case *grammar.ParenExprContext:
		return buildExpr(c.Expression())
	case *grammar.PercentExprContext:
		return buildPercent(c)
	case *grammar.PowExprContext:
		return buildBinary(OpPow, c.AllExpression())
	case *grammar.UnaryExprContext:
		return buildUnary(c)
	case *grammar.MulExprContext:
		return buildBinary(BinaryOp(c.GetOp().GetText()), c.AllExpression())
	case *grammar.AddExprContext:
		return buildBinary(BinaryOp(c.GetOp().GetText()), c.AllExpression())
	case *grammar.ConcatExprContext:
		return buildBinary(OpCat, c.AllExpression())
	case *grammar.CompareExprContext:
		return buildBinary(BinaryOp(c.GetOp().GetText()), c.AllExpression())
	case *grammar.CallExprContext:
		return buildCall(c.FunctionCall())
	case *grammar.RefExprContext:
		return buildRefOperand(c)
	default:
		return buildLeaf(ctx), nil
	}
}

// buildLeaf builds a literal-leaf expression (number, string, boolean, error).
func buildLeaf(ctx grammar.IExpressionContext) Expr {
	switch c := ctx.(type) {
	case *grammar.NumberExprContext:
		return Number{Text: c.NUMBER().GetText()}
	case *grammar.StringExprContext:
		return StringLit{Value: unquote(quoted(c.STRING().GetText()))}
	case *grammar.BoolExprContext:
		return BoolLit{IsTrue: c.TRUE() != nil}
	default: // *grammar.ErrorExprContext
		return ErrorLit{Code: ctx.(*grammar.ErrorExprContext).ERRORCONST().GetText()}
	}
}

// buildBinary builds a binary operation from an operator and its two operands.
func buildBinary(op BinaryOp, operands []grammar.IExpressionContext) (Expr, error) {
	left, err := buildExpr(operands[0])
	if err != nil {
		return nil, err
	}
	right, err := buildExpr(operands[1])
	if err != nil {
		return nil, err
	}
	return Binary{Left: left, Right: right, Op: op}, nil
}

// buildUnary builds a unary sign operation.
func buildUnary(ctx *grammar.UnaryExprContext) (Expr, error) {
	x, err := buildExpr(ctx.Expression())
	if err != nil {
		return nil, err
	}
	op := OpNeg
	if ctx.PLUS() != nil {
		op = OpPos
	}
	return Unary{X: x, Op: op}, nil
}

// buildPercent builds a postfix-percent operation.
func buildPercent(ctx *grammar.PercentExprContext) (Expr, error) {
	x, err := buildExpr(ctx.Expression())
	if err != nil {
		return nil, err
	}
	return Percent{X: x}, nil
}

// buildRefOperand wraps an A1 reference as an expression operand.
func buildRefOperand(ctx *grammar.RefExprContext) (Expr, error) {
	ref, err := buildReference(ctx.Reference())
	if err != nil {
		return nil, err
	}
	return RefOperand{Ref: ref}, nil
}

// buildCall builds a function call, evaluating its argument expressions.
func buildCall(ctx grammar.IFunctionCallContext) (Expr, error) {
	args, err := buildArgs(ctx.ArgList())
	if err != nil {
		return nil, err
	}
	return Call{Name: callName(ctx), Args: args}, nil
}

// callName is the function's case-preserved name (a NAME or an all-caps COL).
func callName(ctx grammar.IFunctionCallContext) string {
	if name := ctx.NAME(); name != nil {
		return name.GetText()
	}
	return ctx.COL().GetText()
}

// buildArgs builds each argument expression; a nil arg list is no arguments.
func buildArgs(ctx grammar.IArgListContext) ([]Expr, error) {
	if ctx == nil {
		return nil, nil
	}
	exprs := ctx.AllExpression()
	args := make([]Expr, len(exprs))
	for i, e := range exprs {
		arg, err := buildExpr(e)
		if err != nil {
			return nil, err
		}
		args[i] = arg
	}
	return args, nil
}
