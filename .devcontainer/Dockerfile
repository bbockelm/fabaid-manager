FROM mcr.microsoft.com/devcontainers/base:ubuntu-22.04

# Install PostgreSQL client 17 (matches the db service) from the official PGDG
# apt repo. Ubuntu 22.04's default postgresql-client is v14, whose pg_dump
# refuses to dump a 16/17 server ("server version mismatch"), which breaks the
# app's backup feature in the devcontainer.
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates curl gnupg jq \
    && install -d /usr/share/postgresql-common/pgdg \
    && curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc \
        -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc \
    && echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] https://apt.postgresql.org/pub/repos/apt jammy-pgdg main" \
        > /etc/apt/sources.list.d/pgdg.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends postgresql-client-17 \
    && rm -rf /var/lib/apt/lists/*
