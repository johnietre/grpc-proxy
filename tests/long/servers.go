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
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/resolver"
)

func init() {
  resolver.Register(NewAddrsResolverBuilder())
}

type resolverBuilder struct {}

func NewAddrsResolverBuilder() resolver.Builder {
  return &resolverBuilder{}
}

func (rb *resolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
  addrStrs := strings.Split(target.Endpoint(), ",")
  addrs := make([]resolver.Address, 0, len(addrStrs))
  for _, addr := range addrStrs {
    addrs = append(addrs, resolver.Address{
      Addr: addr,
    })
  }
  cc.UpdateState(resolver.State{
    Endpoints: []resolver.Endpoint{resolver.Endpoint{Addresses: addrs}},
  })
  return &myResolver{
    target: target,
    cc: cc,
  }, nil
}

func (rb *resolverBuilder) Scheme() string {
  return "myips"
}

type myResolver struct {
  addrs []string
  target resolver.Target
  cc resolver.ClientConn
}

func (mr *myResolver) ResolveNow(resolver.ResolveNowOptions) {}

func (mr *myResolver) Close() {}

type Conns struct {
  conns []*grpc.ClientConn
}

func NewConns() *Conns {
  return &Conns{}
}

func (cs *Conns) NewClient(
  addr string,
) (pb.TestAPIClient, *grpc.ClientConn, error) {
  c, conn, err := newClient(addr)
  if err == nil {
    cs.conns = append(cs.conns, conn)
  }
  return c, conn, err
}

func (cs *Conns) CloseAll() {
  for _, conn := range cs.conns {
    conn.Close()
  }
  cs.conns = nil
}

func newClient(addr string) (pb.TestAPIClient, *grpc.ClientConn, error) {
  conn, err := grpc.NewClient(
    addr,
    grpc.WithTransportCredentials(insecure.NewCredentials()),
    grpc.WithKeepaliveParams(keepalive.ClientParameters{
      Time: time.Second * 60,
      Timeout: time.Second,
      PermitWithoutStream: true,
    }),
    //grpc.WithResolvers(NewAddrsResolverBuilder()),
  )
  if err != nil {
    return nil, nil, nil
  }
  c := pb.NewTestAPIClient(conn)
  return c, conn, nil
}

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

type clientStreamServer struct {
  pb.UnimplementedTestAPIServer
}

func runClientStreamServer(addr string) error {
  ln, err := net.Listen("tcp", addr)
  if err != nil {
    return err
  }
  s := grpc.NewServer()
  pb.RegisterTestAPIServer(s, &clientStreamServer{})
  return s.Serve(ln)
}

func (clientStreamServer) TestClientStream(stream pb.TestAPI_TestClientStreamServer) error {
  req, err, sum := (*pb.TestClientStreamRequest)(nil), (error)(nil), uint64(0)
  for req, err = stream.Recv(); err == nil; req, err = stream.Recv() {
    sum += req.GetNum()
  }
  if err != io.EOF {
    return err
  }
  return stream.SendAndClose(&pb.TestClientStreamResponse{Sum: sum})
}

func runServerStreamServer(addr string) error {
  ln, err := net.Listen("tcp", addr)
  if err != nil {
    return err
  }
  s := grpc.NewServer()
  pb.RegisterTestAPIServer(s, &serverStreamServer{})
  return s.Serve(ln)
}

type serverStreamServer struct {
  pb.UnimplementedTestAPIServer
}

func (serverStreamServer) TestServerStream(
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

type uniStreamServer struct {
  pb.UnimplementedTestAPIServer
  css clientStreamServer
  sss serverStreamServer
}

func runUniStreamServer(addr string) error {
  ln, err := net.Listen("tcp", addr)
  if err != nil {
    return err
  }
  s := grpc.NewServer()
  pb.RegisterTestAPIServer(s, &uniStreamServer{})
  return s.Serve(ln)
}

func (us uniStreamServer) TestClientStream(stream pb.TestAPI_TestClientStreamServer) error {
  return us.css.TestClientStream(stream)
}

func (us uniStreamServer) TestServerStream(
  req *pb.TestServerStreamRequest,
  stream pb.TestAPI_TestServerStreamServer,
) error {
  return us.sss.TestServerStream(req, stream)
}

/*** Bidirectional ***/

const (
  RandWalkRandAbs int = iota
  RandWalkRandPct
  RandWalkPct
)

type biStreamServer struct {
  prices *AtomicList[priceTime]
  subscribers *utils.SyncSet[pb.TestAPI_TestBiStreamServer]

  pb.UnimplementedTestAPIServer
}

func runBiStreamServer(addr string, randWalkType int) error {
  ln, err := net.Listen("tcp", addr)
  if err != nil {
    return err
  }
  s := grpc.NewServer()
  pb.RegisterTestAPIServer(s, newBiStreamServer(randWalkType))
  return s.Serve(ln)
}

func newBiStreamServer(randWalkType int) *biStreamServer {
  bs := &biStreamServer{
    prices: NewAtomicList[priceTime](),
    subscribers: utils.NewSyncSet[pb.TestAPI_TestBiStreamServer](),
  }
  go bs.run(randWalkType)
  return bs
}

func (bs *biStreamServer) TestBiStream(stream pb.TestAPI_TestBiStreamServer) error {
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
    resp := &pb.TestBiStreamResponse{Start: start, End: end}
    if start > end && end != 0 {
      resp.Error = utils.NewT("invalid range")
      if err := stream.Send(resp); err != nil {
        return err
      }
      continue
    }
    startNode := bs.prices.Get(start)
    if startNode == nil {
      resp.Error = utils.NewT("invalid start")
      if err := stream.Send(resp); err != nil {
        return err
      }
      continue
    }
    if end == start {
      end++
    }
    var prices []float64
    if end != 0 {
      prices = make([]float64, 0, end - start)
    }
    startNode.RangeToTail(func(node *AtomicNode[priceTime]) bool {
      if node.Index() == end && end != 0 {
        return false
      }
      prices = append(prices, node.Value().price)
      return true
    })
    resp.Prices, resp.End = prices, start + uint64(len(prices))
    if err := stream.Send(resp); err != nil {
      return err
    }
  }
}

type priceTime struct {
  price float64
  end uint64
}

func (bs *biStreamServer) run(randWalkType int) {
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
    walker = NewRandWalker(initial, NewPctUpDown(ud, ud), 0.7)
  } else if randWalkType == RandWalkPct {
    // 250 trading days * 390 minutes per day * 60 seconds per minute
    walker = NewRandPctWalker(initial, 0.01, 0.01, 0.25, 250*390*60)
  } else {
    ud := initial / 100.0
    walker = NewRandWalker(initial, NewAbsUpDown(ud, ud), 0.6)
  }

  t := uint64(1)
  bs.prices.PushBack(priceTime{walker.Value(), t})
  tick := time.NewTicker(time.Second)
  for range tick.C {
    t++
    pt := priceTime{walker.Next(), t}
    pricesChan.Send(pt)
    bs.prices.PushBack(pt)
  }
}

type biAbsUpDownServer struct {
  pb.UnimplementedTestAPIServer
  bss *biStreamServer
}

func runBiAbsUpDownServer(addr string) error {
  ln, err := net.Listen("tcp", addr)
  if err != nil {
    return err
  }
  s := grpc.NewServer()
  pb.RegisterTestAPIServer(s, newBiAbsUpDownServer())
  return s.Serve(ln)
}

func newBiAbsUpDownServer() *biAbsUpDownServer {
  return &biAbsUpDownServer{bss: newBiStreamServer(RandWalkRandAbs)}
}

func (bs *biAbsUpDownServer) TestBiAbsUpDown(stream pb.TestAPI_TestBiAbsUpDownServer) error {
  return bs.bss.TestBiStream(stream)
}

type biPctUpDownServer struct {
  pb.UnimplementedTestAPIServer
  bss *biStreamServer
}

func runBiPctUpDownServer(addr string) error {
  ln, err := net.Listen("tcp", addr)
  if err != nil {
    return err
  }
  s := grpc.NewServer()
  pb.RegisterTestAPIServer(s, newBiPctUpDownServer())
  return s.Serve(ln)
}

func newBiPctUpDownServer() *biPctUpDownServer {
  return &biPctUpDownServer{bss: newBiStreamServer(RandWalkRandPct)}
}

func (bs *biPctUpDownServer) TestBiPctUpDown(stream pb.TestAPI_TestBiPctUpDownServer) error {
  return bs.bss.TestBiStream(stream)
}

type biPctServer struct {
  pb.UnimplementedTestAPIServer
  bss *biStreamServer
}

func runBiPctServer(addr string) error {
  ln, err := net.Listen("tcp", addr)
  if err != nil {
    return err
  }
  s := grpc.NewServer()
  pb.RegisterTestAPIServer(s, newBiPctServer())
  return s.Serve(ln)
}

func newBiPctServer() *biPctServer {
  return &biPctServer{bss: newBiStreamServer(RandWalkPct)}
}

func (bs *biPctServer) TestBiPct(stream pb.TestAPI_TestBiPctServer) error {
  return bs.bss.TestBiStream(stream)
}

/*** 4 basic ***/

type basic4Server struct {
  pb.UnimplementedTestAPIServer
  rrs *reqRespServer
  uss *uniStreamServer
  bss *biStreamServer
}

func runBasic4Server(addr string, randWalkType int) error {
  ln, err := net.Listen("tcp", addr)
  if err != nil {
    return err
  }
  s := grpc.NewServer()
  pb.RegisterTestAPIServer(s, newBasic4Server(randWalkType))
  
  return s.Serve(ln)
}

func newBasic4Server(randWalkType int) *basic4Server {
  return &basic4Server{
    rrs: &reqRespServer{},
    uss: &uniStreamServer{},
    bss: newBiStreamServer(randWalkType),
  }
}

func (b4s *basic4Server) Test(
  ctx context.Context,
  req *pb.TestRequest,
) (*pb.TestResponse, error) {
  return b4s.rrs.Test(ctx, req)
}

func (b4s *basic4Server) TestClientStream(stream pb.TestAPI_TestClientStreamServer) error {
  return b4s.uss.TestClientStream(stream)
}

func (b4s *basic4Server) TestServerStream(
  req *pb.TestServerStreamRequest,
  stream pb.TestAPI_TestServerStreamServer,
) error {
  return b4s.uss.TestServerStream(req, stream)
}

func (b4s *basic4Server) TestBiStream(stream pb.TestAPI_TestBiStreamServer) error {
  return b4s.bss.TestBiStream(stream)
}

/*** Full ***/

type fullServer struct {
  pb.UnimplementedTestAPIServer
  b4s *basic4Server
  baud *biAbsUpDownServer
  bpud *biPctUpDownServer
  bp *biPctServer
}

func runFullServer(addr string, randWalkType int) error {
  ln, err := net.Listen("tcp", addr)
  if err != nil {
    return err
  }
  s := grpc.NewServer()
  srvr := &fullServer{
    b4s: newBasic4Server(randWalkType),
    baud: newBiAbsUpDownServer(),
    bpud: newBiPctUpDownServer(),
    bp: newBiPctServer(),
  }
  pb.RegisterTestAPIServer(s, srvr)
  
  return s.Serve(ln)
}

func (fs *fullServer) Test(
  ctx context.Context,
  req *pb.TestRequest,
) (*pb.TestResponse, error) {
  return fs.b4s.Test(ctx, req)
}

func (fs *fullServer) TestClientStream(
  stream pb.TestAPI_TestClientStreamServer,
) error {
  return fs.b4s.TestClientStream(stream)
}

func (fs *fullServer) TestServerStream(
  req *pb.TestServerStreamRequest,
  stream pb.TestAPI_TestServerStreamServer,
) error {
  return fs.b4s.TestServerStream(req, stream)
}

func (fs *fullServer) TestBiStream(
  stream pb.TestAPI_TestBiStreamServer,
) error {
  return fs.b4s.TestBiStream(stream)
}

func (fs *fullServer) TestBiAbsUpDown(
  stream pb.TestAPI_TestBiAbsUpDownServer,
) error {
  return fs.baud.TestBiAbsUpDown(stream)
}

func (fs *fullServer) TestBiPctUpDown(
  stream pb.TestAPI_TestBiPctUpDownServer,
) error {
  return fs.bpud.TestBiPctUpDown(stream)
}

func (fs *fullServer) TestBiPct(stream pb.TestAPI_TestBiPctServer) error {
  return fs.bp.TestBiPct(stream)
}
