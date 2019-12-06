package queue

import (
	"net/http"
)

type ResponseCache struct {
}

func (r *ResponseCache) Write(body []byte) (int, error) {
	return len(body), nil
}

func (r *ResponseCache) WriteHeader(code int) {
}

func (r *ResponseCache) Header() http.Header {
	return map[string][]string{}
}

func (r *ResponseCache) CloseNotify() <-chan bool {
	return make(chan bool)
}

func (r *ResponseCache) Flush() {
}
