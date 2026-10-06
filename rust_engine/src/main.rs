use std::net::IpAddr;
use std::time::Duration;

#[tokio::main]
async fn main() {
    let target: IpAddr = "8.8.8.8".parse().unwrap();

    println!("Pinging 8.8.8.8 with 1 second timeout...");
    match ping::ping(target, Duration::from_secs(1)) {
        Ok(reply) => println!("Success! RTT: {:?}", reply.rtt),
        Err(e) => println!("Ping failed: {}", e),
    }
}