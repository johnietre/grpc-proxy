package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	px "github.com/johnietre/grpc-proxy/go"
	"github.com/spf13/cobra"
)

const (
	genConfigName = "grpc-proxy.toml"
)

func makeGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a template config file.",
		Args:  cobra.MaximumNArgs(0),
		Run:   runGenerate,
	}
	flags := cmd.Flags()
	flags.StringP(
		"out", "o",
		genConfigName, "Name of file or directory to put config in",
	)
	flags.String("url", "", "URL of proxy to generate config from")

	return cmd
}

func runGenerate(cmd *cobra.Command, args []string) {
	flags := cmd.Flags()
	out, _ := flags.GetString("out")
	urlStr, _ := flags.GetString("url")
	if urlStr != "" {
		generateFromUrl(urlStr, out)
		return
	}
	generateFromFile(out)
}

func generateFromUrl(urlStr, out string) {
	handleResp := func(resp *http.Response) {
		defer resp.Body.Close()
		req := resp.Request
		if resp.StatusCode != http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				body = []byte{}
				log.Printf("Error reading response body from %s: %v", req.URL, err)
			}
			log.Fatalf(
				"Received %d status with body: %s", resp.StatusCode, body,
			)
		}
		if out == "" {
			out = genConfigName
		}
		f, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY, 0755)
		if err != nil {
			log.Fatalf("Error opening file for %s: %v", req.URL, err)
		}
		defer f.Close()
		if _, err := io.Copy(f, resp.Body); err != nil {
			log.Fatalf("Error writing response for %s: %v", req.URL, err)
		}
	}

	if !strings.HasPrefix(urlStr, "http") {
		// TODO: Delete?
		urlStr = "https://" + urlStr
	}
	url := &px.URL{}
	if err := url.UnmarshalText([]byte(urlStr)); err != nil {
		log.Fatalf("Error parsing URL %s: %v", urlStr, err)
	}
	query := url.URL.Query()
	query.Add("config", "true")
	url.URL.RawQuery = query.Encode()

	pwd := os.Getenv(passwordEnvName)
	errCount := 0
	for attempts := 0; true; attempts++ {
		req := newReq(http.MethodGet, url.URL, nil, pwd)
		resp, err := sendReq(req)
		if err != nil {
			errCount++
			log.Printf("Error sending request to %s: %v", url, err)
			if errCount == 3 {
				log.Fatal("Too many failed attempts")
			}
		}
		if resp.StatusCode == http.StatusUnauthorized {
			log.Printf("Incorrect password for %s", url)
			if errCount == 3 {
				log.Fatal("Too many failed attempts")
			}
			pwd = getPassword("Password: ")
			continue
		}
		handleResp(resp)
		break
	}
}

func generateFromFile(out string) {
	_, thisFile, _, _ := runtime.Caller(0)
	defaultConfigPath := filepath.Join(
		filepath.Dir(filepath.Dir(filepath.Dir(thisFile))),
		"EXAMPLE_CONFIG.toml",
	)

	if out == "" {
		out = genConfigName
	}
	stat, err := os.Lstat(out)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Fatal("Error generating config: ", err)
		}
	} else if stat.IsDir() {
		out = filepath.Join(out, genConfigName)
	}

	bytes, err := os.ReadFile(defaultConfigPath)
	if err != nil {
		log.Fatal("Error reading example config: ", err)
	}
	if err := os.WriteFile(out, bytes, 0666); err != nil {
		log.Fatal("Error generating config: ", err)
	}
}
