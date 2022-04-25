# python-snapify

An executable to check if any packages installed with the host's package manager can be installed
as a [snap](https://snapcraft.io/) package.


## Install

Snapify is available as a [pypi package](https://pypi.org/project/python-snapify/).

```shell
pip install python-snapify
```

## Build

```shell
python setup.py sdist
```

### Deploy

```
twine upload dist/*
```
