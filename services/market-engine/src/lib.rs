use serde::{Deserialize, Serialize};
use std::str::FromStr;

const SCALE: i128 = 1_000_000_000_000;
const MAX_DECIMAL_PLACES: usize = 12;

#[derive(Clone, Copy, Debug, PartialEq, Eq, PartialOrd, Ord)]
struct Fixed(i128);

impl Fixed {
    fn parse(value: &str) -> Result<Self, EngineError> {
        let value = value.trim();
        if value.is_empty() || value.starts_with('+') {
            return Err(EngineError::InvalidDecimal);
        }
        let (negative, unsigned) = match value.strip_prefix('-') {
            Some(rest) => (true, rest),
            None => (false, value),
        };
        let mut parts = unsigned.split('.');
        let whole = parts.next().ok_or(EngineError::InvalidDecimal)?;
        let fraction = parts.next();
        if parts.next().is_some()
            || whole.is_empty()
            || !whole.bytes().all(|byte| byte.is_ascii_digit())
        {
            return Err(EngineError::InvalidDecimal);
        }
        let mut fraction = fraction.unwrap_or("").to_string();
        if unsigned.contains('.') && fraction.is_empty() {
            return Err(EngineError::InvalidDecimal);
        }
        if !fraction.bytes().all(|byte| byte.is_ascii_digit()) {
            return Err(EngineError::InvalidDecimal);
        }
        while fraction.len() > MAX_DECIMAL_PLACES && fraction.ends_with('0') {
            fraction.pop();
        }
        if fraction.len() > MAX_DECIMAL_PLACES {
            return Err(EngineError::PrecisionExceeded);
        }
        fraction.push_str(&"0".repeat(MAX_DECIMAL_PLACES - fraction.len()));
        let whole = i128::from_str(whole).map_err(|_| EngineError::OutOfRange)?;
        let fractional = if fraction.is_empty() {
            0
        } else {
            i128::from_str(&fraction).map_err(|_| EngineError::InvalidDecimal)?
        };
        let magnitude = whole
            .checked_mul(SCALE)
            .and_then(|value| value.checked_add(fractional))
            .ok_or(EngineError::OutOfRange)?;
        let value = if negative {
            magnitude.checked_neg().ok_or(EngineError::OutOfRange)?
        } else {
            magnitude
        };
        Ok(Self(value))
    }

    fn format(self) -> String {
        let negative = self.0 < 0;
        let magnitude = self.0.unsigned_abs();
        let whole = magnitude / SCALE as u128;
        let fraction = magnitude % SCALE as u128;
        if fraction == 0 {
            return format!("{}{}", if negative { "-" } else { "" }, whole);
        }
        let fraction = format!("{fraction:012}");
        let fraction = fraction.trim_end_matches('0');
        format!("{}{}.{}", if negative { "-" } else { "" }, whole, fraction)
    }

    fn abs(self) -> Result<Self, EngineError> {
        self.0
            .checked_abs()
            .map(Self)
            .ok_or(EngineError::OutOfRange)
    }

    fn checked_sub(self, other: Self) -> Result<Self, EngineError> {
        self.0
            .checked_sub(other.0)
            .map(Self)
            .ok_or(EngineError::OutOfRange)
    }

    fn checked_mul_integer(self, value: i128) -> Result<Self, EngineError> {
        self.0
            .checked_mul(value)
            .map(Self)
            .ok_or(EngineError::OutOfRange)
    }

    fn percentage_change(previous: Self, current: Self) -> Result<Option<Self>, EngineError> {
        if previous.0 == 0 {
            return Ok(None);
        }
        let numerator = current
            .checked_sub(previous)?
            .0
            .checked_mul(100)
            .ok_or(EngineError::OutOfRange)?;
        let quotient = numerator / previous.0;
        let remainder = numerator % previous.0;
        let fractional = remainder
            .checked_mul(SCALE)
            .ok_or(EngineError::OutOfRange)?
            / previous.0;
        let scaled = quotient
            .checked_mul(SCALE)
            .and_then(|value| value.checked_add(fractional))
            .ok_or(EngineError::OutOfRange)?;
        Ok(Some(Self(scaled)))
    }
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub enum EngineError {
    InvalidDecimal,
    PrecisionExceeded,
    OutOfRange,
    InvalidConfiguration,
}

impl std::fmt::Display for EngineError {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter.write_str(match self {
            Self::InvalidDecimal => "invalid decimal input",
            Self::PrecisionExceeded => "decimal precision exceeds 12 places",
            Self::OutOfRange => "numeric input is out of range",
            Self::InvalidConfiguration => "signal configuration is invalid",
        })
    }
}

impl std::error::Error for EngineError {}

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(default)]
pub struct SignalConfig {
    pub probability_shift_minor: String,
    pub probability_shift_significant: String,
    pub probability_shift_major: String,
    pub activity_change_significant: String,
    pub liquidity_change_significant: String,
    pub observation_window_seconds: u64,
}

impl Default for SignalConfig {
    fn default() -> Self {
        Self {
            probability_shift_minor: "0.03".into(),
            probability_shift_significant: "0.05".into(),
            probability_shift_major: "0.10".into(),
            activity_change_significant: "0.50".into(),
            liquidity_change_significant: "0.25".into(),
            observation_window_seconds: 86_400,
        }
    }
}

#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct ObservationValues {
    pub yes_probability: Option<String>,
    pub no_probability: Option<String>,
    pub volume_usdc: Option<String>,
    pub liquidity: Option<String>,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct ObservationInput {
    pub market_id: String,
    pub observation_id: i64,
    pub observed_at: String,
    pub observation_window: String,
    pub is_new_market: bool,
    pub previous: Option<ObservationValues>,
    pub window_previous: Option<ObservationValues>,
    pub current: ObservationValues,
}

#[derive(Clone, Copy, Debug, Deserialize, PartialEq, Eq, Serialize)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum SignalType {
    NewMarket,
    ProbabilityShift,
    ActivityChange,
    LiquidityChange,
    MarketMovement,
}

#[derive(Clone, Copy, Debug, Deserialize, PartialEq, Eq, Serialize)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum Severity {
    Info,
    Watch,
    Significant,
    Critical,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
pub struct Signal {
    pub market_id: String,
    pub observation_id: i64,
    pub timestamp: String,
    pub signal_type: SignalType,
    pub severity: Severity,
    pub metric: String,
    pub previous_value: Option<String>,
    pub current_value: Option<String>,
    pub absolute_change: Option<String>,
    pub percentage_change: Option<String>,
    pub percentage_points: Option<String>,
    pub observation_window: String,
    pub source: String,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct EngineRequest {
    #[serde(default)]
    pub config: SignalConfig,
    pub observations: Vec<ObservationInput>,
}

pub fn calculate_absolute_change(previous: &str, current: &str) -> Result<String, EngineError> {
    let previous = Fixed::parse(previous)?;
    let current = Fixed::parse(current)?;
    Ok(current.checked_sub(previous)?.format())
}

pub fn calculate_percentage_change(
    previous: &str,
    current: &str,
) -> Result<Option<String>, EngineError> {
    let previous = Fixed::parse(previous)?;
    let current = Fixed::parse(current)?;
    Ok(Fixed::percentage_change(previous, current)?.map(Fixed::format))
}

pub fn classify_signal_severity(
    magnitude: &str,
    config: &SignalConfig,
) -> Result<Severity, EngineError> {
    let magnitude = Fixed::parse(magnitude)?.abs()?;
    let significant = Fixed::parse(&config.probability_shift_significant)?;
    let major = Fixed::parse(&config.probability_shift_major)?;
    if minor.0 <= 0 || significant < minor || major < significant {
        return Err(EngineError::InvalidConfiguration);
    }
    Ok(if magnitude >= major {
        Severity::Critical
    } else if magnitude >= significant {
        Severity::Significant
    } else if magnitude >= minor {
        Severity::Watch
    } else {
        Severity::Info
    })
}

pub fn calculate_probability_shift(
    input: &ObservationInput,
    config: &SignalConfig,
) -> Result<Option<Signal>, EngineError> {
    let Some(previous) = input
        .window_previous
        .as_ref()
        .and_then(|values| values.yes_probability.as_deref())
    else {
        return Ok(None);
    };
    let Some(current) = input.current.yes_probability.as_deref() else {
        return Ok(None);
    };
    let previous_value = Fixed::parse(previous)?;
    let current_value = Fixed::parse(current)?;
    if !(0..=SCALE).contains(&previous_value.0) || !(0..=SCALE).contains(&current_value.0) {
        return Err(EngineError::InvalidDecimal);
    }
    let delta = current_value.checked_sub(previous_value)?;
    let magnitude = delta.abs()?;
    let severity = classify_signal_severity(&magnitude.format(), config)?;
    if severity == Severity::Info {
        return Ok(None);
    }
    let percentage_points = delta.checked_mul_integer(100)?.format();
    let percentage_change =
        Fixed::percentage_change(previous_value, current_value)?.map(Fixed::format);
    let base = make_signal(
        input,
        SignalType::ProbabilityShift,
        severity,
        "yes_probability",
        Some(previous_value.format()),
        Some(current_value.format()),
        Some(delta.format()),
        percentage_change,
        Some(percentage_points),
    );
    Ok(Some(base))
}

fn detect_market_movement(
    input: &ObservationInput,
    config: &SignalConfig,
) -> Result<Option<Signal>, EngineError> {
    let Some(previous) = input
        .window_previous
        .as_ref()
        .and_then(|values| values.yes_probability.as_deref())
    else {
        return Ok(None);
    };
    let Some(current) = input.current.yes_probability.as_deref() else {
        return Ok(None);
    };
    let previous_value = Fixed::parse(previous)?;
    let current_value = Fixed::parse(current)?;
    if !(0..=SCALE).contains(&previous_value.0) || !(0..=SCALE).contains(&current_value.0) {
        return Err(EngineError::InvalidDecimal);
    }
    let delta = current_value.checked_sub(previous_value)?;
    if delta.abs()? < Fixed::parse(&config.probability_shift_major)? {
        return Ok(None);
    }
    let percentage_change = Fixed::percentage_change(previous_value, current_value)?;
    Ok(Some(make_signal(
        input,
        SignalType::MarketMovement,
        Severity::Critical,
        "yes_probability",
        Some(previous_value.format()),
        Some(current_value.format()),
        Some(delta.format()),
        percentage_change.map(Fixed::format),
        Some(delta.checked_mul_integer(100)?.format()),
    )))
}

pub fn detect_activity_change(
    input: &ObservationInput,
    config: &SignalConfig,
) -> Result<Option<Signal>, EngineError> {
    detect_relative_change(
        input,
        config,
        SignalType::ActivityChange,
        "volume_usdc",
        |values| values.volume_usdc.as_deref(),
        &config.activity_change_significant,
    )
}

pub fn detect_liquidity_change(
    input: &ObservationInput,
    config: &SignalConfig,
) -> Result<Option<Signal>, EngineError> {
    detect_relative_change(
        input,
        config,
        SignalType::LiquidityChange,
        "liquidity",
        |values| values.liquidity.as_deref(),
        &config.liquidity_change_significant,
    )
}

fn detect_relative_change<F>(
    input: &ObservationInput,
    config: &SignalConfig,
    signal_type: SignalType,
    metric: &str,
    value: F,
    threshold: &str,
) -> Result<Option<Signal>, EngineError>
where
    F: Fn(&ObservationValues) -> Option<&str>,
{
    let Some(previous) = input.window_previous.as_ref().and_then(&value) else {
        return Ok(None);
    };
    let Some(current) = value(&input.current) else {
        return Ok(None);
    };
    let previous_value = Fixed::parse(previous)?;
    let current_value = Fixed::parse(current)?;
    let absolute = current_value.checked_sub(previous_value)?;
    let Some(percentage_change) = Fixed::percentage_change(previous_value, current_value)? else {
        return Ok(None);
    };
    let threshold = Fixed::parse(threshold)?;
    if threshold.0 <= 0 {
        return Err(EngineError::InvalidConfiguration);
    }
    let magnitude = percentage_change.abs()?;
    if magnitude < threshold {
        return Ok(None);
    }
    let critical_threshold = threshold.checked_mul_integer(2)?;
    let severity = if magnitude >= critical_threshold {
        Severity::Critical
    } else {
        Severity::Significant
    };
    Ok(Some(make_signal(
        input,
        signal_type,
        severity,
        metric,
        Some(previous_value.format()),
        Some(current_value.format()),
        Some(absolute.format()),
        Some(percentage_change.format()),
        None,
    )))
}

pub fn generate_signals(request: EngineRequest) -> Result<Vec<Signal>, EngineError> {
    validate_config(&request.config)?;
    let mut signals = Vec::new();
    for observation in &request.observations {
        if observation.is_new_market {
            signals.push(make_signal(
                observation,
                SignalType::NewMarket,
                Severity::Info,
                "market_discovered",
                None,
                None,
                None,
                None,
                None,
            ));
        }
        if let Some(signal) = calculate_probability_shift(observation, &request.config)? {
            signals.push(signal);
        }
        if let Some(signal) = detect_market_movement(observation, &request.config)? {
            signals.push(signal);
        }
        if let Some(signal) = detect_activity_change(observation, &request.config)? {
            signals.push(signal);
        }
        if let Some(signal) = detect_liquidity_change(observation, &request.config)? {
            signals.push(signal);
        }
    }
    Ok(signals)
}

fn validate_config(config: &SignalConfig) -> Result<(), EngineError> {
    let minor = Fixed::parse(&config.probability_shift_minor)?;
    let significant = Fixed::parse(&config.probability_shift_significant)?;
    let major = Fixed::parse(&config.probability_shift_major)?;
    let activity = Fixed::parse(&config.activity_change_significant)?;
    let liquidity = Fixed::parse(&config.liquidity_change_significant)?;
    if minor.0 <= 0
        || significant < minor
        || major < significant
        || activity.0 <= 0
        || liquidity.0 <= 0
        || config.observation_window_seconds == 0
    {
        return Err(EngineError::InvalidConfiguration);
    }
    Ok(())
}

fn make_signal(
    input: &ObservationInput,
    signal_type: SignalType,
    severity: Severity,
    metric: &str,
    previous_value: Option<String>,
    current_value: Option<String>,
    absolute_change: Option<String>,
    percentage_change: Option<String>,
    percentage_points: Option<String>,
) -> Signal {
    Signal {
        market_id: input.market_id.clone(),
        observation_id: input.observation_id,
        timestamp: input.observed_at.clone(),
        signal_type,
        severity,
        metric: metric.to_string(),
        previous_value,
        current_value,
        absolute_change,
        percentage_change,
        percentage_points,
        observation_window: input.observation_window.clone(),
        source: "panta".to_string(),
    }
}

pub fn health() -> &'static str {
    "ok"
}

pub fn version() -> &'static str {
    env!("CARGO_PKG_VERSION")
}

#[cfg(test)]
mod tests {
    use super::*;

    fn values(
        yes: Option<&str>,
        volume: Option<&str>,
        liquidity: Option<&str>,
    ) -> ObservationValues {
        ObservationValues {
            yes_probability: yes.map(str::to_string),
            no_probability: None,
            volume_usdc: volume.map(str::to_string),
            liquidity: liquidity.map(str::to_string),
        }
    }

    fn input(
        previous: Option<ObservationValues>,
        window_previous: Option<ObservationValues>,
        current: ObservationValues,
    ) -> ObservationInput {
        ObservationInput {
            market_id: "market-id".into(),
            observation_id: 1,
            observed_at: "2026-09-26T00:00:00Z".into(),
            observation_window: "24h".into(),
            is_new_market: false,
            previous,
            window_previous,
            current,
        }
    }

    #[test]
    fn absolute_and_percentage_changes_are_exact_decimal_strings() {
        assert_eq!(calculate_absolute_change("0.63", "0.71").unwrap(), "0.08");
        assert_eq!(calculate_absolute_change("0.71", "0.63").unwrap(), "-0.08");
        assert_eq!(calculate_absolute_change("100.00", "160.00").unwrap(), "60");
        assert_eq!(
            calculate_percentage_change("100", "160")
                .unwrap()
                .as_deref(),
            Some("60")
        );
        assert_eq!(calculate_percentage_change("0", "1").unwrap(), None);
    }

    #[test]
    fn probability_shift_has_percentage_points_and_significant_severity() {
        let observation = input(
            None,
            Some(values(Some("0.63"), None, None)),
            values(Some("0.71"), None, None),
        );
        let signal = calculate_probability_shift(&observation, &SignalConfig::default())
            .unwrap()
            .unwrap();
        assert_eq!(signal.signal_type, SignalType::ProbabilityShift);
        assert_eq!(signal.severity, Severity::Significant);
        assert_eq!(signal.absolute_change.as_deref(), Some("0.08"));
        assert_eq!(signal.percentage_points.as_deref(), Some("8"));
        assert_eq!(signal.percentage_change.as_deref(), Some("12.698412698412"));
    }

    #[test]
    fn movement_and_severity_boundaries_are_deterministic() {
        let observation = input(
            None,
            Some(values(Some("0.50"), None, None)),
            values(Some("0.60"), None, None),
        );
        let signals = generate_signals(EngineRequest {
            config: SignalConfig::default(),
            observations: vec![observation],
        })
        .unwrap();
        assert_eq!(signals.len(), 2);
        assert_eq!(signals[0].signal_type, SignalType::ProbabilityShift);
        assert_eq!(signals[0].severity, Severity::Critical);
        assert_eq!(signals[1].signal_type, SignalType::MarketMovement);
        assert_eq!(
            classify_signal_severity("0.03", &SignalConfig::default()).unwrap(),
            Severity::Watch
        );
        assert_eq!(
            classify_signal_severity("0.05", &SignalConfig::default()).unwrap(),
            Severity::Significant
        );
        assert_eq!(
            classify_signal_severity("0.10", &SignalConfig::default()).unwrap(),
            Severity::Critical
        );
    }

    #[test]
    fn activity_and_liquidity_require_both_observations() {
        let observation = input(
            Some(values(Some("0.5"), Some("100"), Some("4"))),
            Some(values(Some("0.5"), Some("100"), Some("4"))),
            values(Some("0.5"), Some("160"), Some("5")),
        );
        let activity = detect_activity_change(&observation, &SignalConfig::default())
            .unwrap()
            .unwrap();
        assert_eq!(activity.signal_type, SignalType::ActivityChange);
        assert_eq!(activity.percentage_change.as_deref(), Some("60"));
        let liquidity = detect_liquidity_change(&observation, &SignalConfig::default())
            .unwrap()
            .unwrap();
        assert_eq!(liquidity.signal_type, SignalType::LiquidityChange);
        assert_eq!(liquidity.percentage_change.as_deref(), Some("25"));

        let decreased = input(
            None,
            Some(values(Some("0.5"), Some("200"), Some("8"))),
            values(Some("0.5"), Some("100"), Some("4")),
        );
        let decreased_activity = detect_activity_change(&decreased, &SignalConfig::default())
            .unwrap()
            .unwrap();
        assert_eq!(decreased_activity.absolute_change.as_deref(), Some("-100"));
        assert_eq!(decreased_activity.percentage_change.as_deref(), Some("-50"));

        let missing = input(None, None, values(Some("0.5"), Some("160"), None));
        assert!(detect_activity_change(&missing, &SignalConfig::default())
            .unwrap()
            .is_none());
        assert!(detect_liquidity_change(&missing, &SignalConfig::default())
            .unwrap()
            .is_none());
    }

    #[test]
    fn new_market_and_insufficient_history_are_distinct() {
        let mut observation = input(None, None, values(Some("0.71"), Some("12"), None));
        observation.is_new_market = true;
        let signals = generate_signals(EngineRequest {
            config: SignalConfig::default(),
            observations: vec![observation],
        })
        .unwrap();
        assert_eq!(signals.len(), 1);
        assert_eq!(signals[0].signal_type, SignalType::NewMarket);
        assert_eq!(signals[0].severity, Severity::Info);

        let zero_baseline = input(
            None,
            Some(values(Some("0"), None, None)),
            values(Some("0.05"), None, None),
        );
        let probability = calculate_probability_shift(&zero_baseline, &SignalConfig::default())
            .unwrap()
            .unwrap();
        assert_eq!(probability.percentage_points.as_deref(), Some("5"));
        assert_eq!(probability.percentage_change, None);
    }

    #[test]
    fn rejects_unsupported_precision_probability_range_and_bad_config() {
        assert_eq!(
            calculate_absolute_change("0.0000000000001", "1"),
            Err(EngineError::PrecisionExceeded)
        );
        let invalid_probability = input(
            None,
            Some(values(Some("1.1"), None, None)),
            values(Some("1.2"), None, None),
        );
        assert_eq!(
            calculate_probability_shift(&invalid_probability, &SignalConfig::default()),
            Err(EngineError::InvalidDecimal)
        );
        let invalid_config = SignalConfig {
            probability_shift_major: "0.02".into(),
            ..SignalConfig::default()
        };
        assert_eq!(
            classify_signal_severity("0.05", &invalid_config),
            Err(EngineError::InvalidConfiguration)
        );
    }
}
