# mpv container: detects CD insertion and runs mpv cdda:// with IPC socket for the TUI.
# Expects: --device /dev/sr0 (or CD_DEVICE), volume for socket dir (e.g. -v mpvsocket:/tmp).
# docker-compose sets CD_DEVICE and MPV_SOCKET via environment.
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    mpv \
    coreutils \
    && rm -rf /var/lib/apt/lists/*

ENV CD_DEVICE=/dev/sr0 \
    MPV_SOCKET=/tmp/mpvsocket

COPY mpv-entrypoint.sh /mpv-entrypoint.sh
RUN chmod +x /mpv-entrypoint.sh

ENTRYPOINT ["/mpv-entrypoint.sh"]
