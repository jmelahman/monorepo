from __future__ import annotations

import logging
import unittest

import freezegun

from buildprint import _logging


class TestLogger(unittest.TestCase):
    def test_logger_levels(self) -> None:
        """Confirm for each level, only the desired levels print log messages.

        For some reason, using 'with self.assertLogs()' doesn't initialize the custom formatter.
        Actual logs as they would appear are printed to stderr at runtime.
        """
        for level in _logging.LogLevel:
            with self.subTest(level=level):
                logger = _logging.getLogger(level.value + "_logger", level)
                numeric_level = logging.getLevelName(level.value)
                self.assertEqual(logger.level, numeric_level)
                for loglevel in _logging.LogLevel:
                    numeric_loglevel = logging.getLevelName(loglevel.value)
                    logger.log(numeric_loglevel, "LEVEL_" + level.value)
                    if numeric_level > numeric_loglevel:
                        continue
                    with self.assertLogs(logger, level=level.value) as cm:
                        logger.log(numeric_loglevel, "LEVEL_" + level.value)

    @freezegun.freeze_time("2020-01-01 12:34:56")
    def test_logger_timestamps(self) -> None:
        """Instantiating a logger with 'timestamps=True' prepends the log with a timestamp.

        For some reason, using 'with self.assertLogs()' doesn't initialize the custom formatter.
        Actual logs as they would appear are printed to stderr at runtime.
        """
        logger = _logging.getLogger("with_timestamps", timestamps=True)
        logger.info("info")
        with self.assertLogs(logger, level="INFO") as cm:
            logger.info("info")

    def test_logger_getInstance(self) -> None:
        """getLogger should return the same logger instance if it already exists."""
        test_logger = _logging.getLogger("test_logger")
        test_logger_2 = _logging.getLogger("test_logger")
        self.assertEqual(test_logger, test_logger_2)


if __name__ == "__main__":
    unittest.main()
