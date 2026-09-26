use market_engine::{generate_signals, EngineRequest};
use std::io::{self, Read};

fn main() {
    if let Err(error) = run() {
        eprintln!("market engine failed: {error}");
        std::process::exit(1);
    }
}

fn run() -> Result<(), Box<dyn std::error::Error>> {
    let mut input = String::new();
    io::stdin().take(32 << 20).read_to_string(&mut input)?;
    let request: EngineRequest = serde_json::from_str(&input)?;
    let signals = generate_signals(request)?;
    serde_json::to_writer(io::stdout().lock(), &signals)?;
    Ok(())
}
