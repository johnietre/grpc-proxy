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
	"golang.org/x/net/http2"

	"crypto/tls"
	"net"
	"net/http"
)

const (
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
    "TestBiStream",
    "TestBiAbsUpDown",
    "TestBiPctUpDown",
    "TestBiPct",
  }
)

func main() {
  release := flag.Bool("r", false, "Use release")
  flag.Parse()

  if *release {
  }

  binPath = tests.GetGoPath()

  log.Println("STARTING PROXIES")
  startProxies()
  log.Println("STARTING SERVERS")
  runServers()

  time.Sleep(time.Second * 3)

  done0 := connect0()
  log.Println("STARTING TESTS")
  start := time.Now()

  tests.TimeTest("initial", initialTest)
  tests.TimeTest("failures", failuresTest)
  tests.TimeTest("add", addTest)
  tests.TimeTest("all", allTest)
  tests.TimeTest("del", delTest)
  tests.TimeTest("allFailures", allFailuresTest)
  tests.TimeTest("refresh", refreshTest)
  tests.TimeTest("initial", initialTest)
  tests.TimeTest("shutdown", shutdownTest)
  tests.TimeTest("final", finalTest)
  tests.TimeTest("shutdown0", shutdown0Test)
  tests.TimedTest(time.Second * 5, "wait", waitTest)
  tests.TimedTest(time.Second * 5, "wait0", func() {
    <-done0
  })

  log.Printf("PASSED: %f seconds", time.Since(start).Seconds())
}

func connect0() chan utils.Unit {
  c, conn, err := newClient("127.0.0.1:14200")
  if err != nil {
    log.Fatalf("ERROR: connect0: error connecting to 0: %v", err)
  }

  stream, err := c.TestBiAbsUpDown(context.Background())
  if err != nil {
    log.Fatalf("ERROR: connect0: error getting stream: %v", err)
  }
  err = stream.Send(&pb.TestBiStreamRequest{Subscribe: utils.NewT(true)})
  if err != nil {
    log.Fatalf("ERROR: connect0: error sending on stream: %v", err)
  }

  ch := make(chan utils.Unit)
  go func() {
    for _, err := stream.Recv(); err == nil; _, err = stream.Recv() {}
    conn.Close()
    ok := tests.Timeout(time.Second * 10, func() {
      ch <- utils.Unit{}
    })
    if !ok {
      tests.Fatal(
        "connect0: disconnected without shutdown (or shutdown timed out)",
      )
    }
  }()
  return ch
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
    )
    if i == 0 {
      cmd.Env = append(cmd.Env, "GRPC_PROXY_PASSWORD=test0")
    }
    if err := cmd.Start(); err != nil {
      log.Fatalf("ERROR: error starting proxy %d: %v", i, err)
    }
    proxies.Store(addr, cmd)
  }
}

func initialTest() {
  conns := NewConns()
  defer conns.CloseAll()

  // Test 0
  startOnWg(func() {
    c, _, err := conns.NewClient("127.0.0.1:14203")
    if err != nil {
      tests.Fatal("0: error creating client: ", err)
    }

    startOnWg(func() {
      if err := runTestTest(c); err != nil {
        tests.Fatal("0: Test: ", err)
      }
    })
    startOnWg(func() {
      if err := runTestClientStreamTest(c); err != nil {
        tests.Fatal("0: TestClientStream: ", err)
      }
    })
    startOnWg(func() {
      if err := runTestServerStreamTest(c); err != nil {
        tests.Fatal("0: TestServerStream: ", err)
      }
    })
    startOnWg(func() {
      if err := runTestBiStreamTest(c); err != nil {
        tests.Fatal("0: TestBiStream: ", err)
      }
    })
    startOnWg(func() {
      if err := runTestBiAbsUpDownTest(c); err != nil {
        tests.Fatal("0: TestBiAbsUpDown: ", err)
      }
    })
    startOnWg(func() {
      if err := runTestBiPctUpDownTest(c); err != nil {
        tests.Fatal("0: TestBiPctUpDown: ", err)
      }
    })
    startOnWg(func() {
      if err := runTestBiPctTest(c); err != nil {
        tests.Fatal("0: TestBiPct: ", err)
      }
    })
  })

  // Test 1
  startOnWg(func() {
    c, _, err := conns.NewClient("127.0.0.1:14211")
    if err != nil {
      tests.Fatal("1: error creating client: ", err)
    }

    startOnWg(func() {
      if err := runTestTest(c); err != nil {
        tests.Fatal("1: Test: ", err)
      }
    })
    startOnWg(func() {
      if err := runTestBiStreamTest(c); err != nil {
        tests.Fatal("1: TestBiStream: ", err)
      }
    })
  })

  // Test 2
  startOnWg(func() {
    c, _, err := conns.NewClient("127.0.0.1:14221")
    if err != nil {
      tests.Fatal("2: error creating client: ", err)
    }

    startOnWg(func() {
      if err := runTestClientStreamTest(c); err != nil {
        tests.Fatal("2: TestClientStream: ", err)
      }
    })
    startOnWg(func() {
      if err := runTestServerStreamTest(c); err != nil {
        tests.Fatal("2: TestServerStream: ", err)
      }
    })
  })

  // Test 3
  startOnWg(func() {
    c, _, err := conns.NewClient(
      "myips:///"+strings.Join(
        []string{
          "127.0.0.1:14234",
          "127.0.0.1:14233",
          "127.0.0.1:14232",
          "127.0.0.1:14231",
          "127.0.0.1:14230",
        },
        ",",
      ),
    )
    if err != nil {
      tests.Fatal("3: error creating client: ", err)
    }

    startOnWg(func() {
      if err := runTestBiAbsUpDownTest(c); err != nil {
        tests.Fatal("3: TestBiAbsUpDown: ", err)
      }
    })
    startOnWg(func() {
      if err := runTestBiPctUpDownTest(c); err != nil {
        tests.Fatal("3: TestBiPctUpDown: ", err)
      }
    })
    startOnWg(func() {
      if err := runTestBiPctTest(c); err != nil {
        tests.Fatal("3: TestBiPct: ", err)
      }
    })
  })

  testWg.Wait()
}

func failuresTest() {
  conns := NewConns()
  defer conns.CloseAll()

  // Test 1
  startOnWg(func() {
    c, _, err := conns.NewClient("127.0.0.1:14211")
    if err != nil {
      tests.Fatal("1: error creating client: ", err)
    }

    startOnWg(func() {
      if err := runTestClientStreamTest(c); err == nil {
        tests.Fatal("1: TestClientStream: ", "expected error")
      }
    })
    startOnWg(func() {
      if err := runTestServerStreamTest(c); err == nil {
        tests.Fatal("1: TestServerStream: ", "expected error")
      }
    })
    startOnWg(func() {
      if err := runTestBiAbsUpDownTest(c); err == nil {
        tests.Fatal("1: TestBiAbsUpDown: ", "expected error")
      }
    })
    startOnWg(func() {
      if err := runTestBiPctUpDownTest(c); err == nil {
        tests.Fatal("1: TestBiPctUpDown: ", "expected error")
      }
    })
    startOnWg(func() {
      if err := runTestBiPctTest(c); err == nil {
        tests.Fatal("1: TestBiPct: ", "expected error")
      }
    })
  })

  // Test 2
  startOnWg(func() {
    c, _, err := conns.NewClient("127.0.0.1:14221")
    if err != nil {
      tests.Fatal("2: error creating client: ", err)
    }

    startOnWg(func() {
      if err := runTestTest(c); err == nil {
        tests.Fatal("2: Test: ", "expected error")
      }
    })
    startOnWg(func() {
      if err := runTestBiStreamTest(c); err == nil {
        tests.Fatal("2: TestBiStream: ", "expected error")
      }
    })
    startOnWg(func() {
      if err := runTestBiAbsUpDownTest(c); err == nil {
        tests.Fatal("2: TestBiAbsUpDown: ", "expected error")
      }
    })
    startOnWg(func() {
      if err := runTestBiPctUpDownTest(c); err == nil {
        tests.Fatal("2: TestBiPctUpDown: ", "expected error")
      }
    })
    startOnWg(func() {
      if err := runTestBiPctTest(c); err == nil {
        tests.Fatal("2: TestBiPct: ", "expected error")
      }
    })
  })

  // Test 3
  startOnWg(func() {
    c, _, err := conns.NewClient("127.0.0.1:14230")
    if err != nil {
      tests.Fatal("3: error creating client: ", err)
    }

    startOnWg(func() {
      if err := runTestTest(c); err == nil {
        tests.Fatal("3: Test: ", "expected error")
      }
    })
    startOnWg(func() {
      if err := runTestClientStreamTest(c); err == nil {
        tests.Fatal("3: TestClientStream: ", "expected error")
      }
    })
    startOnWg(func() {
      if err := runTestServerStreamTest(c); err == nil {
        tests.Fatal("3: TestServerStream: ", "expected error")
      }
    })
    startOnWg(func() {
      if err := runTestBiStreamTest(c); err == nil {
        tests.Fatal("3: TestBiStream: ", "expected error")
      }
    })
  })

  testWg.Wait()
}

func addTest() {
  for i := 1; i <= 3; i++ {
    func(i int) {
      startOnWg(func() {
        cmd := tests.NewCommandWithFiles(
          context.Background(),
          testName, fmt.Sprintf("add-%d", i),
          binPath,
          "refresh",
          "--add",
          "--url", fmt.Sprintf("http://127.0.0.1:%d", 14200 + i*10+4),
          "--url", fmt.Sprintf("http://127.0.0.1:%d", 14200 + i*10+3),
          "--url", fmt.Sprintf("http://127.0.0.1:%d", 14200 + i*10+2),
          "--url", fmt.Sprintf("http://127.0.0.1:%d", 14200 + i*10+1),
          "--url", fmt.Sprintf("http://127.0.0.1:%d", 14200 + i*10),
          "--config", tests.GetPath(testName, fmt.Sprintf("add-%d.toml", i)),
          "-o", tests.GetPath(testName, "out"),
        )
        if err := cmd.Run(); err != nil {
          tests.Fatalf("%d: add: %v", i, err)
        }
      })
    }(i)
  }

  testWg.Wait()
}

func allTest() {
  conns := NewConns()
  defer conns.CloseAll()

  for i := 0; i <= 3; i++ {
    var addrs []string
    basePort := 14200 + i*10
    for j := 0; j <= 4; j++ {
      addrs = append(addrs, fmt.Sprintf("127.0.0.1:%d", basePort+j))
    }
    func(i int, addr string) {
      startOnWg(func() {
        c, _, err := conns.NewClient(addr)
        if err != nil {
          tests.Fatalf("%d: error creating client: %v", i, err)
        }

        startOnWg(func() {
          if err := runTestTest(c); err != nil {
            tests.Fatalf("%d: Test: %v", i, err)
          }
        })
        startOnWg(func() {
          if err := runTestClientStreamTest(c); err != nil {
            tests.Fatalf("%d: TestClientStream: %v", i, err)
          }
        })
        startOnWg(func() {
          if err := runTestServerStreamTest(c); err != nil {
            tests.Fatalf("%d: TestServerStream: %v", i, err)
          }
        })
        startOnWg(func() {
          if err := runTestBiStreamTest(c); err != nil {
            tests.Fatalf("%d: TestBiStream: %v", i, err)
          }
        })
        startOnWg(func() {
          if err := runTestBiAbsUpDownTest(c); err != nil {
            tests.Fatalf("%d: TestBiAbsUpDown: %v", i, err)
          }
        })
        startOnWg(func() {
          if err := runTestBiPctUpDownTest(c); err != nil {
            tests.Fatalf("%d: TestBiPctUpDown: %v", i, err)
          }
        })
        startOnWg(func() {
          if err := runTestBiPctTest(c); err != nil {
            tests.Fatalf("%d: TestBiPct: %v", i, err)
          }
        })
      })
    }(i, "myips:///"+strings.Join(addrs, ","))
  }

  testWg.Wait()
}

func delTest() {
  cmd := tests.NewCommandWithFiles(
    context.Background(),
    testName, "del-all",
    binPath,
    "refresh",
    "--del",
    "--url", "http://127.0.0.1:14210",
    "--url", "http://127.0.0.1:14220",
    "--url", "http://127.0.0.1:14230",
    "--all",
    "--config", tests.GetPath(testName, "del-all.toml"),
    "-o", tests.GetPath(testName, "out"),
  )
  if err := cmd.Run(); err != nil {
    tests.Fatal("del: ", err)
  }
}

func allFailuresTest() {
  conns := NewConns()
  defer conns.CloseAll()

  for i := 1; i <= 3; i++ {
    var addrs []string
    basePort := 14200 + i*10
    for j := 0; j <= 4; j++ {
      addr := fmt.Sprintf("127.0.0.1:%d", basePort+j)
      addrs = append(addrs, addr)
      if j != 0 {
        // Ttest to make sure listener is closed
        func(i, j int, addr string) {
          startOnWg(func() {
            _, err := net.Dial("tcp", addr)
            if err == nil {
              tests.Fatalf("%d: expected error for address %s", i, addr)
            }
          })
        }(i, j, addr)
      }
    }
    func(i int, addr string) {
      startOnWg(func() {
        c, _, err := conns.NewClient(addr)
        if err != nil {
          tests.Fatalf("%d: error creating client: %v", i, err)
        }

        startOnWg(func() {
          if err := runTestTest(c); err == nil {
            tests.Fatalf("%d: Test: expected error", i)
          }
        })
        startOnWg(func() {
          if err := runTestClientStreamTest(c); err == nil {
            tests.Fatalf("%d: TestClientStream: expected error", i)
          }
        })
        startOnWg(func() {
          if err := runTestServerStreamTest(c); err == nil {
            tests.Fatalf("%d: TestServerStream: expected error", i)
          }
        })
        startOnWg(func() {
          if err := runTestBiStreamTest(c); err == nil {
            tests.Fatalf("%d: TestBiStream: expected error", i)
          }
        })
        startOnWg(func() {
          if err := runTestBiAbsUpDownTest(c); err == nil {
            tests.Fatalf("%d: TestBiAbsUpDown: expected error", i)
          }
        })
        startOnWg(func() {
          if err := runTestBiPctUpDownTest(c); err == nil {
            tests.Fatalf("%d: TestBiPctUpDown: expected error", i)
          }
        })
        startOnWg(func() {
          if err := runTestBiPctTest(c); err == nil {
            tests.Fatalf("%d: TestBiPct: expected error", i)
          }
        })
      })
    }(i, "myips:///"+strings.Join(addrs, ","))
  }

  testWg.Wait()
}

func refreshTest() {
  cmd := tests.NewCommandWithFiles(
    context.Background(),
    testName, "refresh",
    binPath,
    "refresh",
    "--url", "http://127.0.0.1:14210",
    "--url", "http://127.0.0.1:14220",
    "--url", "http://127.0.0.1:14230",
    "--all",
  )
  if err := cmd.Run(); err != nil {
    tests.Fatal("refresh: ", err)
  }
}

func shutdownTest() {
  startOnWg(func() {
    cmd := tests.NewCommandWithFiles(
      context.Background(),
      testName, "shutdown",
      binPath,
      "shutdown",
      "--url", "http://127.0.0.1:14210",
      "--url", "http://127.0.0.1:14220",
      "--all",
    )
    if err := cmd.Run(); err != nil {
      tests.Fatalf("shutdown: %v", err)
    }
  })
  startOnWg(func() {
    cmd := tests.NewCommandWithFiles(
      context.Background(),
      testName, "shutdown (force)",
      binPath,
      "shutdown",
      "--url", "http://127.0.0.1:14230",
      "--all",
      "--force",
    )
    if err := cmd.Run(); err != nil {
      tests.Fatalf("shutdown (force): %v", err)
    }
  })
  testWg.Wait()
}

func finalTest() {
  conns := NewConns()
  defer conns.CloseAll()

  var addrs []string
  basePort := 14200
  for i := 0; i <= 4; i++ {
    addrs = append(addrs, fmt.Sprintf("127.0.0.1:%d", basePort+i))
  }
  addr := "myips:///"+strings.Join(addrs, ",")

  for i := 1; i <= 100; i++ {
    func(i int) {
      startOnWg(func() {
        c, _, err := conns.NewClient(addr)
        if err != nil {
          tests.Fatalf("%d: error creating client: %v", i, err)
        }

        startOnWg(func() {
          if err := runTestTest(c); err != nil {
            tests.Fatalf("%d: Test: %v", i, err)
          }
        })
        startOnWg(func() {
          if err := runTestClientStreamTest(c); err != nil {
            tests.Fatalf("%d: TestClientStream: %v", i, err)
          }
        })
        startOnWg(func() {
          if err := runTestServerStreamTest(c); err != nil {
            tests.Fatalf("%d: TestServerStream: %v", i, err)
          }
        })
        startOnWg(func() {
          if err := runTestBiStreamTest(c); err != nil {
            tests.Fatalf("%d: TestBiStream: %v", i, err)
          }
        })
        startOnWg(func() {
          if err := runTestBiAbsUpDownTest(c); err != nil {
            tests.Fatalf("%d: TestBiAbsUpDown: %v", i, err)
          }
        })
        startOnWg(func() {
          if err := runTestBiPctUpDownTest(c); err != nil {
            tests.Fatalf("%d: TestBiPctUpDown: %v", i, err)
          }
        })
        startOnWg(func() {
          if err := runTestBiPctTest(c); err != nil {
            tests.Fatalf("%d: TestBiPct: %v", i, err)
          }
        })
      })
    }(i)
  }

  testWg.Wait()
}

func shutdown0Test() {
  // Test with no password
  cmd := tests.NewCommandWithFiles(
    context.Background(),
    testName, "shutdown0",
    binPath,
    "shutdown",
    "--url", "http://127.0.0.1:14204",
  )
  if err := cmd.Run(); err == nil {
    println("rand1")
    tests.Fatal("shutdown0: expected non-zero exit status")
  }
  // Test with incorrect password
  cmd = tests.NewCommandWithFiles(
    context.Background(),
    testName, "shutdown0",
    binPath,
    "shutdown",
    "--url", "http://127.0.0.1:14204",
  )
  cmd.Env = []string{"GRPC_PROXY_PASSWORD=somerandompassword"}
  if err := cmd.Run(); err == nil {
    println("rand2")
    tests.Fatal("shutdown0: expected non-zero exit status")
  }

  // Test correct password
  cmd = tests.NewCommandWithFiles(
    context.Background(),
    testName, "shutdown0",
    binPath,
    "shutdown",
    "--url", "http://127.0.0.1:14204",
  )
  cmd.Env = []string{"GRPC_PROXY_PASSWORD=test0"}
  if err := cmd.Run(); err != nil {
    tests.Fatalf("shutdown0: %v", err)
  }

  // Wait to see if connect0 ends (it shouldn't)
  time.Sleep(time.Second * 5)

  // Test to make sure server is closed
  cmd = tests.NewCommandWithFiles(
    context.Background(),
    testName, "shutdown0",
    binPath,
    "shutdown",
    "--url", "http://127.0.0.1:14204",
  )
  cmd.Env = []string{"GRPC_PROXY_PASSWORD=test0"}
  if err := cmd.Run(); err == nil {
    println("rand3")
    tests.Fatal("shutdown0: expected non-zero exit status")
  }

  // Force shutdown
  cmd = tests.NewCommandWithFiles(
    context.Background(),
    testName, "shutdown0",
    binPath,
    "shutdown",
    "--url", "http://127.0.0.1:14200",
    "--force",
  )
  cmd.Env = []string{"GRPC_PROXY_PASSWORD=test0"}
  if err := cmd.Run(); err != nil {
    tests.Fatalf("shutdown0: %v", err)
  }

  // connect0 should now have ended
}

func waitTest() {
  proxies.Range(func(name string, cmd *exec.Cmd) bool {
    startOnWg(func() {
      if err := cmd.Wait(); err != nil {
        tests.Fatalf("%s: wait: %v", name, err)
      }
    })
    return true
  })
  testWg.Wait()
}

func runTestTest(c pb.TestAPIClient) error {
  buf := make([]byte, 128)
  start := time.Now()
  for i := 0; i < 100_000; i++ {
    if false && i % 1_000 == 0 {
      fmt.Printf("%d: %.4f seconds\n", i, time.Since(start).Seconds())
      start = time.Now()
    }
    if _, err := rand.Read(buf); err != nil {
      tests.Fatal("failed to read random bytes: ", err)
    }
    s := base64.StdEncoding.EncodeToString(buf)
    resp, err := c.Test(context.Background(), &pb.TestRequest{Lower: s})
    if err != nil {
      return err
    }
    if got, want := resp.GetUpper(), strings.ToUpper(s); got != want {
      return fmt.Errorf("expected %s, got %s", want, got)
    }
  }
  return nil
}

func runTestClientStreamTest(c pb.TestAPIClient) error {
  stream, err := c.TestClientStream(context.Background())
  if err != nil {
    return NewIE(err)
  }
  for i := uint64(0); i <= 100_000; i++ {
    req := &pb.TestClientStreamRequest{Num: i}
    if err := stream.Send(req); err != nil {
      return err
    }
  }
  resp, err := stream.CloseAndRecv()
  if err != nil {
    return err
  }
  if got, want := resp.GetSum(), uint64(100_000 * 100_001) / 2; got != want {
    return fmt.Errorf("expected %d, got %d", want, got)
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
    return fmt.Errorf("expected %d, got %d", 0, got)
  }
  // Get rest
  nums, i := [2]uint64{0, 1}, uint32(1)
  for resp, err = stream.Recv(); err == nil; resp, err = stream.Recv() {
    if got := resp.GetNum(); got != nums[1] {
      return fmt.Errorf("expected %d, got %d", nums[1], got)
    }
    nums[0], nums[1] = nums[1], nums[0] + nums[1]
    i++
  }
  expected := fibNum + 1
  if err != io.EOF {
    return err
  } else if i != expected {
    return fmt.Errorf("expected %d nums, got %d: ", expected, i)
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
  const u64Max uint64 = 1<<64-1
  err := stream.Send(&pb.TestBiStreamRequest{Subscribe: utils.NewT(true)})
  if err != nil {
    return NewIE(err)
  }
  // Receive initial values
  pts, prev := []priceTime{}, u64Max
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
      return fmt.Errorf("received unexpected error: %+v", resp)
    } else if prev != u64Max && prev + 1 != end {
      return fmt.Errorf("expected %d end, got %d (%+v)", prev + 1, end, resp)
    }
    prev = end
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
  req := &pb.TestBiStreamRequest{
    Start: pts[0].end - 1,
    End: pts[len(pts)-1].end,
  }
  err = stream.Send(req)
  gotData := false
  recvdPts := []priceTime{}
  for i := 0; i < 10; i++ {
    resp, err := stream.Recv()
    if err != nil {
      return err
    }
    if resp.Error != nil {
      return fmt.Errorf("received unexpected error (query prev): %+v", resp)
    }
    prices, start, end := resp.GetPrices(), resp.GetStart(), resp.GetEnd()
    if start != req.Start {
      continue
    } else if end != req.End {
      return fmt.Errorf("expected %d end, got %d (%+v)", req.End, end, resp)
    } else if l := int(end - start); l != len(pts) {
      return fmt.Errorf(
        "expected length of %d, got %d (%+v)",
        len(pts), l, resp,
      )
    }
    for i := start; i < end; i++ {
      recvdPts = append(recvdPts, priceTime{prices[i - start], i + 1})
    }
    gotData = true
    break
  }
  if !gotData {
    return fmt.Errorf("never received data response")
  }
  for i, rpt := range recvdPts {
    pt := pts[i]
    if rpt.price != pt.price || rpt.end != pt.end {
      return fmt.Errorf("expected %v, got %v", pt, rpt)
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

func startOnWg(f func()) {
  testWg.Add(1)
  go func() {
    defer testWg.Done()
    f()
  }()
}

func runOther() {
  http.DefaultClient.Transport = &http2.Transport{
    AllowHTTP: true,
    DialTLS: func(ntwk, addr string, cfg *tls.Config) (net.Conn, error) {
      return net.Dial(ntwk, addr)
    },
  }
  for i := 0; i < 100_000; i++ {
    req, err := http.NewRequest("GET", "http://127.0.0.1:14200/", nil)
    if err != nil {
      panic(err)
    }
    req.Header.Set("X-Grpc-Proxy", "get")
    resp, err := http.DefaultClient.Do(req)
    if err == nil {
      if resp.StatusCode != http.StatusOK {
        panic(resp.StatusCode)
      }
      resp.Body.Close()
    } else {
      panic(err)
    }
  }
}

func startRustProxies() {
  for i := 0; i <= 3; i++ {
    addr := "127.0.0.1:" + strconv.Itoa(14200 + i*10)
    cmd := tests.NewCommandWithFiles(
      context.Background(),
      testName, fmt.Sprintf("start-%d", i),
      binPath,
      "start",
      "--addr", addr,
      "--config", tests.GetPath(testName, fmt.Sprintf("start-%d.toml", i)),
      "--workers", "5",
      "--log-stderr", "--log-level=trace",
      //"--log-proxy-only",
    )
    cmd.Env = append(
      cmd.Env,
      "GRPC_PROXY_LOG_TARGET="+
        "run,listener,serve_proxy,serve,proxy_connect,proxy,"+
        "handle_req,handle_get,handle_refresh,handle_shutdown,signals",
    )
    cmd.Env = nil
    const pfx = "grpc_proxy::inspect_io::"
    cmd.Env = []string{"GRPC_PROXY_LOG_TARGET="+pfx+"read,"+pfx+"write,"+pfx+"proxy"}
    if err := cmd.Start(); err != nil {
      log.Fatalf("ERROR: error starting proxy %d: %v", i, err)
    }
    proxies.Store(addr, cmd)
    break
  }
}
