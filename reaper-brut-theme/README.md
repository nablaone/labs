# BRUT_ST — a brutalist REAPER theme

A remake of REAPER's stock **Default 7.0** theme in the spirit of Cubase on the
Atari ST: flat, hard-edged, black / white / gray, with three muted signal colors
for the controls that matter.

| Color  | Meaning                                                                 |
|--------|-------------------------------------------------------------------------|
| RED    | record, record-arm, mute, write automation, play cursor, markers, clip  |
| YELLOW | solo, touch/latch automation, FX bypassed, meter top                    |
| GREEN  | play, FX on, read automation, folder, loop, meter signal                |
| B/W/2 grays | everything else                                                     |

Icon shapes and layout are identical to Default 7.0: nothing moved and no
icon changed meaning. Soft shadows, gradients and glows are gone. Pixels are
either there or not.

Every button is a flat rectangle: a 1px black border with the letter or
symbol inside. Off is light gray, hover is white, pressed is dark gray, and on
is filled with its signal color. The round 3D transport buttons are
rectangles too. Toolbar icons are black ink on the gray bar, framed on
hover, and inverted (white on a black plate) when on.

The transport always uses the centered layout and is black on the paper gray:
the readouts (time, BPM, selection, time signature, rate) are black text in
square black-bordered boxes, and the GLOBAL automation button
is a gray pill with black lettering. Only record, play and loop light up in
color.

Like Cubase on the SM124 monochrome monitor, the desktop is one flat sheet:
a single light gray `#D8D8D8` (`PAPER`) for the arrange, ruler, track list,
track panels, toolbar and transport, with black track dividers and a faint
grid. Parts are gray with a black waveform; selected parts invert
to black with a white waveform. The MIDI editor is white too, with black
notes and grid.

## Install

Quick way (macOS / Linux / Windows with Git Bash):

```sh
make install     # builds the theme if needed, copies it into REAPER's ColorThemes
make uninstall   # removes it again
```

If your REAPER resource folder is somewhere else (e.g. a portable install):
`make install REAPER_DIR=/path/to/REAPER`. Then pick it in REAPER under
**Options → Themes → BRUT_ST**.

Manual way:

1. Copy `dist/BRUT_ST.ReaperThemeZip` into REAPER's theme folder:
   - **macOS:** `~/Library/Application Support/REAPER/ColorThemes/`
   - **Windows:** `%APPDATA%\REAPER\ColorThemes\`
   - **Linux:** `~/.config/REAPER/ColorThemes/`

   (Shortcut: in REAPER, *Options → Show REAPER resource path in
   explorer/finder* opens the parent folder.)
2. In REAPER: **Options → Themes → BRUT_ST**.

   Alternatively, drag the `.ReaperThemeZip` file onto the REAPER window, or
   double-click it. REAPER copies and loads it.
3. To go back: **Options → Themes → Default_7.0**.

### Optional

- **Theme adjuster:** the theme still points at Default 7.0's adjuster script
  (*Options → Themes → Theme adjuster*). Its parameters (layouts, TCP
  background color, etc.) are unchanged. The TCP background defaults to `PAPER`
  (`#D8D8D8`) with black ink. Not yet tested on this theme.
- **Font:** every theme font (track names, readouts, ruler, transport, item
  labels) is Menlo at one height per UI scale (12 / 18 / 24 px for 100 / 150 /
  200%), like the single system font in Cubase on the ST. A theme cannot ship
  fonts, so on Windows change `MONO_FONT` in `tools/build.py` to Consolas.
  Fonts set in *Preferences → Appearance → Fonts* override the theme.
- **Transport on top:** where the transport sits is a REAPER setting, not part
  of the theme. Right-click the transport and choose *Dock transport in main
  window*, then *Docked transport position → Top of main window*.
- **Track colors:** custom track colors still tint panels and items. For a pure
  monochrome look, keep tracks uncolored.

## Rebuild / tweak

The theme is generated from the stock theme by `tools/build.py`:

```sh
make            # creates .venv with Pillow + numpy, builds dist/BRUT_ST.ReaperThemeZip
make clean      # removes build/ and dist/
```

What the script does:

- **Images** (all 1417 PNGs, including the 150% / 200% hi-DPI sets): neutral
  pixels snap to 4 grays (`00 55 AA FF`). Colored pixels snap by hue to red,
  yellow or green. Darker shades inside a colored button become black ink, so
  lettering like the "S" on solo stays readable. Alpha is hard-thresholded,
  which removes shadows and glows. Pure pink `255,0,255` and yellow
  `255,255,0` pixels are REAPER's stretch-margin metadata, so they are
  preserved exactly. The accent yellow is `200,164,48` partly for that
  reason.
- **Buttons** (`FLAT_BUTTONS`, `TOOLBAR` patterns): redrawn, not recolored.
  Each of the 3 state frames becomes a flat bordered rectangle. The glyph is
  lifted from the stock image by contrast against the button body, then drawn
  in solid ink. Routing and automation-mode marks keep their signal color.
- **Colors** (`BRUT_ST.ReaperTheme`): the same snapping, plus a hand-picked
  `OVERRIDES` table for the Atari roles (MIDI editor, arrange, items, meters,
  lists). Group colors, envelope colors and routing-wiring colors are left
  stock, because their job is to tell many things apart.
- **rtconfig.txt**: literal colors snap to the palette, and text becomes pure
  black/white or a signal color. The transport is forced to the centered
  layout, and the time display gets vertical margins so it isn't taller than
  the other text.

Tweak the palette constants (`GRAYS`, `RED`, `YELLOW`, `GREEN`) or
`OVERRIDES` at the top of the script, then rebuild.

## Folders

- `original/extracted/` holds the stock Default 7.0 theme (from REAPER 7.80),
  unpacked. The build reads it.
- `docs/` (not in git) is for the downloaded Cockos references: theme image
  reference (`images.html`), WALTER layout reference (`walter.html`), and the
  REAPER User Guide PDF, all from reaper.fm.
- `build/` (not in git) holds the unpacked generated theme (useful for
  inspecting images).
- `dist/` holds the installable `BRUT_ST.ReaperThemeZip`.

The Default theme is © Cockos, LGPL v2 or later. This derivative is under the
same license.
