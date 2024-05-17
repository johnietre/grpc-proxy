package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/johnietre/grpc-proxy/tests"
	pb "github.com/johnietre/grpc-proxy/tests/proto"
	utils "github.com/johnietre/utils/go"
)

const (
  proxyAddr = "127.0.0.1:13100"
  configPath = "grpc-proxy.toml"
  testName = "long"
)

var (
  binPath string
  testWg sync.WaitGroup

  proxies = utils.NewSyncMap[string, *exec.Cmd]()

  services = []string{
    "Test",
    "TestClientStream",
    "TestServerStream",
    "TestBiAbsUpDown",
    "TestBiPctUpDown",
    "TestBiPct",
  }
)

func main() {
  release := flag.Bool("r", false, "Use release")
  flag.Parse()

  binPath = tests.GetBinPath(*release)

  log.Println("STARTING PROXIES")
  startProxies()
  log.Println("STARTING SERVERS")
  runServers()

  time.Sleep(time.Second * 5)

  log.Println("STARTING TESTS")
  start := time.Now()
  tests.TimeTest("initial", initialTest)
  log.Printf("PASSED: %f seconds", time.Since(start))
}

func initialTest() {
  c, conn, err := newClient("127.0.0.1:14204")
  if err != nil {
    tests.Fatal("error connecting to proxy:", err)
  }
  defer conn.Close()

  testWg.Add(1)
  go func() {
    defer testWg.Done()
    if err := runTestTest(c); err != nil {
      tests.Fatal(err)
    }
  }()
  testWg.Add(1)
  go func() {
    defer testWg.Done()
    if err := runTestClientStreamTest(c); err != nil {
      tests.Fatal(err)
    }
  }()
  testWg.Add(1)
  go func() {
    defer testWg.Done()
    if err := runTestServerStreamTest(c); err != nil {
      tests.Fatal(err)
    }
  }()
  testWg.Add(1)
  go func() {
    defer testWg.Done()
    if err := runTestBiStreamTest(c); err != nil {
      tests.Fatal(err)
    }
  }()
  testWg.Add(1)
  go func() {
    defer testWg.Done()
    if err := runTestBiAbsUpDownTest(c); err != nil {
      tests.Fatal(err)
    }
  }()
  testWg.Add(1)
  go func() {
    defer testWg.Done()
    if err := runTestBiPctUpDownTest(c); err != nil {
      tests.Fatal(err)
    }
  }()
  testWg.Add(1)
  go func() {
    defer testWg.Done()
    if err := runTestBiPctTest(c); err != nil {
      tests.Fatal(err)
    }
  }()

  testWg.Wait()
}

func startProxies() {
  for i := 0; i <= 3; i++ {
    addr := "127.0.0.1:" + strconv.Itoa(14200 + i*10)
    cmd := tests.NewCommandWithFiles(
      context.Background(),
      testName, fmt.Sprintf("start-%d", i),
      binPath,
      "start",
      "--addr", addr,
      "--config", tests.GetPath(testName, fmt.Sprintf("start-%d.toml", i)),
      "--log-stderr", "--log-level=info",
    )
    cmd.Env = append(
      cmd.Env,
      "GRPC_PROXY_LOG_TARGET=run,listener,serve_proxy,serve,proxy_connect,proxy,handle_req,handle_get,handle_refresh,handle_shutdown,signals",
    )
    if err := cmd.Start(); err != nil {
      log.Fatalf("ERROR: error starting proxy %d: %v", i, err)
    }
    proxies.Store(addr, cmd)
    break
  }
}

func runTestTest(c pb.TestAPIClient) error {
  buf := make([]byte, 128)
  for i := 0; i < 100_000; i++ {
    if _, err := rand.Read(buf); err != nil {
      tests.Fatal("failed to read random bytes: ", err)
    }
    s := base64.StdEncoding.EncodeToString(buf)
    resp, err := c.Test(context.Background(), &pb.TestRequest{Lower: s})
    if err != nil {
      return err
    }
    if got, want := resp.GetUpper(), strings.ToUpper(s); got != want {
      return fmt.Errorf("ERROR: /test.Test: expected %s, got %s", got, want)
    }
  }
  return nil
}

func runTestClientStreamTest(c pb.TestAPIClient) error {
  stream, err := c.TestClientStream(context.Background())
  if err != nil {
    return NewIE(err)
  }
  for i := uint64(0); i < 100_000; i++ {
    req := &pb.TestClientStreamRequest{Num: i}
    if err := stream.Send(req); err != nil {
      return err
    }
  }
  resp, err := stream.CloseAndRecv()
  if err != nil {
    return err
  }
  if got, want := resp.GetSum(), uint64(100_000 + 100_001) / 2; got != want {
    return fmt.Errorf("expected %d, got %d", got, want)
  }
  return nil
}

func runTestServerStreamTest(c pb.TestAPIClient) error {
  fibNum := uint32(25)
  stream, err := c.TestServerStream(
    context.Background(),
    &pb.TestServerStreamRequest{FibNum: fibNum},
  )
  if err != nil {
    return err
  }
  // Get first 2
  resp, err := stream.Recv()
  if err != nil {
    return err
  } else if got := resp.GetNum(); got != 0 {
    return fmt.Errorf("expected %d, got %d", got, 0)
  }
  // Get rest
  nums, i := [2]uint64{0, 1}, uint32(1)
  for resp, err = stream.Recv(); err == nil; resp, err = stream.Recv() {
    if got := resp.GetNum(); got != nums[1] {
      return fmt.Errorf("expected %d, got %d", got, nums[1])
    }
    nums[0], nums[1] = nums[1], nums[0] + nums[1]
    i++
  }
  if err != io.EOF {
    return err
  } else if i != fibNum {
    return fmt.Errorf("expected %d nums, got %d: ", fibNum, i)
  }
  return nil
}

func runTestBiStreamTest(c pb.TestAPIClient) error {
  stream, err := c.TestBiStream(context.Background())
  if err != nil {
    return err
  }
  defer stream.CloseSend()
  return testBiStream(stream)
}

func runTestBiAbsUpDownTest(c pb.TestAPIClient) error {
  stream, err := c.TestBiAbsUpDown(context.Background())
  if err != nil {
    return err
  }
  defer stream.CloseSend()
  return testBiStream(stream)
}

func runTestBiPctUpDownTest(c pb.TestAPIClient) error {
  stream, err := c.TestBiPctUpDown(context.Background())
  if err != nil {
    return err
  }
  defer stream.CloseSend()
  return testBiStream(stream)
}

func runTestBiPctTest(c pb.TestAPIClient) error {
  stream, err := c.TestBiPct(context.Background())
  if err != nil {
    return err
  }
  defer stream.CloseSend()
  return testBiStream(stream)
}

func testBiStream(stream pb.TestAPI_TestBiStreamClient) error {
  err := stream.Send(&pb.TestBiStreamRequest{Subscribe: utils.NewT(true)})
  if err != nil {
    return NewIE(err)
  }
  // Receive initial values
  pts := []priceTime{}
  for i := 0; i < 10; i++ {
    resp, err := stream.Recv()
    if err != nil {
      return err
    }
    prices, start, end := resp.GetPrices(), resp.GetStart(), resp.GetEnd()
    if end - start != 1 {
      return fmt.Errorf("received invalid times for price")
    } else if len(prices) != 1 {
      return fmt.Errorf("received too many prices: %v", prices)
    } else if resp.Error != nil {
      return fmt.Errorf("%s", resp.GetError())
    }
    pts = append(pts, priceTime{prices[0], end})
  }
  // Test error value
  err = stream.Send(&pb.TestBiStreamRequest{Start: 1_000_000})
  if err != nil {
    return err
  }
  gotErr := false
  for i := 0; i < 10; i++ {
    resp, err := stream.Recv()
    if err != nil {
      return err
    }
    start := resp.GetStart()
    if start != 1_000_000 {
      continue
    }
    if resp.Error == nil {
      return fmt.Errorf("expected error")
    }
    gotErr = true
    break
  }
  if !gotErr {
    return fmt.Errorf("never received error response")
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
      return err
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
    return fmt.Errorf("never received data response")
  }
  for i, opt := range otherPts {
    pt := pts[i]
    if opt.price != pt.price || opt.end != pt.end {
      return fmt.Errorf("expected %v, got %v", pt, opt)
    }
  }
  return nil
}

// Returned when there is an error doing the action (e.g., connecting to start
// a stream).
type InitialError struct {
  Err error
}

func NewIE(err error) *InitialError {
  return &InitialError{Err: err}
}

func (ie *InitialError) Error() string {
  return ie.Err.Error()
}

func runServers() {
  /* Basics */
  go func() {
    // PATH: /test.Test
    if err := runReqRespServer("127.0.0.1:14251"); err != nil {
      log.Fatal("ERROR: error running reqRespServer: ", err)
    }
  }()
  go func() {
    // PATH: /test.TestClientStream
    if err := runClientStreamServer("127.0.0.1:14252"); err != nil {
      log.Fatal("ERROR: error running clientStreamServer: ", err)
    }
  }()
  go func() {
    // PATH: /test.TestServerStream
    if err := runServerStreamServer("127.0.0.1:14253"); err != nil {
      log.Fatal("ERROR: error running serverStreamServer: ", err)
    }
  }()
  go func() {
    // PATH: /test.TestBiStream
    if err := runBiStreamServer("127.0.0.1:14254", RandWalkRandAbs); err != nil {
      log.Fatal("ERROR: error running biStreamServer (randWalkRandAbs): ", err)
    }
  }()

  /* Other BiStream servers */
  go func() {
    // PATH: /test.TestBiAbsUpDown
    if err := runBiAbsUpDownServer("127.0.0.1:14255"); err != nil {
      log.Fatal("ERROR: error running biAbsUpDownServer: ", err)
    }
  }()
  go func() {
    // PATH: /test.TestBiPctUpDown
    if err := runBiPctUpDownServer("127.0.0.1:14256"); err != nil {
      log.Fatal("ERROR: error running biPctUpDownServer: ", err)
    }
  }()
  go func() {
    // PATH: /test.TestBiPct
    if err := runBiPctServer("127.0.0.1:14257"); err != nil {
      log.Fatal("ERROR: error running biPctServer: ", err)
    }
  }()

  /* Combo servers */
  go func() {
    // PATH: /test.TestClientStream, /test.TestServerStream
    if err := runUniStreamServer("127.0.0.1:14258"); err != nil {
      log.Fatal("ERROR: error running uniStreamServer: ", err)
    }
  }()
  go func() {
    // PATH: /test.Test, /test.TestClientStream, /test.TestServerStream, /test.TestBiStream
    if err := runBasic4Server("127.0.0.1:14259", RandWalkRandPct); err != nil {
      log.Fatal("ERROR: error running basic4Server (randWalkRandPct): ", err)
    }
  }()
  go func() {
    // PATH: ALL
    if err := runFullServer("127.0.0.1:14250", RandWalkPct); err != nil {
      log.Fatal("ERROR: error running fullServer (randWalkPct): ", err)
    }
  }()
}
