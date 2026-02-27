#!/bin/sh
# Poll for CD in CD_DEVICE; when present, run mpv cdda://. When mpv exits, go back to polling.
set -e
CD_DEV="${CD_DEVICE:-/dev/sr0}"
SOCKET="${MPV_SOCKET:-/tmp/mpvsocket}"
POLL_INTERVAL="${POLL_INTERVAL:-3}"

while true; do
	# Detect disc: read one CD sector (2048 bytes). Timeout 2s so we don't block on empty drive.
	if n=$(timeout 2 dd if="$CD_DEV" bs=2048 count=1 2>/dev/null | wc -c) && [ "$n" -ge 2048 ]; then
		mpv cdda:// --cdda-device="$CD_DEV" --input-ipc-server="$SOCKET" --no-terminal || true
	fi
	sleep "$POLL_INTERVAL"
done
