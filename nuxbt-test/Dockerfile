FROM node:24-bookworm AS frontend
WORKDIR /build/nuxbt/web/client
COPY ref/nuxbt/nuxbt/web/client/ ./
RUN npm install && npm run build

FROM python:3.13-trixie

RUN apt-get update && apt-get install -y --no-install-recommends \
    bluez \
    dbus \
    libdbus-1-dev \
    libdbus-glib-1-dev \
    libgirepository-2.0-dev \
    libcairo2-dev \
    pkg-config \
    gcc \
    python3-dev \
    rfkill \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY ref/nuxbt/ ./

COPY --from=frontend /build/nuxbt/web/static/dist/ ./nuxbt/web/static/dist/
COPY --from=frontend /build/nuxbt/web/static/assets/ ./nuxbt/web/static/assets/
COPY --from=frontend /build/nuxbt/web/templates/ ./nuxbt/web/templates/

RUN pip install --no-cache-dir poetry \
    && poetry config virtualenvs.create false \
    && poetry install --no-interaction --no-ansi --no-root \
    && pip install --no-cache-dir -e .

COPY nuxbt-test/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 8000

ENTRYPOINT ["/entrypoint.sh"]
CMD ["nuxbt", "webapp", "--port", "8000"]
