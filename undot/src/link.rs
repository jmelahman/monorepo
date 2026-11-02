use std::fs;
use std::os::unix::fs::symlink;
use std::path::Path;
use std::process::ExitCode;

use crate::conf::{Conf, CONF_FILE};
use crate::util::{config_path, read_names, utf8_names};
use crate::CONFIG_DIR;

/// Symlinks every entry of .config/ into the repository root. A missing
/// .config/ is a no-op so the hook is harmless in repos that don't use undot
/// yet.
pub fn cmd_link(dry_run: bool) -> ExitCode {
    let Ok(meta) = fs::symlink_metadata(CONFIG_DIR) else {
        return ExitCode::SUCCESS;
    };
    if !meta.is_dir() {
        complain!("{CONFIG_DIR} exists but is not a directory");
        return ExitCode::FAILURE;
    }
    let names = match read_names(CONFIG_DIR) {
        Ok(names) => names,
        Err(err) => {
            complain!("cannot read {CONFIG_DIR}: {err}");
            return ExitCode::FAILURE;
        }
    };
    let conf = Conf::load();
    let mut fails = 0;
    for name in utf8_names(names, &mut fails) {
        // The conf file is never symlinked into the root; treat it as an
        // implicit nolink entry and clean up any stray link.
        let result = if name == CONF_FILE || conf.is_nolink(&name) {
            remove_nolink_link(&name, dry_run)
        } else {
            ensure_link(&name, dry_run)
        };
        if let Err(err) = result {
            complain!("{err}");
            fails += 1;
        }
    }
    if fails > 0 {
        ExitCode::FAILURE
    } else {
        ExitCode::SUCCESS
    }
}

/// Whether `name` is currently a symlink pointing exactly at `target`.
fn points_at(name: &str, target: &str) -> bool {
    fs::read_link(name).is_ok_and(|dest| dest == Path::new(target))
}

/// Makes the root entry `name` a symlink to .config/name. A correct link is left
/// alone; a stale or wrong link is replaced; a real file or directory is never
/// touched.
pub fn ensure_link(name: &str, dry_run: bool) -> Result<(), String> {
    let target = config_path(name);
    if let Ok(meta) = fs::symlink_metadata(name) {
        if !meta.is_symlink() {
            return Err(format!(
                "{name} already exists and is not a symlink; skipping \
                 (remove it or move it into {CONFIG_DIR}/)"
            ));
        }
        if points_at(name, &target) {
            return Ok(());
        }
        if !dry_run {
            fs::remove_file(name)
                .map_err(|err| format!("cannot remove stale link {name}: {err}"))?;
        }
    }
    if dry_run {
        say!("undot: [dry-run] would link {name} -> {target}");
        return Ok(());
    }
    symlink(&target, name).map_err(|err| format!("cannot link {name} -> {target}: {err}"))?;
    say!("undot: linked {name} -> {target}");
    Ok(())
}

/// Cleans up a root symlink that undot previously created for an entry now
/// marked nolink, so checkouts transition on their own. Anything at the root
/// path that isn't exactly our symlink is left alone.
pub fn remove_nolink_link(name: &str, dry_run: bool) -> Result<(), String> {
    let target = config_path(name);
    if !points_at(name, &target) {
        return Ok(());
    }
    if dry_run {
        say!("undot: [dry-run] would remove {name} (nolink)");
        return Ok(());
    }
    fs::remove_file(name).map_err(|err| format!("cannot remove {name} (nolink): {err}"))?;
    say!("undot: removed {name} (nolink: reference {target} directly)");
    Ok(())
}
