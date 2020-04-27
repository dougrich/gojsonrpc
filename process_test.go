package gojsonrpc

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"math"
	"reflect"
	"testing"
)

func TestNamespaceFunction(t *testing.T) {
	nfs := func(input string, expectedNamespace string, expectedFunction string) func(t *testing.T) {
		return func(t *testing.T) {
			actualNamespace, actualFunction := Request{MethodName: input}.getNamespaceFunction()
			assert.Equal(t, expectedNamespace, actualNamespace, "Namespace did not match")
			assert.Equal(t, expectedFunction, actualFunction, "Function did not match")
		}
	}
	t.Run("test.Add", nfs("test.Add", "test", "Add"))
	t.Run("test", nfs("test", "test", ""))
	t.Run("test.Add.Deep", nfs("test", "test", ""))
}

func TestCallFunction(t *testing.T) {
	cfs := func(
		m reflect.Value,
		expectedResult interface{},
		expectedErr *Error,
		parameters []interface{},
	) func(t *testing.T) {
		return func(t *testing.T) {
			p := jsonParameterize(parameters)
			actualResult, actualErr := callFunction(m, context.Background(), p)
			assert.Equal(t, expectedResult, actualResult)
			assert.Equal(t, expectedErr, actualErr)
		}
	}

	t.Run("Add/ContextError", cfs(reflect.ValueOf(func(_ context.Context, i ...float64) (float64, error) {
		sum := float64(0)
		for _, r := range i {
			sum += r
		}
		return sum, nil
	}), float64(6), nil, []interface{}{float64(1), float64(2), float64(3)}))

	t.Run("Add/MarshalType", cfs(reflect.ValueOf(func(i ...int) int {
		sum := 0
		for _, r := range i {
			sum += r
		}
		return sum
	}), 6, nil, []interface{}{float64(1), float64(2), float64(3)}))

	t.Run("Modulo/ContextError", cfs(reflect.ValueOf(func(_ context.Context, a float64, b float64) (float64, float64, error) {
		d := math.Floor(a / b)
		r := a - (d * b)
		return d, r, nil
	}), []interface{}{float64(2), float64(2)}, nil, []interface{}{float64(8), float64(3)}))

	t.Run("Modulo/InvalidParameterError", cfs(reflect.ValueOf(func(_ context.Context, a float64, b float64) (float64, float64, error) {
		return float64(0), float64(0), nil
	}), nil, &Error{Code: -32602, Message: "parameters should be (context.Context, float64, float64)"}, []interface{}{float64(3), "wrench in works"}))

	t.Run("Throw/RPCError", cfs(reflect.ValueOf(func(_ context.Context) (interface{}, error) {
		return nil, &Error{Code: 1001, Message: "This is a test"}
	}), nil, &Error{Code: 1001, Message: "This is a test"}, []interface{}{}))

	t.Run("Throw/InternalError", cfs(reflect.ValueOf(func(_ context.Context) (interface{}, error) {
		return nil, errors.New("this is a test")
	}), nil, &Error{Code: -32000, Message: "this is a test"}, []interface{}{}))
}
