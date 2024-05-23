fn main() {
    let mut args = std::env::args().skip(1);
    let listen = args.next().expect("missing listen addr");
    let connect = args.next().expect("missing connect addr");
    let connect: &'static str = Box::leak(connect.clone().into_boxed_str());
    if false {
        run1(listen, connect, args.by_ref());
        run2(listen, connect, args.by_ref());
        run3(listen, connect, args.by_ref());
        run4(listen, connect, args.by_ref());
        run5(listen, connect, args.by_ref());
    }
    run5(listen, connect, args);
}

fn run1(listen: &String, connect: &'static str, args: impl Iterator<Item=String>) {
    use tokio::net::{TcpListener, TcpStream};
    use tokio::runtime::Builder as RtBuilder;

    let rt = if let Some(n) = args.next() {
        RtBuilder::new_multi_thread()
            .worker_threads(n.parse().expect("bad number"))
            .enable_all()
            .build()
            .expect("error building runtime")
    } else {
        RtBuilder::new_current_thread()
            .enable_all()
            .build()
            .expect("error building runtime")
    };
    let _guard = rt.enter();
    rt.block_on(async move {
        let ln = TcpListener::bind(listen).await.expect("error listening");
        loop {
            let (mut stream, _) = ln.accept().await.expect("error accepting");
            eprintln!("new conn");
            tokio::spawn(async move {
                let mut other = TcpStream::connect(connect).await.expect("error connecting");
                let _ = tokio::io::copy_bidirectional(&mut stream, &mut other).await;
            });
        }
    });
}

fn run2(listen: &String, connect: &'static str, _args: impl Iterator<Item=String>) {
    use std::net::{TcpListener, TcpStream};
    use std::sync::Arc;

    fn copy_bi(a: Arc<TcpStream>, b: Arc<TcpStream>) -> std::io::Result<(u64, u64)> {
        use std::sync::mpsc::sync_channel;
        use std::thread;
        let (atx, arx) = sync_channel(1);
        let (btx, brx) = sync_channel(1);

        let (ac, bc) = (Arc::clone(&a), Arc::clone(&b));
        let ah = thread::spawn(move || {
            let (a, b) = (ac, bc);
            atx.send(std::io::copy(&mut &*a, &mut &*b)).unwrap();
        });
        let bh = thread::spawn(move || {
            btx.send(std::io::copy(&mut &*b, &mut &*a)).unwrap();
        });
        let an = arx.recv().unwrap()?;
        let bn = brx.recv().unwrap()?;
        let (_, _) = (ah.join(), bh.join());
        Ok((an, bn))
    }

    let ln = TcpListener::bind(listen).expect("error listening");
    loop {
        let (stream, _) = ln.accept().expect("error accepting");
        eprintln!("new conn");
        let other = TcpStream::connect(connect).expect("error connecting");
        let (stream, other) = (Arc::new(stream), Arc::new(other));
        let _ = copy_bi(stream, other);
    }
}

fn run3(listen: &String, connect: &'static str, _args: impl Iterator<Item=String>) {
    use async_std::io::copy;
    use async_std::net::{TcpListener, TcpStream};
    use async_std::task;

    task::block_on(async move {
        let ln = TcpListener::bind(listen).await.expect("error listening");
        loop {
            let (mut stream, _) = ln.accept().await.expect("error accepting");
            eprintln!("new conn");
            task::spawn(async move {
                let mut other = TcpStream::connect(connect).await.expect("error connecting");
                let (s, o) = (stream.clone(), other.clone());
                task::spawn(async {
                    let (mut stream, mut other) = (s, o);
                    let _ = copy(&mut stream, &mut other).await;
                });
                let _ = copy(&mut other, &mut stream).await;
            });
        }
    });
}

fn run4(listen: &String, connect: &'static str, args: impl Iterator<Item=String>) {
    use async_std::io::copy;
    use async_std::net::{TcpListener, TcpStream};
    use tokio::runtime::Builder as RtBuilder;

    let rt = if let Some(n) = args.next() {
        RtBuilder::new_multi_thread()
            .worker_threads(n.parse().expect("bad number"))
            .enable_all()
            .build()
            .expect("error building runtime")
    } else {
        RtBuilder::new_current_thread()
            .enable_all()
            .build()
            .expect("error building runtime")
    };

    let _guard = rt.enter();
    rt.block_on(async move {
        let ln = TcpListener::bind(listen).await.expect("error listening");
        loop {
            let (mut stream, _) = ln.accept().await.expect("error accepting");
            eprintln!("new conn");
            tokio::spawn(async move {
                let mut other = TcpStream::connect(connect).await.expect("error connecting");
                let (s, o) = (stream.clone(), other.clone());
                tokio::spawn(async {
                    let (mut stream, mut other) = (s, o);
                    let _ = copy(&mut stream, &mut other).await;
                });
                let _ = copy(&mut other, &mut stream).await;
            });
        }
    });
}

/*
fn run5(listen: &String, connect: &'static str, args: impl Iterator<Item=String>) {
    use h2::client::{Builder as ClientBuilder};
    use h2::server::{Builder as ServerBuilder};
    use tokio::runtime::Builder as RtBuilder;
    
    let rt = if let Some(n) = args.next() {
        RtBuilder::new_multi_thread()
            .worker_threads(n.parse().expect("bad number"))
            .enable_all()
            .build()
            .expect("error building runtime")
    } else {
        RtBuilder::new_current_thread()
            .enable_all()
            .build()
            .expect("error building runtime")
    };

    let server_builder = {
        let mut builder = ServerBuilder::new();
        server_builder
    };

    let _guard = rt.enter();
    rt.block_on(async move {
        let ln = TcpListener::bind(listen).await.expect("error listening");
        loop {
            let (mut stream, _) = ln.accept().await.expect("error accepting");
            eprintln!("new conn");
            tokio::spawn(async move {
                let other = TcpStream::connect(connect).await.expect("error connecting");
                let (ch2, conn) = client_builder.clone().handshake(other).await.expect("client handshake");
                tokio::spawn(async move {
                    conn.await.unwrap();
                });

                let mut sh2 = server_builder.clone().handshake(stream).await.expect("server handshake");
                while let Some(req) = h2.accept().await {
                    let (req, mut send_resp) = request.unwrap();
                    let body;
                    let req = req.map(|b| {
                        body = b;
                        ()
                    });
                    let is_end_stream = body.is_end_stream();
                    let (resp, mut send_stream) = ch2.send_request(req, is_end_stream).expect("send request");
                    if is_end_stream {
                        // TODO
                        break;
                    }
                    if req.
                    send_stream.await;
                    
                    let body;
                    let resp = resp.map(|b| {
                        body = b;
                        ()
                    });
                    let is_end_stream = body.is_end_stream();
                    let mut send_stream = send_resp.send_response(resp, is_end_stream).expect("send response");
                    if is_end_stream {
                        // TODO
                        break;
                    }
                }
            });
    });
}
*/
