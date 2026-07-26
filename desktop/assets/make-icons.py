"""Build a multi-resolution Windows .ico from desktop-logo.png.

Embeds PNG frames for sizes 16, 24, 32, 48, 64, 128, 256 so taskbar /
desktop shortcuts stay sharp. Also writes icon.png (256).

Run: python desktop/assets/make-icons.py
"""
from __future__ import annotations

import io
import struct
from pathlib import Path

from PIL import Image

HERE = Path(__file__).resolve().parent
SRC = HERE / "desktop-logo.png"
OUT_ICO = HERE / "icon.ico"
OUT_PNG = HERE / "icon.png"
SIZES = [16, 24, 32, 48, 64, 128, 256]


def square_rgba(src: Image.Image, size: int) -> Image.Image:
    img = src.convert("RGBA")
    bbox = img.getbbox()
    if bbox:
        img = img.crop(bbox)
    pad = max(1, round(size * 0.06))
    inner = max(1, size - pad * 2)
    ratio = min(inner / img.width, inner / img.height)
    w = max(1, round(img.width * ratio))
    h = max(1, round(img.height * ratio))
    resized = img.resize((w, h), Image.Resampling.LANCZOS)
    canvas = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    canvas.paste(resized, ((size - w) // 2, (size - h) // 2), resized)
    return canvas


def png_bytes(img: Image.Image) -> bytes:
    buf = io.BytesIO()
    img.save(buf, format="PNG")
    return buf.getvalue()


def write_ico(path: Path, frames: list[Image.Image]) -> None:
    """Write a PNG-in-ICO multi-size icon (Vista+)."""
    pngs = [png_bytes(frame) for frame in frames]
    count = len(frames)
    # ICONDIR (6) + ICONDIRENTRY (16 * count) + payloads
    offset = 6 + 16 * count
    entries = []
    for frame, data in zip(frames, pngs):
        w, h = frame.size
        entries.append((w if w < 256 else 0, h if h < 256 else 0, data, offset))
        offset += len(data)

    out = bytearray()
    out += struct.pack("<HHH", 0, 1, count)  # reserved, type=icon, count
    for w, h, data, off in entries:
        # width, height, colorCount, reserved, planes, bitCount, bytesInRes, imageOffset
        out += struct.pack("<BBBBHHII", w, h, 0, 0, 1, 32, len(data), off)
    for _, _, data, _ in entries:
        out += data
    path.write_bytes(out)


def main() -> None:
    src = Image.open(SRC)
    frames = [square_rgba(src, s) for s in SIZES]
    write_ico(OUT_ICO, frames)
    frames[-1].save(OUT_PNG, format="PNG")

    raw = OUT_ICO.read_bytes()
    count = struct.unpack_from("<H", raw, 4)[0]
    print("wrote", OUT_ICO, "entries=", count)
    for i in range(count):
        w, h, _, _, _, bpp, size, _ = struct.unpack_from("<BBBBHHII", raw, 6 + i * 16)
        print(f"  {w or 256}x{h or 256} bpp={bpp} bytes={size}")
    print("wrote", OUT_PNG, "size=", frames[-1].size)


if __name__ == "__main__":
    main()
