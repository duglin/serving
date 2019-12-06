/*
Copyright 2020 The Knative Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
)

// resp implements http.ResponseWriter writing
type dummyResp struct {
	io.Writer
	h       int
	Flushed bool
}

func newDummyResp() http.ResponseWriter {
	return &dummyResp{Writer: &bytes.Buffer{}}
}

func (w *dummyResp) Header() http.Header { return make(http.Header) }
func (w *dummyResp) WriteHeader(h int)   { w.h = h }
func (w *dummyResp) String() string      { return "" }
func (w *dummyResp) Flush()              { w.Flushed = true }

type AsyncHandler struct {
	NextHandler http.Handler
}

func (h *AsyncHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var isAsync bool
	preferHeader := r.Header.Get("Prefer")
	if preferHeader != "" {
		isAsync = strings.Contains(preferHeader, "respond-async")
	}

	if !isAsync {
		h.NextHandler.ServeHTTP(w, r)
		return
	}
	r = r.Clone(context.Background())
	go func() {
		h.NextHandler.ServeHTTP(newDummyResp(), r)
	}()

	w.WriteHeader(http.StatusAccepted)
}
