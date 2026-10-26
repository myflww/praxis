#!/usr/bin/env python3
# Copyright 2026 myflww
# SPDX-License-Identifier: Apache-2.0
"""Build the independent pick-topic CLI for six OS/architecture pairs."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile


TARGETS = [(system, arch) for system in ("darwin", "linux", "windows") for arch in ("amd64", "arm64")]


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--go", default="go", help="Go executable (default: go on PATH)")
    args = parser.parse_args()
    executable = shutil.which(os.path.expanduser(args.go))
    if executable is None:
        parser.error("Go was not found; specify the executable with --go")
    executable = str(Path(executable).resolve())

    root = Path(__file__).resolve().parents[1]
    output = root / "skills" / "praxis" / "bin"
    output.mkdir(parents=True, exist_ok=True)
    environment = os.environ.copy()
    environment.update({
        "CGO_ENABLED": "0", "GOWORK": "off", "GOENV": "off",
        "GOTOOLCHAIN": "local", "GOPROXY": "off", "GOSUMDB": "off", "GOFLAGS": "",
    })
    version = subprocess.check_output([executable, "version"], env=environment, text=True).strip()
    manifest = {
        "schema_version": 1,
        "tool": "pick-topic",
        "license": "Apache-2.0",
        "build": {"go_version": version, "cgo_enabled": False, "trimpath": True, "vcs": False, "build_id": ""},
        "source_files": {
            relative: {"sha256": sha256(root / relative)}
            for relative in ("go.mod", "cmd/pick-topic/main.go")
        },
        "files": {},
    }
    # Publish only after every target has built. Temporary products are owned by
    # this invocation and are removed on either success or failure.
    with tempfile.TemporaryDirectory(prefix=".picker-build-", dir=output.parent) as temporary:
        staging = Path(temporary)
        for system, arch in TARGETS:
            name = f"pick-topic-{system}-{arch}" + (".exe" if system == "windows" else "")
            target_environment = environment | {"GOOS": system, "GOARCH": arch}
            subprocess.run(
                [executable, "build", "-mod=readonly", "-trimpath", "-buildvcs=false",
                 "-ldflags=-s -w -buildid=", "-o", str(staging / name), "./cmd/pick-topic"],
                cwd=root, env=target_environment, check=True,
            )
            os.chmod(staging / name, 0o755)
            manifest["files"][name] = {
                "sha256": sha256(staging / name), "size": (staging / name).stat().st_size,
                "goos": system, "goarch": arch,
            }
        (staging / "checksums.json").write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
        for name in [*manifest["files"], "checksums.json"]:
            os.replace(staging / name, output / name)
    print(f"Built {len(TARGETS)} pick-topic executables and checksums.json")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, subprocess.CalledProcessError) as error:
        print(f"build-pickers: {error}", file=sys.stderr)
        raise SystemExit(1)
