package gojsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"github.com/gorilla/websocket"
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
		websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}
}

type Handler struct {
	next          HandlerNext
	cachedMethods map[string]*parameterizedMethod
	upgrader websocket.Upgrader
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if websocket.IsWebSocketUpgrade(r) {
		h.ServeWebsocket(w, r)
		return
	}

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

func (h *Handler) ServeWebsocket(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	defer conn.Close()
	if err != nil {
		log.Println(err)
		return
	}
	writer := make(chan interface{})
	go func() {
		for {
			msg, ok := <- writer
			if !ok {
				return
			}
			if err = conn.WriteJSON(msg); err != nil {
				panic(err)
			}
		}
	}()
	for {
		_, p, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("error: %v, user-agent: %v", err, r.Header.Get("User-Agent"))
			}
			close(writer)
			return
		}
		go func(p []byte) {
			var req Request
			if err := json.Unmarshal(p, &req); err != nil {
				panic(err)
			}
			results, err := h.processRequests(r.Context(), []Request{req})
			if err != nil {
				panic(err)
			}
			for _, res := range results {
				writer <- res
			}
		}(p)
	}
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
