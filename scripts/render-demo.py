"""Render exported real Xelif ANSI frames as an annotated PNG and GIF.

Requires Pillow and a Nerd Font Mono TTF. This is a reproducible View-renderer
walkthrough with sample files, not an OS terminal recording or a mockup.
"""

import argparse
import json
import re
import unicodedata
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

ESCAPE = re.compile(r"\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07]*(?:\x07|\x1b\\)")
BACKGROUND = (24, 24, 24)
FOREGROUND = (221, 221, 221)
PALETTE = [(24, 24, 24), (205, 88, 88), (115, 180, 128), (211, 181, 109),
           (123, 161, 204), (175, 141, 190), (115, 182, 185), (221, 221, 221)]


def color256(number):
    if number < 16:
        return PALETTE[number % 8]
    if number >= 232:
        level = 8 + (number - 232) * 10
        return (level,) * 3
    number -= 16
    levels = (0, 95, 135, 175, 215, 255)
    return (levels[number // 36], levels[number // 6 % 6], levels[number % 6])


def render(frame, font):
    cell, line, margin, top = round(font.getlength("M")), 24, 28, 66
    width, height = 120 * cell + margin * 2, 28 * line + top + 92
    image = Image.new("RGB", (width, height), BACKGROUND)
    draw = ImageDraw.Draw(image)
    small = font.font_variant(size=16)
    draw.text((margin, 23), "XELIF", font=font, fill=FOREGROUND)
    draw.text((width - margin, 26), "SAMPLE WORKSPACE", font=small, fill=(155, 155, 155), anchor="rt")
    draw.line((margin, 52, width - margin, 52), fill=(58, 58, 58))
    baseline = top + font.getmetrics()[0]
    for row, text in enumerate(frame["content"].splitlines()[:28]):
        fg, bg, reverse, column, end = FOREGROUND, BACKGROUND, False, 0, 0

        def write(run):
            nonlocal column
            for character in run:
                if unicodedata.combining(character):
                    continue
                cells = 2 if unicodedata.east_asian_width(character) in "WF" else 1
                x, y = margin + column * cell, top + row * line
                foreground, background = (bg, fg) if reverse else (fg, bg)
                draw.rectangle((x, y, x + cells * cell - 1, y + line - 1), fill=background)
                draw.text((x, baseline + row * line), character, font=font, fill=foreground, anchor="ls")
                column += cells

        for match in ESCAPE.finditer(text):
            write(text[end:match.start()])
            token = match.group()
            if token.startswith("\x1b[") and token.endswith("m"):
                values = [int(value or 0) for value in token[2:-1].replace(":", ";").split(";")]
                index = 0
                while index < len(values):
                    value = values[index]
                    if value == 0:
                        fg, bg, reverse = FOREGROUND, BACKGROUND, False
                    elif value == 7:
                        reverse = True
                    elif value == 27:
                        reverse = False
                    elif value == 39:
                        fg = FOREGROUND
                    elif value == 49:
                        bg = BACKGROUND
                    elif value in (38, 48) and index + 1 < len(values):
                        color = None
                        if values[index + 1] == 2 and index + 4 < len(values):
                            color = tuple(values[index + 2:index + 5])
                            index += 4
                        elif values[index + 1] == 5 and index + 2 < len(values):
                            color = color256(values[index + 2])
                            index += 2
                        if color is not None:
                            if value == 38:
                                fg = color
                            else:
                                bg = color
                    elif 30 <= value <= 37 or 90 <= value <= 97:
                        fg = PALETTE[value % 10]
                    elif 40 <= value <= 47 or 100 <= value <= 107:
                        bg = PALETTE[value % 10]
                    index += 1
            end = match.end()
        write(text[end:])
    draw.line((margin, height - 70, width - margin, height - 70), fill=(58, 58, 58))
    draw.text((margin, height - 48), frame["title"], font=small, fill=FOREGROUND)
    draw.text((width - margin, height - 48), "Real Xelif views / sample files", font=small,
              fill=(155, 155, 155), anchor="rt")
    return image


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("frames", type=Path)
    parser.add_argument("font", type=Path)
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    frames = json.loads(args.frames.read_text(encoding="utf-8"))
    font = ImageFont.truetype(str(args.font), 18)
    images = [render(frame, font) for frame in frames]
    args.output.mkdir(parents=True, exist_ok=True)
    images[0].save(args.output / "overview.png")
    images[-2].save(args.output / "commands.png")
    # A shared palette prevents distracting color changes between states.
    palette = images[0].quantize(colors=128)
    indexed = [image.quantize(palette=palette, dither=Image.Dither.NONE) for image in images]
    indexed[0].save(args.output / "walkthrough.gif", save_all=True, append_images=indexed[1:],
                    duration=[frame["duration"] for frame in frames], loop=0, optimize=False)
    print(f"Exported {len(frames)} real application frames to {args.output}")


if __name__ == "__main__":
    main()
