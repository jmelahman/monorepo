#!/usr/bin/env python3.8
from __future__ import annotations

import logging
import os
from typing import TYPE_CHECKING

# Ignore logging on tf import.
os.environ["TF_CPP_MIN_LOG_LEVEL"] = "2"
# os.environ["PYTORCH_CUDA_ALLOC_CONF"] = "max_split_size_mb=500"

import torch
from flask import Flask, redirect, render_template, request, send_file
from transformers import GPT2LMHeadModel, GPT2Tokenizer  # type: ignore[import]

if TYPE_CHECKING:
    from flask.wrappers import Response as FlaskResponse
    from werkzeug.wrappers import Response

app = Flask(__name__)

device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
model_name = "gpt2-large"
tokenizer = GPT2Tokenizer.from_pretrained(model_name, do_lower_case=False)
# special_words_to_add={"additional_special_tokens": ["<python>", "<java>"]}
# tokenizer.add_special_tokens(special_words_to_add)
model = GPT2LMHeadModel.from_pretrained(model_name, pad_token_id=tokenizer.eos_token_id)
model.to(device)


def configure_logger(app: Flask) -> None:
    handler = app.logger.handlers[0]

    log_level = logging.WARNING
    app.logger.setLevel(log_level)
    handler.setLevel(log_level)

    formatter = logging.Formatter(
        "%(asctime)s - %(name)s - %(levelname)s - %(message)s"
    )
    handler.setFormatter(formatter)


def generate_text(prompt: str) -> str:
    encoded_prompt = torch.tensor(tokenizer.encode(prompt, return_tensors="pt")).to(
        device
    )
    output = model.generate(
        encoded_prompt,
        max_length=1024,
        temperature=0.1,
        top_k=50,
        top_p=0.92,
        repetition_penalty=1.5,
        do_sample=True,
    )
    decoded_output = tokenizer.decode(output[0], skip_special_tokens=True)
    assert isinstance(decoded_output, str)
    return decoded_output


@app.route("/")
def status() -> str:
    rendered_template = render_template("index.html")
    assert isinstance(rendered_template, str)
    return rendered_template


@app.route("/update")
def update() -> str:
    user_input = request.args.get("text", "")
    assert isinstance(user_input, str)
    return generate_text(user_input)


@app.route("/", defaults={"path": ""})
@app.route("/<path:path>")
def catch_all(path: str) -> str | Response | FlaskResponse:
    if path == "robots.txt":
        return send_file("robots.txt")
    elif path != "":
        return redirect("/")
    return status()


configure_logger(app)

if __name__ == "__main__":
    app.run()
