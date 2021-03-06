// This file contains functions for transpiling binary operator expressions.

package transpiler

import (
	"fmt"
	goast "go/ast"
	"go/token"
	"reflect"

	"github.com/elliotchance/c2go/ast"
	"github.com/elliotchance/c2go/program"
	"github.com/elliotchance/c2go/traverse"
	"github.com/elliotchance/c2go/types"
	"github.com/elliotchance/c2go/util"
)

func transpileBinaryOperator(n *ast.BinaryOperator, p *program.Program) (
	*goast.BinaryExpr, string, []goast.Stmt, []goast.Stmt, error) {
	preStmts := []goast.Stmt{}
	postStmts := []goast.Stmt{}
	var err error

	left, leftType, newPre, newPost, err := transpileToExpr(n.Children[0], p)
	if err != nil {
		return nil, "", nil, nil, err
	}

	preStmts, postStmts = combinePreAndPostStmts(preStmts, postStmts, newPre, newPost)

	right, rightType, newPre, newPost, err := transpileToExpr(n.Children[1], p)
	if err != nil {
		return nil, "", nil, nil, err
	}

	preStmts, postStmts = combinePreAndPostStmts(preStmts, postStmts, newPre, newPost)

	operator := getTokenForOperator(n.Operator)
	returnType := types.ResolveTypeForBinaryOperator(p, n.Operator, leftType, rightType)

	if operator == token.LAND {
		left, err = types.CastExpr(p, left, leftType, "bool")
		ast.WarningOrError(err, n, left == nil)
		if left == nil {
			left = util.NewStringLit("nil")
		}

		right, err = types.CastExpr(p, right, rightType, "bool")
		ast.WarningOrError(err, n, right == nil)
		if right == nil {
			right = util.NewStringLit("nil")
		}

		return util.NewBinaryExpr(left, operator, right), "bool",
			preStmts, postStmts, nil
	}

	// Convert "(0)" to "nil" when we are dealing with equality.
	if (operator == token.NEQ || operator == token.EQL) &&
		types.IsNullExpr(right) {
		right = goast.NewIdent("nil")
	}

	if operator == token.ASSIGN {
		// Memory allocation is translated into the Go-style.
		allocSize := GetAllocationSizeNode(n.Children[1])

		if allocSize != nil {
			allocSizeExpr, _, newPre, newPost, err := transpileToExpr(allocSize, p)
			preStmts, postStmts = combinePreAndPostStmts(preStmts, postStmts, newPre, newPost)

			if err != nil {
				return nil, "", preStmts, postStmts, err
			}

			derefType, err := types.GetDereferenceType(leftType)
			if err != nil {
				return nil, "", preStmts, postStmts, err
			}

			toType, err := types.ResolveType(p, leftType)
			if err != nil {
				return nil, "", preStmts, postStmts, err
			}

			elementSize, err := types.SizeOf(p, derefType)
			if err != nil {
				return nil, "", preStmts, postStmts, err
			}

			right = util.NewCallExpr(
				"make",
				util.NewStringLit(toType),
				util.NewBinaryExpr(allocSizeExpr, token.QUO, util.NewIntLit(elementSize)),
			)
		} else {
			right, err = types.CastExpr(p, right, rightType, returnType)

			if _, ok := right.(*goast.UnaryExpr); ok {
				deref, err := types.GetDereferenceType(rightType)
				if !ast.IsWarning(err, n) {
					// This is some hackey to convert a reference to a variable
					// into a slice that points to the same location. It will
					// look similar to:
					//
					//     (*[1]int)(unsafe.Pointer(&a))[:]
					//
					p.AddImport("unsafe")
					right = &goast.SliceExpr{
						X: util.NewCallExpr(
							fmt.Sprintf("(*[1]%s)", deref),
							util.NewCallExpr("unsafe.Pointer", right),
						),
					}
				}
			}

			if ast.IsWarning(err, n) && right == nil {
				right = util.NewStringLit("nil")
			}
		}
	}

	return util.NewBinaryExpr(left, operator, right),
		types.ResolveTypeForBinaryOperator(p, n.Operator, leftType, rightType),
		preStmts, postStmts, nil
}

// GetAllocationSizeNode returns the node that, if evaluated, would return the
// size (in bytes) of a memory allocation operation. For example:
//
//     (int *)malloc(sizeof(int))
//
// Would return the node that represents the "sizeof(int)".
//
// If the node does not represent an allocation operation (such as calling
// malloc, calloc, realloc, etc.) then nil is returned.
//
// In the case of calloc() it will return a new BinaryExpr that multiplies both
// arguments.
func GetAllocationSizeNode(node ast.Node) ast.Node {
	exprs := traverse.GetAllNodesOfType(node,
		reflect.TypeOf((*ast.CallExpr)(nil)))

	for _, expr := range exprs {
		functionName, _ := getNameOfFunctionFromCallExpr(expr.(*ast.CallExpr))

		if functionName == "malloc" {
			// Is 1 always the body in this case? Might need to be more careful
			// to find the correct node.
			return expr.(*ast.CallExpr).Children[1]
		}

		if functionName == "calloc" {
			return &ast.BinaryOperator{
				Type:     "int",
				Operator: "*",
				Children: expr.(*ast.CallExpr).Children[1:],
			}
		}

		// TODO: realloc() is not supported
		// https://github.com/elliotchance/c2go/issues/118
		//
		// Realloc will be treated as calloc which will almost certainly cause
		// bugs in your code.
		if functionName == "realloc" {
			return expr.(*ast.CallExpr).Children[2]
		}
	}

	return nil
}
