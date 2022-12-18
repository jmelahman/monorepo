#!/usr/bin/env python3.8
import logging
import os

from flask import Flask, render_template, current_app
import paramiko

app = Flask(__name__)


def configure_logger(app: Flask) -> None:
    handler = app.logger.handlers[0]

    handler.setLevel(logging.WARNING)

    formatter = logging.Formatter(
        "%(asctime)s - %(name)s - %(levelname)s - %(message)s"
    )
    handler.setFormatter(formatter)


def check_ssh_connection() -> bool:
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    private_key_path = os.path.expanduser(os.path.join("~", ".ssh", "id_rsa"))
    private_key = paramiko.RSAKey.from_private_key_file(private_key_path)
    try:
        client.connect("lahman.dev", port=23, username="jamison", pkey=private_key)
        return True
    except Exception as e:
        current_app.logger.warning(f"SSH server is not running: {e}")
    return False


@app.route("/")
def status():
    ssh_status = "online" if check_ssh_connection() else "offline"
    return render_template("index.html", ssh_status=ssh_status)


if __name__ == "__main__":
    configure_logger(app)
    app.run()
