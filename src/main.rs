use ignore::WalkBuilder;
use rayon::prelude::*;
use std::env;
use std::fs;
use std::io::Error;
use std::path::{Path, PathBuf};
use std::process::ExitCode;


fn is_valid_symlink<P: AsRef<Path>>(path: &P) -> Result<bool, Error> {
    if let Ok(target_path) = fs::read_link(path) {
        if target_path.is_absolute() && fs::metadata(&target_path).is_ok() {
            return Ok(true);
        }
        let dirname = path.as_ref().parent().unwrap_or_else(|| Path::new(""));
        let resolved = dirname.join(&target_path);
        if fs::metadata(resolved).is_ok() {
            return Ok(true);
        }
    }
    println!("{:?} is not a valid symlink", path.as_ref());
    Ok(false)
}


fn lint<P: AsRef<Path>>(path: P) -> Result<bool, Error> {
    let path = path.as_ref();
    if let Ok(metadata) = fs::symlink_metadata(path) {
        if metadata.file_type().is_symlink() {
            return is_valid_symlink(&path);
        }
    }
    Ok(true)
}


fn main() -> ExitCode {
    let args: Vec<String> = env::args().skip(1).collect();
    let mut exit_code = 0;

    let files: Vec<PathBuf> = if args.is_empty() {
        WalkBuilder::new("./")
            .hidden(false)
            .build()
            .into_iter()
            .filter_map(Result::ok)
            .filter(|e| !e.path().is_dir())
            .map(|e| e.path().to_owned())
            .collect()
    } else {
        args.iter().map(PathBuf::from).collect()
    };

    let results: Vec<_> = files.par_iter().map(|path| lint(path)).collect();

    for result in results {
        match result {
            Ok(passed) if !passed => exit_code = 1,
            Err(_) => exit_code = 1,
            _ => {}
        }
    }

    ExitCode::from(exit_code)
}

