use anyhow::Result as ARes;
use log4rs::filter::{Filter, Response};
use serde::{Deserialize, Serialize};
use std::{fmt, fs, mem};
use std::collections::hash_map::{Entry as HMEntry, HashMap};
use std::net::{AddrParseError, SocketAddr};
use std::str::FromStr;

#[derive(Debug, PartialEq, Eq, Clone)]
pub struct TargetFilter(pub String);

impl Filter for TargetFilter {
    fn filter(&self, record: &log::Record<'_>) -> Response {
        eprintln!("{record:?}");
        if record.target() == self.0 {
            Response::Neutral
        } else {
            Response::Reject
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TargetsFilter(pub std::collections::HashSet<String>);

impl Filter for TargetsFilter {
    fn filter(&self, record: &log::Record<'_>) -> Response {
        if self.0.contains(record.target()) {
            Response::Neutral
        } else {
            Response::Reject
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct ProxyPaths {
    pub paths: HashMap<String, Vec<Addr>>,
}

impl std::ops::Deref for ProxyPaths {
    type Target = HashMap<String, Vec<Addr>>;

    fn deref(&self) -> &Self::Target {
        &self.paths
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct Addr {
    pub addr: SocketAddr,
    #[serde(default)]
    pub secure: bool,
}

#[derive(Debug)]
pub struct ParseAddrError {
    pub addr_str: String,
    pub ape: AddrParseError,
}

impl ParseAddrError {
    pub fn new(addr_str: impl ToString, ape: AddrParseError) -> Self {
        Self {
            addr_str: addr_str.to_string(),
            ape,
        }
    }
}

impl fmt::Display for ParseAddrError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}: {}", self.addr_str, self.ape)
    }
}

impl std::error::Error for ParseAddrError {}

impl FromStr for Addr {
    //type Err = AddrParseError;
    type Err = ParseAddrError;

    fn from_str(addr_str: &str) -> Result<Self, Self::Err> {
        let mut s = addr_str;
        let secure = if s.starts_with("https://") {
            s = &s[8..];
            true
        } else {
            if s.starts_with("http://") {
                s = &s[7..];
            }
            false
        };
        s.parse()
            .map(|addr| Self { addr, secure })
            .map_err(|e| ParseAddrError::new(addr_str, e))
    }
}

impl fmt::Display for Addr {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        if self.secure {
            write!(f, "https://{}", self.addr)
        } else {
            write!(f, "http://{}", self.addr)
        }
    }
}

pub const EXAMPLE_CONFIG: &str =
    include_str!(concat!(env!("CARGO_MANIFEST_DIR"), "/EXAMPLE_CONFIG.toml"));

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Config {
    #[serde(default)]
    pub proxy: ProxyConfig,
    #[serde(default)]
    pub paths: HashMap<String, ConfigPathAddrs>,
    #[serde(default, rename = "addr")]
    pub addrs: Vec<ConfigAddr>,
}

impl Config {
    pub fn empty() -> Self {
        Config {
            proxy: ProxyConfig {
                addr: AddrsEnum::Multi(Vec::new()),
                lb: None,
            },
            paths: Default::default(),
            addrs: Vec::new(),
        }
    }

    pub fn from_file<P: AsRef<std::path::Path>>(path: P) -> ARes<Self> {
        let config_str = fs::read_to_string(path)?;
        toml::from_str(&config_str).map_err(|e| e.into())
    }

    pub fn make_proxy_paths(&self) -> ARes<ProxyPaths> {
        let mut paths = HashMap::new();
        for (path, cpas) in &self.paths {
            let cpas = match cpas {
                ConfigPathAddrs::Single(ref cpa) => std::slice::from_ref(cpa),
                ConfigPathAddrs::Multi(ref cpas) => cpas.as_slice(),
            };
            let mut addrs = cpas
                .iter()
                .map(|cpa| {
                    let (addr, secure) = match cpa {
                        ConfigPathAddr::Simple(addr) => (addr.as_str(), false),
                        ConfigPathAddr::Combo(combo) => (combo.addr.as_str(), combo.secure),
                    };
                    Ok(Addr {
                        secure,
                        ..Addr::from_str(addr)?
                    })
                })
                .collect::<ARes<Vec<_>>>()?;
            paths
                .entry(path.clone())
                .or_insert(Vec::new())
                .append(&mut addrs);
        }
        for caddr in &self.addrs {
            let addr = Addr {
                secure: caddr.secure,
                ..Addr::from_str(&caddr.addr)?
            };
            for path in &caddr.paths {
                paths.entry(path.clone()).or_insert(Vec::new()).push(addr);
            }
        }
        Ok(ProxyPaths { paths })
    }

    pub fn add_path(&mut self, path: String, cpas: ConfigPathAddrs) -> Result<(), ConfigPathAddrs> {
        match self.paths.entry(path) {
            HMEntry::Occupied(mut ent) => ent.get_mut().add_unique_addrs(cpas),
            HMEntry::Vacant(ent) => {
                ent.insert(cpas);
                Ok(())
            }
        }
    }

    pub fn add_addr(&mut self, new_addr: ConfigAddr) -> Result<(), ConfigAddr> {
        for ca in &mut self.addrs {
            if ca.addr_eq(&new_addr) {
                let ConfigAddr {
                    addr,
                    secure,
                    paths,
                } = new_addr;
                let left_over = Vec::new();
                for path in paths {
                    if !ca.paths.contains(&path) {
                        ca.paths.push(path);
                    }
                }
                if left_over.len() != 0 {
                    return Err(ConfigAddr {
                        addr,
                        secure,
                        paths: left_over,
                    });
                }
                return Ok(());
            }
        }
        self.addrs.push(new_addr);
        Ok(())
    }

    pub fn del_path(&mut self, path: &str, cpas: Option<&ConfigPathAddrs>) {
        let Some(cpas) = cpas else {
            self.paths.remove(path);
            return;
        };
        let Some(old_cpas) = self.paths.get_mut(path) else {
            return;
        };
        old_cpas.del_addrs(cpas);
        if old_cpas.len() == 0 {
            self.paths.remove(path);
        }
    }

    pub fn del_addr(&mut self, addr: &ConfigAddr) -> bool {
        let Some(i) = self.addrs.iter().position(|oa| oa.addr_eq(&addr)) else {
            return false;
        };
        let old_addr = &mut self.addrs[i];
        let old_len = old_addr.paths.len();
        old_addr.paths.retain(|path| !addr.paths.contains(path));
        let new_len = old_addr.paths.len();
        if old_len == new_len {
            return false;
        } else if new_len == 0 {
            self.addrs.remove(i);
        }
        true
    }
}

#[allow(dead_code)]
#[derive(Debug, Default, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[repr(u32)]
pub enum LBType {
    #[default]
    #[serde(alias = "first")]
    First,
    #[serde(alias = "round-robin")]
    RoundRobin,
    #[serde(alias = "least-connections")]
    LeastConnections,
}

impl LBType {
    pub const fn from_u32(u: u32) -> Self {
        match u {
            1 => LBType::RoundRobin,
            2 => LBType::LeastConnections,
            _ => LBType::First,
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ProxyConfig {
    pub addr: AddrsEnum,
    #[allow(dead_code)]
    pub lb: Option<LBType>,
}

impl Default for ProxyConfig {
    fn default() -> Self {
        Self {
            addr: AddrsEnum::Multi(Vec::new()),
            lb: Default::default(),
        }
    }
}

impl ProxyConfig {
    pub fn make_addrs(&self) -> ARes<Vec<Addr>> {
        self.addr.to_addrs_vec()
    }
}

// TODO: Is the secure field necessary/how to handle?
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ConfigAddr {
    pub addr: String,
    #[serde(default)]
    pub secure: bool,
    pub paths: Vec<String>,
}

impl ConfigAddr {
    pub fn addr_eq(&self, other: &Self) -> bool {
        // TODO: Is secure check needed?
        self.addr == other.addr && self.secure == other.secure
    }
}

type AddrsEnum = ConfigPathAddrs;
#[allow(dead_code)]
type AddrEnum = ConfigPathAddr;
#[allow(dead_code)]
type AddrCombo = ConfigPathAddrCombo;

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(untagged)]
pub enum ConfigPathAddrs {
    Single(ConfigPathAddr),
    Multi(Vec<ConfigPathAddr>),
}

impl ConfigPathAddrs {
    pub fn to_addrs_vec(&self) -> ARes<Vec<Addr>> {
        let cpas = match self {
            ConfigPathAddrs::Single(ref cpa) => std::slice::from_ref(cpa),
            ConfigPathAddrs::Multi(ref cpas) => cpas.as_slice(),
        };
        cpas.iter().map(ConfigPathAddr::to_addr).collect()
    }

    #[allow(dead_code)]
    pub fn is_single(&self) -> bool {
        matches!(self, ConfigPathAddrs::Single(_))
    }

    pub fn len(&self) -> usize {
        match self {
            ConfigPathAddrs::Single(_) => 1,
            ConfigPathAddrs::Multi(ref v) => v.len(),
        }
    }

    pub fn add_unique_addr(&mut self, new_cpa: ConfigPathAddr) -> Result<(), ConfigPathAddr> {
        use ConfigPathAddrs::*;
        match self {
            Single(ref mut cpa) => {
                if cpa.addr_eq(&new_cpa) {
                    return Err(new_cpa);
                }
                let cpa = mem::replace(cpa, ConfigPathAddr::Simple(String::new()));
                *self = Multi(vec![cpa, new_cpa]);
            }
            Multi(ref mut cpas) => {
                for cpa in cpas.iter() {
                    if cpa.addr_eq(&new_cpa) {
                        return Err(new_cpa);
                    }
                }
                cpas.push(new_cpa);
            }
        }
        Ok(())
    }

    pub fn add_unique_addrs(&mut self, new_cpas: ConfigPathAddrs) -> Result<(), ConfigPathAddrs> {
        use ConfigPathAddrs::*;
        match new_cpas {
            Single(cpa) => self.add_unique_addr(cpa).map_err(Single),
            Multi(cpas) => {
                let mut left_over = Vec::new();
                for cpa in cpas {
                    if let Err(cpa) = self.add_unique_addr(cpa) {
                        left_over.push(cpa);
                    }
                }
                if left_over.len() != 0 {
                    Err(Multi(left_over))
                } else {
                    Ok(())
                }
            }
        }
    }

    pub fn del_addr(&mut self, cpa: &ConfigPathAddr) -> bool {
        use ConfigPathAddrs::*;
        match self {
            Single(ref mut old_cpa) => {
                if old_cpa.addr_eq(&cpa) {
                    *self = Multi(Vec::new());
                    return true;
                }
            }
            Multi(ref mut cpas) => {
                let old_len = cpas.len();
                cpas.retain(|old_cpa| old_cpa.addr_eq(&cpa));
                return old_len != cpas.len();
            }
        }
        false
    }

    pub fn del_addrs(&mut self, cpas: &ConfigPathAddrs) -> bool {
        use ConfigPathAddrs::*;
        match cpas {
            Single(ref cpa) => self.del_addr(cpa),
            Multi(ref cpas) => {
                let mut removed = false;
                for cpa in cpas {
                    removed |= self.del_addr(cpa);
                }
                removed
            }
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(untagged)]
pub enum ConfigPathAddr {
    Simple(String),
    Combo(ConfigPathAddrCombo),
}

impl ConfigPathAddr {
    pub fn to_addr(&self) -> ARes<Addr> {
        let (addr, secure) = match self {
            ConfigPathAddr::Simple(addr) => (addr.as_str(), false),
            ConfigPathAddr::Combo(combo) => (combo.addr.as_str(), combo.secure),
        };
        Ok(Addr {
            secure,
            ..Addr::from_str(addr)?
        })
    }

    pub fn addr_eq(&self, other: &Self) -> bool {
        match (self.to_addr(), other.to_addr()) {
            (Ok(a1), Ok(a2)) => a1 == a2,
            _ => false,
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ConfigPathAddrCombo {
    pub addr: String,
    #[serde(default)]
    pub secure: bool,
}

#[cfg(test)]
mod tests {
    use super::*;
    use utils::make_map;

    #[test]
    fn example_from_str() {
        let config: Config = toml::from_str(EXAMPLE_CONFIG).unwrap();
        assert_eq!(
            config,
            Config {
                proxy: ProxyConfig {
                    addr: AddrsEnum::Single(AddrEnum::Simple("127.0.0.1:8000".into())),
                    lb: None,
                },
                paths: utils::make_map![
                    "/config.Config/Config".into() => AddrsEnum::Single(
                        AddrEnum::Simple("127.0.0.1:8080".into()),
                    ),
                    "/config.Config/AgainConfig".into() => AddrsEnum::Multi(
                        vec![
                            AddrEnum::Simple("https://127.0.0.1:8008".into()),
                            AddrEnum::Combo(ConfigPathAddrCombo {
                                addr: "127.0.0.1:9000".into(),
                                secure: false,
                            }),
                        ],
                    ),
                ],
                addrs: vec![
                    ConfigAddr {
                        addr: "https://127.0.0.1:8000".into(),
                        secure: false,
                        paths: vec!["/config/HttpsConfig".into()],
                    },
                    ConfigAddr {
                        addr: "127.0.0.1:8008".into(),
                        secure: true,
                        paths: vec![
                            "/config/HttpsConfig".into(),
                            "/config.Config/AnotherConfig".into(),
                        ],
                    }
                ],
            },
        )
    }

    #[test]
    fn another_from_str() {
        const CONF_STR: &str =
r#"
[proxy]

#addr = "http://127.0.0.1;8000" # Same as "127.0.0.1:8000"
#addr = "https://127.0.0.1:8000", # Listen with TLS
#addr = { addr = "127.0.0.1:8000", secure = true }

addr = [
  "https://127.0.0.1:8008",
  { addr = "127.0.0.1:8000", secure = false },
]

[paths]
"/config.Config/Config" = "127.0.0.1:8080"
"/config.Config/AgainConfig" = [
  "https://127.0.0.1:8008",
  { addr = "127.0.0.1:9000", secure = false }
]

[paths."/config/HttpsConfig"]
addr = "https://127.0.0.1:8000"
"#;
        let config: Config = toml::from_str(CONF_STR).unwrap();
        assert_eq!(
            config,
            Config {
                proxy: ProxyConfig {
                    addr: AddrsEnum::Multi(
                        vec![
                            AddrEnum::Simple("https://127.0.0.1:8008".into()),
                            AddrEnum::Combo(ConfigPathAddrCombo {
                                addr: "127.0.0.1:8000".into(),
                                secure: false,
                            }),
                        ],
                    ),
                    lb: None,
                },
                paths: utils::make_map![
                    "/config.Config/Config".into() => AddrsEnum::Single(
                        AddrEnum::Simple("127.0.0.1:8080".into()),
                    ),
                    "/config.Config/AgainConfig".into() => AddrsEnum::Multi(
                        vec![
                            AddrEnum::Simple("https://127.0.0.1:8008".into()),
                            AddrEnum::Combo(ConfigPathAddrCombo {
                                addr: "127.0.0.1:9000".into(),
                                secure: false,
                            }),
                        ],
                    ),
                    "/config/HttpsConfig".into() => AddrsEnum::Single(
                        AddrEnum::Combo(ConfigPathAddrCombo {
                            addr: "https://127.0.0.1:8000".into(),
                            secure: false,
                        }),
                    ),
                ],
                addrs: Vec::new(),
            },
        )
    }

    #[test]
    fn only_paths() {
const CONF_STR: &str =
r#"
[paths]
"/config.Config/Config" = "127.0.0.1:8080"
"/config.Config/AgainConfig" = [
  "https://127.0.0.1:8008",
  { addr = "127.0.0.1:9000", secure = false }
]
"#;
        let config: Config = toml::from_str(CONF_STR).unwrap();
        assert_eq!(
            config,
            Config {
                proxy: ProxyConfig::default(),
                paths: utils::make_map![
                    "/config.Config/Config".into() => AddrsEnum::Single(
                        AddrEnum::Simple("127.0.0.1:8080".into()),
                    ),
                    "/config.Config/AgainConfig".into() => AddrsEnum::Multi(
                        vec![
                            AddrEnum::Simple("https://127.0.0.1:8008".into()),
                            AddrEnum::Combo(ConfigPathAddrCombo {
                                addr: "127.0.0.1:9000".into(),
                                secure: false,
                            }),
                        ],
                    ),
                ],
                addrs: Vec::new(),
            },
        )
    }

    #[test]
    fn only_addrs() {
const CONF_STR: &str =
r#"
[[addr]]
addr = "https://127.0.0.1:8000"
paths = [ "/config/HttpsConfig" ]

[[addr]]
addr = "127.0.0.1:8008"
secure = true
paths = [ "/config/HttpsConfig", "/config.Config/AnotherConfig" ]
"#;
        let config: Config = toml::from_str(CONF_STR).unwrap();
        assert_eq!(
            config,
            Config {
                proxy: ProxyConfig::default(),
                paths: HashMap::new(),
                addrs: vec![
                    ConfigAddr {
                        addr: "https://127.0.0.1:8000".into(),
                        secure: false,
                        paths: vec!["/config/HttpsConfig".into()],
                    },
                    ConfigAddr {
                        addr: "127.0.0.1:8008".into(),
                        secure: true,
                        paths: vec![
                            "/config/HttpsConfig".into(),
                            "/config.Config/AnotherConfig".into(),
                        ],
                    }
                ],
            },
        )
    }
}
