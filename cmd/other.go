package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	urlpkg "net/url"
	"os"
	"sync"
	"sync/atomic"

	px "github.com/johnietre/grpc-proxy/go"
	"golang.org/x/net/http2"
	"golang.org/x/term"
)

const (
	passwordEnvName = "GRPC_PROXY_PASSWORD"
)

var (
	httpClient = &http.Client{
		Transport: &http2.Transport{
			DialTLSContext: func(
				ctx context.Context, ntwk, addr string, cfg *tls.Config,
			) (net.Conn, error) {
				return net.Dial(ntwk, addr)
			},
			AllowHTTP: true,
		},
	}
	httpsClient = &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
		},
	}
)

func die(code int, args ...any) {
	fmt.Fprint(os.Stderr, args...)
	os.Exit(code)
}

func dief(code int, format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(code)
}

func dieln(code int, args ...any) {
	fmt.Fprintln(os.Stderr, args...)
	os.Exit(code)
}

func getPassword(prompt ...string) string {
	if len(prompt) != 0 {
		fmt.Print(prompt[0])
	}
	pwd, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		die(2, "Error reading from stdin: ", err)
	}
	return string(pwd)
}

func newReq(
	method string,
	url *urlpkg.URL,
	body io.Reader,
	pwd string,
) *http.Request {
	req, err := http.NewRequest(method, url.String(), body)
	if err != nil {
		die(2, "Error creating request: ", err)
	}
	switch method {
	case http.MethodGet:
		req.Header.Set(px.HeaderBase, "get")
	case http.MethodPost, http.MethodPut:
		req.Header.Set(px.HeaderBase, "refresh")
	case http.MethodDelete:
		req.Header.Set(px.HeaderBase, "shutdown")
	}
	req.Header.Set(px.HeaderPassword, pwd)
	return req
}

func sendReq(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme == "http" {
		return httpClient.Do(req)
	} else {
		return httpsClient.Do(req)
	}
}

func runConcurrent(
	reqs []*http.Request,
	respHandler func(*http.Response) error,
) int {
	var (
		errCount atomic.Int32
		wg       sync.WaitGroup
	)
	for _, req := range reqs {
		wg.Add(1)
		go func(req *http.Request) {
			defer wg.Done()
			resp, err := sendReq(req)
			if err != nil {
				errCount.Add(1)
				return
			}
			if respHandler != nil {
				if err := respHandler(resp); err != nil {
					log.Print(err)
					errCount.Add(1)
				}
				return
			}
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				log.Print("Error reading response body from %s: %v", req.URL, err)
			}
			if resp.StatusCode == http.StatusOK {
				fmt.Printf("%s: SUCCESS")
				if len(body) != 0 {
					fmt.Printf("%s: BODY\n%s\n", req.URL, body)
				}
			} else {
				errCount.Add(1)
				log.Print("Received %d status with body: %s", resp.StatusCode, body)
			}
		}(req)
	}
	wg.Wait()
	return int(errCount.Load())
}
