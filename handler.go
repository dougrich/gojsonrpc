package gojsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"reflect"
)

type Request struct {
	Version    string            `json:"jsonrpc"`
	MethodName string            `json:"method"`
	Parameters []json.RawMessage `json:"params"`
	ID         interface{}       `json:"id"`
}

type Result struct {
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
	Version string      `json:"jsonrpc"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e Error) Error() string {
	return e.Message
}

func New(next HandlerNext) *Handler {
	return &Handler{
		next,
		make(map[string]*parameterizedMethod),
	}
}

type Handler struct {
	next          HandlerNext
	cachedMethods map[string]*parameterizedMethod
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	requests, err := parseRPCRequests(r)
	if err != nil {
		c := context.WithValue(r.Context(), "error", err)
		r = r.WithContext(c)
		switch err.(type) {
		case errorBadContentType:
			h.next.BadContentType.ServeHTTP(w, r)
		case errorInvalidJson:
			h.next.InvalidJSON.ServeHTTP(w, r)
		default:
			h.next.InternalServerError.ServeHTTP(w, r)
		}
		return
	}

	// for each request
	// - create a new goroutine to solve it
	// - wait until resolved
	results, err := h.processRequests(r.Context(), requests)
	if err != nil {
		panic(err)
	}

	var b []byte
	// serialize result; if one value, then just respond with that; otherwise respond with array
	if len(requests) == 1 {
		b, err = json.Marshal(results[0])
	} else {
		b, err = json.Marshal(results)
	}

	if err != nil {
		// something terrible happened
		panic(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

func (h *Handler) AddNamespace(name string, object interface{}) error {
	v := reflect.ValueOf(object)
	if !v.IsValid() {
		return errors.New(fmt.Sprintf("Invalid value for namespace: %v", v))
	}
	// iterate over every method in the namespace
	t := v.Type()
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		methodname := fmt.Sprintf("%s.%s", name, m.Name)
		parameterized, err := newParameterizedMethod(v.Method(i))
		if err != nil {
			return err
		}
		h.cachedMethods[methodname] = parameterized
	}
	return nil
}

type HandlerNext struct {
	BadContentType      http.Handler
	InvalidJSON         http.Handler
	InternalServerError http.Handler
}

func DefaultNext() HandlerNext {
	return HandlerNext{
		BadContentType: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, http.StatusText(http.StatusUnsupportedMediaType), http.StatusUnsupportedMediaType)
		}),
		InvalidJSON: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		}),
		InternalServerError: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Print(r.Context().Value("error"))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}),
	}
}
