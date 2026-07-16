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

# Drop a solid background so it plays as blank terminal space
ascii-tui render animation.gif --filter-bg

# Play a frames file (or a gif, rendered on the fly)
ascii-tui play animation.frames
ascii-tui play animation.frames --once --speed 2
```

The gallery shows a library panel next to a live preview of the selected
entry (name, frame count, and dimensions); on narrow terminals (under 56
columns or 12 rows) the preview drops and the library panel takes the full
width. Press `?` on either screen for a full key reference; any key closes
it.

> [!NOTE]
> If you're used to the old `left`/`right` switching between animations:
> that's now `n`/`p`. `left`/`h` and `right`/`l` seek within the current
> animation instead, matching mpv/vlc.

**Gallery**

| Key | Action |
|---|---|
| `enter` | play the selected animation |
| `a` | render a new gif into the library |
| `r` | rename the selected entry |
| `d` | delete, with a `y`/`n` confirmation |
| `/` | filter the list |
| `?` | show all key bindings |
| `q` / `ctrl+c` | quit |

**Player**

| Key | Action |
|---|---|
| `space` | pause / resume |
| `left`/`h`, `right`/`l` | seek back/forward 1 second |
| `,` / `.` | step one frame back/forward (pauses) |
| `+`/`=`, `-` | speed up / down (x1.25 steps, 0.25x-8x) |
| `n` / `p` | next / previous animation |
| `f` | toggle background filtering (re-rendered and saved) |
| `?` | show all key bindings |
| `esc` | back to the gallery |
| `q` / `ctrl+c` | quit |

The `a` prompt has fzf-style fuzzy search: it recursively finds `.gif` files
under the directory portion of the typed path (a few levels deep) and filters
them as you type. `tab` completes the selection, `up`/`down` (or
`ctrl+p`/`ctrl+n`) move through the matches, and `enter` renders the chosen
gif. Paths starting with `~` expand to your home directory, both here and in
CLI arguments.

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

## Configuration

An optional TOML file at `<user config dir>/ascii-tui/config.toml`
(`~/.config/ascii-tui/config.toml` on Linux,
`%AppData%\ascii-tui\config.toml` on Windows) sets defaults for playback
speed, new-render options, and the theme. It's entirely optional: a missing
file uses the built-in defaults, and any field you omit keeps its default
too. A file that fails to parse prints a warning and falls back to defaults
rather than stopping the program.

```toml
[playback]
speed = 1.0                 # initial player speed, 0.25-8.0

[render]
filter_background = false   # drop a detected solid background by default
complex = false              # use a denser character ramp by default

[theme]
accent = "212"  # selection marker, borders' title, progress bar
border = "240"  # panel borders
text   = "252"  # normal text
dim    = "241"  # help text, secondary labels
error  = "203"  # error/warning text
```

Theme values accept anything `lipgloss.Color` does: a 256-color index (as
above), a hex code like `"#ff6ac1"`, or an ANSI name.

## Development

Requires Go (see `go.mod`), [Task](https://taskfile.dev), and
[golangci-lint](https://golangci-lint.run).

```sh
task check          # format, lint, test
task build          # build into bin/
task run -- play testdata/giphy.gif
task hooks:install  # run lint before each commit
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
