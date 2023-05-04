use std::env;
use std::fs;
use std::io::Error;
use std::path::{Path, PathBuf};
use std::process::ExitCode;
use std::sync::mpsc::channel;
use threadpool::ThreadPool;
use ignore::WalkBuilder;

fn is_valid_symlink<P: AsRef<std::path::Path>>(path: &P) -> Result<bool, Error> {
    if let Ok(target_path) = fs::read_link(path) {
        if target_path.is_absolute() {
            if let Ok(_) = fs::metadata(&target_path) {
                return Ok(true);
            }
        }
        let dirname = path.as_ref().parent().unwrap_or(Path::new(""));
        if let Ok(_) = std::fs::metadata(PathBuf::from(dirname).join(target_path)) {
            return Ok(true);
        }
    }
    println!("{:?} is not a valid symlink", path.as_ref());
    Ok(false)
}

fn lint<P: AsRef<std::path::Path>>(path: P) -> Result<bool, Error> {
    if let Ok(metadata) = fs::symlink_metadata(&path) {
        if metadata.file_type().is_symlink() {
            return is_valid_symlink(&path);
        }
    }
    Ok(true)
}

fn main() -> ExitCode {
    let args: Vec<String> = env::args().skip(1).collect();
    let mut exit_code = 0;
    let pool = ThreadPool::new(num_cpus::get());

    let (tx, rx) = channel();

    if !args.is_empty() {
        for filename in args {
            let path = Path::new(&filename).to_owned();
            let tx = tx.clone();
            pool.execute(move || {
                let errors = lint(path);
                tx.send(errors).expect("Could not send data!");
            });
        }
    } else {
        for entry in WalkBuilder::new("./").hidden(false).build()
            .into_iter()
            .filter_map(|e| e.ok())
            .filter(|e| !e.path().is_dir())
        {
            let path = entry.path().to_owned();
            let tx = tx.clone();
            pool.execute(move || {
                let errors = lint(path);
                tx.send(errors).expect("Could not send data!");
            });
        }
    }
    drop(tx);
    for t in rx.iter() {
        let Ok(passed) = t else {
            todo!();
        };
        if !passed {
            exit_code = 1;
        }
    }
    ExitCode::from(exit_code)
}
