"""Generate the Weiqi launcher icon as PNG.

Mirrors the vector adaptive icon defined in
android/app/src/main/res/drawable/ic_launcher_{background,foreground}.xml,
which itself was copied from the main branch.

Outputs (all 1024x1024 unless --legacy):
  assets/icon/icon.png             — full-bleed (kaya bg + foreground)
  assets/icon/icon_foreground.png  — foreground only on transparent bg
"""
from __future__ import annotations

import math
import os
import sys

from PIL import Image, ImageDraw, ImageFilter


# Icon coordinates are authored in the Android adaptive-icon 108-unit space.
# We render at SIZE px and scale.
VIEWPORT = 108.0


def _scale(SIZE: int):
    return SIZE / VIEWPORT


def _kaya_bg(SIZE: int) -> Image.Image:
    """Linear gradient #F0D5A8 (top) -> #CFA976 (bottom), full bleed."""
    top = (0xF0, 0xD5, 0xA8)
    bot = (0xCF, 0xA9, 0x76)
    img = Image.new('RGB', (SIZE, SIZE), top)
    px = img.load()
    for y in range(SIZE):
        t = y / (SIZE - 1)
        r = round(top[0] * (1 - t) + bot[0] * t)
        g = round(top[1] * (1 - t) + bot[1] * t)
        b = round(top[2] * (1 - t) + bot[2] * t)
        for x in range(SIZE):
            px[x, y] = (r, g, b)
    return img


def _radial_stone(layer: Image.Image, cx: float, cy: float, r: float,
                  inner_rgb, outer_rgb,
                  inner_off: tuple[float, float],
                  gradient_radius: float):
    """Draw a circle filled with a radial gradient.

    inner_off: offset (dx, dy) in icon units of the gradient center relative to
    the stone center — matches the centerX/centerY in the AAPT vector source.
    """
    icx = cx + inner_off[0]
    icy = cy + inner_off[1]
    # Pixel-level fill: write each pixel's color based on distance to (icx,icy).
    px = layer.load()
    bbox_min_x = max(0, int(math.floor(cx - r)))
    bbox_max_x = min(layer.width, int(math.ceil(cx + r)))
    bbox_min_y = max(0, int(math.floor(cy - r)))
    bbox_max_y = min(layer.height, int(math.ceil(cy + r)))
    r2 = r * r
    for y in range(bbox_min_y, bbox_max_y):
        dy = y + 0.5 - cy
        for x in range(bbox_min_x, bbox_max_x):
            dx = x + 0.5 - cx
            d2 = dx * dx + dy * dy
            if d2 > r2:
                continue
            # Coverage AA on the rim.
            d = math.sqrt(d2)
            cov = min(1.0, max(0.0, (r - d) * 1.5))
            # Gradient parameter from the gradient center (icx,icy).
            gdx = x + 0.5 - icx
            gdy = y + 0.5 - icy
            t = min(1.0, math.sqrt(gdx * gdx + gdy * gdy) / gradient_radius)
            cr = round(inner_rgb[0] * (1 - t) + outer_rgb[0] * t)
            cg = round(inner_rgb[1] * (1 - t) + outer_rgb[1] * t)
            cb = round(inner_rgb[2] * (1 - t) + outer_rgb[2] * t)
            a = int(round(255 * cov))
            existing = px[x, y]
            if a == 255:
                px[x, y] = (cr, cg, cb, 255)
            else:
                # Blend over existing.
                ea = existing[3]
                out_a = a + ea * (255 - a) // 255
                if out_a == 0:
                    continue
                er = (cr * a + existing[0] * ea * (255 - a) // 255) // out_a
                eg = (cg * a + existing[1] * ea * (255 - a) // 255) // out_a
                eb = (cb * a + existing[2] * ea * (255 - a) // 255) // out_a
                px[x, y] = (er, eg, eb, out_a)


def _draw_foreground(canvas: Image.Image):
    """Draw the foreground (grid + stones) onto the RGBA canvas in place."""
    SIZE = canvas.width
    s = _scale(SIZE)

    fg = Image.new('RGBA', (SIZE, SIZE), (0, 0, 0, 0))
    d = ImageDraw.Draw(fg)

    line_color = (0x22, 0x1A, 0x10)
    line_alpha = int(round(0.55 * 255))

    # Grid: 4 vertical & 4 horizontal lines, 0.8u thick, spanning x=27..81 / y=27..81.
    # Vertical lines at x = 26.6, 44.6, 62.6, 80.6 (left edge) -> 27.4, 45.4, 63.4, 81.4
    line_thickness = 0.8 * s
    grid_min = 27 * s
    grid_max = 81 * s
    line_xs = [26.6, 44.6, 62.6, 80.6]
    for lx in line_xs:
        x0 = lx * s
        d.rectangle([x0, grid_min, x0 + line_thickness, grid_max],
                    fill=line_color + (line_alpha,))
    line_ys = [26.6, 44.6, 62.6, 80.6]
    for ly in line_ys:
        y0 = ly * s
        d.rectangle([grid_min, y0, grid_max, y0 + line_thickness],
                    fill=line_color + (line_alpha,))

    # Hoshi (2x2 unit square at 53,53).
    d.rectangle([53 * s, 53 * s, 55 * s, 55 * s],
                fill=line_color + (255,))

    # Stone shadows: filled circles offset by ~(0,2) from stones, alpha 0.22, blurred.
    sh = Image.new('RGBA', (SIZE, SIZE), (0, 0, 0, 0))
    sd = ImageDraw.Draw(sh)
    shadow_alpha = int(round(0.22 * 255))
    sd.ellipse([(45 - 13) * s, (46 - 13) * s, (45 + 13) * s, (46 + 13) * s],
               fill=(0, 0, 0, shadow_alpha))
    sd.ellipse([(63 - 13) * s, (64 - 13) * s, (63 + 13) * s, (64 + 13) * s],
               fill=(0, 0, 0, shadow_alpha))
    sh = sh.filter(ImageFilter.GaussianBlur(radius=1.6 * s))
    fg.alpha_composite(sh)

    # Black stone at (44.6, 44.6), r=12, radial gradient from (40,40), grad radius 18.
    _radial_stone(
        fg,
        cx=44.6 * s, cy=44.6 * s, r=12 * s,
        inner_rgb=(0x3B, 0x3B, 0x3B),
        outer_rgb=(0x0A, 0x0A, 0x0A),
        inner_off=((40 - 44.6) * s, (40 - 44.6) * s),
        gradient_radius=18 * s,
    )

    # White stone at (62.6, 62.6), r=12, radial gradient from (58,58), grad radius 18.
    _radial_stone(
        fg,
        cx=62.6 * s, cy=62.6 * s, r=12 * s,
        inner_rgb=(0xFF, 0xFC, 0xF6),
        outer_rgb=(0xE1, 0xD5, 0xBF),
        inner_off=((58 - 62.6) * s, (58 - 62.6) * s),
        gradient_radius=18 * s,
    )
    # White stone outline (#221A10 alpha 0.55, width 0.6u).
    od = ImageDraw.Draw(fg)
    od.ellipse([(62.6 - 12) * s, (62.6 - 12) * s,
                (62.6 + 12) * s, (62.6 + 12) * s],
               outline=line_color + (line_alpha,),
               width=max(1, int(round(0.6 * s))))

    canvas.alpha_composite(fg)


def make_full(path: str, size: int = 1024):
    bg = _kaya_bg(size).convert('RGBA')
    _draw_foreground(bg)
    bg.convert('RGB').save(path, 'PNG', optimize=True)


def make_foreground(path: str, size: int = 1024):
    img = Image.new('RGBA', (size, size), (0, 0, 0, 0))
    _draw_foreground(img)
    img.save(path, 'PNG', optimize=True)


if __name__ == '__main__':
    out_dir = 'assets/icon'
    os.makedirs(out_dir, exist_ok=True)
    make_full(os.path.join(out_dir, 'icon.png'))
    make_foreground(os.path.join(out_dir, 'icon_foreground.png'))
    print('wrote assets/icon/icon.png and icon_foreground.png')
