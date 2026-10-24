// Copyright 2023 Google LLC
// SPDX-License-Identifier: Apache-2.0

use std::collections::HashMap;

/// Counter counts the number of times each value of type T has been seen.
struct Counter {
    counts: HashMap<char, u64>,
}

impl Counter {
    /// Create a new Counter.
    fn new() -> Self {
        Counter {
            counts: HashMap::new(),
        }
    }

    /// Count an occurrence of the given value.
    fn count(&mut self, value: char) {
        if self.counts.contains_key(&value) {
            *self.counts.get_mut(&value).unwrap() += 1;
        } else {
            self.counts.insert(value, 1);
        }
    }

    /// Return the number of times the given value has been seen.
    fn times_seen(&self, value: char) -> u64 {
        self.counts.get(&value).copied().unwrap_or_default()
    }
}

fn main() {
    let mut charctr = Counter::new();
    charctr.count(' ');
    charctr.count('a');
    charctr.count('¥');
    charctr.count('a');
    println!(
        "most common character: {:?}",
        charctr.counts.keys().max_by_key(|&c| charctr.times_seen(*c))
    );

    let mut intctr = Counter::new();
    intctr.count(13);
    intctr.count(14);
    intctr.count(16);
    intctr.count(14);
    intctr.count(14);
    intctr.count(11);

    for i in 10..20 {
        println!("saw {} values equal to {}", intctr.times_seen(i), i);
    }

    let mut strctr = Counter::new();
    strctr.count("apple");
    strctr.count("orange");
    strctr.count("apple");
    println!("got {} apples", strctr.times_seen("apple"));
}
