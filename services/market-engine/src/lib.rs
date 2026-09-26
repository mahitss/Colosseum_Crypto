pub fn health() -> &'static str {
    "ok"
}

pub fn version() -> &'static str {
    env!("CARGO_PKG_VERSION")
}

#[cfg(test)]
mod tests {
    use super::{health, version};

    #[test]
    fn health_returns_ok() {
        assert_eq!(health(), "ok");
    }

    #[test]
    fn version_returns_package_version() {
        assert!(!version().is_empty());
    }
}
