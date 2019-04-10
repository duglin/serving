package queue

import (
	"net/http"
)

type Status int

const (
	InProgress Status = iota
	Ready
	Failed
)

type SQLDB interface {
	CreateAsyncTable() error
	CreateAsyncReq(guid, podId string) error
	UpdateAsyncReq(guid string, status Status, data []byte, statusCode int) error
	FetchRecord(guid string) (*AsyncCallRecord, error)
	DeleteRecord(guid string) error
}

type AsyncCallRecord struct {
	Guid       string
	Pod        string
	Status     Status
	StatusCode int
	Body       []byte
}

type ResponseCache struct {
	Body       []byte
	StatusCode int
}

func (r *ResponseCache) Write(body []byte) (int, error) {
	r.Body = body
	return len(r.Body), nil
}

func (r *ResponseCache) WriteHeader(code int) {
	r.StatusCode = code
}

func (r *ResponseCache) Header() http.Header {
	return map[string][]string{}
}

func (r *ResponseCache) CloseNotify() <-chan bool {
	return make(chan bool)
}
