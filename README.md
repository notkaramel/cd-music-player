# CD Music Player

- Runs on Raspberry Pi
- Uses `cdda2wav` on the player to play the music
- Has a web client to control playback on localhost
- Perfect for home server/self-hosted music player

## Client
- Simple webapp. Connects to the player via gRPC.

## Player (server-side)
- Has a gRPC server that listens to command sends from the client