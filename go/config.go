package grpcproxy

import (
	"fmt"
	"io"
	"net"
	urlpkg "net/url"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	GrpcProxy GrpcProxyConfig       `toml:"grpc-proxy"`
	Paths     map[string]PathConfig `toml:"path"`
}

func LoadConfigAndCheck(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer f.Close()
	return ReadConfigAndCheck(f)
}

func ReadConfigAndCheck(r io.Reader) (Config, error) {
	var cfg Config
	_, err := toml.DecodeReader(r, &cfg)
	if err == nil {
		err = cfg.CheckValid()
	}
	return cfg, err
}

func (cfg Config) EncodeString() (string, error) {
	b := &strings.Builder{}
	err := cfg.EncodeInto(b)
	return b.String(), err
}

func (cfg Config) EncodeInto(w io.Writer) error {
	return toml.NewEncoder(w).Encode(cfg)
}

func (cfg *Config) CheckValid() error {
	lnConfigs := cfg.GrpcProxy.Listeners
	for i, lnc := range lnConfigs {
		if (lnc.CertPath == "" && lnc.KeyPath != "") ||
			(lnc.CertPath != "" && lnc.KeyPath == "") {
			return fmt.Errorf(
				"Must specify both cert and key paths, or neither (listener index %d)",
				i,
			)
		}
		for j, other := range lnConfigs[i+1:] {
			if lnc.Addr.Port == other.Addr.Port && lnc.Addr.IP.Equal(other.Addr.IP) {
				return fmt.Errorf(
					"Duplicate listener addresses (indexes %d and %d)", i, j,
				)
			}
		}
	}
	for path := range cfg.Paths {
		if path == "" || path == "/" {
			return fmt.Errorf(`Invalid path: %q`, path)
		}
	}
	return nil
}

type GrpcProxyConfig struct {
	Listeners []ListenerConfig
}

type ListenerConfig struct {
	Addr     *Addr
	CertPath string `toml:"cert-path"`
	KeyPath  string `toml:"key-path"`
}

func (lnc ListenerConfig) Eq(other ListenerConfig) bool {
	return lnc.Addr.Port == other.Addr.Port &&
		lnc.Addr.IP.Equal(other.Addr.IP) &&
		lnc.CertPath == other.CertPath &&
		lnc.KeyPath == other.KeyPath
}

func (lnc ListenerConfig) AddrEq(other ListenerConfig) bool {
	return lnc.Addr.Port == other.Addr.Port && lnc.Addr.IP.Equal(other.Addr.IP)
}

type PathConfig struct {
	Urls []*URL
}

/*** Types ***/

type Addr struct {
	*net.TCPAddr
}

func (a *Addr) MarshalText() ([]byte, error) {
	return []byte(a.String()), nil
}

func (a *Addr) UnmarshalText(text []byte) error {
	var err error
	a.TCPAddr, err = net.ResolveTCPAddr("tcp", string(text))
	return err
}

// Set implements the Set method of the pflag.Value interface.
func (a *Addr) Set(s string) error {
	var err error
	a.TCPAddr, err = net.ResolveTCPAddr("tcp", s)
	return err
}

// String implements the String method of the pflag.Value interface.
func (a *Addr) String() string {
	if a.TCPAddr == nil {
		return ""
	}
	return a.TCPAddr.String()
}

// Type implements the Type method of the pflag.Value interface.
func (a *Addr) Type() string {
	return "Addr"
}

type URL struct {
	*urlpkg.URL
}

func (url *URL) MarshalText() ([]byte, error) {
	if url.URL == nil {
		url.URL = new(urlpkg.URL)
	}
	return url.URL.MarshalBinary()
}

func (url *URL) UnmarshalText(text []byte) error {
	if url.URL == nil {
		url.URL = new(urlpkg.URL)
	}
	return url.URL.UnmarshalBinary(text)
}

// TODO: Do better?
func (url *URL) Eq(other *URL) bool {
	return url.String() == other.String()
}
