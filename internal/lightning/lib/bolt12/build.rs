extern crate cbindgen;

use cbindgen::Config;
use std::env;
use std::path::PathBuf;

fn main() {
    let crate_dir = env::var("CARGO_MANIFEST_DIR").unwrap();

    let output_file = env::var("BOLT12_HEADER_FILE").unwrap();

    let config = Config {
        language: cbindgen::Language::C,
        ..Default::default()
    };
    cbindgen::generate_with_config(&crate_dir, config)
        .unwrap()
        .write_to_file(&output_file);
}