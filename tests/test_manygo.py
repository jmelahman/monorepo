import pytest
from manygo import get_platform_tag, GOOS, GOARCH

def test_get_platform_tag_darwin():
    assert get_platform_tag('darwin', 'amd64') == 'macosx_10_9_x86_64'
    assert get_platform_tag('darwin', 'arm64') == 'macosx_11_0_arm64'

def test_get_platform_tag_linux():
    assert get_platform_tag('linux', 'amd64') == 'manylinux_2_17_x86_64'
    assert get_platform_tag('linux', 'arm64') == 'manylinux_2_17_aarch64'
    assert get_platform_tag('linux', '386') == 'linux_i686'
    assert get_platform_tag('linux', 'arm') == 'linux_armv7l'

def test_get_platform_tag_windows():
    assert get_platform_tag('windows', 'amd64') == 'win_amd64'
    assert get_platform_tag('windows', '386') == 'win32'

def test_get_platform_tag_type_hints():
    # These should pass type checking
    def check_types(goos: GOOS, goarch: GOARCH) -> str:
        return get_platform_tag(goos, goarch)
    
    # Verify that only allowed types are accepted
    check_types('darwin', 'amd64')
    check_types('linux', 'arm64')
    check_types('windows', '386')

def test_get_platform_tag_invalid_inputs():
    # These should raise type errors at type checking time
    with pytest.raises(TypeError):
        get_platform_tag('invalid_os', 'amd64')  # type: ignore
    with pytest.raises(TypeError):
        get_platform_tag('darwin', 'invalid_arch')  # type: ignore
