package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	px "github.com/johnietre/grpc-proxy/go"
	utils "github.com/johnietre/utils/go"
	"github.com/spf13/cobra"
)

func makeRefreshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Refresh a proxy",
		Long:  `Refresh a proxy. Without specifying "--add" or "--del", the config on each proxy specified is completely replaced.`,
		Args:  cobra.MaximumNArgs(0),
		Run:   runRefresh,
	}
	flags := cmd.Flags()

	flags.StringArray("url", nil, "URL(s) to connect to")
	flags.StringP("config", "c", "", "Path to config file to send")
	flags.Bool(
		"add", false,
		"Add the content of the specified config to the address(es)",
	)
	flags.Bool(
		"del", false,
		"Delete the content of the specified config from the address(es)",
	)
	flags.Bool(
		"all", false,
		"Send to all servers, no matter if one is successful",
	)
	flags.BoolP(
		"concurrent", "C", false,
		`Send all concurrently (printing out auth errors, implies "--all")`,
	)
	flags.StringP(
		"out", "o", "",
		"Directory to put the new config(s) returned by the server(s) in; if not specified, output is directed to stdout",
	)
	cmd.MarkFlagsMutuallyExclusive("add", "del")

	return cmd
}

func runRefresh(cmd *cobra.Command, args []string) {
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

	cfgStr := ""
	if cfgPath, _ := flags.GetString("config"); cfgPath != "" {
		cfg, err := px.LoadConfigAndCheck(cfgPath)
		if err != nil {
			log.Fatal("Error loading config: ", err)
		}
		cfgStr, err = cfg.EncodeString()
		if err != nil {
			log.Fatal("Error serializing config: ", err)
		}
	}
	makeBody := func() io.Reader {
		if len(cfgStr) == 0 {
			return nil
		}
		return strings.NewReader(cfgStr)
	}

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
		fmt.Printf("%s: SUCCESS\n", req.URL)
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
		if _, err := io.Copy(f, resp.Body); err != nil {
			return fmt.Errorf("Error writing response for %s: %v", req.URL, err)
		}
		return nil
	}

	if concurrent, _ := flags.GetBool("concurrent"); concurrent {
		pwd := os.Getenv(passwordEnvName)
		count := runConcurrent(
			utils.MapSlice(urls, func(url *px.URL) *http.Request {
				req := newReq(http.MethodPut, url.URL, makeBody(), pwd)
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
			req := newReq(http.MethodPut, url.URL, makeBody(), pwd)
			req.Header.Set(px.HeaderRefreshAction, action)
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
