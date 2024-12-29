import os
import sys

def main():
    goroot = os.path.abspath(os.path.dirname(__file__))
    go_bin = os.path.join(goroot, "bin", "go")
    os.environ["GOROOT"] = goroot
    os.execv(go_bin, [go_bin] + sys.argv[1:])
