#!/usr/bin/env python3
"""Build the BRUT_ST REAPER theme from the stock Default 7.0 theme.

Every image and every theme color is pushed onto a hard, Atari-ST-like palette:
black / dark gray / light gray / white, plus three pure signal colors
(red, yellow, green) for the controls that matter. Icon shapes are untouched;
only their color and shading change.

Usage:  python3 tools/build.py
Needs:  Pillow, numpy
"""
import colorsys
import os
import re
import shutil
import sys
import zipfile

import numpy as np
from PIL import Image

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SRC_THEME = os.path.join(ROOT, "original", "extracted", "Default_7.0_unpacked.ReaperTheme")
SRC_IMG = os.path.join(ROOT, "original", "extracted", "Default_7.0_unpacked")
NAME = "BRUT_ST"
BUILD = os.path.join(ROOT, "build")
OUT_DIR = os.path.join(BUILD, NAME)
OUT_THEME = os.path.join(BUILD, NAME + ".ReaperTheme")
OUT_ZIP = os.path.join(ROOT, "dist", NAME + ".ReaperThemeZip")

# ---------------------------------------------------------------- palette
# Grays: four levels, like the 4-color ST medium-res palette.
GRAYS = np.array([0x00, 0x55, 0xAA, 0xFF], dtype=np.float32)

# Signal colors, muted so they don't glare. Yellow is deliberately NOT
# (255,255,0): REAPER reads pure yellow and pure pink (255,0,255) pixels as
# image stretch/extent metadata.
RED = (184, 48, 40)
YELLOW = (200, 164, 48)
GREEN = (72, 152, 72)

SAT_MIN = 0.30   # below this saturation a pixel counts as gray
VAL_MIN = 0.22   # very dark colored pixels count as black/gray
ALPHA_CUT = 110  # soft shadows / glows below this alpha disappear
INK_RATIO = 0.72 # colored pixels darker than this (vs. brightest) -> black


def accent_for_hue(h):
    """h in degrees -> accent RGB, or None to treat the color as neutral.

    red/pink/magenta -> RED, orange/yellow -> YELLOW, green/teal -> GREEN.
    Blues and purples (automation modes, misc) become neutral gray so text
    drawn on top of them stays readable.
    """
    if h >= 320 or h < 22:
        return RED
    if h < 70:
        return YELLOW
    if h < 185:
        return GREEN
    return None


# ---------------------------------------------------------------- images
def convert_image(path_in, path_out):
    im = Image.open(path_in)
    im = im.convert("RGBA")
    a = np.asarray(im).astype(np.float32)
    rgb, alpha = a[..., :3], a[..., 3]
    r, g, b = rgb[..., 0], rgb[..., 1], rgb[..., 2]

    # Layout metadata: exact pink / yellow pixels must survive untouched.
    meta = (alpha == 255) & (r == 255) & (b == 0) & (g == 255) | \
           (alpha == 255) & (r == 255) & (g == 0) & (b == 255)

    mx = rgb.max(axis=2)
    mn = rgb.min(axis=2)
    val = mx / 255.0
    sat = np.where(mx > 0, (mx - mn) / np.maximum(mx, 1), 0)

    # hue in degrees
    d = np.maximum(mx - mn, 1e-6)
    hue = np.select(
        [mx == r, mx == g],
        [((g - b) / d) % 6, (b - r) / d + 2],
        (r - g) / d + 4,
    ) * 60.0

    out = np.zeros_like(rgb)

    # neutral: luma -> nearest of 4 grays
    luma = 0.299 * r + 0.587 * g + 0.114 * b
    idx = np.abs(luma[..., None] - GRAYS[None, None, :]).argmin(axis=2)
    gray = GRAYS[idx]
    out[..., 0] = out[..., 1] = out[..., 2] = gray

    colored = (sat >= SAT_MIN) & (val >= VAL_MIN)
    for lo, hi, col in ((320, 360, RED), (0, 22, RED), (22, 70, YELLOW), (70, 185, GREEN)):
        m = colored & (hue >= lo) & (hue < hi)
        out[m] = col

    # Lettering on colored buttons is a darker shade of the same hue
    # (e.g. dark-olive "S" on the yellow solo button). Relative to the
    # brightest color in the image, those darker shades become black ink.
    solid = colored & (alpha >= ALPHA_CUT)
    if solid.any():
        vmax = val[solid].max()
        ink = colored & (val < INK_RATIO * vmax)
        out[ink] = 0

    # alpha: hard edges, no soft shadows
    new_alpha = np.where(alpha >= ALPHA_CUT, 255, 0).astype(np.float32)

    out[meta] = rgb[meta]
    new_alpha[meta] = 255

    res = np.dstack([out, new_alpha]).clip(0, 255).astype(np.uint8)
    Image.fromarray(res, "RGBA").save(path_out, optimize=True)


# ---------------------------------------------------------------- colors
NON_COLOR = re.compile(r"(mode|dm\d?$|_drawmode|nodarkmode)")
# Keys whose job is to tell many things apart by hue. Collapsing them onto
# three accents would change their meaning, so they keep the stock colors.
KEEP = re.compile(r"^(group_\d+|col_env\d+|env_|wiring_)")

# Hand-picked roles for the Atari look (RGB). Applied after the automatic
# conversion; these win.
W, L, D, K = (255, 255, 255), (0xAA, 0xAA, 0xAA), (0x55, 0x55, 0x55), (0, 0, 0)
# One flat surface color for every empty area (arrange, ruler, track list,
# toolbar, transport). Light gray rather than white, so white is left for
# hover and selection.
PAPER = (0xD8, 0xD8, 0xD8)
PAPER_LINE = (0xC4, 0xC4, 0xC4)  # beat grid on paper
PAPER_SEL = (0xE8, 0xE8, 0xE8)   # selected track lane
OVERRIDES = {
    # --- MIDI editor: paper white, black ink, like Cubase on the SM124
    "midi_trackbg1": W, "midi_trackbg2": (0xEE, 0xEE, 0xEE),
    "midi_trackbg_outer1": L, "midi_trackbg_outer2": L,
    "midi_inline_trackbg1": W, "midi_inline_trackbg2": (0xEE, 0xEE, 0xEE),
    "midioct": K, "midioct_inline": K,
    "midi_grid1": K, "midi_grid2": K, "midi_grid3": K,
    "midi_gridh": K, "midi_gridhc": K,
    "midi_notebg": K, "midi_notefg": K,
    "midi_notemute": L, "midi_notemute_sel": D,
    "midi_selpitch1": L, "midi_selpitch2": L,
    "midi_selbg": K,
    "midi_rulerbg": W, "midi_rulerfg": K,
    "midi_endpt": RED, "midi_ofsn": D, "midi_ofsnsel": RED,
    "midi_editcurs": K,
    "midi_pkey1": W, "midi_pkey2": K, "midi_pkey3": L,
    "midi_leftbg": L,
    "midi_ccbut": L, "midi_ccbut_text": K, "midi_ccbut_arrow": K,
    "midi_noteon_flash": RED,
    "midieditorlist_bg": W, "midieditorlist_fg": K, "midieditorlist_grid": K,
    "midieditorlist_selbg": K, "midieditorlist_selfg": W,
    "midieditorlist_bg2": L, "midieditorlist_fg2": K,
    "midieditorlist_selbg2": D, "midieditorlist_selfg2": W,
    "score_bg": W, "score_fg": K, "score_sel": RED, "score_timesel": L, "score_loop": GREEN,
    "explorer_grid": K, "explorer_pitchtext": K,
    # --- arrange: white paper like Cubase's arrange window on the SM124,
    # black track dividers, faint grid
    "col_arrangebg": PAPER, "arrange_vgrid": L,
    "col_tr1_bg": PAPER, "col_tr2_bg": PAPER,
    "selcol_tr1_bg": PAPER_SEL, "selcol_tr2_bg": PAPER_SEL,
    "col_tr1_divline": K, "col_tr2_divline": K,
    "col_gridlines": L, "col_gridlines2": PAPER_LINE, "col_gridlines3": PAPER_LINE,
    # items: gray part, black waveform; selected parts invert (black part,
    # white waveform), as Cubase did
    "col_tr1_peaks": K, "col_tr2_peaks": K,
    "col_tr1_ps2": W, "col_tr2_ps2": W,
    "col_peaksedge": K, "col_peaksedge2": K,
    "col_peaksedgesel": W, "col_peaksedgesel2": W,
    "col_tr1_itembgsel": K, "col_tr2_itembgsel": K,
    "col_mi_bg": L, "col_mi_bg2": L,
    "col_mi_label": K, "col_mi_label_sel": W,
    "col_mi_label_float": W, "col_mi_label_float_sel": W,
    "selitem_dot": RED,
    # cursors and transport
    "col_cursor": K, "col_cursor2": K,
    "playcursor_color": RED,
    "col_tl_bg": PAPER, "col_tl_fg": K, "col_tl_fg2": K,
    "col_tl_bgsel": W, "col_tl_bgsel2": W,
    "col_trans_bg": PAPER, "col_trans_fg": K, "col_transport_editbk": W,
    "region": L, "region_lane_bg": PAPER, "region_lane_text": K,
    "marker": RED, "marker_lane_bg": PAPER, "marker_lane_text": K,
    "ts_lane_bg": PAPER, "ts_lane_text": K, "col_tsigmark": K,
    "marquee_fill": L, "marquee_outline": K,
    "marqueezoom_fill": W, "marqueezoom_outline": K,
    "areasel_fill": YELLOW, "areasel_outline": K,
    # panels: white desktop, black ink
    "col_tracklistbg": PAPER, "col_mixerbg": PAPER,
    "col_main_bg2": PAPER, "col_main_text2": K,
    "col_main_textshadow": D, "col_main_3dhl": W, "col_main_3dsh": K,
    "col_main_text": K, "col_main_editbk": W,
    "col_tcp_text": K, "col_tcp_textsel": K,
    "col_seltrack": W, "col_seltrack2": W,
    "docker_bg": D, "windowtab_bg": D, "docker_shadow": K,
    "docker_selface": L, "docker_unselface": K,
    "docker_text": L, "docker_text_sel": K,
    "col_toolbar_text": K, "col_toolbar_text_on": RED, "col_toolbar_frame": K,
    "toolbararmed_color": RED,
    "genlist_bg": W, "genlist_fg": K, "genlist_grid": L,
    "genlist_selbg": K, "genlist_selfg": W,
    "genlist_seliabg": L, "genlist_seliafg": K,
    # meters: green / yellow / red
    "col_vubot": GREEN, "col_vumid": GREEN, "col_vutop": YELLOW, "col_vuclip": RED,
    "col_vuintcol": K, "col_vuind1": K, "col_vuind2": D, "col_vuind3": GREEN, "col_vuind4": YELLOW,
    "col_vudoint": K, "col_vumidi": YELLOW,
    # fx / sends lists
    "mcp_fx_normal": K, "mcp_fx_bypassed": YELLOW, "mcp_fx_offlined": RED,
    "mcp_fxparm_normal": K, "mcp_fxparm_bypassed": YELLOW, "mcp_fxparm_offlined": RED,
    "mcp_sends_normal": L, "mcp_sends_muted": RED, "mcp_send_midihw": GREEN,
    "mcp_sends_levels": K,
}


def rgb_to_reaper(c, flags=0):
    r, g, b = c
    v = flags | (b << 16) | (g << 8) | r
    return v - (1 << 32) if v >= 1 << 31 else v


def reaper_to_rgb(v):
    v &= 0xFFFFFFFF
    return (v & 0xFF, (v >> 8) & 0xFF, (v >> 16) & 0xFF), v & 0xFF000000


def convert_color(c):
    r, g, b = c
    h, s, v = colorsys.rgb_to_hsv(r / 255, g / 255, b / 255)
    if s >= SAT_MIN and v >= VAL_MIN:
        acc = accent_for_hue(h * 360)
        if acc:
            return acc
    luma = 0.299 * r + 0.587 * g + 0.114 * b
    lv = int(GRAYS[np.abs(GRAYS - luma).argmin()])
    return (lv, lv, lv)


# One monospace face at one height everywhere, like the Atari ST system font
# in Cubase. A theme can't ship fonts, so this has to be one the OS has:
# Menlo on macOS (on Windows, change to "Consolas").
MONO_FONT = "Menlo"
# Pixel height per UI scale. user_font1..5 serve the 100% layouts,
# user_font6..10 the 150% ones and user_font11..15 the 200% ones; the
# other fonts (labels, ruler, transport, items) are 100%.
FONT_PX = {1.0: 12, 1.5: 18, 2.0: 24}


def font_size(key):
    m = re.match(r"user_font(\d+)$", key)
    n = int(m.group(1)) if m else 0
    scale = 2.0 if n >= 11 else 1.5 if n >= 6 else 1.0
    return -FONT_PX[scale]


def logfont(face, size):
    """REAPER font setting: hex of a Win32 LOGFONTA + 1 checksum byte."""
    import struct
    lf = struct.pack("<lllll8B32s", size, 0, 0, 0, 400,
                     0, 0, 0, 0, 3, 2, 1, 0x31,  # charset .. pitch: FIXED|MODERN
                     face.encode("ascii"))
    lf += bytes([sum(lf) & 0xFF])
    return lf.hex().upper()


def convert_theme():
    lines = open(SRC_THEME, encoding="utf-8").read().splitlines()
    out, section = [], None
    for line in lines:
        m = re.match(r"^\[(.*)\]$", line)
        if m:
            section = m.group(1)
            out.append(line)
            continue
        kv = re.match(r"^([A-Za-z0-9_]+)=(-?\d+)$", line)
        if section == "color theme" and kv:
            key, val = kv.group(1), int(kv.group(2))
            if not NON_COLOR.search(key) and not KEEP.match(key):
                rgb, flags = reaper_to_rgb(val)
                new = OVERRIDES.get(key) or convert_color(rgb)
                line = "%s=%d" % (key, rgb_to_reaper(new, flags))
        elif re.match(r"^\w*_font\d*=[0-9A-F]{100,}$", line):
            key = line.split("=")[0]
            line = key + "=" + logfont(MONO_FONT, size=font_size(key))
        elif line.startswith("ui_img="):
            line = "ui_img=" + NAME
        out.append(line)
    open(OUT_THEME, "w", encoding="utf-8").write("\n".join(out) + "\n")


# ---------------------------------------------------------------- flat buttons
# The stock buttons are round or bevelled, with gradients. Recoloring keeps
# that 3D look, so these images are redrawn instead: each state frame
# becomes a flat rectangle (1px border) with the stock glyph, letter or
# icon, lifted out of the original by contrast, drawn as solid ink on top.
FLAT_BUTTONS = re.compile(
    r"^(track_(mute|solo|recarm|fx|fxempty|fxoff|fxon|io|env|phase|mono|stereo|recmode)"
    r"|mcp_(io|env|mono|stereo|phase|recmode)"
    r"|gen_(mute|solo)_|table_(mute|solo|recarm)_|envcp_(arm|bypass)"
    r"|transport_(home|end|next|previous|pause|play|record|repeat|stop))")
TOOLBAR = re.compile(r"^toolbar_")
KEEP_HUE = re.compile(r"^(track|mcp)_(io|env)")


def _is_meta(a):
    r, g, b, al = a[..., 0], a[..., 1], a[..., 2], a[..., 3]
    return (al == 255) & (r == 255) & (((g == 255) & (b == 0)) | ((g == 0) & (b == 255)))


def _split(a):
    """-> (interior slice y0,y1,x0,x1, list of frame x-ranges).

    Images with stretch metadata keep a 1px border of pink/yellow pixels;
    the 3 state frames (normal / hover / pressed) sit side by side inside.
    """
    h, w = a.shape[:2]
    meta = _is_meta(a)
    edge = meta[0].any() or meta[-1].any() or meta[:, 0].any() or meta[:, -1].any()
    y0, y1, x0, x1 = (1, h - 1, 1, w - 1) if edge else (0, h, 0, w)
    n = 3 if (x1 - x0) % 3 == 0 else 1
    fw = (x1 - x0) // n
    return (y0, y1, x0, x1), [(x0 + i * fw, x0 + (i + 1) * fw) for i in range(n)]


def _luma(rgb):
    return 0.299 * rgb[..., 0] + 0.587 * rgb[..., 1] + 0.114 * rgb[..., 2]


def _body_state(rgb):
    """Median body color -> accent RGB if the button is lit, else None."""
    med = np.median(rgb.reshape(-1, 3), axis=0)
    h, s, v = colorsys.rgb_to_hsv(*(med / 255.0))
    if s >= 0.35 and v >= 0.45:
        return accent_for_hue(h * 360) or K
    return None


def flatten_button(src, dst, scale, name):
    a = np.asarray(Image.open(src).convert("RGBA")).astype(np.int32)
    out = np.zeros_like(a)
    meta = _is_meta(a)
    (y0, y1, _, _), frames = _split(a)
    bw = 2 if scale >= 2 else 1
    for fi, (fx0, fx1) in enumerate(frames):
        f = a[y0:y1, fx0:fx1]
        solid = f[..., 3] >= 60
        if not (f[..., 3] >= 200).any():
            continue
        ys, xs = np.nonzero(solid)
        t, b, l, r = ys.min(), ys.max() + 1, xs.min(), xs.max() + 1
        fh, fw = f.shape[:2]
        # round if a point 12% in from the bbox corner is see-through
        round_ = name.startswith("transport_") or f[t + int(0.12 * (b - t)), l + int(0.12 * (r - l)), 3] < 128
        # keep a 1px gap to the neighbour when the button fills the frame
        if l == 0: l += 1
        if r == fw: r -= 1
        if t == 0: t += 1
        if b == fh: b -= 1
        bh, bwid = b - t, r - l
        rgb = f[..., :3].astype(np.float32)
        # glyph search area: away from the bevel / rim
        m = max(2, int(round(0.10 * min(bh, bwid))))
        yy, xx = np.mgrid[0:fh, 0:fw]
        inner = (yy >= t + m) & (yy < b - m) & (xx >= l + m) & (xx < r - m)
        if round_:  # round button: stay inside the circle
            cy, cx = (t + b - 1) / 2, (l + r - 1) / 2
            ry, rx = (bh / 2) * 0.68, (bwid / 2) * 0.68
            inner &= ((yy - cy) / ry) ** 2 + ((xx - cx) / rx) ** 2 <= 1
        inner &= solid
        if not inner.any():
            continue
        # glyph = anything clearly off the body color, lighter, darker or
        # a different hue (e.g. the colored automation-mode marks)
        # Round buttons have a glossy highlight, so there only one side
        # (lighter or darker than the body) counts.
        med = np.median(rgb[inner], axis=0)
        if round_:
            lum = _luma(rgb)
            diff = np.where(inner, lum - np.median(lum[inner]), 0)
            T = max(20.0, 0.4 * np.abs(diff).max())
            bright, dark = diff > T, diff < -T
            # the gloss sits near the rim, the symbol in the middle
            cy, cx = (t + b - 1) / 2, (l + r - 1) / 2
            core = ((yy - cy) / (bh * 0.25)) ** 2 + ((xx - cx) / (bwid * 0.25)) ** 2 <= 1
            glyph = bright if (bright & core).sum() >= (dark & core).sum() else dark
        else:
            dist = np.where(inner, np.sqrt(((rgb - med) ** 2).sum(axis=2)), 0)
            glyph = dist > max(45.0, 0.35 * dist.max())

        lit = _body_state(rgb[inner & ~glyph]) if (inner & ~glyph).any() else None
        if lit is not None:
            fill, ink = lit, (W if lit == K else K)
        elif fi == 1:
            fill, ink = W, K        # hover
        elif fi == 2:
            fill, ink = D, W        # pressed
        else:
            fill, ink = L, K
        o = out[y0:y1, fx0:fx1]
        o[t:b, l:r, :3] = K
        o[t:b, l:r, 3] = 255
        o[t + bw:b - bw, l + bw:r - bw, :3] = fill
        o[glyph, :3] = ink
        if lit is None and KEEP_HUE.match(name):
            # routing / automation-mode marks keep their signal color
            for y, x in zip(*np.nonzero(glyph)):
                hh, ss, vv = colorsys.rgb_to_hsv(*(rgb[y, x] / 255.0))
                if ss >= 0.4 and vv >= 0.35:
                    acc = accent_for_hue(hh * 360)
                    if acc:
                        o[y, x, :3] = acc
    out[meta] = a[meta]
    Image.fromarray(out.clip(0, 255).astype(np.uint8), "RGBA").save(dst, optimize=True)


def flatten_toolbar(src, dst, scale, name):
    """Toolbar icons on the white bar: one ink, no green. off = black,
    hover = black with a frame, on = inverted (black plate, white icon),
    pressed = gray plate."""
    a = np.asarray(Image.open(src).convert("RGBA")).astype(np.int32)
    out = np.zeros_like(a)
    meta = _is_meta(a)
    (y0, y1, _, _), frames = _split(a)
    is_on = name.endswith("_on.png")
    blank = name == "toolbar_blank.png"
    bw = 2 if scale >= 2 else 1
    for fi, (fx0, fx1) in enumerate(frames):
        f = a[y0:y1, fx0:fx1]
        rgb = f[..., :3].astype(np.float32)
        mx, mn = rgb.max(axis=2), rgb.min(axis=2)
        sat = np.where(mx > 0, (mx - mn) / np.maximum(mx, 1), 0)
        glyph = (f[..., 3] >= ALPHA_CUT) & ((_luma(rgb) > 110) | ((sat > 0.3) & (mx > 100)))
        if blank:
            glyph[:] = False
        o = out[y0:y1, fx0:fx1]
        fh, fw = o.shape[:2]
        if fi == 2:
            plate, edge, ink = L, K, K
        elif is_on:
            plate, edge, ink = (D if fi == 1 else K), K, W
        elif fi == 1:
            plate, edge, ink = None, K, K
        else:
            plate, edge, ink = None, None, K
        if edge is not None:
            o[1:fh - 1, 1:fw - 1, :3] = edge
            o[1:fh - 1, 1:fw - 1, 3] = 255
            if plate is None:
                o[1 + bw:fh - 1 - bw, 1 + bw:fw - 1 - bw, 3] = 0
            else:
                o[1 + bw:fh - 1 - bw, 1 + bw:fw - 1 - bw, :3] = plate
        o[glyph, :3] = ink
        o[glyph, 3] = 255
    out[meta] = a[meta]
    Image.fromarray(out.clip(0, 255).astype(np.uint8), "RGBA").save(dst, optimize=True)


TRANS_INVERT = re.compile(
    r"^(transport_(basis_|tap|timebase_|bpm_bg|group_bg)|transSectionBg|transRateFaderBg)")
GLOBAL_AUTO = re.compile(r"^global_")
TRANS_BOX = re.compile(r"^(transSectionBg|transRateFaderBg)\.png$")


def invert_to_paper(path):
    """Transport readouts: stock is light-on-black. Flip the grays to black
    ink on the paper gray; signal colors and metadata stay."""
    a = np.asarray(Image.open(path).convert("RGBA")).copy()
    meta = _is_meta(a)
    r, g, b = a[..., 0], a[..., 1], a[..., 2]
    gray = (r == g) & (g == b) & ~meta
    lut = {0x00: PAPER, 0x55: L, 0xAA: K, 0xFF: K}
    masks = {v: gray & (r == v) for v in lut}  # before any pixel changes
    for v, c in lut.items():
        a[masks[v], :3] = c
    Image.fromarray(a, "RGBA").save(path, optimize=True)


def global_automode(src, dst):
    """Transport "GLOBAL <mode>" button, from the stock image: black panel ->
    paper, gray pill -> light gray, all lettering (colored or not) -> black."""
    a = np.asarray(Image.open(src).convert("RGBA")).astype(np.float32)
    meta = _is_meta(a.astype(np.int32))
    rgb = a[..., :3]
    out = np.zeros_like(a)
    out[..., :3] = PAPER
    solid = a[..., 3] >= ALPHA_CUT
    # distance from the panel color (the biggest area; red when bypassed)
    panel = np.median(rgb[solid & ~meta], axis=0)
    d = np.sqrt(((rgb - panel) ** 2).sum(axis=2)) / np.sqrt(3)
    off_panel = solid & ~meta & (d >= 12)
    # the pill is the biggest thing that isn't panel; ink is what stands
    # out from both
    mx, mn = rgb.max(axis=2), rgb.min(axis=2)
    colored = (mx - mn) / np.maximum(mx, 1) >= 0.3
    pill = np.median(rgb[off_panel & ~colored], axis=0)
    dp = np.sqrt(((rgb - pill) ** 2).sum(axis=2)) / np.sqrt(3)
    out[off_panel, :3] = L
    out[off_panel & (((d >= 42) & (dp >= 30)) | (colored & (mx >= 110))), :3] = K
    out[..., 3] = np.where(a[..., 3] >= ALPHA_CUT, 255, 0)
    out[meta] = a[meta]
    Image.fromarray(out.astype(np.uint8), "RGBA").save(dst, optimize=True)


def square_box(path, scale):
    """Rounded readout panels -> square paper box with a black border."""
    a = np.asarray(Image.open(path).convert("RGBA")).copy()
    meta = _is_meta(a)
    (y0, y1, x0, x1), _ = _split(a)
    inner = a[y0:y1, x0:x1]
    h, w = inner.shape[:2]
    r = int(round(6 * scale))
    for ys in (slice(0, r), slice(h - r, h)):
        for xs in (slice(0, r), slice(w - r, w)):
            c = inner[ys, xs]
            c[..., :3] = PAPER
            c[..., 3] = 255
    bw = 2 if scale >= 2 else 1
    for sl in (np.s_[:bw, :], np.s_[h - bw:, :], np.s_[:, :bw], np.s_[:, w - bw:]):
        inner[sl][..., :3] = K
        inner[sl][..., 3] = 255
    a[meta] = np.asarray(Image.open(path).convert("RGBA"))[meta]
    Image.fromarray(a, "RGBA").save(path, optimize=True)


# ---------------------------------------------------------------- rtconfig
# TCP light gray with black ink, master a step darker.
BG_PARAMS = {"tcpBgCol": PAPER[0], "masterTcpBgCol": 0x55, "transBgCol": PAPER[0]}


def convert_rtconfig(src, dst):
    """Push the literal RGB triplets in rtconfig.txt onto the palette too.

    Only plain bracketed number lists of 3 (rgb), 4 (rgba), 7 or 8 numbers
    (WALTER color: rgba for text + rgba for fill) are touched; expressions,
    sizes and coordinates are left alone because they live on lines without
    '.color' or in lists of other lengths.
    """
    txt = open(src, encoding="utf-8").read()

    def fix_list(m):
        nums = [int(x) for x in m.group(1).split()]
        def pal(rgb):
            return list(convert_color(tuple(rgb)))
        if len(nums) in (3, 4):
            # 3/4 numbers = text color: signal color if it had a hue
            # (e.g. red label on armed tracks), else pure black or white
            c = convert_color(tuple(nums[:3]))
            if c[0] == c[1] == c[2]:
                v = 255 if sum(nums[:3]) / 3 >= 128 else 0
                c = (v, v, v)
            nums[:3] = list(c)
        elif len(nums) in (7, 8):
            nums[0:3] = pal(nums[0:3])
            nums[4:7] = pal(nums[4:7])
        return "[" + " ".join(str(n) for n in nums) + "]"

    # Transport: always use the centered layout (stock only does this when
    # the user ticks "Center transport controls").
    txt = txt.replace("?trans_center", "?brut_center")
    txt = txt.replace("Macro drawTrans Scale\n", "Macro drawTrans Scale\nset brut_center 1\n", 1)
    # The time display fits its text to the box height, so it came out
    # 3x taller than every other label. Vertical margins shrink it toward
    # the common text height.
    txt = re.sub(r"^(set trans\.status\.margin\s+).*$",
                 r"\g<1>* Scale [transMargin{0} 10 transMargin{0} 10]", txt, flags=re.M)

    out = []
    for line in txt.splitlines():
        # default panel background colours exposed as theme parameters
        pm = re.match(r"^(define_parameter\s+(\w*BgCol)([RGB])\s+'[^']*'\s+)(\d+)(.*)$", line)
        if pm:
            out.append(pm.group(1) + str(BG_PARAMS.get(pm.group(2), 0x55)) + pm.group(5))
            continue
        if re.search(r"\.color|Col[RGB]?\b|emptyTcpSecColor", line) and not line.lstrip().startswith(";"):
            line = re.sub(r"\[((?:\d+\s+){2,7}\d+)\]", fix_list, line)
        out.append(line)
    txt = "\n".join(out) + "\n"
    # Transport readouts: black text on the paper gray (after the palette
    # snap above, which would push PAPER onto the 4-gray scale).
    p = "%d %d %d" % PAPER
    for pat, rep in (
        (r"(set trans\.status\.color\s+).*", r"\g<1>[0 0 0 0 %s 255]" % p),
        (r"(set trans\.(?:rate|bpm\.tap|sel)\.color\s+).*", r"\g<1>[0 0 0]"),
    ):
        txt = re.sub(pat, rep, txt)
    open(dst, "w", encoding="utf-8").write(txt)


# ---------------------------------------------------------------- main
def main():
    if os.path.isdir(BUILD):
        shutil.rmtree(BUILD)
    os.makedirs(OUT_DIR)
    os.makedirs(os.path.dirname(OUT_ZIP), exist_ok=True)

    n = 0
    for dirpath, _, files in os.walk(SRC_IMG):
        rel = os.path.relpath(dirpath, SRC_IMG)
        dst_dir = os.path.normpath(os.path.join(OUT_DIR, rel))
        os.makedirs(dst_dir, exist_ok=True)
        for f in files:
            src = os.path.join(dirpath, f)
            dst = os.path.join(dst_dir, f)
            if f.lower().endswith(".png"):
                scale = {"150": 1.5, "200": 2.0}.get(rel, 1.0)
                if FLAT_BUTTONS.match(f):
                    flatten_button(src, dst, scale, f)
                elif GLOBAL_AUTO.match(f):
                    global_automode(src, dst)
                elif TOOLBAR.match(f):
                    flatten_toolbar(src, dst, scale, f)
                else:
                    convert_image(src, dst)
                    if TRANS_INVERT.match(f):
                        invert_to_paper(dst)
                    if TRANS_BOX.match(f):
                        square_box(dst, scale)
                n += 1
            elif f == "rtconfig.txt":
                convert_rtconfig(src, dst)
            else:
                shutil.copy2(src, dst)
    convert_theme()

    if os.path.exists(OUT_ZIP):
        os.remove(OUT_ZIP)
    with zipfile.ZipFile(OUT_ZIP, "w", zipfile.ZIP_DEFLATED) as z:
        z.write(OUT_THEME, os.path.basename(OUT_THEME))
        for dirpath, _, files in os.walk(OUT_DIR):
            for f in sorted(files):
                p = os.path.join(dirpath, f)
                z.write(p, os.path.relpath(p, BUILD))
    print("converted %d images -> %s" % (n, OUT_ZIP))


if __name__ == "__main__":
    sys.exit(main())
