#!/usr/bin/env python3.8
import logging
import os

# Ignore logging on tf import.
os.environ["TF_CPP_MIN_LOG_LEVEL"] = "2"

from flask import Flask, render_template, redirect, send_file, request
from transformers import GPT2Tokenizer, GPT2LMHeadModel

app = Flask(__name__)

model_name = "gpt2-xl"
tokenizer = GPT2Tokenizer.from_pretrained(model_name)
model = GPT2LMHeadModel.from_pretrained(model_name, pad_token_id=tokenizer.eos_token_id)


def configure_logger(app: Flask) -> None:
    handler = app.logger.handlers[0]

    log_level = logging.WARNING
    app.logger.setLevel(log_level)
    handler.setLevel(log_level)

    formatter = logging.Formatter(
        "%(asctime)s - %(name)s - %(levelname)s - %(message)s"
    )
    handler.setFormatter(formatter)


def generate_text(prompt):
    encoded_prompt = tokenizer.encode(prompt, return_tensors="pt")
    output = model.generate(
        encoded_prompt,
        max_length=1024,
        temperature=0.2,
        top_k=50,
        top_p=0.92,
        repetition_penalty=1.5,
        do_sample=True,
    )
    decoded_output = tokenizer.decode(output[0], skip_special_tokens=True)
    return decoded_output


@app.route("/")
def status():
    return render_template("index.html")


@app.route("/update")
def update():
    user_input = request.args.get("text")
    return generate_text(user_input)


@app.route("/", defaults={"path": ""})
@app.route("/<path:path>")
def catch_all(path):
    if path == "robots.txt":
        return send_file("robots.txt")
    elif path != "":
        return redirect("/")


configure_logger(app)

if __name__ == "__main__":
    app.run()
