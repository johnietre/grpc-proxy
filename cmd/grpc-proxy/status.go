package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	px "github.com/johnietre/grpc-proxy/go"
	utils "github.com/johnietre/utils/go"
	"github.com/spf13/cobra"
)

func makeStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Get configs and/or statuses for a proxy/proxies.",
		Args:  cobra.MaximumNArgs(0),
		Run:   runStatus,
	}
	flags := cmd.Flags()
	flags.BoolP(
		"concurrent", "C", false,
		`Send all concurrently (printing out auth errors, implies "--all")`,
	)
	flags.StringP(
		"out", "o", "",
		"Directory to put the new config(s) returned by the server(s) in; if not specified, output is directed to stdout",
	)
	flags.StringArray("url", nil, "URL(s) to connect to")
	flags.Bool("no-status", false, "Don't get statuses")
	flags.Bool("config", false, "Get configs")

	return cmd
}

func runStatus(cmd *cobra.Command, args []string) {
	flags := cmd.Flags()
	all, _ := flags.GetBool("all")
	//action := "refresh"
	action := "set"
	if add, _ := flags.GetBool("add"); add {
		action = "add"
	} else if del, _ := flags.GetBool("del"); del {
		action = "del"
	}
	outDir, _ := flags.GetString("out")

	getCfg, _ := flags.GetBool("config")
	noStatus, _ := flags.GetBool("no-status")

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
		query := url.Query()
		if getCfg {
			query.Add("config", "true")
		}
		if !noStatus {
			query.Add("status", "true")
		}
		url.RawQuery = query.Encode()
		return url
	})
	if len(urls) == 0 {
		log.Fatal("Must provide at least one URL")
	}

	handleResp := func(resp *http.Response) error {
		defer resp.Body.Close()
		req := resp.Request
		if resp.StatusCode != http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				body = []byte{}
				log.Printf("Error reading response body from %s: %v", req.URL, err)
			}
			return fmt.Errorf(
				"Received %d status with body: %s", resp.StatusCode, body,
			)
		}
		query := req.URL.Query()
		for k, v := range query {
			vals := utils.NewSlice(v)
			if k == "config" && getCfg {
				vals.RemoveFirst(func(s string) bool {
					return s == "true"
				})
			} else if k == "status" && !noStatus {
				vals.RemoveFirst(func(s string) bool {
					return s == "true"
				})
			}
			query[k] = vals.Data()
		}
		req.URL.RawQuery = query.Encode()
		urlStr := req.URL.String()

		cfgLenStr := resp.Header.Get(px.HeaderConfigLen)
		statusLenStr := resp.Header.Get(px.HeaderStatusLen)
		cfgLen, statusLen, err := int64(0), int64(0), error(nil)
		if cfgLenStr != "" {
			cfgLen, err = strconv.ParseInt(cfgLenStr, 10, 64)
			if err != nil {
				log.Printf(
					"Received invalid config length from %s: %s",
					urlStr,
					cfgLenStr,
				)
				return nil
			}
		}
		if statusLenStr != "" {
			statusLen, err = strconv.ParseInt(statusLenStr, 10, 64)
			if err != nil {
				log.Printf(
					"Received invalid status length from %s: %s",
					urlStr,
					statusLenStr,
				)
				return nil
			}
		}

		fmt.Printf("%s: SUCCESS\n", urlStr)
		if cfgLen != 0 {
			f := os.Stdout
			if outDir != "" {
				var err error
				f, err = os.OpenFile(
					filepath.Join(outDir, resp.Request.URL.Host),
					os.O_CREATE|os.O_WRONLY,
					0755,
				)
				if err != nil {
					return fmt.Errorf("Error opening file for %s: %v", req.URL, err)
				}
				defer f.Close()
			}
			if _, err := io.CopyN(f, resp.Body, cfgLen); err != nil {
				return fmt.Errorf("Error writing response for %s: %v", req.URL, err)
			}
		}
		if statusLen != 0 {
			f := os.Stdout
			if outDir != "" {
				var err error
				f, err = os.OpenFile(
					filepath.Join(outDir, resp.Request.URL.Host+".statuses"),
					os.O_CREATE|os.O_WRONLY,
					0755,
				)
				if err != nil {
					return fmt.Errorf("Error opening file for %s: %v", req.URL, err)
				}
				defer f.Close()
			} else if cfgLen != 0 {
				fmt.Println()
			}
			if _, err := io.CopyN(f, resp.Body, statusLen); err != nil {
				return fmt.Errorf("Error writing response for %s: %v", req.URL, err)
			}
		}
		return nil
	}

	if concurrent, _ := flags.GetBool("concurrent"); concurrent {
		pwd := os.Getenv(passwordEnvName)
		count := runConcurrent(
			utils.MapSlice(urls, func(url *px.URL) *http.Request {
				req := newReq(http.MethodGet, url.URL, nil, pwd)
				req.Header.Set(px.HeaderRefreshAction, action)
				return req
			}),
			handleResp,
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
			req := newReq(http.MethodGet, url.URL, nil, pwd)
			resp, err := sendReq(req)
			if err != nil {
				errCount++
				log.Printf("Error sending request to %s: %v", url, err)
				break
			}
			if resp.StatusCode == http.StatusUnauthorized {
				log.Printf("Incorrect password for %s", url)
				if errCount == 3 {
					log.Printf("Too many failed attempts, continuing")
					break
				}
				pwd = getPassword("Password: ")
				continue
			}
			if err := handleResp(resp); err != nil {
				errCount++
				log.Print(err)
			} else if !all {
				return
			}
			break
		}
	}
	if errCount != 0 {
		os.Exit(1)
	}
}
