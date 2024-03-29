from __future__ import annotations


class PyBazelError(Exception):
    """A base class from which all other exceptions inherit.

    If you want to catch all errors that PyBazel might raise,
    catch this base exception.
    """
