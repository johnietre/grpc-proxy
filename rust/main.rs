use bytes::Bytes;
use clap::Parser;
use http_body_util::BodyExt;
use hyper::client::conn::http2::Builder as ClientBuilder;
use hyper::{Request, StatusCode};
use hyper_util::rt::{TokioExecutor, TokioIo};
use std::collections::HashMap;
use std::str::from_utf8 as str_from_utf8;
use std::{env, fs};
use tokio::net::TcpStream;
use tokio::runtime::Builder as RtBuilder;
use tokio::time::Duration;

mod app;
mod args;
mod config;
mod consts;

use app::{empty_body, App, Resp};
use args::*;
use config::*;
use consts::{env_names::*, headers};

#[cfg(not(testing))]
#[macro_export]
macro_rules! die {
    ($code:expr, $($arg:tt)*) => {{
        eprintln!($($arg)*);
        std::process::exit($code)
    }};
    ($code:expr; target: $target:expr, $lvl:expr, $($arg:tt)*) => {{
        log::log!(target: $target, $lvl, $($arg)*);
        std::process::exit($code)
    }};
    ($code:expr; $lvl:expr, $($arg:tt)*) => {{
        log::log!($lvl, $($arg)*);
        std::process::exit($code)
    }};
    ($code:expr) => {
        std::process::exit($code)
    };
}

#[cfg(testing)]
#[macro_export]
macro_rules! die {
    ($code:expr, $($arg:tt)*) => {{
        panic!($($arg)*)
    }};
    ($code:expr; $lvl:expr, $($arg:tt)*) => {{
        panic!($($arg)*)
    }};
    ($code:expr) => {
        panic!("exit code: {}", $code)
    };
}

fn new_rt_builder(multi: bool) -> RtBuilder {
    let mut builder = if multi {
        RtBuilder::new_multi_thread()
    } else {
        RtBuilder::new_current_thread()
    };
    builder.enable_all();
    builder
}

fn main() {
    run(CliArgs::parse());
}

fn run(mut args: CliArgs) {
    let Some(cmd) = args.command.take() else {
        println!(
            "A mediocre gRPC proxy (at best ;]).
Use the -h, --help flags to print help.
Have a nice day! :]"
        );
        return;
    };
    if let Commands::Start(sc) = cmd {
        run_start_command(args, sc);
        return;
    }
    let rt = match new_rt_builder(false).build() {
        Ok(rt) => rt,
        Err(e) => die!(1, "error building runtime: {e}"),
    };
    match cmd {
        Commands::Config(cc) => rt.block_on(run_config_command(args, cc)),
        Commands::Start(_) => unreachable!(),
        Commands::Get(gc) => rt.block_on(run_get_command(args, gc)),
        Commands::Refresh(rc) => rt.block_on(run_refresh_command(args, rc)),
        Commands::Shutdown(sc) => rt.block_on(run_shutdown_command(args, sc)),
    }
}

async fn run_config_command(_: CliArgs, cc: ConfigCommand) {
    let mut path = cc.out.map(|p| p.clone()).unwrap_or("./".into());
    if path.is_dir() {
        path = path.join("grpc-proxy.toml");
    }
    if let Err(e) = fs::write(path, EXAMPLE_CONFIG) {
        die!(1, "error writing config file: {e}");
    }
}

fn run_start_command(_: CliArgs, sc: StartCommand) {
    let ln_addr = match sc.addr.as_ref().map(|a| a.parse::<Addr>()).transpose() {
        Ok(addr) => addr,
        Err(_) => die!(1, "invalid address: {}", sc.addr.as_ref().unwrap()),
    };
    if sc.retry.is_some() && ln_addr.is_none() {
        die!(1, "cannot have --retry without --addr");
    }
    let retry = sc.retry.map(Duration::from_secs);
    let ln_info = ln_addr.map(|a| (a, retry));

    let config_path = sc.config_path.clone();
    let config = match Config::from_file(&config_path) {
        Ok(c) => c,
        Err(e) => die!(1, "error parsing config: {e}"),
    };

    // Set up logging
    let handle = setup_logging(&sc);

    let pwd = if sc.no_password {
        env::var(PWD_ENV_NAME).unwrap_or_default()
    } else {
        String::new()
    };
    let app = match App::new(config_path, config, ln_info, sc.no_username, pwd, handle) {
        Ok(app) => app,
        Err(e) => die!(1, "{e}"),
    };

    let rt = match new_rt_builder(true)
        .worker_threads(sc.workers.get())
        .build()
    {
        Ok(rt) => rt,
        Err(e) => die!(1, "error building runtime: {e}"),
    };
    rt.block_on(async move {
        //console_subscriber::init();
        app.run().await
    });
}

fn setup_logging(sc: &StartCommand) -> Option<log4rs::Handle> {
    use log4rs::append::{
        console::{ConsoleAppender, Target},
        file::FileAppender,
        Append,
    };
    use log4rs::config::{load_config_file, Appender, Config as LogConfig, Root};
    use log4rs::filter::Filter;

    let mut filters: Vec<Box<dyn Filter>> = Vec::new();
    let target_val = env::var(LOG_TARGET_ENV_NAME).unwrap_or_default();
    if target_val != "" {
        let targets_filter = TargetsFilter(target_val.split(',').map(String::from).collect());
        filters.push(Box::new(targets_filter));
    }
    if sc.log_proxy_only {
        filters.push(Box::new(GrpcProxyFilter));
    }

    let config = if let Some(lc_path) = sc.log_config.as_ref() {
        match load_config_file(lc_path, Default::default()) {
            Ok(lc) => lc,
            Err(e) => die!(1, "error setting up logging: {e}"),
        }
    } else {
        let appender: Box<dyn Append> = if sc.log_stdout {
            Box::new(ConsoleAppender::builder().target(Target::Stdout).build())
        } else if sc.log_stderr {
            Box::new(ConsoleAppender::builder().target(Target::Stderr).build())
        } else if let Some(path) = sc.log_path.as_ref() {
            match FileAppender::builder().build(path) {
                Ok(lf) => Box::new(lf),
                Err(e) => die!(1, "error setting up logging: {e}"),
            }
        } else {
            return None;
        };
        let res = LogConfig::builder()
            .appender(
                Appender::builder()
                    .filters(filters)
                    .build("appender", appender),
            )
            .build(Root::builder().appender("appender").build(sc.log_level));
        match res {
            Ok(lc) => lc,
            Err(e) => die!(1, "error setting up logging: {e}"),
        }
    };
    match log4rs::init_config(config) {
        Ok(h) => Some(h),
        Err(e) => die!(1, "error setting up logging: {e}"),
    }
}

// TODO: Password
async fn run_get_command(_: CliArgs, mut gc: GetCommand) {
    let addrs = get_cli_addrs(&gc.addrs);
    let streams = get_clients(&addrs, gc.all).await;
    let query = String::from(if gc.config { "?config=1" } else { "" });
    let query = if gc.status {
        if query.len() == 0 {
            String::from("?status=1")
        } else {
            query + "&status=1"
        }
    } else {
        query
    };
    let config_dir = gc.config_out.take().unwrap_or(".".into());
    let status_dir = gc.status_out.take().unwrap_or(".".into());
    for (addr, stream, builder) in streams {
        let (mut sender, conn) = match builder.handshake(stream).await {
            Ok(ret) => ret,
            Err(e) => {
                eprintln!("error connecting (handshake) {addr}: {e}");
                continue;
            }
        };
        let join = tokio::spawn(async move {
            if let Err(e) = conn.await {
                eprintln!("error occurred with {addr}: {e}");
            }
        });
        let req = Request::builder()
            .uri(addr.to_string() + &query)
            .method("GET")
            .header(headers::BASE, "get")
            .body(empty_body())
            .expect("error creating request");
        match sender
            .send_request(req)
            .await
            .map(|resp| resp.map(|b| b.boxed()))
        {
            Ok(mut resp) => {
                let Some(mut body) = handle_resp(addr, &mut resp).await else {
                    continue;
                };
                let Some(cl) = resp.headers().get(headers::CONFIG_LEN) else {
                    eprintln!("{addr}: missing config length header");
                    continue;
                };
                if cl.len() == 0 {
                    eprintln!("{addr}: missing config length header");
                    continue;
                }
                let Ok(Ok(cl)) = cl.to_str().map(|s| s.parse::<usize>()) else {
                    eprintln!("{addr}: invalid config length: {cl:?}");
                    continue;
                };
                let Some(sl) = resp.headers().get(headers::STATUS_LEN) else {
                    eprintln!("{addr}: missing status length header");
                    continue;
                };
                if sl.len() == 0 {
                    eprintln!("{addr}: missing status length header");
                    continue;
                }
                let Ok(Ok(sl)) = sl.to_str().map(|s| s.parse::<usize>()) else {
                    eprintln!("{addr}: invalid status length: {sl:?}");
                    continue;
                };
                let len = cl + sl;
                if len != body.len() {
                    eprintln!("{addr}: expected body length of {len}, got {}", body.len());
                    continue;
                }
                if body.len() == 0 {
                    continue;
                }
                let config_bytes = body.split_to(cl);
                let Ok(config_str) = str_from_utf8(&config_bytes) else {
                    eprintln!("{addr}: received invalid (non-utf8) config body");
                    continue;
                };
                let Ok(status_str) = str_from_utf8(&body) else {
                    eprintln!("{addr}: received invalid (non-utf8) status body");
                    continue;
                };
                if config_str != "" {
                    let path = config_dir.join(addr.addr.to_string() + ".toml");
                    match toml::from_str::<Config>(config_str) {
                        Ok(config) => match toml::to_string_pretty(&config) {
                            Ok(pretty) => match fs::write(&path, pretty) {
                                Ok(_) => (),
                                Err(e) => {
                                    eprintln!(
                                        "{addr}: error writing config to {}: {e}",
                                        path.display(),
                                    );
                                }
                            },
                            Err(e) => eprintln!("{addr}: error serializing config: {e}"),
                        },
                        Err(e) => eprintln!("{addr}: error parsing config toml: {e}"),
                    }
                }
                if status_str != "" {
                    let path = status_dir.join(addr.addr.to_string() + ".json");
                    match serde_json::from_str::<HashMap<String, String>>(status_str) {
                        Ok(status) => match serde_json::to_string_pretty(&status) {
                            Ok(pretty) => match fs::write(&path, pretty) {
                                Ok(_) => (),
                                Err(e) => {
                                    eprintln!(
                                        "{addr}: error writing status to {}: {e}",
                                        path.display(),
                                    );
                                }
                            },
                            Err(e) => eprintln!("{addr}: error serializing status: {e}"),
                        },
                        Err(e) => eprintln!("{addr}: error parsing status json: {e}"),
                    }
                }
            }
            Err(e) => eprintln!("error sending request to {addr}: {e}"),
        }
        let _ = join.await;
    }
}

async fn run_refresh_command(_: CliArgs, rc: RefreshCommand) {
    let config_str = if let Some(path) = rc.config_path.as_ref() {
        let config_str = match fs::read_to_string(path) {
            Ok(s) => s,
            Err(e) => {
                eprintln!("error reading config file: {e}");
                return;
            }
        };
        if let Err(e) = toml::from_str::<Config>(&config_str) {
            eprintln!("error parsing config file: {e}");
            return;
        }
        config_str
    } else {
        String::new()
    };
    let addrs = get_cli_addrs(&rc.addrs);
    let streams = get_clients(&addrs, rc.all).await;
    for (addr, stream, builder) in streams {
        let (mut sender, conn) = match builder.handshake(stream).await {
            Ok(ret) => ret,
            Err(e) => {
                eprintln!("error connecting (handshake) {addr}: {e}");
                continue;
            }
        };
        let join = tokio::spawn(async move {
            if let Err(e) = conn.await {
                eprintln!("error occurred with {addr}: {e}");
            }
        });
        let req = Request::builder()
            .uri(addr.to_string())
            .method("POST")
            .header(headers::BASE, "refresh")
            .header(
                headers::REFRESH_ACTION,
                if rc.add {
                    "add"
                } else if rc.del {
                    "del"
                } else {
                    ""
                },
            )
            .body(config_str.clone())
            .expect("error creating request");
        match sender
            .send_request(req)
            .await
            .map(|resp| resp.map(|b| b.boxed()))
        {
            Ok(mut resp) => {
                handle_resp(addr, &mut resp).await;
            }
            Err(e) => eprintln!("error sending request to {addr}: {e}"),
        }
        let _ = join.await;
    }
}

async fn run_shutdown_command(_: CliArgs, sc: ShutdownCommand) {
    let addrs = get_cli_addrs(&sc.addrs);
    let streams = get_clients(&addrs, sc.all).await;
    for (addr, stream, builder) in streams {
        let (mut sender, conn) = match builder.handshake(stream).await {
            Ok(ret) => ret,
            Err(e) => {
                eprintln!("error connecting (handshake) {addr}: {e}");
                continue;
            }
        };
        let join = tokio::spawn(async move {
            if let Err(e) = conn.await {
                eprintln!("error occurred with {addr}: {e}");
            }
        });
        let req = Request::builder()
            .uri(addr.to_string() + if sc.force { "?force=1" } else { "" })
            .method("DELETE")
            .header(headers::BASE, "shutdown")
            .body(empty_body())
            .expect("error creating request");
        match sender
            .send_request(req)
            .await
            .map(|resp| resp.map(|b| b.boxed()))
        {
            Ok(mut resp) => {
                handle_resp(addr, &mut resp).await;
            }
            Err(e) => eprintln!("error sending request to {addr}: {e}"),
        }
        let _ = join.await;
    }
}

fn get_cli_addrs(addr_strs: &[String]) -> Vec<Addr> {
    let mut addrs = Vec::with_capacity(addr_strs.len());
    for addr in addr_strs {
        for addr in addr.split(',') {
            let addr = addr.trim();
            let addr: Addr = match addr.parse() {
                Ok(a) => a,
                Err(e) => die!(1, "invalid address: {e}"),
            };
            addrs.push(addr);
        }
    }
    addrs
}

async fn get_clients(
    addrs: &[Addr],
    all: bool,
) -> Vec<(Addr, TokioIo<TcpStream>, ClientBuilder<TokioExecutor>)> {
    let mut res = Vec::new();
    for &addr in addrs {
        let stream = match TcpStream::connect(addr.addr).await {
            Ok(stream) => stream,
            Err(e) => {
                eprintln!("error connecting to {addr}: {e}");
                continue;
            }
        };
        res.push((
            addr,
            TokioIo::new(stream),
            ClientBuilder::new(TokioExecutor::new()),
        ));
        if !all {
            break;
        }
    }
    res
}

async fn handle_resp(addr: Addr, resp: &mut Resp) -> Option<Bytes> {
    // Just to check if the server was actually an instance of a grpc-proxy
    let Some(val) = resp.headers().get(headers::BASE) else {
        eprintln!("invalid response from {addr}: {resp:?}");
        return None;
    };
    if val != "ok" && val != "err" {
        eprintln!("invalid response (header) from {addr}: {resp:?}");
        return None;
    }
    let status = resp.status();
    if status == StatusCode::OK {
        eprintln!("{addr}: SUCCESS");
    }
    match resp.body_mut().collect().await {
        Ok(body) => Some(body.to_bytes()),
        Err(e) => {
            eprintln!("error reading response from {addr}: {e}");
            None
        }
    }
}

/*
#[cfg(test)]
mod main_tests {
    use super::*;
    use config::*;
    use tokio::runtime::{
        self as rt,
        //UnhandledPanic,
    };
    use tokio::time::Duration;
    use utils::make_map;

    const TEST_CONFS_DIR: &str = concat!(env!("CARGO_MANIFEST_DIR"), "/tests/confs");

    /*
    #[test]
    fn basic() {
        let rt = new_rt();

        // Start the server
        let cli_args = CliArgs { command: None };
        let start_cmd = StartCommand {
            addr: Some("127.0.0.1:14200".into()),
            retry: None,
            config_path: format!("{TEST_CONFS_DIR}/basic.toml"),
        };
        let start_task = rt.spawn(async move {
            panic!("start");
            run_start_command(cli_args, start_cmd).await;
        });

        // Shutdown server
        let cli_args = CliArgs { command: None };
        let shutdown_cmd = ShutdownCommand {
            addrs: vec!["127.0.0.1:14200".into()],
            all: false,
            force: false,
        };
        let shutdown_task = rt.spawn(async move {
            panic!("shutwodn");
            run_shutdown_command(cli_args, shutdown_cmd).await;
        });
        rt.block_on(tokio::time::timeout(Duration::from_secs(3), shutdown_task))
            .unwrap()
            .unwrap();

        panic!("waiting");
        rt.block_on(tokio::time::timeout(Duration::from_secs(10), start_task))
            .unwrap()
            .unwrap();
    }
    */

    #[tokio::test(flavor = "multi_thread", worker_threads = 2)]
    async fn basic_force() {
        let task1 = tokio::spawn(async move {
            // Start the server
            let cli_args = CliArgs { command: None };
            let start_cmd = StartCommand {
                addr: Some("127.0.0.1:14200".into()),
                retry: None,
                config_path: format!("{TEST_CONFS_DIR}/basic.toml"),
            };
            run_start_command(cli_args, start_cmd).await;
        });
        tokio::time::sleep(Duration::from_secs(3)).await;
        let task2 = tokio::spawn(async move {
            // Shutdown server
            let cli_args = CliArgs { command: None };
            let shutdown_cmd = ShutdownCommand {
                addrs: vec!["127.0.0.1:14200".into()],
                all: false,
                force: true,
            };
            run_shutdown_command(cli_args, shutdown_cmd).await;
        });
        timeout(Duration::from_secs(3), task2).await.unwrap_or_else(die).unwrap_or_else(die);
        timeout(Duration::from_secs(3), task1).await.unwrap_or_else(die).unwrap_or_else(die);
    }

    #[tokio::test]
    async fn basic() {
        tokio::time::sleep(Duration::from_secs(10)).await;
    }

    async fn timeout<F: std::future::Future>(dur: Duration, f: F) -> Result<F::Output, &'static str> {
        tokio::select! {
            _ = tokio::time::sleep(dur) => Err("timed out"),
            ret = f => Ok(ret),
        }
    }

    fn die<T, E: std::fmt::Debug>(e: E) -> T {
        die!(1, "error: {e:?}")
    }

    fn new_rt() -> rt::Runtime {
        //rt::Builder::new_current_thread()
        rt::Builder::new_multi_thread()
            .worker_threads(10)
            .enable_all()
            //.unhandled_panic(UnhandledPanic::ShutdownRuntime)
            .build()
            .unwrap()
    }
}
*/
