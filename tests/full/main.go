package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	pb "github.com/johnietre/grpc-proxy/tests/proto"
	utils "github.com/johnietre/utils/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
  proxyAddr = "127.0.0.1:13100"
  configPath = "grpc-proxy.toml"
)

var (
  proxyPath string
  testWg sync.WaitGroup
)

func main() {
  release := flag.Bool("r", false, "Use release")
  flag.Parse()

  _, err := startProxy()
  if err != nil {
    log.Fatal("ERROR: error starting proxy: ", err)
  }

  runServers()

  proxyPath = getProxyPath(*release)

  testWg.Wait()
}

func startProxy() (*exec.Cmd, error) {
  cmd := exec.Command(
    proxyPath,
    "start",
    "--addr", proxyAddr,
    "--config", configPath,
  )
  return cmd, cmd.Start()
}

func runTest(addr string) {
  testWg.Add(1)
  defer testWg.Done()
  conn, err := grpc.Dial(
    addr,
    grpc.WithTransportCredentials(insecure.NewCredentials()),
  )
  if err != nil {
    log.Fatal("ERROR: error connecting to proxy: ", err)
  }
  defer conn.Close()
  c := pb.NewTestAPIClient(conn)

  var wg sync.WaitGroup

  // Test ReqResp
  wg.Add(1)
  go func() {
    defer wg.Done()
    for i := 0; i < 100_000; i++ {
      s := "" // TODO
      resp, err := c.Test(context.Background(), &pb.TestRequest{Lower: s})
      if err != nil {
        log.Fatal("ERROR: /test.Test: got error: ", err)
      }
      if got, want := resp.GetUpper(), strings.ToUpper(s); got != want {
        log.Fatalf("ERROR: /test.Test: expected %s, got %s", got, want)
      }
    }
  }()

  // TestClientStream
  wg.Add(1)
  go func() {
    defer wg.Done()
    stream, err := c.TestClientStream(context.Background())
    if err != nil {
      log.Fatal("ERROR: /test.TestClientStream: error getting stream: ", err)
    }
    for i := uint64(0); i < 100_000; i++ {
      req := &pb.TestClientStreamRequest{Num: i}
      if err := stream.Send(req); err != nil {
        log.Fatal("ERROR: /test.TestClientStream: error sending: ", err)
      }
    }
    resp, err := stream.CloseAndRecv()
    if err != nil {
      log.Fatal("ERROR: /test.TestClientStream: error receiving: ", err)
    }
    if got, want := resp.GetSum(), uint64(100_000 + 100_001) / 2; got != want {
      log.Fatalf(
        "ERROR: /test.TestClientStream: expected %d, got %d",
        got, want,
      )
    }
  }()

  // TestServerStream
  wg.Add(1)
  go func() {
    defer wg.Done()
    fibNum := uint32(25)
    stream, err := c.TestServerStream(
      context.Background(),
      &pb.TestServerStreamRequest{FibNum: fibNum},
    )
    if err != nil {
      log.Fatal("ERROR: /test.TestServerStream: error getting stream: ", err)
    }
    // Get first 2
    resp, err := stream.Recv()
    if err != nil {
      log.Fatal("ERROR: /test.TestServerStream: unexpected error: ", err)
    } else if got := resp.GetNum(); got != 0 {
      log.Fatalf(
        "ERROR: /test.TestServerStream: expected %d, got %d",
        got, 0,
      )
    }
    // Get rest
    nums, i := [2]uint64{0, 1}, uint32(1)
    for resp, err = stream.Recv(); err == nil; resp, err = stream.Recv() {
      if got := resp.GetNum(); got != nums[1] {
        log.Fatalf(
          "ERROR: /test.TestServerStream: expected %d, got %d",
          got, nums[1],
        )
      }
      nums[0], nums[1] = nums[1], nums[0] + nums[1]
      i++
    }
    if err != io.EOF {
      log.Fatal("ERROR: /test.TestServerStream: unexpected error: ", err)
    } else if i != fibNum {
      log.Fatalf(
        "ERROR: /test.TestServerStream: expected %d nums, got %d: ",
        fibNum, i,
      )
    }
  }()

  // TestBiStream
  wg.Add(1)
  go func() {
    defer wg.Done()
    stream, err := c.TestBiStream(context.Background())
    if err != nil {
      log.Fatal("ERROR: /test.TestBiStream: error getting stream: ", err)
    }
    defer stream.CloseSend()
    err = stream.Send(&pb.TestBiStreamRequest{Subscribe: utils.NewT(true)})
    if err != nil {
      log.Fatal("ERROR: /test.TestBiStream: error sending: ", err)
    }
    // Receive initial values
    pts := []priceTime{}
    for i := 0; i < 10; i++ {
      resp, err := stream.Recv()
      if err != nil {
        log.Fatal("ERROR: /test.TestBiStream: error receiving: ", err)
      }
      prices, start, end := resp.GetPrices(), resp.GetStart(), resp.GetEnd()
      if end - start != 1 {
        log.Fatal(
          "ERROR: /test.TestBiStream: received invalid times for price",
        )
      } else if len(prices) != 1 {
        log.Fatal(
          "ERROR: /test.TestBiStream: received too many prices: ",
          prices,
        )
      } else if resp.Error != nil {
        log.Fatal(
          "ERROR: /test.TestBiStream: received unexpected error: ",
          resp.GetError(),
        )
      }
      pts = append(pts, priceTime{prices[0], end})
    }
    // Test error value
    err = stream.Send(&pb.TestBiStreamRequest{Start: 1_000_000})
    if err != nil {
      log.Fatal("ERROR: /test.TestBiStream: error sending: ", err)
    }
    gotErr := false
    for i := 0; i < 10; i++ {
      resp, err := stream.Recv()
      if err != nil {
        log.Fatal("ERROR: /test.TestBiStream: error receiving: ", err)
      }
      start := resp.GetStart()
      if start != 1_000_000 {
        continue
      }
      if resp.Error == nil {
        log.Fatal("ERROR: /test.TestBiStream: expected error")
      }
      gotErr = true
      break
    }
    if !gotErr {
      log.Fatal("ERROR: /test.TestBiStream: never received error response")
    }
    // Test getting back intial received values
    err = stream.Send(&pb.TestBiStreamRequest{
      Start: pts[0].end - 1,
      End: pts[len(pts)-1].end,
    })
    gotData := false
    otherPts := []priceTime{}
    for i := 0; i < 10; i++ {
      resp, err := stream.Recv()
      if err != nil {
        log.Fatal("ERROR: /test.TestBiStream: error receiving: ", err)
      }
      prices, start, end := resp.GetPrices(), resp.GetStart(), resp.GetEnd()
      if int(end - start) != len(pts) {
        continue
      }
      for i := start; i != end; i++ {
        otherPts = append(otherPts, priceTime{prices[i - start], i + 1})
      }
      gotData = true
      break
    }
    if !gotData {
      log.Fatal("ERROR: /test.TestBiStream: never received data response")
    }
    for i, opt := range otherPts {
      pt := pts[i]
      if opt.price != pt.price || opt.end != pt.end {
        log.Fatalf("ERROR: /test.TestBiStream: expected %v, got %v", pt, opt)
      }
    }
  }()

  // TestBiStream2
  wg.Add(1)
  go func() {
    defer wg.Done()
    // Test initial connection
    _, err := c.TestBiStream2(context.Background())
    if err == nil {
      // TODO
      log.Fatal("ERROR: /test.TestBiStream2: expected error, got nil")
    }
    // Add new to config file
    // TODO
  }()

  wg.Wait()
}

func runServers() {
  go func() {
    // PATH: /test.Test
    if err := runReqRespServer("127.0.0.1:13101"); err != nil {
      log.Fatal("ERROR: error running reqRespServer: ", err)
    }
  }()
  go func() {
    // PATH: /test.TestClientStream, /test.TestServerStream
    if err := runUniServer("127.0.0.1:13102"); err != nil {
      log.Fatal("ERROR: error running uniServer: ", err)
    }
  }()
  go func() {
    // PATH: /test.TestBiStream
    if err := runBiServer("127.0.0.1:13103", RandWalkRandAbs); err != nil {
      log.Fatal("ERROR: error running biServer (randWalkRandAbs): ", err)
    }
  }()
  go func() {
    // PATH: /test.TestBiStream
    if err := runBiServer("127.0.0.1:13104", RandWalkRandPct); err != nil {
      log.Fatal("ERROR: error running biServer (randWalkRandPct): ", err)
    }
  }()
  go func() {
    // PATH: /test.TestBiStream
    if err := runBiServer("127.0.0.1:13105", RandWalkPct); err != nil {
      log.Fatal("ERROR: error running biServer (randWalkPct): ", err)
    }
  }()
}

func getProxyPath(release bool) string {
  _, thisFile, _, _ := runtime.Caller(0)
  path := filepath.Dir(thisFile)
  if release {
    return filepath.Join(path, "release", "grpc-proxy")
  }
  return filepath.Join(path, "debug", "grpc-proxy")
}
