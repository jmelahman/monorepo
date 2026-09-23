use std::io::Write;

use log::{LevelFilter, Log, Metadata, Record};

/// A minimal stderr logger.
///
/// This replaces `env_logger`, which pulls in a datetime formatter and a full
/// regex-based filter for features this binary never uses: the only runtime
/// choice is whether `--debug` was passed.
struct Logger;

impl Log for Logger {
    fn enabled(&self, metadata: &Metadata<'_>) -> bool {
        metadata.level() <= log::max_level()
    }

    fn log(&self, record: &Record<'_>) {
        if !self.enabled(record.metadata()) {
            return;
        }
        // A failed log write must not take the program down with it.
        let _ = writeln!(std::io::stderr(), "[{}] {}", record.level(), record.args());
    }

    fn flush(&self) {
        let _ = std::io::stderr().flush();
    }
}

pub fn init(debug: bool) {
    log::set_max_level(if debug {
        LevelFilter::Debug
    } else {
        LevelFilter::Info
    });
    let _ = log::set_logger(&Logger);
}
