package grpcproxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	urlpkg "net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	utils "github.com/johnietre/utils/go"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type PathsMap = map[string]PathConfig

type AppConfig struct {
	ConfigPath     string
	Config         Config
	Password       string
	Srvr0          *ListenerConfig
	GlobalCertPath string
	GlobalKeyPath  string
}

type App struct {
	configPath                    string
	config                        *utils.RWMutex[Config]
	paths                         *utils.AValue[PathsMap]
	globalCertPath, globalKeyPath string

	password string

	// srvr0 is the server that never closes (unless of error).
	srvr0   *Server
	servers *utils.RWMutex[Slice[*Server]]
	srvrId  atomic.Uint64

	shuttingDown atomic.Bool
	shutdownChan chan utils.Unit
	wg           sync.WaitGroup
}

func NewApp(cfg *AppConfig) *App {
	app := &App{
		configPath:     cfg.ConfigPath,
		config:         utils.NewRWMutex[Config](cfg.Config),
		paths:          utils.NewAValue[PathsMap](PathsMap{}),
		globalCertPath: cfg.GlobalCertPath,
		globalKeyPath:  cfg.GlobalKeyPath,
		password:       cfg.Password,
		servers: utils.NewRWMutex[Slice[*Server]](Slice[*Server]{
			Data: []*Server{},
		}),
		shutdownChan: make(chan utils.Unit),
	}
	if cfg.Srvr0 != nil {
		app.srvr0 = app.newServer(0, *cfg.Srvr0)
	}
	return app
}

func (a *App) Run() {
	if a.srvr0 != nil {
		log.Printf("Starting srvr0 on %s", a.srvr0.LnConfig.Addr)
		go a.runSrvr0()
	}
	cfg := *a.config.RLock()
	a.config.RUnlock()
	if err := cfg.CheckValid(); err != nil {
		log.Print("invalid config: ", err)
	}
	if err := a.applyConfig(cfg); err != nil {
		log.Print("error setting config: ", err)
	}

	intCh := make(chan os.Signal, 5)
	signal.Notify(intCh, os.Interrupt)
	go a.listenSignals(intCh)

	<-a.shutdownChan
	a.wg.Wait()
	log.Print("EXITING")
}

func (a *App) proxy(w http.ResponseWriter, r *http.Request) {
	a.wg.Add(1)
	defer a.wg.Done()
	pm := ProxyMapFromCtx(r.Context())
	if pm == nil {
		// TODO: LOG
		log.Print("missing proxy map in context")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	path := r.URL.Path
	if path == "" || path == "/" {
		a.handle(w, r)
		return
	}
	proxy := pm.Get(path)
	if proxy == nil {
		paths := a.getPaths()
		pathConfig, ok := paths[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		for _, url := range pathConfig.Urls {
			if !hostReachable(url.Host) {
				// TODO: Do something with error?
				continue
			}
			proxy = pm.GetOrNew(path, url.URL)
		}
	}
	if proxy == nil {
		// TODO: LOG
		// TODO: Status code
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	proxy.ServeHTTP(w, r)
}

const (
	HeaderBase          = "X-Grpc-Proxy"
	HeaderPassword      = "X-Grpc-Proxy-Password"
	HeaderRefreshAction = "X-Grpc-Proxy-Refresh-Action"
	HeaderConfigLen     = "X-Grpc-Proxy-Config-Len"
	HeaderStatusLen     = "X-Grpc-Proxy-Status-Len"
)

func (a *App) handle(w http.ResponseWriter, r *http.Request) {
	if a.password != "" {
		pwd := r.Header.Get(HeaderPassword)
		if pwd != a.password {
			http.Error(w, "invalid password", http.StatusUnauthorized)
			return
		}
	}

	f, want := a.handleGet, ""
	switch r.Method {
	case http.MethodGet:
		f, want = a.handleGet, "get"
	case http.MethodPost, http.MethodPut:
		f, want = a.handleRefresh, "refresh"
	case http.MethodDelete:
		f, want = a.handleShutdown, "shutdown"
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
	got := strings.ToLower(r.Header.Get(HeaderBase))
	if got != want {
		http.Error(
			w,
			fmt.Sprintf(
				`expected value %q, got %q for %q header`,
				want, got, HeaderBase,
			),
			http.StatusBadRequest,
		)
		return
	}
	if !f(w, r) {
		return
	}
	a.config.RApply(func(cfg *Config) {
		if err := json.NewEncoder(w).Encode(cfg); err != nil {
			// TODO
		}
	})
}

func (a *App) handleGet(w http.ResponseWriter, r *http.Request) (ok bool) {
	getConfig, _ := strconv.ParseBool(r.URL.Query().Get("config"))
	getStatus, _ := strconv.ParseBool(r.URL.Query().Get("status"))

	buf, err := bytes.NewBuffer(nil), error(nil)
	if getConfig {
		a.config.RApply(func(cp *Config) {
			err = cp.EncodeInto(buf)
		})
		if err != nil {
			errMsg := fmt.Sprint("error serializing config: ", err)
			log.Print(errMsg)
			http.Error(w, errMsg, http.StatusInternalServerError)
			return
		}
	}
	cfgLen := buf.Len()
	if getStatus {
		var statuses map[string]string
		a.servers.RApply(func(sp *Slice[*Server]) {
			statuses = make(map[string]string, sp.Len())
			for _, srvr := range sp.Data {
				switch srvr.status.Load() {
				case StatusRunning:
					statuses[srvr.LnConfig.Addr.String()] = "RUNNING"
				case StatusShuttingDown:
					statuses[srvr.LnConfig.Addr.String()] = "SHUTTING DOWN"
				case StatusStopped:
					statuses[srvr.LnConfig.Addr.String()] = "STOPPED"
				}
				srvr.status.Load()
			}
		})
		if err := json.NewEncoder(buf).Encode(statuses); err != nil {
			errMsg := fmt.Sprint("error serializing statuses: ", err)
			log.Print(errMsg)
			http.Error(w, errMsg, http.StatusInternalServerError)
			return
		}
	}
	statusLen := buf.Len() - cfgLen
	w.Header().Set(HeaderBase, "ok")
	w.Header().Set(HeaderConfigLen, strconv.Itoa(cfgLen))
	w.Header().Set(HeaderStatusLen, strconv.Itoa(statusLen))
	w.Write(buf.Bytes())
	return false
}

func (a *App) handleRefresh(w http.ResponseWriter, r *http.Request) (ok bool) {
	action := r.Header.Get(HeaderRefreshAction)
	switch action {
	case "set", "refresh", "add", "del":
	default:
		http.Error(
			w,
			fmt.Sprintf(
				`invalid value %q for %q header`,
				action, HeaderRefreshAction,
			),
			http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		// TODO: What to do for error
		http.Error(w, "error reading body: "+err.Error(), http.StatusBadRequest)
		return
	}
	var cfg Config
	if len(body) != 0 {
		cfg, err = ReadConfigAndCheck(bytes.NewBuffer(body))
	} else {
		cfg, err = LoadConfigAndCheck(a.configPath)
	}
	if err != nil {
		http.Error(w, "error parsing config: "+err.Error(), http.StatusBadRequest)
		return
	}
	switch action {
	case "set", "refresh":
		a.applyConfig(cfg)
	case "add":
		a.addToConfig(cfg)
	case "del":
		a.delFromConfig(cfg)
	}
	return true
}

func (a *App) handleShutdown(w http.ResponseWriter, r *http.Request) (ok bool) {
	force, _ := strconv.ParseBool(r.URL.Query().Get("force"))
	if force {
		Die(0, "EXITING")
	}
	a.shutdown()
	return false
}

// applyConfig expects the Config to be valid.
func (a *App) applyConfig(cfg Config) error {
	if a.isShuttingDown() {
		return ErrShuttingDown
	}
	if err := a.checkCfgPaths(cfg); err != nil {
		return err
	}
	lnConfigs := NewClonedSlice(cfg.GrpcProxy.Listeners)
	a.config.Apply(func(cp *Config) {
		a.servers.Apply(func(sp *Slice[*Server]) {
			for i := sp.Len() - 1; i >= 0; i-- {
				srvr := sp.Get(i)
				removed := false
				for j := lnConfigs.Len() - 1; j >= 0; j-- {
					other := lnConfigs.Get(j)
					if srvr.LnConfig.Eq(other) {
						lnConfigs.Remove(j)
						removed = true
						break
					}
				}
				if !removed {
					// NOTE: server removed when the server exits
					go srvr.Shutdown(context.Background())
				}
			}
			for _, lnc := range lnConfigs.Data {
				id := a.lockedNextSrvrId(sp)
				srvr := a.newServer(id, lnc)
				a.wg.Add(1)
				go func(srvr *Server) {
					a.runServer(srvr)
					a.wg.Done()
				}(srvr)
			}
		})
		a.setPaths(cfg.Paths)
		*cp = cfg
	})
	return nil
}

func (a *App) addToConfig(cfg Config) error {
	if a.isShuttingDown() {
		return ErrShuttingDown
	}
	if err := a.checkCfgPaths(cfg); err != nil {
		return err
	}
	lnConfigs := NewClonedSlice(cfg.GrpcProxy.Listeners)
	a.config.Apply(func(cp *Config) {
		a.servers.Apply(func(sp *Slice[*Server]) {
			// Start servers that aren't running and add new listeners
			lncs := NewClonedSlice(cp.GrpcProxy.Listeners)
			for _, lnc := range lnConfigs.Data {
				if !sp.Contains(func(srvr *Server) bool {
					return srvr.LnConfig.Eq(lnc)
				}) {
					id := a.lockedNextSrvrId(sp)
					srvr := a.newServer(id, lnc)
					a.wg.Add(1)
					go func(srvr *Server) {
						a.runServer(srvr)
						a.wg.Done()
					}(srvr)
				}
				if !lncs.Contains(func(other ListenerConfig) bool {
					return lnc.AddrEq(other)
				}) {
					lncs.Append(lnc)
				}
			}

			// Add paths
			paths := CloneMap(cp.Paths)
			for path, pathCfg := range cfg.Paths {
				if pc, ok := paths[path]; !ok {
					paths[path] = pathCfg
				} else {
					urls := NewSlice(pc.Urls)
					// Check to see if new URLs are in old
					for _, nurl := range pathCfg.Urls {
						if !urls.Contains(func(url *URL) bool {
							return url.Eq(nurl)
						}) {
							urls.Append(nurl)
						}
					}
					paths[path] = PathConfig{Urls: urls.Data}
				}
			}
			// Add set listeners and paths
			cp.GrpcProxy.Listeners = lncs.Data
			cp.Paths = paths
			a.setPaths(paths)
		})
	})
	return nil
}

func (a *App) delFromConfig(cfg Config) error {
	if a.isShuttingDown() {
		return ErrShuttingDown
	}
	lnConfigs := NewClonedSlice(cfg.GrpcProxy.Listeners)
	a.config.Apply(func(cp *Config) {
		a.servers.Apply(func(sp *Slice[*Server]) {
			// Shutdown servers
			for i := sp.Len() - 1; i >= 0; i-- {
				srvr := sp.Get(i)
				if lnConfigs.Contains(func(other ListenerConfig) bool {
					return srvr.LnConfig.Eq(other)
				}) {
					go srvr.Shutdown(context.Background())
				}
			}
			// Remove from listeners
			lncs := NewClonedSlice(cp.GrpcProxy.Listeners)
			for _, dlnc := range lnConfigs.Data {
				lncs.RemoveFirst(func(lnc ListenerConfig) bool {
					return lnc.Eq(dlnc)
				})
			}
			// Remove the paths
			paths := CloneMap(cp.Paths)
			for path, pathCfg := range cfg.Paths {
				if pc, ok := paths[path]; ok {
					urls := NewSlice(pc.Urls)
					for _, durl := range pathCfg.Urls {
						urls.RemoveFirst(func(url *URL) bool {
							return url.Eq(durl)
						})
					}
					if urls.Len() != 0 {
						paths[path] = PathConfig{Urls: urls.Data}
					} else {
						delete(paths, path)
					}
				}
			}
			cp.GrpcProxy.Listeners = lncs.Data
			cp.Paths = paths
			a.setPaths(paths)
		})
	})
	return nil
}

func (a *App) shutdown() {
	if a.shuttingDown.Swap(true) {
		return
	}
	close(a.shutdownChan)
	log.Print("SHUTTING DOWN")
	a.servers.Apply(func(sp *Slice[*Server]) {
		for _, srvr := range sp.Data {
			a.wg.Add(1)
			go func(srvr *Server) {
				srvr.Shutdown(context.Background())
				a.wg.Done()
			}(srvr)
		}
	})
}

// checkCfgPaths checks the CertPath and KeyPath fields of the listeners to
// make sure they are valid.
func (a *App) checkCfgPaths(cfg Config) error {
	for i := range cfg.GrpcProxy.Listeners {
		lnc := &cfg.GrpcProxy.Listeners[i]
		if lnc.CertPath == "-" {
			if a.globalCertPath == "" {
				return ErrNoGlobalCertFile
			}
			lnc.CertPath = a.globalCertPath
		}
		if lnc.KeyPath == "-" {
			if a.globalKeyPath == "" {
				return ErrNoGlobalKeyFile
			}
			lnc.KeyPath = a.globalKeyPath
		}
	}
	return nil
}

func (a *App) newServer(id uint64, lnc ListenerConfig) *Server {
	h2s := &http2.Server{
		MaxConcurrentStreams: 50,
	}
	return &Server{
		Id:       id,
		LnConfig: lnc,
		Srvr: &http.Server{
			Handler:  h2c.NewHandler(http.HandlerFunc(a.proxy), h2s),
			ErrorLog: log.Default(),
			ConnState: func(_ net.Conn, state http.ConnState) {
			},
			ConnContext: func(ctx context.Context, conn net.Conn) context.Context {
				return context.WithValue(ctx, pmCtxKey, NewProxyMap())
			},
		},
	}
}

func (a *App) runServer(srvr *Server) {
	a.servers.Apply(func(sp *Slice[*Server]) {
		sp.Append(srvr)
	})
	defer a.servers.Apply(func(sp *Slice[*Server]) {
		for i, s := range sp.Data {
			if s.Id == srvr.Id {
				sp.Remove(i)
				return
			}
		}
	})
	if a.isShuttingDown() {
		return
	}
	log.Printf("Starting Server %d (%s)", srvr.Id, srvr.LnConfig.Addr)
	err := srvr.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		// TODO: LOG
		log.Printf("Server %d (%s) closed", srvr.Id, srvr.LnConfig.Addr)
	} else {
		// TODO: LOG
		log.Printf(
			"Server %d (%s) stopped with error: %v",
			srvr.Id, srvr.LnConfig.Addr, err,
		)
	}
}

func (a *App) runSrvr0() {
	// TODO
	err := a.srvr0.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		// TODO: LOG
		log.Printf("Server 0 (%s) closed", a.srvr0.LnConfig.Addr)
	} else {
		// TODO: LOG
		log.Printf(
			"Server 0 (%s) stopped with error: %v",
			a.srvr0.LnConfig.Addr, err,
		)
	}
}

func (a *App) getPaths() map[string]PathConfig {
	return a.paths.Load()
}

func (a *App) setPaths(paths map[string]PathConfig) {
	a.paths.Store(paths)
}

func (a *App) nextSrvrId() uint64 {
	defer a.servers.RUnlock()
	return a.lockedNextSrvrId(a.servers.RLock())
}

func (a *App) lockedNextSrvrId(srvrs *Slice[*Server]) uint64 {
	for {
		id := a.srvrId.Add(1)
		if id == 0 {
			continue
		}
		for _, srvr := range srvrs.Data {
			if srvr.Id == id {
				continue
			}
		}
		return id
	}
}

func (a *App) isShuttingDown() bool {
	return a.shuttingDown.Load()
}

func (a *App) listenSignals(ch chan os.Signal) {
	// TODO: Other signals
	exiting := false
	for range ch {
		if exiting {
			Die(0, "EXITING")
		}
		exiting = true
		a.shutdown()
	}
}

type pmCtxKeyType string

const (
	pmCtxKey pmCtxKeyType = "proxymapkey"
)

var (
	ErrShuttingDown     = errors.New("shutting down")
	ErrNoGlobalCertFile = errors.New("new global cert.pem file")
	ErrNoGlobalKeyFile  = errors.New("new global key.pem file")
	_                   = io.EOF
)

const (
	StatusNotRunning uint32 = iota
	StatusRunning
	StatusShuttingDown
	StatusStopped
)

type Server struct {
	Id       uint64
	LnConfig ListenerConfig
	Srvr     *http.Server
	status   atomic.Uint32
}

func (s *Server) ListenAndServe() error {
	if !s.status.CompareAndSwap(StatusNotRunning, StatusRunning) {
		return http.ErrServerClosed
	}
	defer s.status.CompareAndSwap(StatusRunning, StatusShuttingDown)
	ln, err := newListener(s.LnConfig.Addr)
	if err != nil {
		return err
	}
	if s.LnConfig.CertPath != "" {
		return s.Srvr.ServeTLS(ln, s.LnConfig.CertPath, s.LnConfig.KeyPath)
	} else {
		return s.Srvr.Serve(ln)
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.status.CompareAndSwap(StatusRunning, StatusShuttingDown)
	defer s.status.Store(StatusStopped)
	return s.Srvr.Shutdown(ctx)
}

func (s *Server) Status() uint32 {
	return s.status.Load()
}

type ProxyMap struct {
	Rps *utils.SyncMap[string, *httputil.ReverseProxy]
}

func NewProxyMap() *ProxyMap {
	return &ProxyMap{
		Rps: utils.NewSyncMap[string, *httputil.ReverseProxy](),
	}
}

func ProxyMapFromCtx(ctx context.Context) *ProxyMap {
	ipm := ctx.Value(pmCtxKey)
	if ipm == nil {
		return nil
	}
	return ipm.(*ProxyMap)
}

func (pm *ProxyMap) Get(path string) *httputil.ReverseProxy {
	rp, _ := pm.Rps.Load(path)
	return rp
}

func (pm *ProxyMap) GetOrNew(path string, url *urlpkg.URL) *httputil.ReverseProxy {
	rp, loaded := pm.Rps.Load(path)
	if !loaded {
		rp, _ = pm.Rps.LoadOrStore(path, newRevProxy(url))
	}
	return rp
}

func newRevProxy(url *urlpkg.URL) *httputil.ReverseProxy {
	rp := httputil.NewSingleHostReverseProxy(url)
	transport := &http2.Transport{
		AllowHTTP: true,
	}
	if url.Scheme != "https" {
		transport.DialTLSContext = func(
			ctx context.Context, ntwk, addr string, cfg *tls.Config,
		) (net.Conn, error) {
			dialer := &net.Dialer{}
			return dialer.DialContext(ctx, ntwk, addr)
		}
	}
	rp.Transport = transport
	return rp
}

func hostReachable(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		// TODO: Do more with error?
		return false
	}
	conn.Close()
	return true
}

func newListener(addr *Addr) (net.Listener, error) {
	//return net.ListenTCP("tcp", addr.TCPAddr)
	lc := net.ListenConfig{
		Control: func(network, addr string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				if err := syscall.SetsockoptInt(
					int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1,
				); err != nil {
					// TODO
					_ = fmt.Errorf("error setting SO_REUSEADDR: %v", err)
				}
			})
		},
	}
	return lc.Listen(context.Background(), "tcp", addr.String())
}
