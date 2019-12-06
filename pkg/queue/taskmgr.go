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

package queue

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"knative.dev/pkg/network/handlers"
)

// TaskmgrPrefix is the executable prefix used when
// invoking the Queue in taskmgr mode.
const TaskmgrPrefix = "/knative"

// Check that the binary exists and that it is executable.
// This is to guard against common error cases.
func checkValidBinary(binary string) error {
	stat, err := os.Stat(binary)
	if err != nil {
		return fmt.Errorf("Unable to stat %q: %v", binary, err)
	}
	if stat.Mode()&0111 == 0 {
		return fmt.Errorf("%q is not executable", binary)
	}
	return nil
}

// IsTaskmgr checks whether the queue has been invoked as a "taskmgr"
func IsTaskmgr() bool {
	return filepath.Dir(os.Args[0]) == TaskmgrPrefix
}

// Taskmgr start up the queue in taskmgr mode.
// This should only be called when IsTaskmgr is true.
func Taskmgr(ctx context.Context) {
	h := &handlers.Drainer{Inner: &TaskHandler{}}

	server := http.Server{
		Addr:    ":" + os.Getenv("PORT"),
		Handler: h,
	}
	go server.ListenAndServe()

	// When the context is cancelled, start to drain.
	<-ctx.Done()
	h.Drain()
	server.Shutdown(context.Background())
}

type TaskHandler struct{}

func (t TaskHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// log.Printf("Got a request\n")

	if os.Getenv("K_TYPE") == "fnode" {
		res, err := http.Get("http://cos.fun.cloud.ibm.com/apps/app.js")
		if err != nil {
			body, err := ioutil.ReadAll(res.Body)
			if err != nil {
				err = ioutil.WriteFile("/app/app.js", body, 0777)
			}
		}
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error: %s\n", err)
		}
	}

	if os.Getenv("K_TYPE") == "pull" {
		// PULL
		taskCmd := []string{"/app"}
		taskEnv := os.Environ()

		// os.Args[1] is a JSON serialization of the ENTRYPOINT cmd
		if len(os.Args) > 1 {
			taskCmd = os.Args[1:]
		}

		for name, values := range r.Header {
			name = strings.ToUpper(name)

			// Not sure this can ever happen but just in case
			if len(values) == 0 {
				continue
			}

			// Env vars are copied and there could be lots
			if name == "K_ENV" {
				for _, value := range values {
					taskEnv = append(taskEnv, value)
				}
				continue
			}

			if strings.HasPrefix(name, "K_ARG_") {
				continue
			}

			// All others are just copied - only assume only one value
			if strings.HasPrefix(name, "K_") {
				taskEnv = append(taskEnv, name+"="+values[0])
			}
		}

		// Use the incoming URL "Path" as the args
		for _, part := range strings.Split(r.URL.Path, "/") {
			part, err := url.PathUnescape(part)
			if part == "" || err != nil {
				continue
			}
			taskCmd = append(taskCmd, part)
		}

		// Use the incoming URL "Query Params" as flags
		for key, values := range r.URL.Query() {
			for _, val := range values {
				// Only single char keys and vals of "" map to - flags
				if val == "" && len(key) == 1 {
					taskCmd = append(taskCmd, fmt.Sprintf("-%s", key))
				} else {
					if val != "" {
						val = "=" + val
					}
					taskCmd = append(taskCmd, fmt.Sprintf("--%s%s", key, val))
				}
			}
		}

		// Append any K_ARG_# env vars as args to the cmd line
		for i := 1; ; i++ {
			arg, ok := r.Header[fmt.Sprintf("K_arg_%d", i)]
			if !ok {
				break
			}
			taskCmd = append(taskCmd, arg[0])
		}

		tmpURL := r.URL
		tmpURL.Host = r.Host

		headersJson, _ := json.Marshal(r.Header)
		taskEnv = append(taskEnv, "K_HEADERS="+string(headersJson))
		taskEnv = append(taskEnv, "K_URL="+tmpURL.String())
		taskEnv = append(taskEnv, "K_METHOD="+r.Method)

		// Normally we loop until we want to stop reading from Queue
		for i := 0; i < 5; i++ {
			str := fmt.Sprintf("{ cool CloudEvent #%d }\n", i+1)

			var outBuf bytes.Buffer
			var outWr io.Writer
			var inRd io.Reader

			inRd = bytes.NewReader([]byte(str))
			outBuf = bytes.Buffer{}
			outWr = bufio.NewWriter(&outBuf)

			cmd := exec.Command(taskCmd[0], taskCmd[1:]...)
			cmd.Env = taskEnv

			/*
				cmd := exec.Cmd{
					Path: taskCmd[0],
					Args: taskCmd[0:],
					Env:  taskEnv,
					// Stdin:  inRd,  // bytes.NewReader(body),
					// Stdout: outWr, // os.Stdout, // buffer these
					// Stderr: outWr, // os.Stderr,
				}
			*/

			cmd.Stdin = inRd
			cmd.Stdout = outWr
			// cmd.Stderr = outWr
			// err := cmd.Run()
			cmd.Run()
			// 'err' is any possible error from trying to run the command

			/*
				if err == nil { // Worked
				fmt.Printf("Passed\n")
				} else { // Command failed
				fmt.Printf("Failed\n")
				}
			*/

			if outBuf.Len() > 0 {
				// log.Printf("Output:\n%s\n", string(outBuf.Bytes()))
				fmt.Printf("%s", outBuf.Bytes())
			}

			time.Sleep(1 * time.Second)
		}

		// 1/2 the time fail
		if time.Now().Unix()%2 == 0 {
			w.WriteHeader(http.StatusInternalServerError)
		}
	} else {
		// PUSH/TASK
		taskCmd := []string{}
		taskEnv := os.Environ()

		doStream := false
		for _, v := range r.Header["Upgrade"] {
			if strings.HasPrefix(v, "websocket") {
				doStream = true
				taskEnv = append(taskEnv, "K_STREAM=true")
				break
			}
		}

		// os.Args[1] is a JSON serialization of the ENTRYPOINT cmd
		if len(os.Args) > 1 {
			taskCmd = os.Args[1:]
		}

		for name, values := range r.Header {
			name = strings.ToUpper(name)

			// Not sure this can ever happen but just in case
			if len(values) == 0 {
				continue
			}

			// Env vars are copied and there could be lots
			if name == "K_ENV" {
				for _, value := range values {
					taskEnv = append(taskEnv, value)
				}
				continue
			}

			if strings.HasPrefix(name, "K_ARG_") {
				continue
			}

			// Grab all K_ ones - only assume only one value
			if strings.HasPrefix(name, "K_") {
				taskEnv = append(taskEnv, name+"="+values[0])
			}

			// Grab all CloudEvent ones - only assume only one value
			if strings.HasPrefix(name, "CE-") {
				taskEnv = append(taskEnv,
					strings.ReplaceAll(name, "-", "_")+"="+values[0])
			}
		}

		// Use the incoming URL "Query Params" as flags
		for key, values := range r.URL.Query() {
			for _, val := range values {
				// Only single char keys and vals of "" map to - flags
				if val == "" && len(key) == 1 {
					taskCmd = append(taskCmd, fmt.Sprintf("-%s", key))
				} else {
					if val != "" {
						val = "=" + val
					}
					taskCmd = append(taskCmd, fmt.Sprintf("--%s%s", key, val))
				}
			}
		}

		// Use the incoming URL "Path" as the args
		for _, part := range strings.Split(r.URL.Path, "/") {
			part, err := url.PathUnescape(part)
			if part == "" || err != nil {
				continue
			}
			taskCmd = append(taskCmd, part)
		}

		// Append any K_ARG_# env vars as args to the cmd line
		for i := 1; ; i++ {
			arg, ok := r.Header[fmt.Sprintf("K_arg_%d", i)]
			if !ok {
				break
			}
			taskCmd = append(taskCmd, arg[0])
		}

		if len(taskCmd) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Missing command to run\n"))
			return
		}

		tmpURL := r.URL
		tmpURL.Host = r.Host

		headersJson, _ := json.Marshal(r.Header)
		taskEnv = append(taskEnv, "K_HEADERS="+string(headersJson))
		taskEnv = append(taskEnv, "K_URL="+tmpURL.String())
		taskEnv = append(taskEnv, "K_METHOD="+r.Method)

		var outBuf bytes.Buffer
		var outWr io.Writer
		var inRd io.Reader
		var conn *websocket.Conn
		var err error

		if !doStream {
			inRd = r.Body
			outBuf = bytes.Buffer{}
			outWr = bufio.NewWriter(&outBuf)
		} else {
			upgrader := websocket.Upgrader{}
			conn, err = upgrader.Upgrade(w, r, nil)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error() + "\n"))
				return
			}
			defer conn.Close()
		}

		cmd := exec.Command(taskCmd[0], taskCmd[1:]...)
		cmd.Env = taskEnv

		/*`
		cmd := exec.Cmd{
			Path: taskCmd[0],
			Args: taskCmd[0:],
			Env:  taskEnv,
			// Stdin:  inRd,  // bytes.NewReader(body),
			// Stdout: outWr, // os.Stdout, // buffer these
			// Stderr: outWr, // os.Stderr,
		}
		*/

		if !doStream {
			cmd.Stdin = inRd
			cmd.Stdout = outWr
			// cmd.Stderr = outWr
			err = cmd.Run()
		} else {
			stdin, _ := cmd.StdinPipe()
			stdout, _ := cmd.StdoutPipe()
			// stderr, _ := cmd.StderrPipe()
			go func() {
				for {
					_, p, err := conn.ReadMessage()
					if len(p) > 0 {
						stdin.Write(p)
						stdin.Write([]byte("\n"))
					}
					if err != nil {
						break
					}
				}
				stdin.Close()
			}()
			lock := sync.Mutex{}
			go func() {
				buf := make([]byte, 1024)
				for {
					n, err := stdout.Read(buf)
					if n <= 0 && err != nil {
						break
					}
					outBuf.Write([]byte(buf[:n]))
					s := []byte(strings.TrimRight(string(buf[:n]), "\n\r"))
					lock.Lock()
					conn.WriteMessage(websocket.TextMessage, s)
					lock.Unlock()
				}
				stdin.Close()
			}()
			/*
				go func() {
					buf := make([]byte, 1024)
					for {
						n, err := stderr.Read(buf)
						if n <= 0 && err != nil {
							break
						}
						outBuf.Write([]byte(buf[:n]))
						s := []byte(strings.TrimRight(string(buf[:n]), "\n\r"))
						lock.Lock()
						conn.WriteMessage(websocket.TextMessage, s)
						lock.Unlock()
					}
					stdin.Close()
				}()
			*/

			err = cmd.Start()
			if err == nil {
				err = cmd.Wait()
			}
			log.Printf("Stream ended\n")
		}
		// 'err' is any possible error from trying to run the command

		// err := cmd.Run()
		if err == nil { // Worked
			if !doStream {
				w.WriteHeader(http.StatusOK)
			}
		} else { // Command failed
			// jobName := r.Header.Get("K_JOB_NAME")
			// log.Printf("Error(%s/%s,%s): %s\n", jobName, jobID, index, err)
			if !doStream {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error() + "\n"))
			}
		}

		if outBuf.Len() > 0 {
			if !doStream {
				w.Write(outBuf.Bytes())
			}
			// log.Printf("Output:\n%s\n", string(outBuf.Bytes()))
			os.Stdout.Write(outBuf.Bytes())
		}
	}
}
