// Code generated from TsvsheetParser.g4 by ANTLR 4.13.2. DO NOT EDIT.

package tsvsheetgrammar // TsvsheetParser
import "github.com/antlr4-go/antlr/v4"

// A complete Visitor for a parse tree produced by TsvsheetParser.
type TsvsheetParserVisitor interface {
	antlr.ParseTreeVisitor

	// Visit a parse tree produced by TsvsheetParser#errorExpr.
	VisitErrorExpr(ctx *ErrorExprContext) interface{}

	// Visit a parse tree produced by TsvsheetParser#numberExpr.
	VisitNumberExpr(ctx *NumberExprContext) interface{}

	// Visit a parse tree produced by TsvsheetParser#parenExpr.
	VisitParenExpr(ctx *ParenExprContext) interface{}

	// Visit a parse tree produced by TsvsheetParser#concatExpr.
	VisitConcatExpr(ctx *ConcatExprContext) interface{}

	// Visit a parse tree produced by TsvsheetParser#stringExpr.
	VisitStringExpr(ctx *StringExprContext) interface{}

	// Visit a parse tree produced by TsvsheetParser#unaryExpr.
	VisitUnaryExpr(ctx *UnaryExprContext) interface{}

	// Visit a parse tree produced by TsvsheetParser#addExpr.
	VisitAddExpr(ctx *AddExprContext) interface{}

	// Visit a parse tree produced by TsvsheetParser#refExpr.
	VisitRefExpr(ctx *RefExprContext) interface{}

	// Visit a parse tree produced by TsvsheetParser#mulExpr.
	VisitMulExpr(ctx *MulExprContext) interface{}

	// Visit a parse tree produced by TsvsheetParser#percentExpr.
	VisitPercentExpr(ctx *PercentExprContext) interface{}

	// Visit a parse tree produced by TsvsheetParser#callExpr.
	VisitCallExpr(ctx *CallExprContext) interface{}

	// Visit a parse tree produced by TsvsheetParser#boolExpr.
	VisitBoolExpr(ctx *BoolExprContext) interface{}

	// Visit a parse tree produced by TsvsheetParser#powExpr.
	VisitPowExpr(ctx *PowExprContext) interface{}

	// Visit a parse tree produced by TsvsheetParser#compareExpr.
	VisitCompareExpr(ctx *CompareExprContext) interface{}

	// Visit a parse tree produced by TsvsheetParser#functionCall.
	VisitFunctionCall(ctx *FunctionCallContext) interface{}

	// Visit a parse tree produced by TsvsheetParser#argList.
	VisitArgList(ctx *ArgListContext) interface{}

	// Visit a parse tree produced by TsvsheetParser#reference.
	VisitReference(ctx *ReferenceContext) interface{}

	// Visit a parse tree produced by TsvsheetParser#sheetQualifier.
	VisitSheetQualifier(ctx *SheetQualifierContext) interface{}

	// Visit a parse tree produced by TsvsheetParser#cellRef.
	VisitCellRef(ctx *CellRefContext) interface{}
}
