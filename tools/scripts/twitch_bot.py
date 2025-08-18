#!/usr/bin/env python3
# /// script
# requires-python = ">=3.12"
# dependencies = [
#     "selenium",
# ]
# ///
from __future__ import annotations

import argparse
import atexit
import shutil
import time
import typing

from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.common.keys import Keys
from selenium.webdriver.firefox.firefox_profile import FirefoxProfile
from selenium.webdriver.firefox.options import Options

PROFILE_PATH = "/home/jamison/.mozilla/firefox/jamison.default"


class Args(typing.NamedTuple):
    loop: bool
    message: str
    channel: str


def send_message(driver: webdriver.Firefox, message: str) -> None:
    chat_input = driver.find_element(By.CSS_SELECTOR, ".chat-wysiwyg-input__editor")
    chat_input.click()
    time.sleep(1)
    for ch in message:
        chat_input.send_keys(ch)
        time.sleep(0.1)
    chat_input.send_keys(Keys.ENTER)


def get_default_firefox_options() -> Options:
    options = Options()
    options.binary_location = shutil.which("firefox")  # type: ignore[invalid-assignment]
    options.set_preference("dom.webnotifications.enabled", value=False)
    options.profile = FirefoxProfile(PROFILE_PATH)
    return options


def parse_args() -> Args:
    parser = argparse.ArgumentParser()
    parser.add_argument("message", help="Message to send")
    parser.add_argument("--channel", help="Channel in which to send the message")
    parser.add_argument("--noloop", action="store_true", help="Exit after sending message")
    args = parser.parse_args()

    return Args(
        loop=not args.noloop,
        message=args.message,
        channel=args.channel,
    )


def main() -> int:
    args = parse_args()
    options = get_default_firefox_options()
    driver = webdriver.Firefox(options)
    atexit.register(driver.quit)

    driver.get(f"https://www.twitch.tv/{args.channel}")

    send_message(driver, args.message)
    while args.loop:
        send_message(driver, args.message)
        time.sleep(30)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
