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
    kick: bool
    dry_run: bool
    period: int
    message: str
    channel: str


def send_message(
    driver: webdriver.Firefox, message: str, *_: typing.Any, kick: bool, dry_run: bool
) -> None:
    chat_input = driver.find_element(
        By.CSS_SELECTOR, ".editor-input" if kick else ".chat-wysiwyg-input__editor"
    )
    chat_input.click()
    time.sleep(1)
    is_emoji = False
    for ch in message:
        if ch == ":" and is_emoji:
            chat_input.send_keys(Keys.TAB)
        else:
            chat_input.send_keys(ch)
        time.sleep(0.1)
        if ch == ":":
            is_emoji = not is_emoji
    if not dry_run:
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
    parser.add_argument(
        "--period", type=int, default=60, help="Frequency at which to send messages"
    )
    parser.add_argument("--kick", action="store_true", help="Send message on kick")
    parser.add_argument("--dry-run", action="store_true", help="Don't send the message")
    parser.add_argument("--noloop", action="store_true", help="Exit after sending message")
    args = parser.parse_args()

    return Args(
        loop=not args.noloop,
        kick=args.kick,
        dry_run=args.dry_run,
        period=args.period,
        message=args.message,
        channel=args.channel,
    )


def main() -> int:
    args = parse_args()
    options = get_default_firefox_options()
    driver = webdriver.Firefox(options)
    atexit.register(driver.quit)

    driver.get(f"https://www.{'kick.com' if args.kick else 'twitch.tv'}/{args.channel}")

    send_message(driver, args.message, kick=args.kick, dry_run=args.dry_run)
    while args.loop:
        for i in range(args.period):
            print(f"\r{i:02}", end="", flush=True)
            time.sleep(1)
        send_message(driver, args.message, kick=args.kick, dry_run=args.dry_run)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
