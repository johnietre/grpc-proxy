package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

const (
	genConfigName = "grpc-proxy.toml"
)

func makeGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a template config file.",
		Run:   runGenerate,
	}
	flags := cmd.Flags()
	flags.StringP(
		"out", "o",
		genConfigName, "Name of file or directory to put config in",
	)

	return cmd
}

func runGenerate(cmd *cobra.Command, args []string) {
	_, thisFile, _, _ := runtime.Caller(0)
	defaultConfigPath := filepath.Join(
		filepath.Dir(filepath.Dir(thisFile)),
		"EXAMPLE_CONFIG.toml",
	)

	flags := cmd.Flags()
	out, _ := flags.GetString("out")
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
