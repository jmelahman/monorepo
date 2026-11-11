"""Tests for oc.py's pure parts: python3 -m pytest scripts/test_oc.py (or run it directly)."""

import importlib.util
import os

spec = importlib.util.spec_from_file_location(
    "oc", os.path.join(os.path.dirname(__file__), "oc.py")
)
oc = importlib.util.module_from_spec(spec)
spec.loader.exec_module(oc)


def test_parse_tint():
    assert oc.parse_tint("Dirt=0.58,0.6,0.63") == ("Dirt", [0.58, 0.6, 0.63])
    assert oc.parse_tint("Rock mix=1,1,1")[0] == "Rock mix"
    for bad in ("Dirt", "Dirt=1,2", "=1,2,3", "Dirt=a,b,c"):
        try:
            oc.parse_tint(bad)
        except ValueError:
            continue
        raise AssertionError(bad)


def test_clean_log_drops_nuls_and_false_crash_matches():
    raw = b"ok\0 line\nr.GPUCrashDebugging=1\nbad \xff byte\nFatal error: x\n"
    lines = oc.clean_log(raw)
    assert lines[0] == "ok line"
    assert not any("GPUCrash" in ln for ln in lines)
    assert oc.crash_lines(lines) == ["Fatal error: x"]


def test_job_status():
    names = {"1_a.json", "2_b.running", "3_c.done", "4_d.failed", "5_e.crashed"}
    got = [oc.job_status(names, f"{i}_{c}") for i, c in enumerate("abcde", 1)]
    assert got == ["queued", "running", "done", "failed", "crashed"]
    assert oc.job_status(names, "9_z") is None


if __name__ == "__main__":
    for name, fn in list(globals().items()):
        if name.startswith("test_"):
            fn()
    print("ok")
