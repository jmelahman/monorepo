// Copyright 2023 Google LLC
// SPDX-License-Identifier: Apache-2.0

fn transpose(matrix: [[i32; 3]; 3]) -> [[i32; 3]; 3] {
    todo!()
}

fn main() {
    let matrix = [
        [101, 102, 103], // <-- the comment makes rustfmt add a newline
        [201, 202, 203],
        [301, 302, 303],
    ];

    println!("Original:");
    for row in matrix {
        println!("{row:?}");
    }

    let transposed = transpose(matrix);

    println!("\nTransposed:");
    for row in transposed {
        println!("{row:?}");
    }
}
