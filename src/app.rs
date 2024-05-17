use crate::config::*;
use crate::consts::{headers, targets::*};
use crate::die;

use anyhow::Result as ARes;
use bytes::Bytes;
use crossbeam::sync::WaitGroup;
use futures::future::{BoxFuture, FutureExt};
use http_body_util::{combinators::BoxBody, BodyExt};
use hyper::body::{Body, Frame, Incoming, SizeHint};
use hyper::client::conn::http2::Builder as ClientBuilder;
use hyper::server::conn::http2::Builder as ServerBuilder;
use hyper::service::service_fn;
use hyper::{Method, Request, Response, StatusCode};
use hyper_util::rt::{TokioExecutor, TokioIo};
use log::Level;
use std::collections::HashMap;
use std::error::Error as _;
use std::io::{Error as IoError, ErrorKind};
//use std::ops::{Deref, DerefMut};
use std::pin::Pin;
use std::process::exit;
use std::str::from_utf8 as str_from_utf8;
use std::sync::{
    atomic::{AtomicPtr, AtomicU32, AtomicU64, Ordering},
    Arc,
};
use std::task::{Context, Poll};
use std::time::SystemTime;
use tokio::net::{TcpListener, TcpSocket, TcpStream};
use tokio::signal;
use tokio::sync::{
    oneshot::{channel as ochannel, Receiver as OReceiver, Sender as OSender},
    watch::{channel as wchannel, Receiver as WReceiver, Sender as WSender},
    RwLock,
};
use tokio::time::{sleep, Duration};
use utils::atomic_value::NEAtomicArcValue as NEAAV;

/*
struct AddrInfo {
    addr: Addr,
    conns: AtomicU32,
}

#[derive(Default)]
struct Paths {
    paths: HashMap<Arc<str>, Vec<Arc<PathInfo>>>,
}

impl Deref for Paths {
    type Target = HashMap<Arc<str>, Vec<Arc<PathInfo>>>;

    fn deref(&self) -> &Self::Target {
        &self.paths
    }
}

impl DerefMut for Paths {
    fn deref_mut(&mut self) -> &mut Self::Target {
        &mut self.paths
    }
}

struct PathInfo {
    addr_info: Vec<Arc<AddrInfo>>,
    next_rr: AtomicUsize,
}

fn proxy_paths_to_paths(paths: Arc<Paths>, proxy_paths: ProxyPaths) -> Arc<Paths> {
    let ProxyPaths { paths: proxy_paths } = proxy_paths;
    let mut new_paths = Paths::default();
    let mut proxy_paths = proxy_paths.into_iter().collect::<Vec<_>>();
    for i in 0..proxy_paths.len() {
        let t = &proxy_paths[i];
        if paths.paths.contains(&t.0) {
            new_paths.insert(Arc::clone(&t.0), Arc::clone(&t.1));
        }
    }
}
*/

pub struct App {
    config_path: String,
    config: RwLock<Config>,
    paths: NEAAV<ProxyPaths>,
    //paths: NEAAV<Paths>,
    #[allow(dead_code)]
    lb: AtomicU32,

    no_username: bool,
    password: Arc<str>,

    extra_ln_info: ExtraLnInfo,
    extra_ln_sender: Option<WSender<SystemTime>>,

    server_builder: ServerBuilder<TokioExecutor>,
    client_builder: ClientBuilder<TokioExecutor>,

    // The u64 is an ID used so that a listener can know if it needs to remove itself from the map.
    // An example is as follows. A listener fails with some kind of error, thus it's hasn't yet
    // been removed from map. Simultaneously, a new listener is created on the same address and
    // placed into the map before the old is removed (but after it has decided to remove itself).
    // Without an ID, the listener would then remove the new one, shutting it down.
    listeners: RwLock<HashMap<Addr, (u64, OSender<()>)>>,
    listener_id: AtomicU64,
    wg: AtomicPtr<WaitGroup>,

    log_handle: Option<log4rs::Handle>,
}

type ExtraLnInfo = Option<(Addr, Option<Duration>)>;

impl App {
    pub fn new(
        config_path: String,
        config: Config,
        extra_ln_info: ExtraLnInfo,
        no_username: bool,
        password: impl Into<Arc<str>>,
        log_handle: Option<log4rs::Handle>,
    ) -> ARes<Self> {
        let paths = config.make_proxy_paths()?;
        let lb = AtomicU32::new(config.proxy.lb.unwrap_or_default() as _);
        Ok(Self {
            config_path,
            config: RwLock::new(config),
            paths: NEAAV::new(paths),
            lb,

            no_username,
            password: password.into(),

            extra_ln_info,
            extra_ln_sender: None,

            server_builder: ServerBuilder::new(TokioExecutor::new()),
            client_builder: ClientBuilder::new(TokioExecutor::new()),

            listeners: RwLock::new(HashMap::new()),
            listener_id: AtomicU64::new(0),
            wg: AtomicPtr::new(Box::into_raw(Box::new(WaitGroup::new()))),

            log_handle,
        })
    }

    pub async fn run(mut self) {
        let elr = if self.extra_ln_info.is_some() {
            let (sender, rcvr) = wchannel(SystemTime::now());
            self.extra_ln_sender = Some(sender);
            Some(rcvr)
        } else {
            None
        };
        Box::leak::<'static>(Box::new(self)).run_leaked(elr).await
    }

    async fn run_leaked(&'static self, elr: Option<WReceiver<SystemTime>>) {
        log::info!(target: LM_RUN, Level::Info, "STARTING");
        tokio::spawn(async move {
            handle_signals(self).await;
        });
        let temp_conf = Config::empty();
        let config = std::mem::replace(&mut *self.config.write().await, temp_conf);
        if let Err(e) = self.apply_config(config).await {
            log::warn!(target: LM_RUN, "error setting config: {e}");
        }
        let Some(wg) = self.new_wg() else {
            log::error!(target: LM_RUN, "error getting waitgroup in run function");
            return;
        };
        if self.extra_ln_info.is_some() {
            tokio::spawn(async move {
                self.run_extra_ln(elr.unwrap()).await;
            });
        }
        wg.wait();
        die!(0; target: LM_RUN, Level::Info, "EXITING");
    }

    async fn run_extra_ln(&'static self, mut rcvr: WReceiver<SystemTime>) {
        let Some((addr, retry)) = self.extra_ln_info else {
            return;
        };
        loop {
            let listener = match new_listener(addr).await {
                Ok(ln) => ln,
                Err(e) => {
                    log::error!(target: LM_LISTENER, "(0|{addr}): {e}");
                    if let Some(retry) = retry {
                        sleep(retry).await;
                    } else {
                        let start = SystemTime::now();
                        loop {
                            if rcvr.changed().await.is_err() {
                                // TODO: Error msg
                                log::error!(target: LM_LISTENER, "extra listener sender dropped");
                                return;
                            }
                            let t = *rcvr.borrow_and_update();
                            if t >= start {
                                break;
                            }
                        }
                    }
                    continue;
                }
            };
            log::info!(target: LM_LISTENER, "(0|{addr}): started");
            let res: ARes<()> = loop {
                let (stream, raddr) = match listener.accept().await {
                    Ok((stream, raddr)) => (TokioIo::new(stream), raddr),
                    Err(e) => break Err(e.into()),
                };
                let svc = service_fn(move |req| self.proxy(req));
                tokio::spawn(async move {
                    // TODO: logs
                    log::trace!(target: LM_SERVE_PROXY, "({raddr})");
                    let Err(e) = self.server_builder.serve_connection(stream, svc).await else {
                        log::trace!(target: LM_SERVE_PROXY, "({raddr}");
                        return;
                    };
                    if let Some(ioe) = e.source().and_then(|s| s.downcast_ref::<IoError>()) {
                        if ioe.kind() == ErrorKind::NotConnected {
                            log::trace!(target: LM_SERVE_PROXY, "({raddr}");
                            return;
                        }
                    }
                    log::info!(target: LM_SERVE_PROXY, "({raddr}): {e}");
                });
            };
            if let Err(e) = res {
                log::error!(target: LM_LISTENER, "(0|{addr}): {e}");
            } else {
                log::error!(target: LM_LISTENER, "(0|{addr}): stopped without error");
            }
        }
    }

    // TODO: Backlog?
    async fn run_listener(&'static self, addr: Addr) {
        let Some(_wg) = self.new_wg() else {
            return;
        };

        let listener = match new_listener(addr).await {
            Ok(ln) => ln,
            Err(e) => {
                log::warn!(target: LM_LISTENER, "({addr}): {e}");
                return;
            }
        };
        let (send, done) = ochannel();
        let id = {
            let mut listeners = self.listeners.write().await;
            let mut id = 0;
            let ids = listeners.values().map(|(id, _)| *id).collect::<Vec<_>>();
            while id == 0 || ids.contains(&id) {
                id = self.listener_id.fetch_add(1, Ordering::Relaxed);
            }
            listeners.insert(addr, (id, send));
            id
        };

        log::info!(target: LM_LISTENER, "({id}|{addr}): started");
        let res = self.serve(listener, addr, id, done).await;
        log::info!(target: LM_LISTENER, "({id}|{addr}): completed");
        // If no error, then the listener was shutdown externally
        let Err(e) = res else {
            return;
        };
        let mut listeners = self.listeners.write().await;
        if let Some((lid, os)) = listeners.remove(&addr) {
            if lid != id {
                // Insert the oneshot sender back if not the correct one
                listeners.insert(addr, (lid, os));
            }
        }
        drop(listeners);
        log::warn!(target: LM_LISTENER, "({id}|{addr}): {e}");
    }

    async fn serve(
        &'static self,
        listener: TcpListener,
        _addr: Addr,
        _id: u64,
        mut done: OReceiver<()>,
    ) -> ARes<()> {
        loop {
            tokio::select! {
                res = listener.accept() => {
                    let (stream, raddr) = match res {
                        Ok((stream, raddr)) => (TokioIo::new(stream), raddr),
                        Err(e) => break Err(e.into()),
                    };
                    let Some(wg) = self.new_wg() else {
                        break Ok(())
                    };
                    let svc = service_fn(move |req| self.proxy(req));
                    tokio::spawn(async move {
                        log::trace!(target: LM_SERVE, "({raddr})");
                        let _wg = wg;
                        let Err(e) = self.server_builder.serve_connection(stream, svc).await else {
                            log::trace!(target: LM_SERVE, "({raddr})");
                            return;
                        };
                        if let Some(ioe) = e.source().and_then(|s| s.downcast_ref::<IoError>()) {
                            if ioe.kind() == ErrorKind::NotConnected {
                                log::trace!(target: LM_SERVE, "({raddr})");
                                return;
                            }
                        }
                        log::info!(target: LM_SERVE, "({raddr}): {e}");
                    });
                }
                _ = &mut done => break Ok(()),
            }
        }
    }

    async fn proxy(&'static self, req: Request<Incoming>) -> RespRes {
        let path = req.uri().path();
        if path == "/" || path == "" {
            return self.handle_req(req).await;
        }
        log::trace!(target: LM_PROXY, "");
        let (addr, stream) = 'stream: {
            let paths = self.paths();
            let Some(addrs) = paths.get(path) else {
                return empty_resp(StatusCode::NOT_FOUND);
            };
            for addr in addrs {
                if addr.secure {
                    // TODO: Secure/TLS
                    continue;
                }
                match TcpStream::connect(addr.addr).await {
                    Ok(stream) => break 'stream (*addr, TokioIo::new(stream)),
                    Err(e) => log::warn!(target: LM_PROXY_CONNECT, "({addr}/{path}): {e}"),
                }
            }
            return empty_resp(StatusCode::SERVICE_UNAVAILABLE); // NOTE: 500, 502, or 503?
        };

        // TODO: Do something with error?
        let (mut sender, conn) = self.client_builder.handshake(stream).await?;
        let path = path.to_string();
        tokio::task::spawn(async move {
            if let Err(e) = conn.await {
                // TODO: Do something with error
                log::info!(target: LM_PROXY, "({addr}/{path}): {e}");
            }
        });

        // TODO: Do something with error?
        let resp = sender.send_request(req).await?;
        Ok(resp.map(|b| b.boxed()))
    }

    async fn handle_req(&'static self, req: Request<Incoming>) -> RespRes {
        //use hyper::header::HeaderValue;
        //const EMPTY_HEADER_VAL: &HeaderValue = &HeaderValue::from_static("");

        let Some(val) = req.headers().get(headers::BASE) else {
            return make_resp(StatusCode::BAD_REQUEST, String::from("missing header"));
        };
        if self.password.len() != 0 {
            let ok = if let Some(pwd) = req.headers().get(headers::PASSWORD) {
                pwd == &*self.password
            } else {
                false
            };
            if !ok {
                return make_resp(StatusCode::UNAUTHORIZED, String::from("invalid password"));
            }
        }
        if !self.no_username {
            let username = req.headers()
                .get(headers::USERNAME)
                .map(|hv| hv.to_str().unwrap_or(""))
                .unwrap_or("");
            log::debug!(target: LM_HANDLE_REQ, "USERNAME: {username}");
        }
        match req.method() {
            &Method::GET => {
                if val != "get" {
                    make_resp(
                        StatusCode::BAD_REQUEST,
                        format!(
                            r#"expected value "get", got {val:?} for {} header"#,
                            headers::BASE,
                        ),
                    )
                } else {
                    self.handle_get(req).await
                }
            }
            &Method::POST => {
                if val != "refresh" {
                    make_resp(
                        StatusCode::BAD_REQUEST,
                        format!(
                            r#"expected value "get", got {val:?} for {} header"#,
                            headers::BASE,
                        ),
                    )
                } else {
                    self.handle_refresh(req).await
                }
            }
            // TODO: This won't get called a second time if a force is needed since the first
            // call SHOULD have initiated the shutdown, thus not allowing new connections.
            &Method::DELETE => {
                if val != "shutdown" {
                    make_resp(
                        StatusCode::BAD_REQUEST,
                        format!(
                            r#"expected value "get", got {val:?} for {} header"#,
                            headers::BASE,
                        ),
                    )
                } else {
                    self.handle_shutdown(req).await
                }
            }
            _ => empty_resp(StatusCode::METHOD_NOT_ALLOWED),
        }
    }

    async fn handle_get(&'static self, req: Request<Incoming>) -> RespRes {
        let (mut get_config, mut get_status) = (false, false);
        let Some(query) = req.uri().query() else {
            return empty_resp(StatusCode::OK);
        };
        for (key, val) in query
            .split('&')
            .map(|kv| kv.split_once('=').unwrap_or((kv, "")))
        {
            if key == "config" {
                match val {
                    "1" | "true" => get_config = true,
                    "0" | "false" => get_config = false,
                    _ => (),
                }
            } else if key == "status" {
                match val {
                    "1" | "true" => get_status = true,
                    "0" | "false" => get_status = false,
                    _ => (),
                }
            }
        }
        let config_str = if get_config {
            match toml::to_string(&*self.config.read().await) {
                Ok(s) => s,
                Err(e) => {
                    log::info!(target: LM_HANDLE_GET, "error serializing config: {e}");
                    return make_resp(
                        StatusCode::INTERNAL_SERVER_ERROR,
                        format!("error serializing config: {e}"),
                    );
                }
            }
        } else {
            String::new()
        };
        let status_str = if get_status {
            let statuses = self
                .listeners
                .read()
                .await
                .keys()
                .map(|addr| (addr.to_string(), "RUNNING"))
                .collect::<HashMap<_, _>>();
            match serde_json::to_string(&statuses) {
                Ok(s) => s,
                Err(e) => {
                    log::info!(target: LM_HANDLE_GET, "error serializing statuses: {e}");
                    return make_resp(
                        StatusCode::INTERNAL_SERVER_ERROR,
                        format!("error serializing statuses: {e}"),
                    );
                }
            }
        } else {
            String::new()
        };
        Ok(Response::builder()
            .status(StatusCode::OK)
            .header(headers::BASE, "ok")
            .header(headers::CONFIG_LEN, config_str.len().to_string())
            .header(headers::STATUS_LEN, status_str.len().to_string())
            .body(make_body(Bytes::from(config_str + &status_str)))
            .unwrap())
    }

    async fn handle_refresh(&'static self, req: Request<Incoming>) -> RespRes {
        let action = match req.headers().get(headers::REFRESH_ACTION) {
            Some(v) if v == "" || v == "add" || v == "del" => v.to_str().unwrap().to_string(),
            Some(val) => {
                return make_resp(
                    StatusCode::BAD_REQUEST,
                    format!(r#"invalid value {val:?} for "{}""#, headers::REFRESH_ACTION),
                )
            }
            None => {
                return make_resp(
                    StatusCode::BAD_REQUEST,
                    format!(r#"missing "{}" header"#, headers::REFRESH_ACTION),
                )
            }
        };
        let body_bytes = match req.into_body().collect().await {
            Ok(body) => body.to_bytes(),
            Err(e) => {
                log::debug!(target: LM_HANDLE_REFRESH, "{e}");
                return empty_resp(StatusCode::INTERNAL_SERVER_ERROR);
            }
        };
        let Ok(body_str) = str_from_utf8(&body_bytes) else {
            return make_resp(StatusCode::BAD_REQUEST, String::from("invalid body"));
        };
        let config = if body_str == "" {
            match Config::from_file(&self.config_path) {
                Ok(c) => c,
                Err(e) => {
                    return make_resp(StatusCode::INTERNAL_SERVER_ERROR, e.to_string());
                }
            }
        } else {
            match toml::from_str(body_str).map_err(|e| anyhow::anyhow!("{e}")) {
                Ok(c) => c,
                Err(e) => {
                    return make_resp(StatusCode::BAD_REQUEST, e.to_string());
                }
            }
        };
        if action == "add" {
            if let Err(e) = self.add_to_config(config).await {
                return make_resp(StatusCode::BAD_REQUEST, e.to_string());
            }
        } else if action == "del" {
            if let Err(e) = self.del_from_config(config).await {
                return make_resp(StatusCode::BAD_REQUEST, e.to_string());
            }
        } else {
            if let Err(e) = self.apply_config(config).await {
                return make_resp(StatusCode::BAD_REQUEST, e.to_string());
            }
        }
        let config_str = match toml::to_string(&*self.config.read().await) {
            Ok(s) => s,
            Err(e) => {
                log::info!(target: LM_HANDLE_REFRESH, "error serializing config: {e}");
                return make_resp(
                    StatusCode::INTERNAL_SERVER_ERROR,
                    format!("error serializing config: {e}"),
                );
            }
        };
        Ok(Response::builder()
            .status(StatusCode::OK)
            .header(headers::BASE, "ok")
            .header(headers::REFRESH_ACTION, action)
            .header(headers::CONFIG_LEN, config_str.len().to_string())
            .body(make_body(Bytes::from(config_str)))
            .unwrap())
    }

    async fn handle_shutdown(&'static self, req: Request<Incoming>) -> RespRes {
        let force = req
            .uri()
            .query()
            .map(|qs| {
                qs.split('&')
                    .any(|kv| kv == "force=1" || kv == "force=true")
            })
            .unwrap_or(false);
        if force {
            // TODO: Do better?
            die!(0; target: LM_HANDLE_SHUTDOWN, Level::Info, "EXITING");
        }
        self.shutdown().await;
        empty_resp(StatusCode::OK)
    }

    fn apply_config(&'static self, config: Config) -> BoxFuture<'static, ARes<()>> {
        async move {
            self.check_shutting_down()?;
            // Get the proxy addrs and paths
            let mut proxy_addrs = config.proxy.make_addrs()?;
            let paths = config.make_proxy_paths()?;
            let mut conf = self.config.write().await;
            // Only keep the address that aren't already running
            {
                let mut listeners = self.listeners.write().await;
                listeners.retain(|addr, _| proxy_addrs.contains(addr));
                proxy_addrs.retain(|addr| !listeners.contains_key(addr));
            }
            // Start the new listeners
            for addr in proxy_addrs {
                tokio::spawn(async move {
                    self.run_listener(addr).await;
                });
            }
            // Set the config and paths
            *conf = config;
            self.set_paths(paths);
            // Make sure it's not shutting down
            if self.shutting_down() {
                self.listeners.write().await.clear();
                return Err(anyhow::anyhow!("shutting down"));
            }
            Ok(())
        }
        .boxed()
    }

    fn add_to_config(&'static self, config: Config) -> BoxFuture<'static, ARes<()>> {
        let Config {
            proxy,
            paths,
            addrs,
        } = config;
        async move {
            self.check_shutting_down()?;
            // Get the proxy addrs
            let mut proxy_addrs = proxy.make_addrs()?;
            let mut conf = self.config.write().await;
            // Add the new stuff to the config
            let mut new_config = conf.clone();
            for (path, addr) in paths {
                let _ = new_config.add_path(path, addr);
            }
            for addr in addrs {
                let _ = new_config.add_addr(addr);
            }
            // Get the paths
            let paths = new_config.make_proxy_paths()?;
            // Only keep the address that aren't already running
            {
                let listeners = self.listeners.read().await;
                proxy_addrs.retain(|addr| !listeners.contains_key(addr));
            }
            // Start the new listeners
            for addr in proxy_addrs {
                tokio::spawn(async move {
                    self.run_listener(addr).await;
                });
            }
            // Set the config and paths
            *conf = new_config;
            self.set_paths(paths);
            // Make sure it's not shutting down
            if self.shutting_down() {
                self.listeners.write().await.clear();
                return Err(anyhow::anyhow!("shutting down"));
            }
            Ok(())
        }
        .boxed()
    }

    fn del_from_config(&'static self, config: Config) -> BoxFuture<'static, ARes<()>> {
        let Config {
            proxy,
            paths,
            addrs,
        } = config;
        async move {
            self.check_shutting_down()?;
            // Get the proxy addrs
            let proxy_addrs = proxy.make_addrs()?;
            let mut conf = self.config.write().await;
            // Delete the new stuff from the config
            let mut new_config = conf.clone();
            for (path, addr) in paths {
                let _ = new_config.del_path(&path, Some(&addr));
            }
            for addr in addrs {
                let _ = new_config.del_addr(&addr);
            }
            // Get the paths
            let paths = new_config.make_proxy_paths()?;
            // Stop the appropriate listeners
            {
                let mut listeners = self.listeners.write().await;
                listeners.retain(|addr, _| proxy_addrs.contains(addr));
            }
            // Set the config and paths
            *conf = new_config;
            self.set_paths(paths);
            // Make sure it's not shutting down
            if self.shutting_down() {
                self.listeners.write().await.clear();
                return Err(anyhow::anyhow!("shutting down"));
            }
            Ok(())
        }
        .boxed()
    }

    fn check_shutting_down(&'static self) -> ARes<()> {
        if self.shutting_down() {
            Err(anyhow::anyhow!("shutting down"))
        } else {
            Ok(())
        }
    }

    fn paths(&'static self) -> Arc<ProxyPaths> {
        //unsafe { Arc::clone(&*self.paths.load(Ordering::Acquire)) }
        self.paths.load(Ordering::Acquire)
    }

    fn set_paths(&'static self, paths: ProxyPaths) {
        /*
        let ptr = Box::into_raw(Box::new(Arc::new(paths)));
        let _ = unsafe { Box::from_raw(self.paths.swap(ptr, Ordering::AcqRel)) };
        */
        self.paths.store(paths, Ordering::SeqCst);
    }

    fn new_wg(&'static self) -> Option<WaitGroup> {
        let ptr = self.wg.load(Ordering::Acquire);
        if ptr.is_null() {
            None
        } else {
            unsafe { Some((*ptr).clone()) }
        }
    }

    fn shutting_down(&'static self) -> bool {
        self.wg.load(Ordering::Acquire).is_null()
    }

    async fn shutdown(&'static self) {
        tokio::spawn(async move {
            let ptr = self.wg.swap(std::ptr::null_mut(), Ordering::AcqRel);
            if ptr.is_null() {
                return;
            }
            let _wg = unsafe { Box::from_raw(ptr) };
            // TODO: Target?
            log::info!(target: LM_RUN, "SHUTTING DOWN");
            self.listeners.write().await.clear();
        });
    }
}

async fn new_listener(addr: Addr) -> ARes<TcpListener> {
    let sock = if addr.addr.is_ipv4() {
        TcpSocket::new_v4()?
    } else {
        TcpSocket::new_v6()?
    };
    sock.set_reuseaddr(true)?;
    sock.bind(addr.addr)?;
    sock.listen(128).map_err(|e| e.into())
}

// RESP/BODY

pub type Resp = Response<BoxBody<Bytes, hyper::Error>>;
type RespRes = Result<Resp, hyper::Error>;

pub fn empty_body() -> BoxBody<Bytes, hyper::Error> {
    BoxBody::new(EmptyBody)
}

fn make_body(body: impl Into<BytesBody>) -> BoxBody<Bytes, hyper::Error> {
    BoxBody::new(body.into())
}

fn empty_resp(status: StatusCode) -> RespRes {
    let hv = if status == StatusCode::OK {
        "ok"
    } else {
        "err"
    };
    Ok(Response::builder()
        .status(status)
        .header(headers::BASE, hv)
        .body(empty_body())
        .unwrap())
}

fn make_resp(status: StatusCode, body: impl Into<Bytes>) -> RespRes {
    let hv = if status == StatusCode::OK {
        "ok"
    } else {
        "err"
    };
    Ok(Response::builder()
        .status(status)
        .header(headers::BASE, hv)
        .body(make_body(body.into()))
        .unwrap())
}

struct BytesBody(Bytes);

impl From<Bytes> for BytesBody {
    fn from(b: Bytes) -> Self {
        Self(b)
    }
}

impl Body for BytesBody {
    type Data = Bytes;
    type Error = hyper::Error;

    fn poll_frame(
        mut self: Pin<&mut Self>,
        _cx: &mut Context<'_>,
    ) -> Poll<Option<Result<Frame<Self::Data>, Self::Error>>> {
        if !self.0.is_empty() {
            let b = std::mem::take(&mut self.0);
            Poll::Ready(Some(Ok(Frame::data(b))))
        } else {
            Poll::Ready(None)
        }
    }

    fn is_end_stream(&self) -> bool {
        self.0.is_empty()
    }

    fn size_hint(&self) -> SizeHint {
        SizeHint::with_exact(self.0.len() as u64)
    }
}

struct EmptyBody;

impl Body for EmptyBody {
    type Data = Bytes;
    type Error = hyper::Error;

    fn poll_frame(
        self: Pin<&mut Self>,
        _cx: &mut Context<'_>,
    ) -> Poll<Option<Result<Frame<Self::Data>, Self::Error>>> {
        Poll::Ready(None)
    }

    fn is_end_stream(&self) -> bool {
        true
    }

    fn size_hint(&self) -> SizeHint {
        SizeHint::with_exact(0)
    }
}

/***  Signal Handling ***/

#[cfg(unix)]
async fn handle_signals(app: &'static App) {
    use signal::unix as signal;
    let mut ctrlc_sig = match signal::signal(signal::SignalKind::interrupt()) {
        Ok(sig) => sig,
        Err(e) => die!(1; target: LM_SIGNALS, Level::Error, "error creating ctrl-c listener: {e}"),
    };
    let mut user1_sig = match signal::signal(signal::SignalKind::user_defined1()) {
        Ok(sig) => sig,
        Err(e) => die!(1; target: LM_SIGNALS, Level::Error, "error creating USR1 listener: {e}"),
    };
    let mut user2_sig = match signal::signal(signal::SignalKind::user_defined2()) {
        Ok(sig) => sig,
        Err(e) => die!(1; target: LM_SIGNALS, Level::Error, "error creating USR2 listener: {e}"),
    };
    let mut exiting = false;
    loop {
        tokio::select! {
            ret = ctrlc_sig.recv() => {
                if ret.is_none() {
                    log::error!(
                        target: LM_SIGNALS,
                        "error listening for ctrl-c: no more events can be received",
                    );
                    break;
                } else if exiting {
                    break;
                }
                exiting = true;
                app.shutdown().await;
            }
            ret = user1_sig.recv() => {
                if ret.is_none() {
                    log::error!(
                        target: LM_SIGNALS,
                        "error listening for USR1: no more events can be received",
                    );
                }
                if let Some(sender) = app.extra_ln_sender.as_ref() {
                    if sender.send(SystemTime::now()).is_err() {
                        log::error!(target: LM_SIGNALS, "extra listener watcher is closed");
                    }
                }
            }
            ret = user2_sig.recv() => {
                if ret.is_none() {
                    log::error!(
                        target: LM_SIGNALS,
                        "error listening for USR2: no more events can be received",
                    );
                }
                // TODO
            }
        }
    }
    exit(0);
}

#[cfg(not(unix))]
async fn handle_signals(app: &'static App) {
    let mut exiting = false;
    loop {
        if let Err(e) = signal::ctrl_c().await {
            die!(1; target: LM_SIGNALS, Level::Error, "error listening for ctrl-c: {e}");
        }
        if exiting {
            break;
        }
        exiting = false;
        app.shutdown().await;
    }
    exit(0);
}
