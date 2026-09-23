fn main() {
    // `option_env!` bakes these in at compile time, so a rebuild must be
    // triggered when they change rather than reusing a stale version string.
    println!("cargo:rerun-if-env-changed=TAG_VERSION");
    println!("cargo:rerun-if-env-changed=TAG_COMMIT");
}
