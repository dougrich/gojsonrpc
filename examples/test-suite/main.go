package main

import (
	"github.com/dougrich/gojsonrpc"
	"log"
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
	log.Fatal(http.ListenAndServe(":8080", h))
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
