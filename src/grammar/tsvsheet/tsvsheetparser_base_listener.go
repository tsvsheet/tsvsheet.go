// Code generated from TsvsheetParser.g4 by ANTLR 4.13.2. DO NOT EDIT.

package tsvsheetgrammar // TsvsheetParser
import "github.com/antlr4-go/antlr/v4"

// BaseTsvsheetParserListener is a complete listener for a parse tree produced by TsvsheetParser.
type BaseTsvsheetParserListener struct{}

var _ TsvsheetParserListener = &BaseTsvsheetParserListener{}

// VisitTerminal is called when a terminal node is visited.
func (s *BaseTsvsheetParserListener) VisitTerminal(node antlr.TerminalNode) {}

// VisitErrorNode is called when an error node is visited.
func (s *BaseTsvsheetParserListener) VisitErrorNode(node antlr.ErrorNode) {}

// EnterEveryRule is called when any rule is entered.
func (s *BaseTsvsheetParserListener) EnterEveryRule(ctx antlr.ParserRuleContext) {}

// ExitEveryRule is called when any rule is exited.
func (s *BaseTsvsheetParserListener) ExitEveryRule(ctx antlr.ParserRuleContext) {}

// EnterErrorExpr is called when production errorExpr is entered.
func (s *BaseTsvsheetParserListener) EnterErrorExpr(ctx *ErrorExprContext) {}

// ExitErrorExpr is called when production errorExpr is exited.
func (s *BaseTsvsheetParserListener) ExitErrorExpr(ctx *ErrorExprContext) {}

// EnterNumberExpr is called when production numberExpr is entered.
func (s *BaseTsvsheetParserListener) EnterNumberExpr(ctx *NumberExprContext) {}

// ExitNumberExpr is called when production numberExpr is exited.
func (s *BaseTsvsheetParserListener) ExitNumberExpr(ctx *NumberExprContext) {}

// EnterParenExpr is called when production parenExpr is entered.
func (s *BaseTsvsheetParserListener) EnterParenExpr(ctx *ParenExprContext) {}

// ExitParenExpr is called when production parenExpr is exited.
func (s *BaseTsvsheetParserListener) ExitParenExpr(ctx *ParenExprContext) {}

// EnterConcatExpr is called when production concatExpr is entered.
func (s *BaseTsvsheetParserListener) EnterConcatExpr(ctx *ConcatExprContext) {}

// ExitConcatExpr is called when production concatExpr is exited.
func (s *BaseTsvsheetParserListener) ExitConcatExpr(ctx *ConcatExprContext) {}

// EnterStringExpr is called when production stringExpr is entered.
func (s *BaseTsvsheetParserListener) EnterStringExpr(ctx *StringExprContext) {}

// ExitStringExpr is called when production stringExpr is exited.
func (s *BaseTsvsheetParserListener) ExitStringExpr(ctx *StringExprContext) {}

// EnterUnaryExpr is called when production unaryExpr is entered.
func (s *BaseTsvsheetParserListener) EnterUnaryExpr(ctx *UnaryExprContext) {}

// ExitUnaryExpr is called when production unaryExpr is exited.
func (s *BaseTsvsheetParserListener) ExitUnaryExpr(ctx *UnaryExprContext) {}

// EnterAddExpr is called when production addExpr is entered.
func (s *BaseTsvsheetParserListener) EnterAddExpr(ctx *AddExprContext) {}

// ExitAddExpr is called when production addExpr is exited.
func (s *BaseTsvsheetParserListener) ExitAddExpr(ctx *AddExprContext) {}

// EnterRefExpr is called when production refExpr is entered.
func (s *BaseTsvsheetParserListener) EnterRefExpr(ctx *RefExprContext) {}

// ExitRefExpr is called when production refExpr is exited.
func (s *BaseTsvsheetParserListener) ExitRefExpr(ctx *RefExprContext) {}

// EnterMulExpr is called when production mulExpr is entered.
func (s *BaseTsvsheetParserListener) EnterMulExpr(ctx *MulExprContext) {}

// ExitMulExpr is called when production mulExpr is exited.
func (s *BaseTsvsheetParserListener) ExitMulExpr(ctx *MulExprContext) {}

// EnterPercentExpr is called when production percentExpr is entered.
func (s *BaseTsvsheetParserListener) EnterPercentExpr(ctx *PercentExprContext) {}

// ExitPercentExpr is called when production percentExpr is exited.
func (s *BaseTsvsheetParserListener) ExitPercentExpr(ctx *PercentExprContext) {}

// EnterCallExpr is called when production callExpr is entered.
func (s *BaseTsvsheetParserListener) EnterCallExpr(ctx *CallExprContext) {}

// ExitCallExpr is called when production callExpr is exited.
func (s *BaseTsvsheetParserListener) ExitCallExpr(ctx *CallExprContext) {}

// EnterBoolExpr is called when production boolExpr is entered.
func (s *BaseTsvsheetParserListener) EnterBoolExpr(ctx *BoolExprContext) {}

// ExitBoolExpr is called when production boolExpr is exited.
func (s *BaseTsvsheetParserListener) ExitBoolExpr(ctx *BoolExprContext) {}

// EnterPowExpr is called when production powExpr is entered.
func (s *BaseTsvsheetParserListener) EnterPowExpr(ctx *PowExprContext) {}

// ExitPowExpr is called when production powExpr is exited.
func (s *BaseTsvsheetParserListener) ExitPowExpr(ctx *PowExprContext) {}

// EnterCompareExpr is called when production compareExpr is entered.
func (s *BaseTsvsheetParserListener) EnterCompareExpr(ctx *CompareExprContext) {}

// ExitCompareExpr is called when production compareExpr is exited.
func (s *BaseTsvsheetParserListener) ExitCompareExpr(ctx *CompareExprContext) {}

// EnterFunctionCall is called when production functionCall is entered.
func (s *BaseTsvsheetParserListener) EnterFunctionCall(ctx *FunctionCallContext) {}

// ExitFunctionCall is called when production functionCall is exited.
func (s *BaseTsvsheetParserListener) ExitFunctionCall(ctx *FunctionCallContext) {}

// EnterArgList is called when production argList is entered.
func (s *BaseTsvsheetParserListener) EnterArgList(ctx *ArgListContext) {}

// ExitArgList is called when production argList is exited.
func (s *BaseTsvsheetParserListener) ExitArgList(ctx *ArgListContext) {}

// EnterReference is called when production reference is entered.
func (s *BaseTsvsheetParserListener) EnterReference(ctx *ReferenceContext) {}

// ExitReference is called when production reference is exited.
func (s *BaseTsvsheetParserListener) ExitReference(ctx *ReferenceContext) {}

// EnterSheetQualifier is called when production sheetQualifier is entered.
func (s *BaseTsvsheetParserListener) EnterSheetQualifier(ctx *SheetQualifierContext) {}

// ExitSheetQualifier is called when production sheetQualifier is exited.
func (s *BaseTsvsheetParserListener) ExitSheetQualifier(ctx *SheetQualifierContext) {}

// EnterCellRef is called when production cellRef is entered.
func (s *BaseTsvsheetParserListener) EnterCellRef(ctx *CellRefContext) {}

// ExitCellRef is called when production cellRef is exited.
func (s *BaseTsvsheetParserListener) ExitCellRef(ctx *CellRefContext) {}
