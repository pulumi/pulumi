// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package binder

import (
	"fmt"
	"reflect"

	"github.com/pulumi/lumi/pkg/compiler/ast"
	"github.com/pulumi/lumi/pkg/compiler/errors"
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/compiler/types"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// astBinder is an AST visitor implementation that understands how to deal with all sorts of node types.  It
// does not visit children, however, as it relies on the depth-first order walk supplied by the AST package.  The
// overall purpose of this is to perform validation, and record types and symbols that're needed during evaluation.
type astBinder struct {
	b      *binder
	fnc    ast.Function                  // the current function.
	sig    *symbols.FunctionType         // the signature type for the current function.
	labels map[tokens.Name]ast.Statement // a map of known labels (for validation of jumps).
}

func newASTBinder(b *binder, fnc ast.Function, sig *symbols.FunctionType) ast.Visitor {
	return &astBinder{
		b:      b,
		fnc:    fnc,
		sig:    sig,
		labels: make(map[tokens.Name]ast.Statement),
	}
}

var _ ast.Visitor = (*astBinder)(nil)

func (a *astBinder) Visit(node ast.Node) ast.Visitor {
	// Lambdas are special because we want to visit in an entirely fresh context.
	if n, islambda := node.(*ast.LambdaExpression); islambda {
		a.visitLambdaExpression(n)
		return nil // don't trigger the automatic visitation.
	}

	// Otherwise, do some pre-validation if necessary, and then return the current visitor to keep on visiting.
	switch n := node.(type) {
	case *ast.Block:
		a.visitBlock(n)
	case *ast.LocalVariable:
		a.visitLocalVariable(n)
	case *ast.LabeledStatement:
		a.visitLabeledStatement(n)
	}
	return a
}

func (a *astBinder) After(node ast.Node) {
	switch n := node.(type) {
	// Statements
	case *ast.Block:
		a.checkBlock(n)
	case *ast.BreakStatement:
		a.checkBreakStatement(n)
	case *ast.ContinueStatement:
		a.checkContinueStatement(n)
	case *ast.IfStatement:
		a.checkIfStatement(n)
	case *ast.ReturnStatement:
		a.checkReturnStatement(n)
	case *ast.ThrowStatement:
		a.checkThrowStatement(n)
	case *ast.WhileStatement:
		a.checkWhileStatement(n)
	case *ast.ForStatement:
		a.checkForStatement(n)

	// Expressions
	case *ast.NullLiteral:
		a.b.ctx.RegisterType(n, types.Null) // register a null type.
	case *ast.BoolLiteral:
		a.b.ctx.RegisterType(n, types.Bool) // register a bool type.
	case *ast.NumberLiteral:
		a.b.ctx.RegisterType(n, types.Number) // register as a number type.
	case *ast.StringLiteral:
		a.b.ctx.RegisterType(n, types.String) // register as a string type.
	case *ast.ArrayLiteral:
		a.checkArrayLiteral(n)
	case *ast.ObjectLiteral:
		a.checkObjectLiteral(n)
	case *ast.LoadLocationExpression:
		a.checkLoadLocationExpression(n)
	case *ast.LoadDynamicExpression:
		a.checkLoadDynamicExpression(n)
	case *ast.TryLoadDynamicExpression:
		a.checkTryLoadDynamicExpression(n)
	case *ast.NewExpression:
		a.checkNewExpression(n)
	case *ast.InvokeFunctionExpression:
		a.checkInvokeFunctionExpression(n)
	case *ast.UnaryOperatorExpression:
		a.checkUnaryOperatorExpression(n)
	case *ast.BinaryOperatorExpression:
		a.checkBinaryOperatorExpression(n)
	case *ast.CastExpression:
		a.checkCastExpression(n)
	case *ast.IsInstExpression:
		a.checkIsInstExpression(n)
	case *ast.TypeOfExpression:
		a.checkTypeOfExpression(n)
	case *ast.ConditionalExpression:
		a.checkConditionalExpression(n)
	case *ast.SequenceExpression:
		a.checkSequenceExpression(n)
	}

	// Ensure that all expression types resulted in a type registration.
	expr, isExpr := node.(ast.Expression)
	contract.Assert(!isExpr || a.b.ctx.HasType(expr))
}

// Utility functions

// isLValue checks whether the target is a valid l-value.
func (a *astBinder) isLValue(expr ast.Expression) bool {
	// If the target is the result of a load operation, it is okay.
	switch expr.(type) {
	case *ast.LoadLocationExpression, *ast.LoadDynamicExpression, *ast.TryLoadDynamicExpression:
		// BUGBUG: ensure the target isn't a readonly location; if it is, issue an error.
		return true
	}

	// Otherwise, if the target is a pointer dereference, it is also okay.
	if unop, isunop := expr.(*ast.UnaryOperatorExpression); isunop {
		if unop.Operator == ast.OpDereference {
			return true
		}
	}

	return false
}

// Statements

func (a *astBinder) checkImport(node *ast.Import) {
	imptok := node.Referent
	if !imptok.Tok.HasModule() {
		a.b.Diag().Errorf(errors.ErrorMalformedToken.At(imptok),
			"Module", imptok.Tok, "missing module part")
	} else {
		// Just perform a lookup to ensure the symbol exists (and error out if not).
		a.b.ctx.LookupSymbol(imptok, imptok.Tok, true)
	}
}

func (a *astBinder) visitBlock(node *ast.Block) {
	// Entering a new block requires a fresh lexical scope.
	a.b.ctx.Scope.Push(false)
}

func (a *astBinder) checkBlock(node *ast.Block) {
	// Exiting a block restores the prior lexical context.
	a.b.ctx.Scope.Pop()
}

func (a *astBinder) checkBreakStatement(node *ast.BreakStatement) {
	// If the break specifies a label, ensure that it exists.
	if node.Label != nil {
		label := node.Label.Ident
		if _, has := a.labels[label]; !has {
			a.b.Diag().Errorf(errors.ErrorUnknownJumpLabel.At(node), label, "break")
		}
	}
}

func (a *astBinder) checkContinueStatement(node *ast.ContinueStatement) {
	// If the continue specifies a label, ensure that it exists.
	if node.Label != nil {
		label := node.Label.Ident
		if _, has := a.labels[label]; !has {
			a.b.Diag().Errorf(errors.ErrorUnknownJumpLabel.At(node), label, "continue")
		}
	}
}

func (a *astBinder) checkIfStatement(node *ast.IfStatement) {
	// Ensure that the condition is a boolean expression.
	a.checkExprType(node.Condition, types.Bool)
}

func (a *astBinder) visitLocalVariable(node *ast.LocalVariable) {
	// Encountering a new local variable results in registering it; both to the type and symbol table.
	ty := a.b.ctx.LookupType(node.Type)
	contract.Assert(ty != nil)
	sym := symbols.NewLocalVariableSym(node, ty)
	a.b.ctx.RegisterSymbol(node, sym)
	a.b.ctx.Scope.TryRegister(node, sym)
}

func (a *astBinder) visitLabeledStatement(node *ast.LabeledStatement) {
	// Ensure this label doesn't already exist and then register it.
	label := node.Label.Ident
	if other, has := a.labels[label]; has {
		a.b.Diag().Errorf(errors.ErrorDuplicateLabel.At(node), label, other)
	}
	a.labels[label] = node
}

func (a *astBinder) checkReturnStatement(node *ast.ReturnStatement) {
	// Ensure that the return expression is correct (present or missing; and its type).
	if ret := a.sig.Return; ret == nil {
		if node.Expression != nil {
			// The function has no return type ("void"), and yet the return had an expression.
			a.b.Diag().Errorf(errors.ErrorUnexpectedReturnExpr.At(*node.Expression))
		}
	} else {
		if node.Expression == nil {
			// The function has a return type, but there was no return expression.
			a.b.Diag().Errorf(errors.ErrorExpectedReturnExpr.At(node))
		} else {
			// Ensure that the returned expression is convertible to the expected return type.
			a.checkExprType(*node.Expression, ret)
		}
	}
}

func (a *astBinder) checkThrowStatement(node *ast.ThrowStatement) {
	// It's legal to throw anything.
	contract.Assert(node.Expression != nil)
}

func (a *astBinder) checkWhileStatement(node *ast.WhileStatement) {
	// Ensure that the loop condition is a boolean expression.
	if node.Condition != nil {
		a.checkExprType(*node.Condition, types.Bool)
	}
}

func (a *astBinder) checkForStatement(node *ast.ForStatement) {
	// Ensure that the loop condition is a boolean expression.
	if node.Condition != nil {
		a.checkExprType(*node.Condition, types.Bool)
	}
}

// Expressions

func (a *astBinder) visitLambdaExpression(node *ast.LambdaExpression) {
	lambdaType := a.b.bindLambdaExpression(node)
	a.b.ctx.RegisterType(node, lambdaType)
}

func (a *astBinder) checkExprType(expr ast.Expression, expect symbols.Type, alts ...symbols.Type) bool {
	actual := a.b.ctx.RequireType(expr)
	var conv bool
	if conv = types.CanConvert(actual, expect); !conv {
		// If the primary didn't convert, check the alternatives.
		for _, alt := range alts {
			if conv = types.CanConvert(actual, alt); conv {
				break
			}
		}
	}
	if !conv {
		expects := expect.Token().String()
		if len(alts) > 0 {
			for _, alt := range alts {
				expects += fmt.Sprintf(" or %v", alt.Token())
			}
		}
		a.b.Diag().Errorf(errors.ErrorIncorrectExprType.At(expr), expects, actual)
		return false
	}
	return true
}

func (a *astBinder) checkArrayLiteral(node *ast.ArrayLiteral) {
	// If there is a size, ensure it's a number.
	if node.Size != nil {
		a.checkExprType(*node.Size, types.Number)
	}
	// Now mark the resulting expression as an array of the right type.
	if node.ElemType == nil {
		a.b.ctx.RegisterType(node, symbols.NewArrayType(types.Object))
	} else {
		elemType := a.b.ctx.LookupType(node.ElemType)
		a.b.ctx.RegisterType(node, symbols.NewArrayType(elemType))

		// Ensure the elements, if any, are of the right type.
		if node.Elements != nil {
			for _, elem := range *node.Elements {
				a.checkExprType(elem, elemType)
			}
		}
	}
}

func (a *astBinder) checkObjectLiteral(node *ast.ObjectLiteral) {
	// Mark the resulting object literal with the correct type.
	if node.Type == nil {
		a.b.ctx.RegisterType(node, types.Object)
	} else {
		ty := a.b.ctx.LookupType(node.Type)
		a.b.ctx.RegisterType(node, ty)

		// Check invariants of object literal types and properties.  In particular, only permit object literals for
		// records and interfaces (since lasses have constructors), and ensure the required ones are present.  Note that
		// all of this is skipped for so-called "anonymous" literals (of type `any`), since they are just bags.
		if ty != types.Dynamic {
			if !ty.Record() && !ty.Interface() {
				a.b.Diag().Errorf(errors.ErrorIllegalObjectLiteralType.At(node.Type), ty)
			} else {
				// Ensure that all required properties have been supplied, and that they are of the right type.
				props := make(map[tokens.ClassMemberName]bool)
				if node.Properties != nil {
					for _, init := range *node.Properties {
						switch p := init.(type) {
						case *ast.ObjectLiteralNamedProperty:
							// Ensure this property is known to exist.
							prop := p.Property
							if prop.Tok.HasClassMember() {
								clm := tokens.ClassMember(prop.Tok)
								sym := a.b.ctx.RequireClassMember(prop, ty, clm)
								if sym != nil {
									switch s := sym.(type) {
									case *symbols.ClassProperty, *symbols.ClassMethod:
										a.checkExprType(p.Value, s.Type())
									default:
										contract.Failf("Unrecognized class member symbol: %v", sym)
									}
									props[clm.Name()] = true // record that we've seen this one.
								}
							}
						case *ast.ObjectLiteralComputedProperty:
							// A computed property is required to be a string.
							a.checkExprType(p.Property, types.String)
						}
					}
				}

				// Issue an error about any missing required properties.
				membs := ty.TypeMembers()
				for _, name := range symbols.StableClassMemberMap(membs) {
					if _, has := props[name]; !has {
						member := membs[name]
						if !member.Optional() && member.Default() == nil {
							a.b.Diag().Errorf(errors.ErrorMissingRequiredProperty.At(node), name)
						}
					}
				}

				// IDEA: consider issuing an error for "excess" properties.
			}
		}
	}
}

func (a *astBinder) checkLoadLocationExpression(node *ast.LoadLocationExpression) {
	// Bind the token to a location.
	var sym symbols.Symbol
	if node.Object == nil {
		// If there is no object, we either have a "simple" local variable reference or a qualified module property or
		// function identifier.  In both cases, requireToken will handle it for us.
		sym = a.b.ctx.RequireToken(node.Name, node.Name.Tok)
	} else {
		// If there's an object, we are accessing a class member property or function.
		typ := a.b.ctx.RequireType(*node.Object)
		sym = a.b.ctx.RequireClassMember(node.Name, typ, tokens.ClassMember(node.Name.Tok))
	}

	// Produce a type of the right kind from the target location.
	var ty symbols.Type
	if sym == nil {
		ty = types.Error // no symbol found, use an error type.
	} else {
		var usesThis bool
		for ty == nil {
			// Check that the this object is well-formed and extract the symbol.
			switch s := sym.(type) {
			case *symbols.LocalVariable:
				ty = s.Type()
				usesThis = false
			case *symbols.ClassMethod:
				ty = s.Signature()
				usesThis = !s.Static()
			case *symbols.ClassProperty:
				ty = s.Type()
				usesThis = !s.Static()
			case *symbols.ModuleMethod:
				ty = s.Signature()
				usesThis = false
			case *symbols.ModuleProperty:
				ty = s.Type()
				usesThis = false
			case *symbols.Export:
				// For exports, let's keep digging until we hit something concrete.
				contract.Assertf(s.Referent != s, "Unexpected self-referential export")
				sym = s.Referent
			default:
				contract.Failf("Unrecognized load location token '%v' symbol type: %v", sym.Token(), reflect.TypeOf(s))
			}
		}

		if usesThis {
			if node.Object == nil {
				a.b.Diag().Errorf(errors.ErrorExpectedObject.At(node))
			}
		} else if node.Object != nil {
			a.b.Diag().Errorf(errors.ErrorUnexpectedObject.At(node))
		}
	}

	// Register the type; not that, although this is a valid l-value, we do not create a pointer out of it implicitly.
	a.b.ctx.RegisterType(node, ty)
}

func (a *astBinder) checkLoadDynamicExpression(node *ast.LoadDynamicExpression) {
	// Ensure that the name is either a string, number, or a dynamic.
	a.checkExprType(node.Name, types.String, types.Number)

	// No matter the outcome, a load dynamic always produces a dynamically typed thing.
	a.b.ctx.RegisterType(node, types.Dynamic)
}

func (a *astBinder) checkTryLoadDynamicExpression(node *ast.TryLoadDynamicExpression) {
	// Ensure that the name is either a string, number, or a dynamic.
	a.checkExprType(node.Name, types.String, types.Number)

	// No matter the outcome, a load dynamic always produces a dynamically typed thing.
	a.b.ctx.RegisterType(node, types.Dynamic)
}

func (a *astBinder) checkNewExpression(node *ast.NewExpression) {
	// Figure out which type we're instantiating.
	var ty symbols.Type
	if node.Type == nil {
		ty = types.Object
	} else {
		ty = a.b.ctx.LookupType(node.Type)
		if class, isclass := ty.(*symbols.Class); isclass {
			// Ensure we're not creating an abstract class.
			if class.Abstract() {
				a.b.Diag().Errorf(errors.ErrorCannotNewAbstractClass.At(node.Type), class)
			}
		}
	}

	if ty != types.Dynamic {
		// Find the constructor for that type and check the arguments.
		argc := 0
		if node.Arguments != nil {
			argc = len(*node.Arguments)
		}
		if ctor, has := ty.TypeMembers()[tokens.ClassConstructorFunction]; has {
			// The constructor should be a method.
			if ctormeth, isfunc := ctor.(*symbols.ClassMethod); isfunc {
				if ctormeth.Sig.Return != nil {
					// Constructors ought not to have return types.
					a.b.Diag().Errorf(errors.ErrorConstructorReturnType, ctormeth, ctormeth.Sig.Return)
				}

				// Typecheck the arguments.
				parc := len(ctormeth.Sig.Parameters)
				if parc != argc {
					a.b.Diag().Errorf(errors.ErrorArgumentCountMismatch.At(node), parc, argc)
				}
				if argc > 0 {
					for i := 0; i < parc && i < argc; i++ {
						a.checkExprType((*node.Arguments)[i].Expr, ctormeth.Sig.Parameters[i])
					}
				}
			} else {
				a.b.Diag().Errorf(errors.ErrorConstructorNotMethod, ctormeth, reflect.TypeOf(ctor))
			}
		} else if argc > 0 {
			a.b.Diag().Errorf(errors.ErrorArgumentCountMismatch.At(node), 0, argc)
		}
	}

	// The expression evaluates to the type that is instantiated.
	a.b.ctx.RegisterType(node, ty)
}

func (a *astBinder) checkInvokeFunctionExpression(node *ast.InvokeFunctionExpression) {
	ty := a.b.ctx.RequireType(node.Function)
	if funty, isfun := ty.(*symbols.FunctionType); isfun {
		// Typecheck the arguments.
		parc := len(funty.Parameters)
		argc := 0
		if node.Arguments != nil {
			argc = len(*node.Arguments)
		}
		if parc != argc {
			a.b.Diag().Errorf(errors.ErrorArgumentCountMismatch.At(node), parc, argc)
		}
		if argc > 0 {
			for i := 0; i < parc && i < argc; i++ {
				a.checkExprType((*node.Arguments)[i].Expr, funty.Parameters[i])
			}
		}

		// The resulting type of this expression is the same as the function's return type.  Note that if the return is
		// nil ("void"-returning), we still register it; a nil entry is distinctly different from a missing entry.
		a.b.ctx.RegisterType(node, funty.Return)
	} else if ty == types.Dynamic {
		// It's ok to invoke a dynamically typed object; we simply might fail at runtime.
		a.b.ctx.RegisterType(node, types.Dynamic)
	} else {
		a.b.Diag().Errorf(errors.ErrorCannotInvokeNonFunction.At(node), ty)
		a.b.ctx.RegisterType(node, types.Error)
	}
}

func (a *astBinder) checkUnaryOperatorExpression(node *ast.UnaryOperatorExpression) {
	// First check that prefix/postfix isn't wrong.
	switch node.Operator {
	case ast.OpDereference, ast.OpAddressof, ast.OpUnaryPlus,
		ast.OpUnaryMinus, ast.OpLogicalNot, ast.OpBitwiseNot:
		if node.Postfix {
			a.b.Diag().Errorf(errors.ErrorUnaryOperatorMustBePrefix.At(node), node.Operator)
		}
	case ast.OpPlusPlus, ast.OpMinusMinus:
		// both prefix/post are fine.
	default:
		contract.Failf("Unrecognized unary operator: %v", node.Operator)
	}

	// Now check the types and assign a type to this expression.
	opty := a.b.ctx.RequireType(node.Operand)
	switch node.Operator {
	case ast.OpDereference:
		// The right hand side must be a pointer; produce its element type:
		switch ot := opty.(type) {
		case *symbols.PointerType:
			a.b.ctx.RegisterType(node, ot.Element)
		default:
			a.b.Diag().Errorf(errors.ErrorUnaryOperatorInvalidForType.At(node), node.Operator, ot, "pointer type")
			a.b.ctx.RegisterType(node, ot)
		}
	case ast.OpAddressof:
		// The target must be a legal l-value expression:
		if !a.isLValue(node.Operand) {
			a.b.Diag().Errorf(errors.ErrorUnaryOperatorInvalidForOperand.At(node), node.Operator,
				node.Operand.GetKind(), "load location")
		}
		a.b.ctx.RegisterType(node, symbols.NewPointerType(opty))
	case ast.OpUnaryPlus, ast.OpUnaryMinus, ast.OpBitwiseNot:
		// The target must be a number:
		if !types.CanConvert(opty, types.Number) {
			a.b.Diag().Errorf(errors.ErrorUnaryOperatorInvalidForType.At(node), node.Operator, opty, types.Number)
		}
		a.b.ctx.RegisterType(node, types.Number)
	case ast.OpLogicalNot:
		// The target must be a boolean:
		if !types.CanConvert(opty, types.Bool) {
			a.b.Diag().Errorf(errors.ErrorUnaryOperatorInvalidForType.At(node), node.Operator, opty, types.Bool)
		}
		a.b.ctx.RegisterType(node, types.Bool)
	case ast.OpPlusPlus, ast.OpMinusMinus:
		// The target must be a numeric l-value.
		if !a.isLValue(node.Operand) {
			a.b.Diag().Errorf(errors.ErrorIllegalAssignmentLValue.At(node))
		} else if !types.CanConvert(opty, types.Number) {
			a.b.Diag().Errorf(errors.ErrorUnaryOperatorInvalidForType.At(node), node.Operator, opty, types.Number)
		}
		a.b.ctx.RegisterType(node, types.Number)
	default:
		contract.Failf("Unrecognized unary operator: %v", node.Operator)
	}
}

func (a *astBinder) checkBinaryOperatorExpression(node *ast.BinaryOperatorExpression) {
	// Check that the operands are of the right type, and assign a type to this node.
	lhs := a.b.ctx.RequireType(node.Left)
	rhs := a.b.ctx.RequireType(node.Right)
	switch node.Operator {
	// Arithmetic and bitwise operators:
	case ast.OpAdd:
		// Lhs and rhs can be numbers (for addition) or strings (for concatenation) or dynamic.
		if lhs == types.Number {
			if !types.CanConvert(rhs, types.Number) {
				a.b.Diag().Errorf(errors.ErrorBinaryOperatorInvalidForType.At(node),
					node.Operator, "RHS", rhs, types.Number)
			}
			a.b.ctx.RegisterType(node, types.Number)
		} else if lhs == types.String {
			if !types.CanConvert(rhs, types.String) {
				a.b.Diag().Errorf(errors.ErrorBinaryOperatorInvalidForType.At(node),
					node.Operator, "RHS", rhs, types.String)
			}
			a.b.ctx.RegisterType(node, types.String)
		} else if lhs == types.Dynamic {
			if types.CanConvert(rhs, types.String) {
				a.b.ctx.RegisterType(node, types.String)
			} else {
				a.b.ctx.RegisterType(node, types.Dynamic)
			}
		} else {
			a.b.Diag().Errorf(errors.ErrorBinaryOperatorInvalidForType.At(node),
				node.Operator, "LHS", lhs, "string or number")
			a.b.ctx.RegisterType(node, types.String)
		}
	case ast.OpSubtract, ast.OpMultiply, ast.OpDivide, ast.OpRemainder, ast.OpExponentiate,
		ast.OpBitwiseShiftLeft, ast.OpBitwiseShiftRight, ast.OpBitwiseAnd, ast.OpBitwiseOr, ast.OpBitwiseXor:
		// Both lhs and rhs must be numbers:
		if !types.CanConvert(lhs, types.Number) {
			a.b.Diag().Errorf(errors.ErrorBinaryOperatorInvalidForType.At(node),
				node.Operator, "LHS", lhs, types.Number)
		}
		if !types.CanConvert(rhs, types.Number) {
			a.b.Diag().Errorf(errors.ErrorBinaryOperatorInvalidForType.At(node),
				node.Operator, "RHS", rhs, types.Number)
		}
		a.b.ctx.RegisterType(node, types.Number)

	// Assignment operators:
	case ast.OpAssign:
		if !a.isLValue(node.Left) {
			a.b.Diag().Errorf(errors.ErrorIllegalAssignmentLValue.At(node))
		} else if !types.CanConvert(rhs, lhs) {
			a.b.Diag().Errorf(errors.ErrorIllegalAssignmentTypes.At(node), rhs, lhs)
		}
		a.b.ctx.RegisterType(node, lhs)
	case ast.OpAssignSum:
		// Lhs and rhs can be numbers (for addition) or strings (for concatenation).
		if !a.isLValue(node.Left) {
			a.b.Diag().Errorf(errors.ErrorIllegalAssignmentLValue.At(node))
			a.b.ctx.RegisterType(node, types.Number)
		} else if lhs == types.Number {
			if !types.CanConvert(rhs, types.Number) {
				a.b.Diag().Errorf(errors.ErrorBinaryOperatorInvalidForType.At(node),
					node.Operator, "RHS", rhs, types.Number)
			}
			a.b.ctx.RegisterType(node, types.Number)
		} else if lhs == types.String {
			if !types.CanConvert(rhs, types.String) {
				a.b.Diag().Errorf(errors.ErrorBinaryOperatorInvalidForType.At(node),
					node.Operator, "RHS", rhs, types.String)
			}
			a.b.ctx.RegisterType(node, types.String)
		} else {
			a.b.Diag().Errorf(errors.ErrorBinaryOperatorInvalidForType.At(node),
				node.Operator, "LHS", lhs, "string or number")
			a.b.ctx.RegisterType(node, types.Number)
		}
	case ast.OpAssignDifference, ast.OpAssignProduct, ast.OpAssignQuotient,
		ast.OpAssignRemainder, ast.OpAssignExponentiation, ast.OpAssignBitwiseShiftLeft, ast.OpAssignBitwiseShiftRight,
		ast.OpAssignBitwiseAnd, ast.OpAssignBitwiseOr, ast.OpAssignBitwiseXor:
		// These operators require numeric values.
		if !a.isLValue(node.Left) {
			a.b.Diag().Errorf(errors.ErrorIllegalAssignmentLValue.At(node))
		} else if !types.CanConvert(lhs, types.Number) {
			a.b.Diag().Errorf(errors.ErrorIllegalNumericAssignmentLValue.At(node), node.Operator)
		} else if !types.CanConvert(rhs, lhs) {
			a.b.Diag().Errorf(errors.ErrorIllegalAssignmentTypes.At(node), rhs, lhs)
		}
		a.b.ctx.RegisterType(node, lhs)

	// Conditional operators:
	case ast.OpLogicalAnd, ast.OpLogicalOr:
		// Both lhs and rhs must be booleans.
		if !types.CanConvert(lhs, types.Bool) {
			a.b.Diag().Errorf(errors.ErrorBinaryOperatorInvalidForType.At(node),
				node.Operator, "LHS", lhs, types.Bool)
		}
		if !types.CanConvert(rhs, types.Bool) {
			a.b.Diag().Errorf(errors.ErrorBinaryOperatorInvalidForType.At(node),
				node.Operator, "RHS", rhs, types.Bool)
		}
		a.b.ctx.RegisterType(node, types.Bool)

	// Relational operators:
	case ast.OpLt, ast.OpLtEquals, ast.OpGt, ast.OpGtEquals:
		// Both lhs and rhs must be numbers, and it produces a boolean.
		if !types.CanConvert(lhs, types.Number) {
			a.b.Diag().Errorf(errors.ErrorBinaryOperatorInvalidForType.At(node),
				node.Operator, "LHS", lhs, types.Number)
		}
		if !types.CanConvert(rhs, types.Number) {
			a.b.Diag().Errorf(errors.ErrorBinaryOperatorInvalidForType.At(node),
				node.Operator, "RHS", rhs, types.Number)
		}
		a.b.ctx.RegisterType(node, types.Bool)
	case ast.OpEquals, ast.OpNotEquals:
		// Equality checking is valid on any type, and it always produces a boolean.
		a.b.ctx.RegisterType(node, types.Bool)
	}
}

func (a *astBinder) checkCastExpression(node *ast.CastExpression) {
	// If we know statically that the cast is invalid -- that is, the destination type could never have been converted
	// to the source type -- issue an error.  Otherwise, defer to runtime.
	from := a.b.ctx.RequireType(node.Expression)
	to := a.b.ctx.LookupType(node.Type)
	if !types.CanConvert(to, from) {
		a.b.Diag().Errorf(errors.ErrorInvalidCast.At(node), from, to)
	}
	a.b.ctx.RegisterType(node, to)
}

func (a *astBinder) checkIsInstExpression(node *ast.IsInstExpression) {
	// An isinst produces a bool indicating whether the expression is of the target type.
	a.b.ctx.RegisterType(node, types.Bool)
}

func (a *astBinder) checkTypeOfExpression(node *ast.TypeOfExpression) {
	// A typeof produces a string representation of the expression's type.
	a.b.ctx.RegisterType(node, types.String)
}

func (a *astBinder) checkConditionalExpression(node *ast.ConditionalExpression) {
	// TODO[pulumi/lumi#213]: unify the consequent and alternate types.
	contract.Failf("Binding of %v nodes not yet implemented", node.GetKind())
}

func (a *astBinder) checkSequenceExpression(node *ast.SequenceExpression) {
	// Ensure that all prelude nodes are statements or expressions.
	for _, prelnode := range node.Prelude {
		switch prelnode.(type) {
		case ast.Expression, ast.Statement:
			// good
		default:
			a.b.Diag().Errorf(errors.ErrorSequencePreludeExprStmt.At(prelnode))
		}
	}

	// The type of a sequence expression is just the type of the last expression in the sequence.
	a.b.ctx.RegisterType(node, a.b.ctx.RequireType(node.Value))
}
