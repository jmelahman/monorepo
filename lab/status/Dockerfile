FROM python:3.8-slim

EXPOSE 5000

WORKDIR /app

RUN pip install --upgrade pip
RUN pip install gunicorn==20.1.0 flask==2.2.2 paramiko==2.12.0 matplotlib==3.4.3

COPY . /app

CMD ["gunicorn", "app:app", "--bind", "0.0.0.0:5000"]
