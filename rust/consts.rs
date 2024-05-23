pub mod env_names {
    //pub const ADDR_ENV_NAME: &str = "GRPC_PROXY_ADDR";
    pub const PWD_ENV_NAME: &str = "GRPC_PROXY_PASSWORD";
    pub const LOG_TARGET_ENV_NAME: &str = "GRPC_PROXY_LOG_TARGET";
}

pub mod headers {
    pub const BASE: &str = "X-Grpc-Proxy";
    pub const USERNAME: &str = "X-Grpc-Proxy-Username";
    pub const PASSWORD: &str = "X-Grpc-Proxy-Password";
    pub const REFRESH_ACTION: &str = "X-Grpc-Proxy-Refresh-Action";
    pub const CONFIG_LEN: &str = "X-Grpc-Proxy-Config-Len";
    pub const STATUS_LEN: &str = "X-Grpc-Proxy-Status-Len";
}

pub mod targets {
    #[allow(dead_code)]
    pub const LM: &str = "grpc_proxy";
    pub const LM_RUN: &str = "grpc_proxy::run";
    pub const LM_LISTENER: &str = "grpc_proxy::listener";
    pub const LM_SERVE_PROXY: &str = "grpc_proxy::serve_proxy";
    pub const LM_SERVE: &str = "grpc_proxy::serve";
    pub const LM_PROXY_CONNECT: &str = "grpc_proxy::proxy_connect";
    pub const LM_PROXY: &str = "grpc_proxy::proxy";
    pub const LM_HANDLE_REQ: &str = "grpc_proxy::handle_req";
    pub const LM_HANDLE_GET: &str = "grpc_proxy::handle_get";
    pub const LM_HANDLE_REFRESH: &str = "grpc_proxy::handle_refresh";
    pub const LM_HANDLE_SHUTDOWN: &str = "grpc_proxy::handle_shutdown";
    pub const LM_SIGNALS: &str = "grpc_proxy::signals";
}
