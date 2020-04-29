package gojsonrpc

import (
	"github.com/stretchr/testify/assert"
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
