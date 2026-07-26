"""Generate NSIS installer/uninstaller sidebar bitmaps (164x314) from the app logo.

The "Completing Setup" (finish) page and the welcome page use this sidebar image.
Run: python desktop/assets/make-sidebar.py
"""
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont

HERE = Path(__file__).resolve().parent
LOGO = HERE / "desktop-logo.png"
W, H = 164, 314

# Brand indigo gradient (top -> bottom)
TOP = (124, 118, 255)
BOTTOM = (74, 63, 214)


def vertical_gradient(w, h, top, bottom):
    base = Image.new("RGB", (w, h), top)
    px = base.load()
    for y in range(h):
        t = y / max(1, h - 1)
        r = round(top[0] + (bottom[0] - top[0]) * t)
        g = round(top[1] + (bottom[1] - top[1]) * t)
        b = round(top[2] + (bottom[2] - top[2]) * t)
        for x in range(w):
            px[x, y] = (r, g, b)
    return base


def load_font(size):
    for name in ("segoeuib.ttf", "arialbd.ttf", "seguisb.ttf", "arial.ttf"):
        try:
            return ImageFont.truetype(name, size)
        except OSError:
            continue
    return ImageFont.load_default()


def main():
    bg = vertical_gradient(W, H, TOP, BOTTOM)
    draw = ImageDraw.Draw(bg, "RGBA")

    # Soft white disc behind the logo for contrast.
    disc = 108
    cx, cy = W // 2, 118
    draw.ellipse(
        [cx - disc // 2, cy - disc // 2, cx + disc // 2, cy + disc // 2],
        fill=(255, 255, 255, 255),
    )

    # Logo, trimmed of transparent margins, centered on the disc.
    logo = Image.open(LOGO).convert("RGBA")
    bbox = logo.getbbox()
    if bbox:
        logo = logo.crop(bbox)
    target = 74
    ratio = min(target / logo.width, target / logo.height)
    logo = logo.resize(
        (max(1, round(logo.width * ratio)), max(1, round(logo.height * ratio))),
        Image.LANCZOS,
    )
    bg.paste(logo, (cx - logo.width // 2, cy - logo.height // 2), logo)

    # Wordmark.
    title_font = load_font(26)
    sub_font = load_font(12)
    title = "TwinTube"
    tb = draw.textbbox((0, 0), title, font=title_font)
    draw.text(
        ((W - (tb[2] - tb[0])) / 2, 196),
        title,
        font=title_font,
        fill=(255, 255, 255, 255),
    )

    sub = "Watch together"
    sb = draw.textbbox((0, 0), sub, font=sub_font)
    draw.text(
        ((W - (sb[2] - sb[0])) / 2, 232),
        sub,
        font=sub_font,
        fill=(255, 255, 255, 210),
    )

    out_installer = HERE / "installerSidebar.bmp"
    out_uninstaller = HERE / "uninstallerSidebar.bmp"
    bg.save(out_installer, "BMP")
    bg.save(out_uninstaller, "BMP")
    print("wrote", out_installer)
    print("wrote", out_uninstaller)


if __name__ == "__main__":
    main()
