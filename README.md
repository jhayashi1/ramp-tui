# ascii-tui

Convert GIFs into colorized ASCII animations and play them in the terminal.

Animations are pre-rendered once into a portable `.frames` file, so playback
is instant and works on any machine with a truecolor terminal, no conversion
step required.

## Install

```sh
go install github.com/jhayashi1/ascii-tui@latest
```

## Usage

```sh
# Open the interactive gallery of rendered animations
ascii-tui

# Render a gif into the library and loop it
ascii-tui path/to/animation.gif

# Pre-render explicitly (writes to the library by default)
ascii-tui render animation.gif
ascii-tui render animation.gif -o animation.frames -W 120

# Play a frames file (or a gif, rendered on the fly)
ascii-tui play animation.frames
ascii-tui play animation.frames --once --speed 2
```

In the gallery: `enter` plays the selected animation, `a` renders a new gif,
`d` deletes an entry, `/` filters. In the player: `space` pauses, `left`/`right`
switch between animations, `esc` returns to the gallery.

The `a` prompt has fzf-style fuzzy completion: as you type a path it lists
matching directories and `.gif` files, `tab` completes the selection,
`up`/`down` (or `ctrl+p`/`ctrl+n`) move through the matches, and `enter`
descends into a directory or renders the chosen gif. Paths starting with `~`
expand to your home directory, both here and in CLI arguments.

### Cross-machine playback

`.frames` files are self-contained (frames, delays, dimensions, gzipped).
Render on one machine, copy the file anywhere, and `ascii-tui play` it there:

```sh
ascii-tui render giphy.gif -o giphy.frames
scp giphy.frames other-machine:
ssh other-machine ascii-tui play giphy.frames
```

The library lives in the platform cache directory
(`~/.cache/ascii-tui/library` on Linux, `%LOCALAPPDATA%\ascii-tui\library`
on Windows).

## Development

Requires Go (see `go.mod`), [Task](https://taskfile.dev), and
[golangci-lint](https://golangci-lint.run).

```sh
task check   # format, lint, test
task build   # build into bin/
task run -- play testdata/giphy.gif
```

## How it works

1. `image/gif` decodes all frames; partial frames are composited onto a
   persistent canvas honoring GIF disposal methods.
2. Each frame is downscaled to the target character grid with CatmullRom
   resampling (`golang.org/x/image/draw`), at half vertical resolution to
   match terminal cell aspect.
3. Each cell maps luminance to a character ramp and pixel color to a 24-bit
   ANSI foreground escape, coalescing consecutive same-color runs.
4. Frames and per-frame delays are stored as gzipped gob with a versioned
   header, then played back with alt-screen cursor-home repositioning.

The character-ramp approach was inspired by
[TheZoraiz/ascii-image-converter](https://github.com/TheZoraiz/ascii-image-converter).
