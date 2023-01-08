#!/usr/bin/env python3.8
import logging
import os

# Ignore logging on tf import.
os.environ["TF_CPP_MIN_LOG_LEVEL"] = "2"
# os.environ["PYTORCH_CUDA_ALLOC_CONF"] = "max_split_size_mb=500"

import torch
from flask import Flask, redirect, render_template, request, send_file
from transformers import GPT2LMHeadModel, GPT2Tokenizer

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


def generate_text(prompt):
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
    import pprint

    pprint.pprint(decoded_output)
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
