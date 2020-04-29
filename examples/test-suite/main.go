package main

import (
	"github.com/dougrich/gojsonrpc"
	"log"
	"context"
	"net/http"
)

func must(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {

	t := &TestNamespace{}

	h := gojsonrpc.New(gojsonrpc.DefaultNext())
	must(h.AddNamespace("test", t))
	log.Fatal(http.ListenAndServe(":8080", AuthMiddleware(h)))
}

type TestNamespace struct {
}

func (t *TestNamespace) Sum(a int, b int, nums ...int) int {
	total := a + b
	for _, num := range nums {
		total += num
	}
	return total
}

func (t *TestNamespace) SecureSum(c context.Context, a int, b int, nums ...int) (int, error) {
	if !HasUser(c) {
		return 0, &gojsonrpc.Error{ Code: 1003, Message: "current user is not authorized" }
	}

	return t.Sum(a, b, nums...), nil
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func (res http.ResponseWriter, req *http.Request) {
		// get the basic header
		c := req.Context()
		authheader := req.Header.Get("Authorization")
		if authheader != "" {
			c = WithUser(c, &BasicUser{ header: authheader })
			req = req.WithContext(c)
		}
		next.ServeHTTP(res, req)
	})
}

type BasicUser struct {
	header string
}
type contextKey string

func WithUser(c context.Context, user *BasicUser) context.Context {
	return context.WithValue(c, contextKey("user"), user)
}

func HasUser(c context.Context) bool {
	_, ok := c.Value(contextKey("user")).(*BasicUser)
	return ok
}