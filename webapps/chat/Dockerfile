FROM python:3.8-slim as prebuild

RUN apt-get update && apt-get install -y \
  gcc

RUN pip install --upgrade pip
RUN pip install --user gunicorn[gevent]==20.1.0 flask==2.2.2

FROM python:3.8-slim

COPY --from=prebuild /root/.local /root/.local

WORKDIR /app

COPY . /app

EXPOSE 5000

ENV PATH=/root/.local/bin:$PATH

CMD ["gunicorn", "app:app", "--bind", "0.0.0.0:5000", "--preload", "-k", "gevent", "--worker-connections", "1000"]
