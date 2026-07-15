// Code generated from TsvsheetParser.g4 by ANTLR 4.13.2. DO NOT EDIT.

package tsvsheetgrammar // TsvsheetParser
import "github.com/antlr4-go/antlr/v4"

// TsvsheetParserListener is a complete listener for a parse tree produced by TsvsheetParser.
type TsvsheetParserListener interface {
	antlr.ParseTreeListener

	// EnterErrorExpr is called when entering the errorExpr production.
	EnterErrorExpr(c *ErrorExprContext)

	// EnterNumberExpr is called when entering the numberExpr production.
	EnterNumberExpr(c *NumberExprContext)

	// EnterParenExpr is called when entering the parenExpr production.
	EnterParenExpr(c *ParenExprContext)

	// EnterConcatExpr is called when entering the concatExpr production.
	EnterConcatExpr(c *ConcatExprContext)

	// EnterStringExpr is called when entering the stringExpr production.
	EnterStringExpr(c *StringExprContext)

	// EnterUnaryExpr is called when entering the unaryExpr production.
	EnterUnaryExpr(c *UnaryExprContext)

	// EnterAddExpr is called when entering the addExpr production.
	EnterAddExpr(c *AddExprContext)

	// EnterRefExpr is called when entering the refExpr production.
	EnterRefExpr(c *RefExprContext)

	// EnterMulExpr is called when entering the mulExpr production.
	EnterMulExpr(c *MulExprContext)

	// EnterPercentExpr is called when entering the percentExpr production.
	EnterPercentExpr(c *PercentExprContext)

	// EnterCallExpr is called when entering the callExpr production.
	EnterCallExpr(c *CallExprContext)

	// EnterBoolExpr is called when entering the boolExpr production.
	EnterBoolExpr(c *BoolExprContext)

	// EnterPowExpr is called when entering the powExpr production.
	EnterPowExpr(c *PowExprContext)

	// EnterCompareExpr is called when entering the compareExpr production.
	EnterCompareExpr(c *CompareExprContext)

	// EnterFunctionCall is called when entering the functionCall production.
	EnterFunctionCall(c *FunctionCallContext)

	// EnterArgList is called when entering the argList production.
	EnterArgList(c *ArgListContext)

	// EnterReference is called when entering the reference production.
	EnterReference(c *ReferenceContext)

	// EnterSheetQualifier is called when entering the sheetQualifier production.
	EnterSheetQualifier(c *SheetQualifierContext)

	// EnterCellRef is called when entering the cellRef production.
	EnterCellRef(c *CellRefContext)

	// ExitErrorExpr is called when exiting the errorExpr production.
	ExitErrorExpr(c *ErrorExprContext)

	// ExitNumberExpr is called when exiting the numberExpr production.
	ExitNumberExpr(c *NumberExprContext)

	// ExitParenExpr is called when exiting the parenExpr production.
	ExitParenExpr(c *ParenExprContext)

	// ExitConcatExpr is called when exiting the concatExpr production.
	ExitConcatExpr(c *ConcatExprContext)

	// ExitStringExpr is called when exiting the stringExpr production.
	ExitStringExpr(c *StringExprContext)

	// ExitUnaryExpr is called when exiting the unaryExpr production.
	ExitUnaryExpr(c *UnaryExprContext)

	// ExitAddExpr is called when exiting the addExpr production.
	ExitAddExpr(c *AddExprContext)

	// ExitRefExpr is called when exiting the refExpr production.
	ExitRefExpr(c *RefExprContext)

	// ExitMulExpr is called when exiting the mulExpr production.
	ExitMulExpr(c *MulExprContext)

	// ExitPercentExpr is called when exiting the percentExpr production.
	ExitPercentExpr(c *PercentExprContext)

	// ExitCallExpr is called when exiting the callExpr production.
	ExitCallExpr(c *CallExprContext)

	// ExitBoolExpr is called when exiting the boolExpr production.
	ExitBoolExpr(c *BoolExprContext)

	// ExitPowExpr is called when exiting the powExpr production.
	ExitPowExpr(c *PowExprContext)

	// ExitCompareExpr is called when exiting the compareExpr production.
	ExitCompareExpr(c *CompareExprContext)

	// ExitFunctionCall is called when exiting the functionCall production.
	ExitFunctionCall(c *FunctionCallContext)

	// ExitArgList is called when exiting the argList production.
	ExitArgList(c *ArgListContext)

	// ExitReference is called when exiting the reference production.
	ExitReference(c *ReferenceContext)

	// ExitSheetQualifier is called when exiting the sheetQualifier production.
	ExitSheetQualifier(c *SheetQualifierContext)

	// ExitCellRef is called when exiting the cellRef production.
	ExitCellRef(c *CellRefContext)
}
