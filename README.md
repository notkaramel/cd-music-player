# CD Music Player

- Runs on Raspberry Pi
- Uses `cdda2wav` on the player to play the music
- Has a web client to control playback on localhost
- Perfect for home server/self-hosted music player

## Working with .proto files
```sh
git submodule add -b protofiles git@github.com:notkaramel/cd-music-player player/protofiles
git submodule add -b protofiles git@github.com:notkaramel/cd-music-player client/protofiles

# Sync
git submodule foreach git pull origin protofiles
```

## Client
- Simple webapp. Connects to the player via gRPC-web
- Docs:
    - Marko (v6): https://markojs.com
    - gRPC-web
    - TailwindCSS (v4.1): https://tailwindcss.com/docs/styling-with-utility-classes

## Player (server-side)
- Has a gRPC server that listens to command sends from the client
- Packages: cdparanoia (for cdda2wav), grpc-go
- Docs:
    - gRPC with Go: https://grpc.io/docs/languages/go/quickstart/
    - cdda2wav: https://manpages.debian.org/stretch/cdparanoia/cdda2wav.1.en.html