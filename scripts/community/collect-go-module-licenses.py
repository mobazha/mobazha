#!/usr/bin/env python3
"""Create a deterministic third-party license bundle from the Go module graph."""

from __future__ import annotations

import argparse
import hashlib
import json
import subprocess
from pathlib import Path
from typing import Any, Iterator


LICENSE_PREFIXES = ("license", "copying", "notice", "copyright", "unlicense")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Collect license and notice files for the modules linked by a Go build target."
    )
    parser.add_argument("output", type=Path, help="New output directory for the license bundle")
    parser.add_argument(
        "--conclusions",
        type=Path,
        default=Path("config/community/license-conclusions.json"),
        help="Reviewed conclusions and exceptional license-file locations",
    )
    parser.add_argument("--package", default=".", help="Go package that builds the release binary")
    parser.add_argument("--tags", default="goolm", help="Comma-separated Go build tags")
    return parser.parse_args()


def decode_json_stream(raw: str) -> Iterator[dict[str, Any]]:
    decoder = json.JSONDecoder()
    offset = 0
    while offset < len(raw):
        while offset < len(raw) and raw[offset].isspace():
            offset += 1
        if offset >= len(raw):
            return
        value, offset = decoder.raw_decode(raw, offset)
        yield value


def module_key(module: dict[str, Any], main_module: str) -> tuple[str, str]:
    if module.get("Main"):
        return ".", ""
    return str(module.get("Path", "")), str(module.get("Version", ""))


def conclusion_index(manifest: dict[str, Any]) -> dict[tuple[str, str], dict[str, Any]]:
    result: dict[tuple[str, str], dict[str, Any]] = {}
    for entry in manifest.get("entries", []):
        key = str(entry.get("name", "")), str(entry.get("version", ""))
        if not key[0] or key in result:
            raise SystemExit(f"invalid or duplicate conclusion entry: {key!r}")
        result[key] = entry
    return result


def root_license_files(module_dir: Path) -> list[Path]:
    return sorted(
        path
        for path in module_dir.iterdir()
        if path.is_file() and path.name.lower().startswith(LICENSE_PREFIXES)
    )


def store_text(output: Path, source: Path) -> tuple[str, str]:
    data = source.read_bytes()
    digest = hashlib.sha256(data).hexdigest()
    relative = f"texts/{digest}.txt"
    destination = output / relative
    if not destination.exists():
        destination.write_bytes(data)
    return digest, relative


def resolve_module_dir(module: dict[str, Any]) -> Path:
    effective = module.get("Replace") or module
    directory_value = effective.get("Dir")
    if directory_value:
        return Path(str(directory_value))

    module_path = str(effective.get("Path", ""))
    version = str(effective.get("Version", ""))
    if not module_path or not version:
        raise SystemExit(f"module source is unavailable: {module_path}@{version}")
    result = subprocess.run(
        ["go", "mod", "download", "-json", f"{module_path}@{version}"],
        check=True,
        capture_output=True,
        text=True,
    )
    downloaded = json.loads(result.stdout)
    if not downloaded.get("Dir"):
        raise SystemExit(f"downloaded module has no source directory: {module_path}@{version}")
    return Path(str(downloaded["Dir"]))


def main() -> int:
    args = parse_args()
    if args.output.exists():
        raise SystemExit(f"output already exists: {args.output}")

    manifest = json.loads(args.conclusions.read_text(encoding="utf-8"))
    conclusions = conclusion_index(manifest)
    raw_packages = subprocess.run(
        ["go", "list", "-deps", "-json", "-tags", args.tags, args.package],
        check=True,
        capture_output=True,
        text=True,
    ).stdout
    module_index: dict[tuple[str, str], dict[str, Any]] = {}
    for package in decode_json_stream(raw_packages):
        module = package.get("Module")
        if not module:
            continue
        key = str(module.get("Path", "")), str(module.get("Version", ""))
        module_index[key] = module
    modules = list(module_index.values())
    main_modules = [str(item.get("Path", "")) for item in modules if item.get("Main")]
    if len(main_modules) != 1:
        raise SystemExit("expected exactly one main Go module")

    args.output.mkdir(parents=True)
    (args.output / "texts").mkdir()
    package_records: list[dict[str, Any]] = []

    for module in sorted(modules, key=lambda item: (str(item.get("Path", "")), str(item.get("Version", "")))):
        key = module_key(module, main_modules[0])
        entry = conclusions.get(key, {})
        module_dir = resolve_module_dir(module)

        candidates = root_license_files(module_dir)
        for relative in entry.get("moduleLicenseFiles", []):
            candidate = module_dir / str(relative)
            if not candidate.is_file():
                raise SystemExit(f"missing reviewed module license file: {key!r} {relative}")
            candidates.append(candidate)
        for relative in entry.get("licenseFiles", []):
            candidate = Path(str(relative))
            if not candidate.is_file():
                raise SystemExit(f"missing repository license override: {key!r} {relative}")
            candidates.append(candidate)

        unique_candidates = sorted(set(candidates), key=lambda path: str(path))
        if not unique_candidates:
            raise SystemExit(f"no license or notice files found: {key[0]}@{key[1]}")

        files: list[dict[str, str]] = []
        seen_hashes: set[str] = set()
        for candidate in unique_candidates:
            digest, bundled_path = store_text(args.output, candidate)
            if digest in seen_hashes:
                continue
            seen_hashes.add(digest)
            try:
                source_name = str(candidate.relative_to(module_dir))
            except ValueError:
                source_name = str(candidate)
            files.append(
                {
                    "source": source_name,
                    "sha256": digest,
                    "bundledPath": bundled_path,
                }
            )

        record: dict[str, Any] = {
            "module": key[0],
            "version": key[1],
            "licenseFiles": files,
        }
        if entry.get("licenseConcluded"):
            record["licenseConcluded"] = entry["licenseConcluded"]
        replacement = module.get("Replace")
        if replacement:
            record["replacement"] = {
                "module": str(replacement.get("Path", "")),
                "version": str(replacement.get("Version", "")),
            }
        package_records.append(record)

    bundle = {
        "schemaVersion": 1,
        "mainModule": main_modules[0],
        "buildPackage": args.package,
        "buildTags": args.tags,
        "moduleCount": len(package_records),
        "uniqueLicenseTextCount": len(list((args.output / "texts").iterdir())),
        "packages": package_records,
    }
    (args.output / "index.json").write_text(json.dumps(bundle, indent=2) + "\n", encoding="utf-8")
    print(
        f"collected {bundle['moduleCount']} modules and "
        f"{bundle['uniqueLicenseTextCount']} unique license/notice texts"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
