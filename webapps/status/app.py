#!/usr/bin/env python3.8
from __future__ import annotations

import base64
import datetime
import io
import logging
import os
import sqlite3
import statistics
from typing import TYPE_CHECKING

from apscheduler.schedulers.background import BackgroundScheduler  # type: ignore[import]
from apscheduler.triggers.interval import IntervalTrigger  # type: ignore[import]
from flask import current_app
from flask import Flask
from flask import redirect
from flask import render_template
from flask import send_file
import matplotlib  # type: ignore[import]

matplotlib.use("Agg")
import matplotlib.pyplot as plt  # type: ignore[import]
import paramiko

if TYPE_CHECKING:
    from werkzeug.wrappers import Response

app = Flask(__name__)
scheduler = BackgroundScheduler()


def init_statuses_table() -> None:
    conn = sqlite3.connect("database.db")
    cursor = conn.cursor()
    cursor.execute(
        """CREATE TABLE IF NOT EXISTS statuses (
                        id INTEGER PRIMARY KEY,
                        datetime TIME NOT NULL,
                        status BOOLEAN NOT NULL
                    )"""
    )
    conn.close()


def update_statuses() -> None:
    with app.app_context():
        current_app.logger.info("Updating SSH connectivity status")
    ssh_status = check_ssh_connection()
    current_time = datetime.datetime.now()
    datetime_string = current_time.strftime("%Y-%m-%d %H:%M:%S")
    conn = sqlite3.connect("database.db")
    cursor = conn.cursor()
    cursor.execute(
        "INSERT INTO statuses (datetime, status) VALUES (?, ?)",
        (datetime_string, ssh_status),
    )
    conn.commit()
    conn.close()


def configure_logger(app: Flask) -> None:
    handler = app.logger.handlers[0]

    log_level = logging.WARNING
    app.logger.setLevel(log_level)
    handler.setLevel(log_level)

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
        client.close()
        return True
    except Exception as e:
        with app.app_context():
            current_app.logger.warning(f"SSH server is not running: {e}")
    return False


def plot(
    x_values: list[str], y_values: list[str], uptime_percent: float, buffer: io.BytesIO
) -> None:

    # Create the plot
    fig, ax = plt.subplots()
    ax.plot(x_values, y_values, color="cyan")
    fig.set_facecolor("none")
    ax.set_facecolor("none")
    ax.tick_params(color="white", labelcolor="white")
    for spine in ax.spines.values():
        spine.set_edgecolor("white")
    plt.xticks(color="white")
    ax.text(x=0.0, y=1.7, s="% Uptime: {:.2f}".format(uptime_percent), color="white")
    ax.set_yticks([0, 1])
    ax.set_yticklabels(["Offline", "Online"])
    ax.set_ylim(bottom=-1.0, top=2.0)
    ax.set_xticks([x_values[0], x_values[-1]])
    ax.set_xticklabels([x_values[0], x_values[-1]])
    plt.savefig(buffer, format="png")
    plt.close()


@app.route("/")
def status() -> str:
    conn = sqlite3.connect("database.db")
    cursor = conn.cursor()
    try:
        cursor.execute(
            "SELECT datetime, status FROM statuses ORDER BY datetime DESC LIMIT 720"
        )
    except sqlite3.OperationalError as e:
        current_app.logger.warning(e)
    ssh_statuses = cursor.fetchall()
    conn.close()

    ssh_plot_uri = None
    if ssh_statuses:
        buffer = io.BytesIO()
        datetime_values = [row[0] for row in reversed(ssh_statuses)]
        status_values = [row[1] for row in reversed(ssh_statuses)]
        uptime_percent = statistics.mean(status_values) * 100
        plot(datetime_values, status_values, uptime_percent, buffer)
        buffer.seek(0)
        ssh_plot_data = buffer.getvalue()
        ssh_plot_uri = base64.b64encode(ssh_plot_data).decode("utf-8")
        latest_status = ssh_statuses[0][1]
    else:
        current_app.logger.warning("Checking SSH Connectivity directly")
        latest_status = check_ssh_connection()
        uptime_percent = latest_status

    return render_template(
        "index.html",
        ssh_status=latest_status,
        ssh_plot_uri=ssh_plot_uri,
        uptime_percent=uptime_percent,
    )


@app.route("/", defaults={"path": ""})
@app.route("/<path:path>")
def catch_all(path: str) -> str | Response:
    if path == "robots.txt":
        return send_file("robots.txt")
    elif path != "":
        return redirect("/")
    return status()


configure_logger(app)
init_statuses_table()
scheduler.add_job(
    func=update_statuses,
    trigger=IntervalTrigger(seconds=5),
    id="update_statuses",
    name="Call update_statuses every 5 seconds",
    replace_existing=True,
)
scheduler.start()


if __name__ == "__main__":
    app.run()
