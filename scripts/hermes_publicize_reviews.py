#!/usr/bin/env python3
"""Preserve original Hermes reports locally; normalize only private path prefixes."""
from pathlib import Path
import argparse

HEADER = '> 发布副本：仅对本机路径作规范化；原始报告本地保留于 `review/private/`。问题、结论和测试结果未改动。\n\n'
REPLACEMENTS = (
    (str(Path(__file__).resolve().parents[1]), '<PROJECT>'),
    (str(Path.home() / '.hermes'), '<HERMES_HOME>'),
    (str(Path.home() / '.local/bin'), '<LOCAL_BIN>'),
    (str(Path.home()), '<HOME>'),
)


def publicize(path: Path) -> bool:
    text = path.read_text()
    if text.startswith(HEADER):
        return False
    public = text
    for private, replacement in REPLACEMENTS:
        public = public.replace(private, replacement)
    if public == text:
        return False
    original = path.parent / 'private' / path.name
    original.parent.mkdir(parents=True, exist_ok=True)
    if original.exists() and original.read_text() != text:
        raise RuntimeError(f'Original already exists with different contents: {original.name}')
    original.write_text(text)
    path.write_text(HEADER + public)
    return True


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('files', nargs='+')
    args = parser.parse_args()
    for name in args.files:
        if publicize(Path(name)):
            print(Path(name).name)
