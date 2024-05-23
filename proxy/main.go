package main

import (
	"context"
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	urlpkg "net/url"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

const addr = "127.0.0.1:14200"

func main() {
  log.SetFlags(log.Lshortfile)

  h2s := &http2.Server{
    MaxConcurrentStreams: 50,
  }
  rp = httputil.NewSingleHostReverseProxy(url)
  rp.Transport = transport

  run1 := func() {
    println("run1")
    ln, err := net.Listen("tcp", addr)
    if err != nil {
      log.Fatal(err)
    }
    for {
      conn, err := ln.Accept()
      if err != nil {
        log.Fatal(err)
      }
      log.Println("new conn")
      go func(conn net.Conn) {
        h2s.ServeConn(conn, &http2.ServeConnOpts{
          Handler: h2c.NewHandler(http.HandlerFunc(handle), h2s),
        })
      }(conn)
    }
  }
  var closed, hijacked atomic.Uint32
  run2 := func() {
    println("run2")
    h1s := &http.Server{
      Addr: addr,
      Handler: h2c.NewHandler(http.HandlerFunc(handle), h2s),
      ErrorLog: log.Default(),
      ConnState: func(_ net.Conn, state http.ConnState) {
        if state == http.StateClosed { 
          println(closed.Add(1))
        } else if state == http.StateHijacked {
          println(hijacked.Add(1))
        }
      },
      ConnContext: func(ctx context.Context, _ net.Conn) context.Context {
        return ctx
      },
    }
    h1s.ListenAndServe()
  }
  run3 := func() {
    println("run3")
    ln, err := net.Listen("tcp", addr)
    if err != nil {
      log.Fatal(err)
    }
    for {
      conn, err := ln.Accept()
      if err != nil {
        log.Fatal(err)
      }
      log.Println("new conn")
      go func(conn net.Conn) {
        other, err := net.Dial("tcp", "127.0.0.1:14250")
        if err != nil {
          log.Fatal(err)
        }
        go func() {
          io.Copy(other, conn)
        }()
        io.Copy(conn, other)
        conn.Close()
        other.Close()
      }(conn)
    }
  }
  if false {
    run1()
    run2()
    run3()
  }
  log.Println("Serving on", addr)
  run2()
}

var transport = &http2.Transport{
  /*
  DialTLSContext: func(
    ctx context.Context, ntwk, addr string, cfg *tls.Config,
  ) (net.Conn, error) {
    log.Print("dialed")
    dialer := &net.Dialer{}
    return dialer.DialContext(ctx, ntwk, addr)
  },
  */
  AllowHTTP: true,
}

var (
  url, _ = urlpkg.Parse("http://127.0.0.1:14250")
  rp = httputil.NewSingleHostReverseProxy(url)
  ok atomic.Bool
  mtx sync.Mutex

  _ = tls.Config{}
  _ = io.EOF
  _ = textproto.CanonicalMIMEHeaderKey
  _ = time.Second
)

func handle(w http.ResponseWriter, r *http.Request) {
  if !ok.Swap(true) {
    log.Println(r.Proto, r.Method, r.Header, r.Trailer)
  }
  rp.ServeHTTP(w, r)
}

/*
func handle(w http.ResponseWriter, r *http.Request) {
  log.Print(r.Header, r.Trailer)
  time.Sleep(0)
  r.URL.Scheme, r.URL.Host = "http", "127.0.0.1:14250"
  resp, err := transport.RoundTrip(r)
  if err != nil {
    log.Print(err)
    w.WriteHeader(http.StatusInternalServerError)
    return
  }
  defer resp.Body.Close()
  log.Print(resp.Header, resp.Trailer)

  for k := range resp.Trailer {
    w.Header().Add("Trailer", k)
  }
  for key, val := range resp.Header {
    w.Header()[key] = val
    key = textproto.CanonicalMIMEHeaderKey(key)
  }
  _, err = io.Copy(w, resp.Body)
  if err != nil {
    log.Println(err)
  }
  for k, v := range resp.Trailer {
    w.Header()[k] = v
  }
}
*/

func contains[T comparable](s []T, t T) bool {
  for _, v := range s {
    if v == t {
      return true
    }
  }
  return false
}

/*
import (
	"bufio"
	"log"
	"net"

	//"github.com/dgrr/http2"
	"github.com/valyala/fasthttp"
)

func main() {
  ln, err := net.Listen("tcp", "127.0.0.1:14200")
  if err != nil {
    panic(err)
  }
  log.Println("running on 127.0.0.1:14200")
  for {
    conn, err := ln.Accept()
    if err != nil {
      panic(err)
    }
    go handle(conn)
  }
}

func handle(conn net.Conn) {
  defer conn.Close()
  other, err := net.Dial("tcp", "127.0.0.1:14250")
  if err != nil {
    panic(err)
  }
  defer other.Close()
  go func() {
    defer conn.Close()
    defer other.Close()

    req := fasthttp.AcquireRequest()
    defer fasthttp.ReleaseRequest(req)
    br, bw := bufio.NewReader(conn), bufio.NewWriter(other)
    for {
      if err := req.Read(br); err != nil {
        log.Print("read req: ", err)
        return
      }
      req.Header.Set(fasthttp.HeaderHost, "grpc-proxy")
      if err := req.Write(bw); err != nil {
        log.Print("write req: ", err)
        return
      } else if bw.Flush(); err != nil {
        log.Print("flush req: ", err)
        return
      }
    }
  }()
  func() {
    resp := fasthttp.AcquireResponse()
    defer fasthttp.ReleaseResponse(resp)
    br, bw := bufio.NewReader(other), bufio.NewWriter(conn)
    for {
      if err := resp.Read(br); err != nil {
        log.Print("read resp: ", err)
        return
      } else if err := resp.Write(bw); err != nil {
        log.Print("write resp: ", err)
        return
      } else if bw.Flush(); err != nil {
        log.Print("flush resp: ", err)
        return
      }
    }
  }()
}
*/
