use clap::{Args, Parser, Subcommand};
use std::path::PathBuf;

/// A gRPC proxy.
/// NOTE: Using secured connections of any kind (TLS) is not yet implemented!
#[derive(Parser, Debug)]
#[command(version, about, long_about = None)]
pub struct CliArgs {
    #[command(subcommand)]
    pub command: Option<Commands>,
}

#[derive(Debug, Subcommand)]
pub enum Commands {
    Config(ConfigCommand),
    Start(StartCommand),
    Get(GetCommand),
    Refresh(RefreshCommand),
    Shutdown(ShutdownCommand),
}

/// Used to generate new configuration files.
#[derive(Debug, Args)]
pub struct ConfigCommand {
    /// The name of the output file or directory to put it in (defaults to grpc-proxy.toml).
    #[arg(short, long)]
    pub out: Option<PathBuf>,
}

// TODO: Secure listener
/// Used to start a new proxy server.
#[derive(Debug, Args)]
pub struct StartCommand {
    /// Starts a listener on the given address that will never be closed, even when the proxy is
    /// shutting donw. Connections from this listener will not be waited on when shutting down. If
    /// this listener gets prematurely disconnected, it can be restarted by sending a SIG_USR1
    /// signal, or a restart/retry interval can be set. This listener can be used for all of the
    /// same types of functions as others but is most useful in cases like when the proxy is
    /// shuttin down and other listeners are closed. This cannot be set in the config and won't be
    /// sent with a status check (as of right now).
    #[arg(long)]
    pub addr: Option<String>,

    /// The number of seconds to wait between each attempt to reconnect the listener set by the
    /// --addr flag. If not set or if 0 is passed, no reconnect attempts are made.
    #[arg(long)]
    pub retry: Option<u64>,

    /// Disables the username for the proxy's base path ("/").
    #[arg(long)]
    pub no_username: bool,

    /// Disables the password for the proxy's base path ("/"). The password can be set by using the
    /// GRPC_PROXY_PASSWORD.
    #[arg(long)]
    pub no_password: bool,

    /// The path to the config file to read and refresh from.
    #[arg(long = "config")]
    pub config_path: String,

    /// The number of threads to spawn to use (must be greater than 0).
    #[arg(short, long, default_value = "1")]
    pub workers: std::num::NonZeroUsize,

    /// Optional path to log configuration file. Useful for anything beyond basic logging.
    /// See https://docs.rs/log4rs/latest/log4rs/config/index.html#configuration for usage/examples.
    #[arg(long, group = "log")]
    pub log_config: Option<PathBuf>,

    /// Optional path to the log file.
    #[arg(long, group = "log")]
    pub log_path: Option<PathBuf>,

    /// Log to stdout.
    #[arg(long, group = "log")]
    pub log_stdout: bool,

    /// Log to stderr.
    #[arg(long, group = "log")]
    pub log_stderr: bool,

    /// The minimum level required to log if logging. Does nothing if "log-config" is passed.
    #[arg(long, default_value_t = log::LevelFilter::Info, requires = "log")]
    pub log_level: log::LevelFilter,

    /// Only log messages from the GRPC proxy (this).
    #[arg(long, requires = "log")]
    pub log_proxy_only: bool,
    /*
    /// The minimum level required to log if logging.
    #[arg(long, default_value_t = log::Level::Info, group = "log", requires = "log_path")]
    pub log_level: log::Level,

    #[arg(long, group = "log", requires = "log_path")]
    pub log_trigger: Option<LogTrigger>,

    #[arg(long, group = "log", requires = "log_path")]
    pub log_roller: Option<LogTrigger>,
    */
}

/*
#[derive(Debug, Serialize, Deserialize)]
pub enum LogTrigger {
    Size(SizeTrigger),
    Time(TimeTrigger),
}

#[derive(Debug, Serialize, Deserialize)]
pub enum LogRoller {
}
*/

/// Gets information from the proxies at the specified address(es).
#[derive(Debug, Args)]
pub struct GetCommand {
    /// Address(es) to connect to. Can be set multiple times as well as split by comma.
    /// "https://" prefixed before an address makes it use a secure connection while "http://"
    /// or no prefix is unsecure. If not trying all addresses, attempts stop after the first
    /// success.
    /// If no address is specified, the value of the GRPC_PROXY_URL environment variable is used.
    #[arg(long = "addr")]
    pub addrs: Vec<String>,

    /// Use all addresses, even even if one succeeds.
    #[arg(short, long)]
    pub all: bool,

    /// Get the config from the address(es).
    #[arg(short, long)]
    pub config: bool,

    /// The directory to output config file(s) to. If not specified but getting the config,
    /// outputs to the current directory.
    #[arg(long)]
    pub config_out: Option<PathBuf>,

    /// Get the status from the address(es).
    #[arg(short, long)]
    pub status: bool,

    /// The directory to output status file(s) to. If not specified but getting the status,
    /// outputs to the current directory.
    #[arg(long)]
    pub status_out: Option<PathBuf>,
}

/// Attempts to force a refresh of the proxies at the specified address(es). Also used to
/// add/delete stuff from the proxies.
#[derive(Debug, Args)]
pub struct RefreshCommand {
    /// Address(es) to connect to. Can be set multiple times as well as split by comma.
    /// "https://" prefixed before an address makes it use a secure connection while "http://"
    /// or no prefix is unsecure. If not trying all addresses, attempts stop after the first
    /// success.
    /// If no address is specified, the value of the GRPC_PROXY_URL environment variable is used.
    #[arg(long = "addr")]
    pub addrs: Vec<String>,

    /// Use all addresses, even even if one succeeds.
    #[arg(short, long)]
    pub all: bool,

    /// The path to the config file to read and parse. If not specified, and neither --add nor
    /// --del are specified, the given address(es) will attempt to read from the respective
    /// currently set config path. This will replace everything on each server specified.
    #[arg(short, long = "config")]
    pub config_path: Option<String>,

    /// Add the content of the specified config to the address(es).
    #[arg(long, group = "action", requires = "config_path")]
    pub add: bool,

    /// Delete the content of the specified config from the address(es).
    #[arg(long, group = "action", requires = "config_path")]
    pub del: bool,

    /// Optional directory to output the new config(s) returned by the server(s). If not specified,
    /// nothing is done.
    #[arg(short, long)]
    pub out: Option<PathBuf>,
}

/// Shuts down the proxies at the specified address(es).
#[derive(Debug, Args)]
pub struct ShutdownCommand {
    /// Address(es) to connect to. Can be set multiple times as well as split by comma.
    /// "https://" prefixed before an address makes it use a secure connection while "http://"
    /// or no prefix is unsecure. If not trying all addresses, attempts stop after the first
    /// success.
    /// If no address is specified, the value of the GRPC_PROXY_URL environment variable is used.
    #[arg(long = "addr")]
    pub addrs: Vec<String>,

    /// Use all addresses, even even if one succeeds.
    #[arg(short, long)]
    pub all: bool,

    /// Force shutdown and exit of the entire program, without waiting for anything to finish.
    #[arg(short, long)]
    pub force: bool,
}
