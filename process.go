package gojsonrpc

import (
	"context"
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

func (h *Handler) asyncProcessRequest(req *Request, wg *sync.WaitGroup, results chan Result) {
	defer wg.Done()
	method, ok := h.cachedMethods[req.MethodName]
	if !ok {
		results <- Result{
			ID: req.ID,
			Error: &Error{
				Code:    -32601,
				Message: "method not found on server",
			},
			Version: "2.0-x",
		}
		return
	}

	result, err := method.Call(context.Background(), req.Parameters)

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
