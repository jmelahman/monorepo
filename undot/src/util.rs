use std::ffi::OsString;
use std::fs;
use std::io;

use crate::CONFIG_DIR;

/// The path of root entry `name` once stored under .config/.
pub fn config_path(name: &str) -> String {
    format!("{CONFIG_DIR}/{name}")
}

/// Lists a directory's entry names, sorted so that runs are deterministic.
pub fn read_names(dir: &str) -> io::Result<Vec<OsString>> {
    let mut names = fs::read_dir(dir)?
        .map(|entry| Ok(entry?.file_name()))
        .collect::<io::Result<Vec<_>>>()?;
    names.sort();
    Ok(names)
}

/// Keeps the names that are valid UTF-8. The rest can't be written to
/// .gitignore or matched by a rule, so each one is reported and counted as a
/// problem rather than silently left unmanaged.
pub fn utf8_names(names: Vec<OsString>, fails: &mut usize) -> Vec<String> {
    names
        .into_iter()
        .filter_map(|name| {
            name.into_string()
                .map_err(|name| {
                    complain!("skipping {:?} (name is not valid UTF-8)", name);
                    *fails += 1;
                })
                .ok()
        })
        .collect()
}

/// Reads a text file, returning `None` only if it doesn't exist. Any other
/// failure is an error: callers rewrite these files, so treating an unreadable
/// or non-UTF-8 file as empty would overwrite the user's content.
pub fn read_optional(path: &str) -> Result<Option<String>, String> {
    match fs::read_to_string(path) {
        Ok(content) => Ok(Some(content)),
        Err(err) if err.kind() == io::ErrorKind::NotFound => Ok(None),
        Err(err) => Err(format!("cannot read {path}: {err}")),
    }
}
