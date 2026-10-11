#!/usr/bin/env python3
"""go.mod at a carry: upstream's, with ours applied (#198).

Upstream changes go.mod and go.sum some forty times a year, and merging them
by hand is where a carry loses a dependency we raised for a reason (#187,
#200). So what we hold differently is written down in ice9-go-mod.txt - the go
directive, the toolchain, our replaces, and every require whose version is not
upstream's - and a carry rebuilds both files from upstream's:

    scripts/carry_go_mod.py apply <upstream-ref>   # take upstream's, apply ours, tidy, build
    scripts/carry_go_mod.py record                 # after changing a dependency on purpose
    scripts/carry_go_mod.py check                  # the list says what go.mod holds

The base is upstream's commit of the last carry, and moves with the next one.
tests/test_go_mod_recipe.py runs `check` from tools/check.sh.
"""
from __future__ import annotations

import json
import subprocess
import sys
import tempfile
from pathlib import Path

SERVER = Path(__file__).resolve().parent.parent
LIST = SERVER / "ice9-go-mod.txt"
BASE = "708fb953"
HEADER = """\
# What ice9's go.mod holds differently from upstream's at the base of the last
# carry. Written by scripts/carry_go_mod.py record, applied by apply; do not
# edit by hand. github.com/teamgram/proto is replaced by our fork for the MLS
# methods: incoming objects are built from a map the upstream module keeps
# private, so a method cannot be added without it.
"""


class GoMod:
    """What one go.mod says, read by the go tool itself."""

    def __init__(self, text: str):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "go.mod"
            path.write_text(text)
            done = subprocess.run(["go", "mod", "edit", "-json", str(path)], capture_output=True, text=True)
        if done.returncode != 0:
            raise RuntimeError(f"go mod edit -json: {done.stderr.strip()}")
        self.data = json.loads(done.stdout)

    @classmethod
    def at(cls, ref: str) -> "GoMod":
        return cls(git("show", f"{ref}:go.mod"))

    @classmethod
    def here(cls) -> "GoMod":
        return cls((SERVER / "go.mod").read_text())

    def go(self) -> str:
        return self.data.get("Go", "")

    def toolchain(self) -> str:
        return self.data.get("Toolchain", "")

    def requires(self) -> dict[str, str]:
        return {r["Path"]: r["Version"] for r in self.data.get("Require") or []}

    def replaces(self) -> dict[str, str]:
        found = {}
        for r in self.data.get("Replace") or []:
            old = r["Old"]["Path"] + (f"@{r['Old']['Version']}" if r["Old"].get("Version") else "")
            new = r["New"]["Path"] + (f"@{r['New']['Version']}" if r["New"].get("Version") else "")
            found[old] = new
        return found


class Recipe:
    """The lines that turn upstream's go.mod into ours."""

    def __init__(self, lines: list[str]):
        self.lines = lines

    @classmethod
    def between(cls, upstream: GoMod, ours: GoMod) -> "Recipe":
        lines = []
        if ours.go() != upstream.go():
            lines.append(f"go {ours.go()}")
        if ours.toolchain() != upstream.toolchain():
            lines.append(f"toolchain {ours.toolchain()}")
        theirs = upstream.replaces()
        for old, new in sorted(ours.replaces().items()):
            if theirs.get(old) != new:
                lines.append(f"replace {old} => {new}")
        base = upstream.requires()
        for path, version in sorted(ours.requires().items()):
            if base.get(path) != version:
                lines.append(f"require {path} {version}")
        return cls(lines)

    @classmethod
    def read(cls) -> "Recipe":
        return cls([line for line in LIST.read_text().splitlines() if line and not line.startswith("#")])

    def write(self) -> None:
        LIST.write_text(HEADER + "\n".join(self.lines) + "\n")

    def edits(self) -> list[str]:
        arguments = []
        for line in self.lines:
            word, rest = line.split(" ", 1)
            if word == "go":
                arguments.append(f"-go={rest}")
            elif word == "toolchain":
                arguments.append(f"-toolchain={rest}")
            elif word == "replace":
                old, new = rest.split(" => ")
                arguments.append(f"-replace={old}={new}")
            elif word == "require":
                path, version = rest.split(" ")
                arguments.append(f"-require={path}@{version}")
            else:
                raise ValueError(f"{LIST.name}: cannot read {line!r}")
        return arguments


def git(*arguments: str) -> str:
    done = subprocess.run(["git", "-C", str(SERVER), *arguments], capture_output=True, text=True)
    if done.returncode != 0:
        raise RuntimeError(f"git {' '.join(arguments)}: {done.stderr.strip()}")
    return done.stdout


def run(*command: str) -> None:
    print("+ " + " ".join(command))
    subprocess.run(command, cwd=SERVER, check=True)


def main(arguments: list[str]) -> int:
    if arguments == ["record"]:
        Recipe.between(GoMod.at(BASE), GoMod.here()).write()
        print(f"{LIST.name}: {len(Recipe.read().lines)} lines against {BASE}")
        return 0
    if arguments == ["check"]:
        wanted = Recipe.between(GoMod.at(BASE), GoMod.here()).lines
        held = Recipe.read().lines
        missing = [line for line in wanted if line not in held]
        stale = [line for line in held if line not in wanted]
        for line in missing:
            print(f"go.mod holds, {LIST.name} does not: {line}")
        for line in stale:
            print(f"{LIST.name} holds, go.mod does not: {line}")
        if missing or stale:
            print("run scripts/carry_go_mod.py record after a deliberate change")
            return 1
        print(f"{LIST.name} says what go.mod holds against {BASE} ({len(held)} lines)")
        return 0
    if len(arguments) == 2 and arguments[0] == "apply":
        ref = arguments[1]
        recipe = Recipe.read()
        (SERVER / "go.mod").write_text(git("show", f"{ref}:go.mod"))
        (SERVER / "go.sum").write_text(git("show", f"{ref}:go.sum"))
        run("go", "mod", "edit", *recipe.edits())
        run("go", "mod", "tidy")
        run("go", "build", "./...")
        print("now: tools/check.sh, and scripts/carry_go_mod.py record once BASE in this file is moved to the carry")
        return 0
    print(__doc__)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
