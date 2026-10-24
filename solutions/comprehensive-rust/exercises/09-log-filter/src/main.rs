// Copyright 2025 Google LLC
// SPDX-License-Identifier: Apache-2.0

pub trait Logger {
    /// Log a message at the given verbosity level.
    fn log(&self, verbosity: u8, message: &str);
}

struct StderrLogger;

impl Logger for StderrLogger {
    fn log(&self, verbosity: u8, message: &str) {
        eprintln!("[stderr] verbosity={verbosity}: {message}");
    }
}

struct StdoutLogger;

impl Logger for StdoutLogger {
    fn log(&self, verbosity: u8, message: &str) {
        println!("[stdout] verbosity={verbosity}: {message}");
    }
}

// TODO: Define and implement `Filter`.

fn main() {
    let prefix = "yikes";
    let max_verbosity = 3;

    // Logger that writes to stderr and filters based on message contents.
    let logger = Filter::new(StderrLogger, |_verbosity, msg| msg.contains(prefix));
    logger.log(5, "FYI");
    logger.log(1, "yikes, something went wrong");
    logger.log(2, "uhoh");

    // Logger that writes to stdout and filters based on verbosity.
    let logger =
        Filter::new(StdoutLogger, |verbosity, _msg| verbosity <= max_verbosity);
    logger.log(5, "FYI");
    logger.log(1, "yikes, something went wrong");
    logger.log(2, "uhoh");
}
