#!/usr/bin/env python3
"""Render Dockafé ANSI demo frames to PNG screenshots."""

from __future__ import annotations

import re
import sys
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

# xterm 256 color approx (first 16 + cube + grayscale enough for our frames)
def xterm256(n: int) -> tuple[int, int, int]:
    if n < 16:
        table = [
            (0, 0, 0), (205, 0, 0), (0, 205, 0), (205, 205, 0),
            (0, 0, 238), (205, 0, 205), (0, 205, 205), (229, 229, 229),
            (127, 127, 127), (255, 0, 0), (0, 255, 0), (255, 255, 0),
            (92, 92, 255), (255, 0, 255), (0, 255, 255), (255, 255, 255),
        ]
        return table[n]
    if n < 232:
        n -= 16
        r = n // 36
        g = (n % 36) // 6
        b = n % 6
        ramp = [0, 95, 135, 175, 215, 255]
        return ramp[r], ramp[g], ramp[b]
    v = 8 + (n - 232) * 10
    return v, v, v


CSI_RE = re.compile(r"\x1b\[([0-9;]*)m")


def parse_ansi(text: str) -> list[list[tuple[str, tuple[int, int, int], tuple[int, int, int] | None]]]:
    """Return rows of (char, fg, bg) cells."""
    fg = (200, 200, 200)
    bg = None
    bold = False
    rows: list[list[tuple[str, tuple[int, int, int], tuple[int, int, int] | None]]] = []
    row: list[tuple[str, tuple[int, int, int], tuple[int, int, int] | None]] = []

    i = 0
    while i < len(text):
        if text[i] == "\x1b" and i + 1 < len(text) and text[i + 1] == "[":
            m = CSI_RE.match(text, i)
            if not m:
                i += 1
                continue
            params = m.group(1)
            i = m.end()
            if params == "" or params == "0":
                fg, bg, bold = (200, 200, 200), None, False
                continue
            parts = [int(p) for p in params.split(";") if p != ""]
            j = 0
            while j < len(parts):
                p = parts[j]
                if p == 0:
                    fg, bg, bold = (200, 200, 200), None, False
                elif p == 1:
                    bold = True
                elif p == 22:
                    bold = False
                elif p == 39:
                    fg = (200, 200, 200)
                elif p == 49:
                    bg = None
                elif 30 <= p <= 37:
                    fg = xterm256(p - 30)
                elif 90 <= p <= 97:
                    fg = xterm256(p - 90 + 8)
                elif 40 <= p <= 47:
                    bg = xterm256(p - 40)
                elif 100 <= p <= 107:
                    bg = xterm256(p - 100 + 8)
                elif p == 38 and j + 2 < len(parts) and parts[j + 1] == 5:
                    fg = xterm256(parts[j + 2])
                    j += 2
                elif p == 48 and j + 2 < len(parts) and parts[j + 1] == 5:
                    bg = xterm256(parts[j + 2])
                    j += 2
                j += 1
            continue

        ch = text[i]
        i += 1
        if ch == "\n":
            rows.append(row)
            row = []
            continue
        if ch == "\r":
            continue
        color = fg
        if bold:
            color = tuple(min(255, c + 25) for c in fg)  # type: ignore
        row.append((ch, color, bg))  # type: ignore
    if row:
        rows.append(row)
    return rows


def render_png(ansi_path: Path, png_path: Path, cell_w: int = 9, cell_h: int = 17) -> None:
    text = ansi_path.read_text(encoding="utf-8", errors="replace")
    # Drop OSC / other escapes
    text = re.sub(r"\x1b\][^\x07]*\x07", "", text)
    text = re.sub(r"\x1b\[[0-9;]*[A-Za-z]", lambda m: m.group(0) if m.group(0).endswith("m") else "", text)

    rows = parse_ansi(text)
    if not rows:
        rows = [[(" ", (200, 200, 200), None)]]
    cols = max(len(r) for r in rows)
    cols = max(cols, 80)
    rows_n = max(len(rows), 24)

    pad = 24
    img_w = cols * cell_w + pad * 2
    img_h = rows_n * cell_h + pad * 2
    img = Image.new("RGB", (img_w, img_h), (18, 18, 22))
    draw = ImageDraw.Draw(img)

    # subtle terminal chrome
    draw.rounded_rectangle(
        (8, 8, img_w - 8, img_h - 8),
        radius=12,
        fill=(24, 24, 30),
        outline=(55, 55, 65),
        width=2,
    )

    font = None
    for cand in (
        "/usr/share/fonts/liberation-mono/LiberationMono-Regular.ttf",
        "/usr/share/fonts/dejavu/DejaVuSansMono.ttf",
        "/usr/share/fonts/TTF/DejaVuSansMono.ttf",
        "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf",
    ):
        if Path(cand).exists():
            font = ImageFont.truetype(cand, 14)
            break
    if font is None:
        font = ImageFont.load_default()

    for y, row in enumerate(rows):
        for x, (ch, fg, bg) in enumerate(row):
            px = pad + x * cell_w
            py = pad + y * cell_h
            if bg is not None:
                draw.rectangle((px, py, px + cell_w, py + cell_h), fill=bg)
            if ch != " ":
                draw.text((px, py), ch, fill=fg, font=font)

    img.save(png_path, optimize=True)
    print(f"wrote {png_path}")


def main() -> None:
    assets = Path(sys.argv[1] if len(sys.argv) > 1 else "docs/assets")
    mapping = {
        "screenshot-splash.ansi": "screenshot-splash.png",
        "screenshot-compose.ansi": "screenshot-compose.png",
        "screenshot-volumes.ansi": "screenshot-volumes.png",
        "screenshot-volume-files.ansi": "screenshot-volume-files.png",
    }
    for ansi_name, png_name in mapping.items():
        ansi_path = assets / ansi_name
        if not ansi_path.exists():
            print(f"missing {ansi_path}", file=sys.stderr)
            continue
        render_png(ansi_path, assets / png_name)


if __name__ == "__main__":
    main()
