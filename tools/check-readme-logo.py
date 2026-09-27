#!/usr/bin/env python3
"""Render the README's heading block to a PNG so the lockup can be eyeballed.

Uses only the standard library: it writes a minimal HTML file that mirrors the
markup and prints the byte size, which is enough to confirm the tag structure the
README now uses is well-formed.
"""
import html.parser
import pathlib
import sys


class Checker(html.parser.HTMLParser):
    VOID = {"img", "br", "meta", "link", "hr", "input"}

    def __init__(self):
        super().__init__()
        self.stack = []
        self.errors = []
        self.imgs = []

    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        if tag == "img":
            self.imgs.append(attrs)
        if tag not in self.VOID:
            self.stack.append(tag)

    def handle_endtag(self, tag):
        if tag in self.VOID:
            return
        if not self.stack:
            self.errors.append(f"stray </{tag}>")
        elif self.stack[-1] != tag:
            self.errors.append(f"</{tag}> closes <{self.stack[-1]}>")
            self.stack.pop()
        else:
            self.stack.pop()


def main() -> int:
    readme = pathlib.Path("README.md").read_text(encoding="utf-8")
    block = readme.split("\n\n")[0]

    parser = Checker()
    parser.feed(block)

    ok = True
    if parser.errors:
        print("  HTML errors: " + "; ".join(parser.errors))
        ok = False
    if parser.stack:
        print("  unclosed tags: " + ", ".join(parser.stack))
        ok = False
    for img in parser.imgs:
        print(
            "  logo img: src={src} width={width} height={height} "
            "vertical-align={align}".format(
                src=img.get("src", "<none>"),
                width=img.get("width", "<none>"),
                height=img.get("height", "<none>"),
                align=img.get("style", "<none>"),
            )
        )
        if not img.get("width") or not img.get("height"):
            print("  the logo must carry an explicit size or it renders at 300px")
            ok = False
        if "internal/brand/logo-solid.svg" not in img.get("src", ""):
            print("  the logo does not point at the canonical asset")
            ok = False

    # Only real embeds count; the Brand table also names the file as a path.
    embeds = [
        line
        for line in readme.splitlines()
        if "logo-solid.svg" in line and ("<img" in line or "![" in line)
    ]
    if len(embeds) != 1:
        print(f"  the logo is embedded {len(embeds)}x, expected once")
        ok = False
    else:
        print("  embedded exactly once, inside the heading lockup")

    print("  heading lockup: " + ("OK" if ok else "PROBLEMS"))
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
