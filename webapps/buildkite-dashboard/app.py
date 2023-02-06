#!/usr/bin/env python3.8
from __future__ import annotations

import collections
import logging
import os
from typing import TYPE_CHECKING, TypedDict

from apscheduler.schedulers.background import BackgroundScheduler  # type: ignore[import]
from apscheduler.triggers.interval import IntervalTrigger  # type: ignore[import]
from flask import Flask
from flask import request
from flask import redirect
from flask import render_template
from flask import send_file
import requests

if TYPE_CHECKING:
    from werkzeug.wrappers import Response

app = Flask(__name__)
scheduler = BackgroundScheduler()


_BUILDKITE_AGENT_TOKEN = os.environ["BUILDKITE_AGENT_TOKEN"]
_UPDATE_INTERVAL = 15  # seconds
_DATA_LIFESPAN = 3600  # seconds (60 min)
_QUEUE_LEN = _DATA_LIFESPAN // _UPDATE_INTERVAL
_TOTAL_QUEUE = "total"
_JOBS_QUEUE: dict[str, collections.deque[float]] = {
    _TOTAL_QUEUE: collections.deque(maxlen=_QUEUE_LEN)
}
_AGENTS_QUEUE: dict[str, collections.deque[float]] = {
    _TOTAL_QUEUE: collections.deque(maxlen=_QUEUE_LEN)
}


class AgentsQueueStatus(TypedDict):
    idle: int
    busy: int
    total: int


class AgentsData(TypedDict):
    idle: int
    busy: int
    total: int
    queues: dict[str, AgentsQueueStatus]


class JobsQueueState(TypedDict):
    scheduled: int
    running: int
    waiting: int
    total: int


class JobsData(TypedDict):
    scheduled: int
    running: int
    waiting: int
    total: int
    queues: dict[str, JobsQueueState]


class OrganizationData(TypedDict):
    slug: str


class MetricsData(TypedDict):
    agents: AgentsData
    jobs: JobsData
    organization: OrganizationData


def _get_metrics_data() -> MetricsData:
    resp = requests.get(
        "https://agent.buildkite.com/v3/metrics",
        headers={
            "Authorization": f"Token {_BUILDKITE_AGENT_TOKEN}",
            "Content-Type": "application/json",
        },
        timeout=60,
    )
    resp.raise_for_status()
    return MetricsData(resp.json())  # type: ignore # TODO


def update() -> None:
    metrics_data = _get_metrics_data()
    total_agent_utilization = (
        metrics_data["agents"]["busy"] / metrics_data["agents"]["total"]
        if metrics_data["agents"]["total"]
        else 0
    )
    _AGENTS_QUEUE[_TOTAL_QUEUE].append(total_agent_utilization)
    total_percent_scheduled = (
        metrics_data["jobs"]["scheduled"] / metrics_data["jobs"]["total"]
        if metrics_data["jobs"]["total"]
        else 0
    )
    _JOBS_QUEUE[_TOTAL_QUEUE].append(total_percent_scheduled)
    for queue, agent_statuses in metrics_data["agents"]["queues"].items():
        if not _AGENTS_QUEUE.get(queue):
            _AGENTS_QUEUE[queue] = collections.deque(maxlen=_QUEUE_LEN)
        queue_utilization = (
            agent_statuses["busy"] / agent_statuses["total"]
            if agent_statuses["total"]
            else 0
        )
        _AGENTS_QUEUE[queue].append(queue_utilization)
    for queue, job_statuses in metrics_data["jobs"]["queues"].items():
        if not _JOBS_QUEUE.get(queue):
            _JOBS_QUEUE[queue] = collections.deque(maxlen=_QUEUE_LEN)
        percent_scheduled = (
            job_statuses["scheduled"] / job_statuses["total"]
            if job_statuses["total"]
            else 0
        )
        _JOBS_QUEUE[queue].append(percent_scheduled)
    for queue in list(_AGENTS_QUEUE):
        if queue != _TOTAL_QUEUE and queue not in metrics_data["agents"]["queues"]:
            del _AGENTS_QUEUE[queue]
    for queue in list(_JOBS_QUEUE):
        if queue != _TOTAL_QUEUE and queue not in metrics_data["jobs"]["queues"]:
            del _JOBS_QUEUE[queue]


def configure_logger(app: Flask) -> None:
    handler = app.logger.handlers[0]

    log_level = logging.WARNING
    app.logger.setLevel(log_level)
    handler.setLevel(log_level)

    formatter = logging.Formatter(
        "%(asctime)s - %(name)s - %(levelname)s - %(message)s"
    )
    handler.setFormatter(formatter)


@app.route("/")
def status() -> str:
    return render_template(
        "index.html", queues=set(list(_AGENTS_QUEUE) + list(_JOBS_QUEUE))
    )


@app.route("/data", methods=["GET"])
def data() -> str:
    queue = request.args.get("queue", _TOTAL_QUEUE)
    return (
        " ".join([str(value) for value in _JOBS_QUEUE.get(queue, [])])
        + ","
        + " ".join([str(value) for value in _AGENTS_QUEUE.get(queue, [])])
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
scheduler.add_job(
    func=update,
    trigger=IntervalTrigger(seconds=_UPDATE_INTERVAL),
    id="update",
    name="Update every 5 seconds",
    replace_existing=True,
)
scheduler.start()


if __name__ == "__main__":
    app.run(debug=True)
