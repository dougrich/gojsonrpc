package gojsonrpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

func parseRPCRequests(r *http.Request) ([]Request, error) {

	// do cursory type check
	mimetype := r.Header.Get("Content-Type")
	if mimetype != "application/json" {
		return nil, errorBadContentType{}
	}

	var d json.RawMessage
	// parse out the JSON body
	err := json.NewDecoder(r.Body).Decode(&d)
	if err != nil {
		return nil, errorInvalidJson{err, "wrapper"}
	}

	var requests []Request
	d = bytes.TrimSpace(d)

	if bytes.HasPrefix(d, []byte{'['}) {
		if err := json.Unmarshal(d, &requests); err != nil {
			return nil, errorInvalidJson{err, "array"}
		}
	} else {
		var r Request
		if err := json.Unmarshal(d, &r); err != nil {
			return nil, errorInvalidJson{err, "singleton"}
		}
		requests = append(requests, r)
	}

	return requests, nil
}

type errorBadContentType struct {
	actual string
}

func (b errorBadContentType) Error() string {
	return "Bad content type; expecting application/json"
}

type errorInvalidJson struct {
	underlying  error
	destination string
}

func (e errorInvalidJson) Error() string {
	return fmt.Sprintf("Invalid JSON; an error occured parsing json into %s", e.destination)
}
