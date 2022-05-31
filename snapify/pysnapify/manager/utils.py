import shutil


def get_executable(bin_name: str) -> str:
    executable = shutil.which(bin_name)
    assert isinstance(executable, str)
    return executable
