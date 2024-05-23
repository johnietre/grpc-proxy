package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"syscall"

	px "github.com/johnietre/grpc-proxy/go"
	utils "github.com/johnietre/utils/go"
	"github.com/spf13/cobra"
)

func makeShutdownCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shutdown",
		Short: "Shutdown a proxy",
		Run:   runShutdown,
	}
	flags := cmd.Flags()

	flags.StringArray("url", nil, "URL(s) to connect to")
	flags.Bool(
		"force", false,
		"Force proxy to close without waiting for connections to end gracefully",
	)
	flags.Bool(
		"all", false,
		"Send to all servers, no matter if one is successful",
	)
	flags.BoolP(
		"concurrent", "C", false,
		`Send all concurrently (printing out auth errors, implies "--all")`,
	)

	return cmd
}

func runShutdown(cmd *cobra.Command, args []string) {
	flags := cmd.Flags()
	all, _ := flags.GetBool("all")
	force, _ := flags.GetBool("force")
	urlStrs, _ := flags.GetStringArray("url")
	urls := utils.MapSlice(urlStrs, func(s string) *px.URL {
		if !strings.HasPrefix(s, "http") {
			// TODO: Delete?
			s = "https://" + s
		}
		url := &px.URL{}
		if err := url.UnmarshalText([]byte(s)); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing URL %s: %v", s, err)
			os.Exit(1)
		}
		if force {
			values := url.Query()
			values.Set("force", "1")
			url.RawQuery = values.Encode()
		}
		return url
	})
	if concurrent, _ := flags.GetBool("concurrent"); concurrent {
		pwd := os.Getenv(passwordEnvName)
		count := runConcurrent(
			utils.MapSlice(urls, func(url *px.URL) *http.Request {
				return newReq(http.MethodDelete, url.URL, nil, pwd)
			}),
			nil,
		)
		if count != 0 {
			os.Exit(1)
		}
		return
	}
	errCount := 0
	for _, url := range urls {
		pwd := os.Getenv(passwordEnvName)
		for attempts := 0; true; attempts++ {
			req := newReq(http.MethodDelete, url.URL, nil, pwd)
			resp, err := sendReq(req)
			if err != nil {
				if !errors.Is(err, io.ErrUnexpectedEOF) &&
					!errors.Is(err, syscall.ECONNRESET) {
					errCount++
					log.Printf("Error sending request to %s: %v", url, err)
					break
				}
				if !all {
					return
				}
				break
			}
			if resp.StatusCode == 200 {
				if !all {
					return
				}
				break
			} else if resp.StatusCode == http.StatusUnauthorized {
				log.Printf("Incorrect password for %s", url)
				if errCount == 3 {
					log.Printf("Too many failed attempts, continuing")
					break
				}
				pwd = getPassword("Password: ")
				continue
			} else {
				errCount++
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					log.Printf("Error reading response body from %s: %v", url, err)
					body = []byte{}
				}
				log.Printf("Received %d status with body: %s", resp.Status, body)
			}
			resp.Body.Close()
			break
		}
	}
	if errCount != 0 {
		os.Exit(1)
	}
}
