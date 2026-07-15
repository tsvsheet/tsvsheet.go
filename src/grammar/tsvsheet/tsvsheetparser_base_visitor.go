// Code generated from TsvsheetParser.g4 by ANTLR 4.13.2. DO NOT EDIT.

package tsvsheetgrammar // TsvsheetParser
import "github.com/antlr4-go/antlr/v4"

type BaseTsvsheetParserVisitor struct {
	*antlr.BaseParseTreeVisitor
}

func (v *BaseTsvsheetParserVisitor) VisitErrorExpr(ctx *ErrorExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseTsvsheetParserVisitor) VisitNumberExpr(ctx *NumberExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseTsvsheetParserVisitor) VisitParenExpr(ctx *ParenExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseTsvsheetParserVisitor) VisitConcatExpr(ctx *ConcatExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseTsvsheetParserVisitor) VisitStringExpr(ctx *StringExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseTsvsheetParserVisitor) VisitUnaryExpr(ctx *UnaryExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseTsvsheetParserVisitor) VisitAddExpr(ctx *AddExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseTsvsheetParserVisitor) VisitRefExpr(ctx *RefExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseTsvsheetParserVisitor) VisitMulExpr(ctx *MulExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseTsvsheetParserVisitor) VisitPercentExpr(ctx *PercentExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseTsvsheetParserVisitor) VisitCallExpr(ctx *CallExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseTsvsheetParserVisitor) VisitBoolExpr(ctx *BoolExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseTsvsheetParserVisitor) VisitPowExpr(ctx *PowExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseTsvsheetParserVisitor) VisitCompareExpr(ctx *CompareExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseTsvsheetParserVisitor) VisitFunctionCall(ctx *FunctionCallContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseTsvsheetParserVisitor) VisitArgList(ctx *ArgListContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseTsvsheetParserVisitor) VisitReference(ctx *ReferenceContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseTsvsheetParserVisitor) VisitSheetQualifier(ctx *SheetQualifierContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseTsvsheetParserVisitor) VisitCellRef(ctx *CellRefContext) interface{} {
	return v.VisitChildren(ctx)
}
