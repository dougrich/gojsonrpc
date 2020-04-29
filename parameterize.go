package gojsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

var (
	reflectionTypeContext = reflect.TypeOf((*context.Context)(nil)).Elem()
	reflectionTypeError   = reflect.TypeOf((*error)(nil)).Elem()
)

type parameterizedMethod struct {
	methodType            reflect.Type
	method                reflect.Value
	parameters            []parameterizedMethodParameter
	signature             string
	requiredArgumentCount int
	outputArgumentCount   int
	isLastArgumentError   bool
}

func newParameterizedMethod(m reflect.Value) (*parameterizedMethod, error) {
	// create the analysis from here
	t := m.Type()
	var parameters []parameterizedMethodParameter
	var publicParams []string
	inputIndex := 0
	lenArgs := t.NumIn()
	isVariadic := t.IsVariadic()
	for i := 0; i < lenArgs; i++ {
		p, err := newParameterizedMethodParameter(t.In(i), inputIndex, isVariadic && i == lenArgs-1)
		if err != nil {
			fmt.Printf("Error inside: %v\n", err)
			return nil, err
		}
		parameters = append(parameters, p)
		if !p.isContext {
			publicParams = append(publicParams, p.typeName)
			inputIndex++
		}
	}
	if isVariadic {
		inputIndex = -1
	}

	outputArgumentCount := t.NumOut()
	isLastArgumentError := outputArgumentCount > 0 && t.Out(outputArgumentCount-1).Implements(reflectionTypeError)

	return &parameterizedMethod{t, m, parameters, strings.Join(publicParams, ", "), inputIndex, outputArgumentCount, isLastArgumentError}, nil
}

func (p parameterizedMethod) Call(c context.Context, params []json.RawMessage) (interface{}, *Error) {
	var methodArgs []reflect.Value
	var err error
	if p.requiredArgumentCount > 0 && len(params) > p.requiredArgumentCount {
		return nil, &Error{Code: -32602, Message: fmt.Sprintf("parameters should be (%s)", p.signature)}
	}
	for _, param := range p.parameters {
		methodArgs, err = param.marshal(c, methodArgs, params)
		if err != nil {
			return nil, &Error{Code: -32602, Message: fmt.Sprintf("parameters should be (%s)", p.signature)}
		}
	}
	returnValues := p.method.Call(methodArgs)
	lenResults := p.outputArgumentCount

	if lenResults == 0 {
		return nil, nil
	} else {
		// check if the last value is an error; if it is, then return as an error, otherwise it (and other values) are returned as an array
		var err *Error = nil
		if p.isLastArgumentError {
			// it is an error at the end
			err = marshalError(returnValues[lenResults-1])
			returnValues = returnValues[:lenResults-1]
		}
		var result interface{}
		if len(returnValues) == 1 {
			result = returnValues[0].Interface()
		} else {
			var rslice []interface{}
			for _, rv := range returnValues {
				rslice = append(rslice, rv.Interface())
			}
			result = rslice
		}
		return result, err
	}
}

type parameterizedMethodParameter struct {
	underlying  reflect.Type
	isVariadic  bool
	isContext   bool
	sourceIndex int
	typeName    string
}

func (p parameterizedMethodParameter) marshal(c context.Context, methodArgs []reflect.Value, params []json.RawMessage) ([]reflect.Value, error) {
	lenArgs := len(params)
	if p.isContext {
		return append(methodArgs, reflect.ValueOf(c)), nil
	}

	if !p.isVariadic {
		if lenArgs <= p.sourceIndex {
			return nil, errors.New("Missing required parameter")
		}
		v, err := marshalJSONType(p.underlying, params[p.sourceIndex])
		if err != nil {
			return nil, err
		} else {
			return append(methodArgs, reflect.ValueOf(v)), nil
		}
	}

	// aggregate variadic
	for i := p.sourceIndex; i < lenArgs; i++ {
		v, err := marshalJSONType(p.underlying, params[i])
		if err != nil {
			return nil, err
		} else {
			methodArgs = append(methodArgs, reflect.ValueOf(v))
		}
	}

	return methodArgs, nil
}

func newParameterizedMethodParameter(t reflect.Type, i int, isVariadic bool) (parameterizedMethodParameter, error) {
	isContext := t == reflectionTypeContext
	if isContext {
		return parameterizedMethodParameter{
			underlying:  t,
			isVariadic:  false,
			isContext:   true,
			sourceIndex: i,
			typeName:    "",
		}, nil
	}
	var typename string
	var err error

	if isVariadic {
		t = t.Elem()
	}

	typename, err = getJSONType(t)
	if err != nil {
		return parameterizedMethodParameter{}, err
	}

	if isVariadic {
		typename = fmt.Sprintf("...%s", typename)
	}
	return parameterizedMethodParameter{
		underlying:  t,
		isVariadic:  isVariadic,
		isContext:   false,
		sourceIndex: i,
		typeName:    typename,
	}, nil
}

// getJSONType returns the json type (one of object, array, bool, number, string)
// for objects this includes the _optional_ name (so "object (ExampleStruct)" or "array (number)")
func getJSONType(r reflect.Type) (string, error) {
	switch r.Kind() {
	case reflect.Int:
		fallthrough
	case reflect.Int8:
		fallthrough
	case reflect.Int16:
		fallthrough
	case reflect.Int32:
		fallthrough
	case reflect.Int64:
		fallthrough
	case reflect.Uint:
		fallthrough
	case reflect.Uint8:
		fallthrough
	case reflect.Uint16:
		fallthrough
	case reflect.Uint32:
		fallthrough
	case reflect.Uint64:
		return fmt.Sprintf("number (%s)", r.String()), nil
	case reflect.Float32:
		return "number (float)", nil
	case reflect.Float64:
		return "number", nil
	case reflect.String:
		return "string", nil
	case reflect.Bool:
		return "bool", nil
	case reflect.Array:
		fallthrough
	case reflect.Slice:
		return "array", nil
	case reflect.Map:
		return "object", nil
	case reflect.Struct:
		if m, ok := r.MethodByName("RPCName"); ok {
			reciever := reflect.Zero(r)
			resultValues := m.Func.Call([]reflect.Value{reciever})
			return resultValues[0].Interface().(string), nil
		}
		return fmt.Sprintf("object (%s)", r.Name()), nil
	case reflect.Ptr:
		underlying, err := getJSONType(r.Elem())
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s?", underlying), nil
	}
	return "", errors.New("Unsupported Type")
}

func marshalJSONType(t reflect.Type, v json.RawMessage) (interface{}, error) {
	switch t.Kind() {
	case reflect.Int:
		fallthrough
	case reflect.Int8:
		fallthrough
	case reflect.Int16:
		fallthrough
	case reflect.Int32:
		fallthrough
	case reflect.Int64:
		fallthrough
	case reflect.Uint:
		fallthrough
	case reflect.Uint8:
		fallthrough
	case reflect.Uint16:
		fallthrough
	case reflect.Uint32:
		fallthrough
	case reflect.Uint64:
		fallthrough
	case reflect.Float32:
		fallthrough
	case reflect.Float64:
		fallthrough
	case reflect.String:
		fallthrough
	case reflect.Bool:
		fallthrough
	case reflect.Struct:
		fallthrough
	case reflect.Ptr:
		r := reflect.New(t)
		if err := json.Unmarshal(v, r.Interface()); err != nil {
			if unmarshal, ok := err.(*json.UnmarshalTypeError); ok {
				// error handling
				return nil, newParameterTypeMismatchError(unmarshal)
			}
			return nil, err
		}
		return r.Elem().Interface(), nil
		// migrate to new parameter
	}
	return nil, nil
}

type parameterTypeMismatchError struct {
	message string
}

func (p *parameterTypeMismatchError) Error() string {
	return p.message
}

func newParameterTypeMismatchError(unmarshal *json.UnmarshalTypeError) error {
	var message string
	if unmarshal.Field != "" {
		message = fmt.Sprintf("expected %s at path \"%s\"; got %s", unmarshal.Type.Name(), unmarshal.Field, unmarshal.Value)
	} else {
		message = fmt.Sprintf("expected %s; got %s", unmarshal.Type.Name(), unmarshal.Value)
	}
	return &parameterTypeMismatchError{
		message,
	}
}

func marshalError(e reflect.Value) *Error {
	i := e.Interface()
	if i == nil {
		return nil
	}
	switch v := i.(type) {
	case *Error:
		return v
	case error:
		return &Error{
			Code:    -32000,
			Message: v.Error(),
		}
	default:
		panic(errors.New(fmt.Sprintf("Should not be here; should've checked to see if an error beforehand %v", v)))
	}
}
