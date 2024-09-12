package main

import (
	"fmt"
	"log"
	"os"

	px "github.com/johnietre/grpc-proxy/go"
	"github.com/spf13/cobra"
)

func makeStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a proxy",
		Run:   runStart,
	}
	flags := cmd.Flags()

	flags.StringP("config", "c", "", "Path to config file")

	flags.Var(&px.Addr{}, "addr", "Address to run Srvr0 on")
	flags.String("cert", "", "Path to cert.pem file for Srvr0")
	flags.String("key", "", "Path to key.pem file for Srvr0")
	flags.String(
		"global-cert", "",
		"Path to cert.pem file to make available globally",
	)
	flags.String(
		"global-key", "",
		"Path to key.pem file to make available globally",
	)
	flags.Bool(
		"no-password", false,
		fmt.Sprintf(
			`Disables the password for the proxy's base path ("/"). `+
				`The password can be set using the %s environment variable`,
			passwordEnvName,
		),
	)
	cmd.MarkFlagsRequiredTogether("cert", "key")
	cmd.MarkFlagsRequiredTogether("global-cert", "global-key")

	return cmd
}

func runStart(cmd *cobra.Command, args []string) {
	if false {
		log.SetFlags(log.Lshortfile)
	}

	flags := cmd.Flags()
	appCfg := &px.AppConfig{}

	cfgPath, _ := flags.GetString("config")
	if cfgPath != "" {
		cfg, err := px.LoadConfigAndCheck(cfgPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing config: %v\n", err)
			os.Exit(1)
		}
		appCfg.ConfigPath, appCfg.Config = cfgPath, cfg
	}

	if np, _ := flags.GetBool("no-password"); !np {
		appCfg.Password = os.Getenv(passwordEnvName)
	}

	addr := cmd.Flag("addr").Value.(*px.Addr)
	cert, _ := flags.GetString("cert")
	key, _ := flags.GetString("key")
	if addr.TCPAddr != nil {
		appCfg.Srvr0 = &px.ListenerConfig{
			Addr:     addr,
			CertPath: cert,
			KeyPath:  key,
		}
	} else if cert != "" {
		// TODO
		fmt.Fprintf(
			os.Stderr,
			`Must provide "--addr" flag to use "--cert" and "--key" flags`,
		)
		os.Exit(1)
	}

	globalCert, _ := flags.GetString("global-cert")
	globalKey, _ := flags.GetString("global-key")
	if globalCert != "" {
		appCfg.GlobalCertPath, appCfg.GlobalKeyPath = globalCert, globalKey
	}

	app := px.NewApp(appCfg)
	app.Run()
}
