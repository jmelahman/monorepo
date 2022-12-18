#!/usr/bin/env python3.8
import base64
import datetime
import io
import logging
import os
import sqlite3
import time
import threading

from flask import Flask, render_template, current_app, redirect
import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
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


def maybe_create_statuses_table(cursor) -> None:
    cursor.execute(
        """CREATE TABLE IF NOT EXISTS statuses (
                        id INTEGER PRIMARY KEY,
                        datetime TIME NOT NULL,
                        status BOOLEAN NOT NULL
                    )"""
    )


def update_statuses():
    conn = sqlite3.connect("database.db")
    cursor = conn.cursor()
    maybe_create_statuses_table(cursor)
    while True:
        ssh_status = check_ssh_connection()
        current_time = datetime.datetime.now()
        datetime_string = current_time.strftime("%Y-%m-%d %H:%M:%S")
        cursor.execute(
            "INSERT INTO statuses (datetime, status) VALUES (?, ?)",
            (datetime_string, ssh_status),
        )
        conn.commit()
        time.sleep(60)


def plot(data, buffer):
    datetime_values = [row[0] for row in reversed(data)]
    value_values = [row[1] for row in reversed(data)]

    # Create the plot
    fig, ax = plt.subplots()
    ax.plot(datetime_values, value_values, color="blue")
    fig.set_facecolor("none")
    ax.set_facecolor("none")
    ax.tick_params(color="white", labelcolor="white")
    for spine in ax.spines.values():
        spine.set_edgecolor("white")
    plt.xticks(color="white")
    ax.set_yticks([0, 1])
    ax.set_yticklabels(["Offline", "Online"])
    ax.set_xticks([datetime_values[0], datetime_values[-1]])
    ax.set_xticklabels([datetime_values[0], datetime_values[-1]])
    plt.savefig(buffer, format="png")


@app.route("/")
def status():
    conn = sqlite3.connect("database.db")
    cursor = conn.cursor()
    maybe_create_statuses_table(cursor)
    cursor.execute(
        "SELECT datetime, status FROM statuses ORDER BY datetime DESC LIMIT 1000"
    )
    ssh_statuses = cursor.fetchall()
    conn.close()

    buffer = io.BytesIO()
    plot(ssh_statuses, buffer)
    buffer.seek(0)
    ssh_plot_data = buffer.getvalue()
    ssh_plot_uri = base64.b64encode(ssh_plot_data).decode("utf-8")

    return render_template(
        "index.html", ssh_status=ssh_statuses[0][1], ssh_plot_uri=ssh_plot_uri
    )


@app.route("/", defaults={"path": ""})
@app.route("/<path:path>")
def catch_all(path):
    if path != "":
        return redirect("/")


if __name__ == "__main__":
    configure_logger(app)
    thread = threading.Thread(target=update_statuses)
    thread.start()
    app.run()
