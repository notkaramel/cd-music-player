`mpv` can communicate via IPC JSON. What we can do is:
- Have an instance of mpv running in the background, and continously send updates to the IPC server
- The "client", either a TUI or a webapp, can use those information to parse and display the progress

---

cdda2wav is not a good choice since it was made for burning and extraction. Alternatives that are made for playback include: (prompted to ChatGPT)

| Goal                                      | Best Option       |
| ----------------------------------------- | ----------------- |
| Full control, accurate track/gap handling | libcdio           |
| Fast development, playback only           | libVLC            |
| Rust project                              | GStreamer         |
| Minimal architecture                      | mpv IPC           |
| Classic hardware-style control            | Linux CDROM ioctl |

`mpv` seems to work really well and has its TUI keybinds (`?` to see the menu)
```sh
mpv cdda:// --cdda-device=/dev/sr0 
```

---

Before knowing how to play CD via the command line, I use a command suggested by ChatGPT to play my CD:

```sh
alias playcd='cdda2wav -D /dev/sr0 -t 1+ -B - | pw-play --rate 44100 --quality 15 -'
```

Explaining the options:
- `cdda2wav`: converts cd info to wav format. This is usually used to get tracks to local disk
- `-D /dev/sr0`: the disk
- `-t 1+`: track, from first track to the end. 
- `-` : destination of extraction, aka, pipe directly to player
- `pw-play`: Pipewire music player
- `--rate 44100 --quality 15`: Sample rate and quaility of the audio output
- `-`: the data from previous command

It works, but very limited and doesn't allow me to do anything other than listen through the whole CD one song at a time.


