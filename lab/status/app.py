#!/usr/bin/env python3.8
import base64
import datetime
import io
import logging
import os
import sqlite3
import statistics
import time

from apscheduler.schedulers.background import BackgroundScheduler
from apscheduler.triggers.interval import IntervalTrigger
from flask import Flask, render_template, current_app, redirect
import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import paramiko


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


def update_statuses():
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
        return True
    except Exception as e:
        current_app.logger.warning(f"SSH server is not running: {e}")
    return False


def plot(data, buffer):
    datetime_values = [row[0] for row in reversed(data)]
    value_values = [row[1] for row in reversed(data)]
    uptime_precent = statistics.mean(value_values) * 100

    # Create the plot
    fig, ax = plt.subplots()
    ax.plot(datetime_values, value_values, color="cyan")
    fig.set_facecolor("none")
    ax.set_facecolor("none")
    ax.tick_params(color="white", labelcolor="white")
    for spine in ax.spines.values():
        spine.set_edgecolor("white")
    plt.xticks(color="white")
    ax.text(x=0.0, y=1.7, s="% Uptime: {:.2f}".format(uptime_precent), color="white")
    ax.set_yticks([0, 1])
    ax.set_yticklabels(["Offline", "Online"])
    ax.set_ylim(bottom=-1.0, top=2.0)
    ax.set_xticks([datetime_values[0], datetime_values[-1]])
    ax.set_xticklabels([datetime_values[0], datetime_values[-1]])
    plt.savefig(buffer, format="png")
    plt.close()


@app.route("/")
def status():
    conn = sqlite3.connect("database.db")
    cursor = conn.cursor()
    try:
        cursor.execute(
            "SELECT datetime, status FROM statuses ORDER BY datetime DESC LIMIT 10000"
        )
    except sqlite3.OperationalError as e:
        current_app.logger.warning(e)
    ssh_statuses = cursor.fetchall()
    conn.close()

    ssh_plot_uri = None
    if ssh_statuses:
        buffer = io.BytesIO()
        plot(ssh_statuses, buffer)
        buffer.seek(0)
        ssh_plot_data = buffer.getvalue()
        ssh_plot_uri = base64.b64encode(ssh_plot_data).decode("utf-8")
        latest_status = ssh_statuses[0][1]
    else:
        current_app.logger.warning("Checking SSH Connectivity directly")
        latest_status = check_ssh_connection()

    return render_template(
        "index.html", ssh_status=latest_status, ssh_plot_uri=ssh_plot_uri
    )


@app.route("/", defaults={"path": ""})
@app.route("/<path:path>")
def catch_all(path):
    if path != "":
        return redirect("/")


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
