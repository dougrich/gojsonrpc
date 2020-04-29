package gojsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

type TestJSONStruct struct {
	Member string `json:"member"`
}

type TestBreakJSONStruct struct {
	Member float64 `json:"member"`
}

type TestCustomStruct struct {
	underlying string
}

func (t TestCustomStruct) RPCName() string {
	return "string"
}

func (t *TestCustomStruct) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if len(s) < 3 {
		return errors.New("custom validation; must have length of at least 3")
	}
	*t = TestCustomStruct{s}
	return nil
}

func (t TestCustomStruct) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.underlying)
}

func TestGetJSONType(t *testing.T) {
	gjts := func(
		m reflect.Type,
		expectedName string,
		expectedError error,
	) func(t *testing.T) {
		return func(t *testing.T) {
			actualName, actualError := getJSONType(m)
			assert.Equal(t, expectedName, actualName)
			assert.Equal(t, expectedError, actualError)
		}
	}

	t.Run("Int", gjts(reflect.TypeOf(int(5)), "number (int)", nil))
	t.Run("Int8", gjts(reflect.TypeOf(int8(5)), "number (int8)", nil))
	t.Run("Int16", gjts(reflect.TypeOf(int16(5)), "number (int16)", nil))
	t.Run("Int32", gjts(reflect.TypeOf(int32(5)), "number (int32)", nil))
	t.Run("Int64", gjts(reflect.TypeOf(int64(5)), "number (int64)", nil))
	t.Run("Uint", gjts(reflect.TypeOf(uint(5)), "number (uint)", nil))
	t.Run("Uint8", gjts(reflect.TypeOf(uint8(5)), "number (uint8)", nil))
	t.Run("Uint16", gjts(reflect.TypeOf(uint16(5)), "number (uint16)", nil))
	t.Run("Uint32", gjts(reflect.TypeOf(uint32(5)), "number (uint32)", nil))
	t.Run("Uint64", gjts(reflect.TypeOf(uint64(5)), "number (uint64)", nil))
	t.Run("Float32", gjts(reflect.TypeOf(float32(5)), "number (float)", nil))
	t.Run("Float64", gjts(reflect.TypeOf(float64(5)), "number", nil))
	t.Run("String", gjts(reflect.TypeOf(""), "string", nil))
	t.Run("Bool", gjts(reflect.TypeOf(false), "bool", nil))
	t.Run("Map", gjts(reflect.TypeOf(map[string]int{}), "object", nil))
	t.Run("Array", gjts(reflect.TypeOf([1]string{"one"}), "array", nil))
	t.Run("Slice", gjts(reflect.TypeOf(make([]string, 0)), "array", nil))
	t.Run("Struct", gjts(reflect.TypeOf(TestJSONStruct{}), "object (TestJSONStruct)", nil))
	t.Run("CustomStruct", gjts(reflect.TypeOf(TestCustomStruct{}), "string", nil))
	t.Run("Ptr/Int", gjts(reflect.PtrTo(reflect.TypeOf(int(5))), "number (int)?", nil))
	t.Run("Ptr/String", gjts(reflect.PtrTo(reflect.TypeOf("1234")), "string?", nil))
	// interface, func, chan, complex64, complex128, unsafeptr
}

func TestMarshalJSONType(t *testing.T) {
	passthroughCase := func(
		value interface{},
	) func(t *testing.T) {
		return func(t *testing.T) {
			m := reflect.TypeOf(value)
			b, err := json.Marshal(value)
			must(err)
			var rm json.RawMessage
			must(json.Unmarshal(b, &rm))
			actualOutput, actualError := marshalJSONType(m, rm)
			assert.Equal(t, value, actualOutput)
			assert.NoError(t, actualError)
		}
	}

	t.Run("Int", passthroughCase(int(5)))
	t.Run("Int8", passthroughCase(int8(5)))
	t.Run("Int16", passthroughCase(int16(5)))
	t.Run("Int32", passthroughCase(int32(5)))
	t.Run("Int64", passthroughCase(int64(5)))
	t.Run("Uint", passthroughCase(uint(5)))
	t.Run("Uint8", passthroughCase(uint8(5)))
	t.Run("Uint16", passthroughCase(uint16(5)))
	t.Run("Uint32", passthroughCase(uint32(5)))
	t.Run("Uint64", passthroughCase(uint64(5)))
	t.Run("Float32", passthroughCase(float32(5)))
	t.Run("Float64", passthroughCase(float64(5)))
	t.Run("String", passthroughCase("test"))
	t.Run("Bool", passthroughCase(true))
	t.Run("Struct", passthroughCase(TestJSONStruct{"1234"}))
	t.Run("CustomStruct", passthroughCase(TestCustomStruct{"1234"})) // note that this one is going into a string in json and then back out

	pointerCase := func(
		value interface{},
		m reflect.Type,
	) func(t *testing.T) {
		return func(t *testing.T) {
			b, err := json.Marshal(value)
			must(err)
			var rm json.RawMessage
			must(json.Unmarshal(b, &rm))
			actualOutput, actualError := marshalJSONType(m, rm)
			assert.NoError(t, actualError)
			e := reflect.ValueOf(actualOutput)
			if e.IsNil() {
				assert.Nil(t, value)
			} else {
				assert.Equal(t, value, e.Elem().Interface())
			}
		}
	}

	t.Run("Ptr/Int", pointerCase(int(5), reflect.PtrTo(reflect.TypeOf(int(5)))))
	t.Run("Ptr/IntNil", pointerCase(nil, reflect.PtrTo(reflect.TypeOf(int(5)))))

	errorCase := func(
		input interface{},
		expectedOutput interface{},
		expectedError string,
	) func(t *testing.T) {
		return func(t *testing.T) {
			m := reflect.TypeOf(expectedOutput)
			b, err := json.Marshal(input)
			must(err)
			var rm json.RawMessage
			must(json.Unmarshal(b, &rm))
			_, actualError := marshalJSONType(m, rm)
			assert.EqualError(t, actualError, expectedError)
		}
	}

	t.Run("Int/Rounding", errorCase(float64(9.8), int(9), "expected int; got number 9.8"))
	t.Run("Int/String", errorCase("testing", int(9), "expected int; got string"))
	t.Run("Uint/Rounding", errorCase(float64(-9.8), uint(9), "expected uint; got number -9.8"))
	t.Run("Struct/String", errorCase("testing", TestJSONStruct{"1234"}, "expected TestJSONStruct; got string"))
	t.Run("Struct/Nested", errorCase(TestBreakJSONStruct{float64(-9.9)}, TestJSONStruct{"1234"}, "expected string at path \"member\"; got number"))
	t.Run("CustomStruct/CustomValidation", errorCase("a", TestCustomStruct{"a"}, "custom validation; must have length of at least 3"))
	t.Run("CustomStruct/Number", errorCase(float64(-9.9), TestCustomStruct{"1234"}, "expected string; got number"))
}

func TestNewParameterizedMethod(t *testing.T) {

	signatureCase := func(
		fn interface{},
		expectedSignature string,
	) func(t *testing.T) {
		return func(t *testing.T) {
			m := reflect.ValueOf(fn)
			p, err := newParameterizedMethod(m)
			assert.Nil(t, err)
			assert.Equal(t, expectedSignature, p.signature)
		}
	}

	t.Run("Signature/Flat", signatureCase(func(abc int, efg string, hij *int) {}, "number (int), string, number (int)?"))
	t.Run("Signature/Variadic", signatureCase(func(abc int, efg string, hij ...int) {}, "number (int), string, ...number (int)"))
	t.Run("Signature/Context", signatureCase(func(c context.Context, abc int, efg string) {}, "number (int), string"))

	callCase := func(
		fn interface{},
		expectedOutput interface{},
		params ...interface{},
	) func(t *testing.T) {
		return func(t *testing.T) {
			m := reflect.ValueOf(fn)
			p, err := newParameterizedMethod(m)
			assert.NoError(t, err)
			parameters := jsonParameterize(params)
			actualOutput, actualError := p.Call(context.Background(), parameters)
			assert.Equal(t, expectedOutput, actualOutput)
			assert.Nil(t, actualError)
		}
	}

	t.Run("Call/Flat", callCase(func(abc int) int { return abc + 1 }, 4, 3))
	t.Run("Call/Variadic/Empty", callCase(func(abc int, def ...int) int {
		total := abc + 1
		for _, v := range def {
			total += v
		}
		return total
	}, 4, 3))
	t.Run("Call/Variadic/Values", callCase(func(abc int, def ...int) int {
		total := abc + 1
		for _, v := range def {
			total += v
		}
		return total
	}, 7, 3, 3))

	t.Run("Call/MultipleReturn", callCase(func(abc int) (int, int) {
		return abc + 1, abc + 4
	}, []interface{}{4, 7}, 3))

	callErrorCase := func(
		fn interface{},
		expectedError Error,
		params ...interface{},
	) func(t *testing.T) {
		return func(t *testing.T) {
			m := reflect.ValueOf(fn)
			p, err := newParameterizedMethod(m)
			assert.NoError(t, err)
			parameters := jsonParameterize(params)
			actualOutput, actualError := p.Call(context.Background(), parameters)
			assert.NotNil(t, actualError)
			if actualError != nil {
				assert.Equal(t, expectedError, *actualError)
				assert.Nil(t, actualOutput)
			}
		}
	}

	t.Run("Call/FlatMissingArgs", callErrorCase(func(abc int) int { return abc + 1 }, Error{Code: -32602, Message: "parameters should be (number (int))"}))
	t.Run("Call/FlatTooManyArgs", callErrorCase(func(abc int) int { return abc + 1 }, Error{Code: -32602, Message: "parameters should be (number (int))"}, 5, 8))
	t.Run("Call/UnknownInternalError", callErrorCase(func() error { return errors.New("what happened here") }, Error{Code: -32000, Message: "what happened here"}))
	t.Run("Call/CustomInternalError", callErrorCase(func() error { return &Error{Code: -1000, Message: "what what"} }, Error{Code: -1000, Message: "what what"}))
}
