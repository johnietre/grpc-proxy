package main

import (
	"log"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	log.SetFlags(0)

	rootCmd := &cobra.Command{
		Use: "grpc-proxy",
	}
	rootCmd.AddCommand(
		makeGenerateCmd(), makeStartCmd(), makeRefreshCmd(), makeShutdownCmd(),
	)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
