package gojsonrpc

import (
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"net/http"
	"strings"
	"testing"
)

func must(e error) {
	if e != nil {
		panic(e)
	}
}

func jsonParameterize(parameters []interface{}) []json.RawMessage {
	var p []json.RawMessage
	for _, r := range parameters {
		b, err := json.Marshal(r)
		must(err)
		var rm json.RawMessage
		must(json.Unmarshal(b, &rm))
		p = append(p, rm)
	}
	return p
}

func TestParseRPCRequests(t *testing.T) {
	makerequest := func(r ...Request) *http.Request {
		var body []byte
		var err error
		if len(r) == 1 {
			body, err = json.Marshal(r[0])
		} else {
			body, err = json.Marshal(r)
		}
		must(err)
		req, err := http.NewRequest("POST", "/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		must(err)
		return req
	}

	t.Run("object", func(t *testing.T) {
		assert := assert.New(t)

		src := Request{Version: "2.0-x", MethodName: "test.Add", Parameters: jsonParameterize([]interface{}{float64(1), float64(2), float64(3)}), ID: float64(1)}
		r := makerequest(src)
		result, err := parseRPCRequests(r)
		assert.NoError(err)
		assert.Len(result, 1)
		assert.Equal(result[0].Version, src.Version)
		assert.Equal(result[0].MethodName, src.MethodName)
		assert.Equal(result[0].Parameters, src.Parameters)
		assert.Equal(result[0].ID, src.ID)
	})

	t.Run("array", func(t *testing.T) {
		assert := assert.New(t)

		src := Request{Version: "2.0-x", MethodName: "test.Add", Parameters: jsonParameterize([]interface{}{float64(1), float64(2), float64(3)}), ID: float64(1)}
		r := makerequest(src, src)
		result, err := parseRPCRequests(r)
		assert.NoError(err)
		assert.Len(result, 2)
		assert.Equal(result[0].Version, src.Version)
		assert.Equal(result[0].MethodName, src.MethodName)
		assert.Equal(result[0].Parameters, src.Parameters)
		assert.Equal(result[0].ID, src.ID)
		assert.Equal(result[1].Version, src.Version)
		assert.Equal(result[1].MethodName, src.MethodName)
		assert.Equal(result[1].Parameters, src.Parameters)
		assert.Equal(result[1].ID, src.ID)
	})

	t.Run("BadContentType", func(t *testing.T) {
		assert := assert.New(t)

		src := Request{Version: "2.0-x", MethodName: "test.Add", Parameters: jsonParameterize([]interface{}{float64(1), float64(2), float64(3)}), ID: float64(1)}
		r := makerequest(src)
		r.Header.Set("Content-Type", "invalid/mime")
		result, err := parseRPCRequests(r)
		assert.Nil(result)
		assert.Error(err)
		assert.IsType(errorBadContentType{}, err)
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		assert := assert.New(t)

		r, err := http.NewRequest("POST", "/", strings.NewReader("garbagejson"))
		r.Header.Set("Content-Type", "application/json")
		must(err)
		result, err := parseRPCRequests(r)
		assert.Nil(result)
		assert.Error(err)
		assert.IsType(errorInvalidJson{}, err)
	})
}
