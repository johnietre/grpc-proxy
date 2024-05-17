package main

import (
	"context"
	"io"
	"math/rand"
	"net"
	"strings"
	"time"

	pb "github.com/johnietre/grpc-proxy/tests/proto"
	utils "github.com/johnietre/utils/go"
	"google.golang.org/grpc"
)

/*
func newClient(addr string) (pb.TestAPIClient, error) {
}
*/

/*** Request/Response (No Streams) ***/

type reqRespServer struct {
  pb.UnimplementedTestAPIServer
}

func runReqRespServer(addr string) error {
  ln, err := net.Listen("tcp", addr)
  if err != nil {
    return err
  }
  s := grpc.NewServer()
  pb.RegisterTestAPIServer(s, &reqRespServer{})
  return s.Serve(ln)
}

func (reqRespServer) Test(
  ctx context.Context,
  req *pb.TestRequest,
) (*pb.TestResponse, error) {
  return &pb.TestResponse{Upper: strings.ToUpper(req.GetLower())}, nil
}

/*** Unidirectional Streams ***/

type uniServer struct {
  pb.UnimplementedTestAPIServer
}

func runUniServer(addr string) error {
  ln, err := net.Listen("tcp", addr)
  if err != nil {
    return err
  }
  s := grpc.NewServer()
  pb.RegisterTestAPIServer(s, &uniServer{})
  return s.Serve(ln)
}

func (uniServer) TestClientStream(stream pb.TestAPI_TestClientStreamServer) error {
  req, err, sum := (*pb.TestClientStreamRequest)(nil), (error)(nil), uint64(0)
  for req, err = stream.Recv(); err == nil; req, err = stream.Recv() {
    sum += req.GetNum()
  }
  if err != io.EOF {
    return err
  }
  return stream.SendAndClose(&pb.TestClientStreamResponse{Sum: sum})
}

func (uniServer) TestServerStream(
  req *pb.TestServerStreamRequest,
  stream pb.TestAPI_TestServerStreamServer,
) error {
  n := req.GetFibNum()
  err := stream.Send(&pb.TestServerStreamResponse{Num: 0})
  if err != nil || n <= 1 {
    return err
  }
  nums := [2]uint64{0, 1}
  for i := uint32(1); i <= n; i++ {
    err = stream.Send(&pb.TestServerStreamResponse{Num: nums[1]})
    if err != nil {
      return err
    }
    nums[0], nums[1] = nums[1], nums[0] + nums[1]
  }
  return nil
}

/*** Bidirectional Streams ***/

const (
  RandWalkRandAbs int = iota
  RandWalkRandPct
  RandWalkPct
)

type biServer struct {
  prices *AtomicList[priceTime]
  subscribers *utils.SyncSet[pb.TestAPI_TestBiStreamServer]

  pb.UnimplementedTestAPIServer
}

func runBiServer(addr string, randWalkType int) error {
  ln, err := net.Listen("tcp", addr)
  if err != nil {
    return err
  }
  s := grpc.NewServer()
  pb.RegisterTestAPIServer(s, newBiServer(randWalkType))
  return s.Serve(ln)
}

func newBiServer(randWalkType int) *biServer {
  bs := &biServer{
    prices: NewAtomicList[priceTime](),
    subscribers: utils.NewSyncSet[pb.TestAPI_TestBiStreamServer](),
  }
  go bs.run(randWalkType)
  return bs
}

func (bs *biServer) TestBiStream(stream pb.TestAPI_TestBiStreamServer) error {
  defer bs.subscribers.Remove(stream)
  subscribed := false
  for {
    req, err := stream.Recv()
    if err != nil {
      if err == io.EOF {
        return nil
      }
      return err
    }
    if req.Subscribe != nil {
      if sub := req.GetSubscribe(); sub != subscribed {
        if sub {
          bs.subscribers.Insert(stream)
        } else {
          bs.subscribers.Remove(stream)
        }
      }
      continue
    }
    start, end := req.GetStart(), req.GetEnd()
    if start > end {
      resp := &pb.TestBiStreamResponse{Error: utils.NewT("invalid range")}
      if err := stream.Send(resp); err != nil {
        return err
      }
      continue
    }
    startNode := bs.prices.Get(start)
    if startNode == nil {
      resp := &pb.TestBiStreamResponse{Error: utils.NewT("invalid start")}
      if err := stream.Send(resp); err != nil {
        return err
      }
      continue
    }
    if end == start {
      end++
    }
    prices := make([]float64, 0, end - start)
    startNode.RangeToTail(func(node *AtomicNode[priceTime]) bool {
      if node.Index() == end {
        return false
      }
      prices = append(prices, node.Value().price)
      return true
    })
    resp := &pb.TestBiStreamResponse{
      Prices: prices,
      Start: start,
      End: start + uint64(len(prices)),
    }
    if err := stream.Send(resp); err != nil {
      return err
    }
  }
}

type priceTime struct {
  price float64
  end uint64
}

func (bs *biServer) run(randWalkType int) {
  pricesChan := utils.NewUChan[priceTime](50)
  go func() {
    for {
      pt, _ := pricesChan.Recv()
      bs.subscribers.Range(func(stream pb.TestAPI_TestBiStreamServer) bool {
        resp := &pb.TestBiStreamResponse{
          Prices: []float64{pt.price},
          Start: pt.end - 1,
          End: pt.end,
        }
        stream.Send(resp)
        return true
      })
    }
  }()

  initial := rand.Float64() * 500.0
  var walker Walker
  if randWalkType == RandWalkRandPct {
    ud := 0.08125
    NewRandWalker(initial, NewPctUpDown(ud, ud), 0.7)
  } else if randWalkType == RandWalkPct {
    // 250 trading days * 390 minutes per day * 60 seconds per minute
    NewRandPctWalker(initial, 0.01, 0.01, 0.25, 250*390*60)
  } else {
    ud := initial / 100.0
    NewRandWalker(initial, NewAbsUpDown(ud, ud), 0.6)
  }

  bs.prices.PushBack(priceTime{walker.Value(), 0})
  tick := time.NewTicker(time.Second)
  t := uint64(0)
  for range tick.C {
    t++
    pt := priceTime{walker.Next(), t}
    pricesChan.Send(pt)
    bs.prices.PushBack(pt)
  }
}
