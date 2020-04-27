package gojsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

func (r Request) getNamespaceFunction() (string, string) {
	v := strings.SplitN(r.MethodName, ".", 2)
	if len(v) == 0 {
		return "", ""
	} else if len(v) == 1 {
		return v[0], ""
	} else {
		return v[0], v[1]
	}
}

func marshalParameter(t reflect.Type, i int, v json.RawMessage, parameterMessage string) (interface{}, *Error) {
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
	case reflect.Array:
		fallthrough
	case reflect.Map:
		fallthrough
	case reflect.Slice:
		// attempt to unmarshal json into the new type; if an error occurs reject with invalid parameter
		r := reflect.New(t)
		if err := json.Unmarshal(v, r.Interface()); err != nil {
			return nil, &Error{Code: -32602, Message: parameterMessage}
		}
		return r.Elem().Interface(), nil
	case reflect.Interface:
		fallthrough
	case reflect.Func:
		fallthrough
	case reflect.Chan:
		fallthrough
	case reflect.Complex64:
		fallthrough
	case reflect.Complex128:
		fallthrough
	case reflect.Ptr:
		fallthrough
	case reflect.UnsafePointer:
		fallthrough
	default:
		return nil, &Error{Code: -32602, Message: parameterMessage}
	}
}

func callFunction(m reflect.Value, c context.Context, parameters []json.RawMessage) (interface{}, *Error) {
	// todo: actual context here
	t := m.Type()
	// create the parameters error message
	lenArgs := t.NumIn()
	isVariadic := t.IsVariadic()

	var argumentList []string
	for i := 0; i < lenArgs; i++ {
		var nthArgType reflect.Type
		if isVariadic && i == lenArgs - 1 {
			nthArgType = t.In(lenArgs - 1).Elem()
		} else {
			nthArgType = t.In(i)
		}
		typename := nthArgType.String()
		if isVariadic && i == lenArgs - 1 {
			typename = fmt.Sprintf("...%s", typename)
		}
		argumentList = append(argumentList, typename)
	}
	argsList := strings.Join(argumentList, ", ")
	parameterMessage := fmt.Sprintf("parameters should be (%s)", argsList)

	var in []reflect.Value
	inIndex := 0
	firstArgType := t.In(inIndex)
	if firstArgType.Kind() == reflect.Interface && reflect.TypeOf(c).Implements(firstArgType) {
		in = append(in, reflect.ValueOf(c))
		inIndex++
	}

	lenParams := len(parameters)
	if !isVariadic {
		// assert that length is not greater than our total length
		if lenParams > lenArgs-inIndex {
			return nil, &Error{Code: -32602, Message: parameterMessage}
		}
	}
	if lenParams < lenArgs-inIndex {
		// assert that we have enough arguments
		return nil, &Error{Code: -32602, Message: parameterMessage}
	}


	for i, p := range parameters {
		var nthArgType reflect.Type
		if isVariadic && inIndex == (lenArgs-1) {
			nthArgType = t.In(lenArgs - 1).Elem()
		} else {
			nthArgType = t.In(inIndex)
			inIndex++
		}
		v, err := marshalParameter(nthArgType, i, p, parameterMessage)
		if err != nil {
			return nil, err
		}
		in = append(in, reflect.ValueOf(v))
	}

	returnValues := m.Call(in)
	lenResults := t.NumOut()

	if lenResults == 0 {
		return nil, nil
	} else if lenResults == 1 {
		return returnValues[0].Interface(), nil
	} else {
		// check if the last value is an error; if it is, then return as an error, otherwise it (and other values) are returned as an array
		tEnd := t.Out(lenResults - 1)
		var err *Error = nil
		if reflect.TypeOf(errors.New("")).Implements(tEnd) {
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

func (h *Handler) asyncProcessRequest(req *Request, wg *sync.WaitGroup, results chan Result) {
	defer wg.Done()
	namespaceName, fn := req.getNamespaceFunction()
	namespace, ok := h.namespaces[namespaceName]
	if !ok {
		results <- Result{
			ID:      req.ID,
			Error:   &Error{
				Code: -32601,
				Message: "method not found on server",
			},
			Version: "2.0-x",
		}
		return
	}

	m := namespace.MethodByName(fn)
	if !m.IsValid() {
		results <- Result{
			ID:      req.ID,
			Error:   &Error{
				Code: -32601,
				Message: "method not found on server",
			},
			Version: "2.0-x",
		}
		return
	}

	result, err := callFunction(m, context.Background(), req.Parameters)

	results <- Result{
		ID:      req.ID,
		Result:  result,
		Error:   err,
		Version: "2.0-x",
	}
}

func (h *Handler) processRequests(requests []Request) ([]Result, error) {
	wg := sync.WaitGroup{}
	rchan := make(chan Result)
	wg.Add(len(requests))
	for i := range requests {
		go h.asyncProcessRequest(&requests[i], &wg, rchan)
	}
	go func() {
		wg.Wait()
		close(rchan)
	}()
	var results []Result
	for r := range rchan {
		results = append(results, r)
	}

	return results, nil
}
