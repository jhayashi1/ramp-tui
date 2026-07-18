# ramp

Convert GIFs into colorized ASCII animations and play them in the terminal.

Animations are pre-rendered once into a portable `.frames` file, so playback
is instant and works on any machine with a truecolor terminal, no conversion
step required.

<table align="center">
  <tr>
    <th>Original</th>
    <th>ramp</th>
  </tr>
  <tr>
    <td><img src="gifs/duck.gif" height="220" alt="original duck GIF" /></td>
    <td><img src="demo/duck.gif" height="220" alt="ramp playing the duck animation" /></td>
  </tr>
  <tr>
    <td><img src="gifs/deer.gif" height="220" alt="original deer GIF" /></td>
    <td><img src="demo/deer.gif" height="220" alt="ramp playing the deer animation" /></td>
  </tr>
</table>
<p align="center"><sub>Demo source GIFs: <a href="https://giphy.com/gifs/hamlet-duck-dance-PQHZH0iHMCmrNhRcE9">duck</a> · <a href="https://giphy.com/gifs/loop-stag-l1KdbPzEUJ5ife8Ra">deer</a>, via Giphy</sub></p>

## Install

```sh
go install github.com/jhayashi1/ramp-tui/cmd/ramp@latest
```

## Usage

```sh
# Open the interactive gallery of rendered animations
ramp

# Render a gif into the library and loop it
ramp path/to/animation.gif

# Pre-render explicitly (writes to the library by default)
ramp render animation.gif
ramp render animation.gif -o animation.frames -W 120

# Drop a solid background so it plays as blank terminal space
ramp render animation.gif --filter-bg

# Play a frames file (or a gif, rendered on the fly)
ramp play animation.frames
ramp play animation.frames --once --speed 2
```

The gallery is a three-column layout: the library list on the left, a live
preview of the selected entry in the middle (with a header line showing its
name and the ramp wordmark), and a detail column on the right with the full
metadata (dimensions, length, source, file path, size, modified time). On
narrower terminals the detail column drops first, then the preview (under 56
columns or 12 rows), leaving a full-width list. The status bar at the bottom
shows the current mode, key hints, and the library summary. Press `?` on
either screen for a full key reference; any key closes it.

> [!NOTE]
> If you're used to the old `left`/`right` switching between animations:
> that's now `>`/`<`. `left`/`right` scrub frame-by-frame within
> the current animation instead, accelerating the longer you hold them.

**Gallery**

| Key | Action |
|---|---|
| `enter` | play the selected animation |
| `a` | render a new gif into the library |
| `r` | rename the selected entry |
| `d` | delete, via a centered confirmation menu (↑/↓ select, enter confirm, esc cancel) |
| `/` | filter the list |
| `t` | cycle the color theme (saved to the config file) |
| `k` | configure player keybinds |
| `?` | show all key bindings |
| `ctrl+c` | quit |

**Player** (defaults; rebindable via the gallery's `k` menu)

| Key | Action |
|---|---|
| `space` | pause / resume |
| `left` / `right` | scrub one frame back/forward (pauses); hold to accelerate |
| `,` / `.` | step one frame back/forward (pauses) |
| `+`/`=`, `-` | speed up / down (0.25 steps, 0.25x-8x) |
| `>` / `<` | next / previous animation |
| `f` | toggle background filtering (re-rendered and saved) |
| `?` | show all key bindings |
| `esc` | back to the gallery |
| `ctrl+c` | quit |

**Keybinds menu** (`k` from the gallery)

Every playback action above can be rebound. `↑`/`↓` (or `k`/`j`) select an
action, `enter` rebinds it to the next key you press, `a` adds an extra key
alongside the existing ones, `d` resets the action to its default, and `D`
resets everything. Changes are saved to the config file immediately.
`esc`, `ctrl+c`, and `?` are reserved (they navigate the player itself)
and a key already used by another action is rejected, so bindings can never
collide or lock you out.

The `a` prompt has fzf-style fuzzy search: it recursively finds `.gif` files
under the directory portion of the typed path (a few levels deep) and filters
them as you type. `tab` completes the selection, `up`/`down` (or
`ctrl+p`/`ctrl+n`) move through the matches, and `enter` renders the chosen
gif. Paths starting with `~` expand to your home directory, both here and in
CLI arguments.

### Cross-machine playback

`.frames` files are self-contained (frames, delays, dimensions, gzipped).
Render on one machine, copy the file anywhere, and `ramp play` it there:

```sh
ramp render giphy.gif -o giphy.frames
scp giphy.frames other-machine:
ssh other-machine ramp play giphy.frames
```

The library lives in the platform cache directory
(`~/.cache/ascii-tui/library` on Linux, `%LOCALAPPDATA%\ascii-tui\library`
on Windows).

## Configuration

An optional TOML file at `<user config dir>/ascii-tui/config.toml`
(`~/.config/ascii-tui/config.toml` on Linux,
`%AppData%\ascii-tui\config.toml` on Windows) sets defaults for playback
speed, new-render options, the theme, and the player keybinds (the in-app
`k` menu reads and writes the same file). It's entirely optional: a missing
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
name         = "pink" # built-in preset the gallery's `t` key cycles from
                      # (pink, matrix, amber, ocean); omit for custom colors
accent       = "212"  # selection, header glyph, progress bar
accent_alt   = "179"  # section headers (ANIMATIONS, FILE)
border       = "240"  # section rules
text         = "252"  # normal text
dim          = "243"  # column titles, help text, secondary labels
error        = "203"  # error/warning text, delete chip
bg           = "234"  # app background fill
selection_bg = "237"  # selected-row bar
chip_text    = "234"  # text inside the mode chip

[keys]                # player bindings; each action takes a list of keys
pause        = ["space"]
seek_back    = ["left"]
seek_forward = ["right"]
step_back    = [","]
step_forward = ["."]
speed_up     = ["+", "="]
speed_down   = ["-"]
next         = [">"]
prev         = ["<"]
filter       = ["f"]
```

Theme values accept anything `lipgloss.Color` does: a 256-color index (as
above) or a hex code like `"#ff6ac1"`. The background colors (`bg`,
`selection_bg`) support 256-color indexes and hex codes; set `bg = ""` to
keep your terminal's own background instead of the fill.

Key values are Bubble Tea key names: single characters (`"n"`, `","`),
special keys (`"left"`, `"enter"`, `"tab"`), and modifier combos
(`"ctrl+n"`, `"shift+up"`); the space bar is spelled `"space"`. An empty
list falls back to that action's default binding.

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
