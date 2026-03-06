FROM mcr.microsoft.com/devcontainers/base:ubuntu-22.04

RUN apt-get update && apt-get install -y \
    postgresql-client \
    curl \
    jq \
    && rm -rf /var/lib/apt/lists/*
