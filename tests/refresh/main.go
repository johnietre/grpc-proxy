package main

import (
	"log"
	"time"

	"github.com/johnietre/grpc-proxy/tests"
	_ "github.com/johnietre/utils/go"
)

var (
  binPath = tests.GetBinPath(false)
)

func main() {
  log.SetFlags(0)
  runTest()
  log.Print("PASSED")
}

func runTest() {
  ctx, cf := tests.TimeoutContext(time.Second * 30)
  defer cf()
  startCmd := tests.NewCommandWithFiles(
    ctx,
    "basic", "start",
    binPath,
    "start",
    "--addr", "127.0.0.1:14210",
    "--config", tests.GetConfigPath("refresh.toml"),
    "--log-stderr", "--log-level=trace",
  )
  if err := startCmd.Start(); err != nil {
    tests.Fatal(err)
  }

  time.Sleep(time.Second * 3)
  ctx, cf = tests.TimeoutContext(time.Second * 5)
  defer cf()
  shutdownCmd := tests.NewCommandWithFiles(
    ctx,
    "basic", "shutdown",
    binPath,
    "shutdown",
    "--addr", "127.0.0.1:14210",
  )
  if err := shutdownCmd.Start(); err != nil {
    tests.Fatal(err)
  }

  /*
  if err := shutdownCmd.Wait(); err != nil {
    tests.Fatal(err)
  }
  if err := startCmd.Wait(); err != nil {
    tests.Fatal(err)
  }
  */
  tests.Wait(
    tests.CmdName{Name: "start", Cmd: startCmd},
    tests.CmdName{Name: "shutdown", Cmd: shutdownCmd},
  ).Wait()
}
