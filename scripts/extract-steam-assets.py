#!/usr/bin/env python3
"""Export named Texture2D objects from Ark Nova's Unity Addressable bundles."""

import argparse
import json
import os
import re
import sys
from pathlib import Path

import UnityPy


DEFAULT_DATA = Path.home() / (
    "Library/Application Support/Steam/steamapps/common/Ark Nova/"
    "ArkNova.app/Contents/Resources/Data"
)

BUNDLE_GROUPS = {
    "cards": ["base-cardfront_assets_.bundle"],
    "playmats": ["playmat_assets_.bundle"],
    "maps": ["atlas_mapthumbs_assets_.bundle", "texture_t_map*_assets_.bundle"],
    "icons": ["base-scenes_atlas_icons_assets_.bundle", "atlas_frontendicons_assets_.bundle"],
}


def safe_name(value: str) -> str:
    value = re.sub(r"[^A-Za-z0-9._-]+", "_", value).strip("._")
    return value or "unnamed"


def find_bundles(data_dir: Path, groups: list[str]) -> list[Path]:
    bundle_dir = data_dir / "StreamingAssets/aa/StandaloneOSX"
    if not bundle_dir.is_dir():
        raise FileNotFoundError(f"Addressables directory not found: {bundle_dir}")

    if "all" in groups:
        return sorted(bundle_dir.glob("*.bundle"))

    patterns: list[str] = []
    for group in groups:
        patterns.extend(BUNDLE_GROUPS[group])

    bundles = {path for pattern in patterns for path in bundle_dir.glob(pattern)}
    return sorted(bundles)


def export_bundle(bundle: Path, output: Path) -> list[dict[str, object]]:
    print(f"Reading {bundle.name} ...", flush=True)
    environment = UnityPy.load(str(bundle))
    destination = output / safe_name(bundle.stem)
    destination.mkdir(parents=True, exist_ok=True)
    records: list[dict[str, object]] = []
    used_names: dict[str, int] = {}

    textures = [obj for obj in environment.objects if obj.type.name == "Texture2D"]
    for index, obj in enumerate(textures, start=1):
        texture = obj.read()
        base = safe_name(getattr(texture, "m_Name", "") or f"texture_{obj.path_id}")
        duplicate = used_names.get(base, 0)
        used_names[base] = duplicate + 1
        filename = f"{base}_{duplicate + 1}.png" if duplicate else f"{base}.png"
        target = destination / filename

        try:
            image = texture.image
            image.save(target, format="PNG")
            records.append(
                {
                    "bundle": bundle.name,
                    "name": getattr(texture, "m_Name", ""),
                    "pathId": obj.path_id,
                    "file": str(target.relative_to(output)),
                    "width": image.width,
                    "height": image.height,
                }
            )
        except Exception as error:  # Continue so one unsupported texture does not lose the set.
            print(f"  warning: could not export {base}: {error}", file=sys.stderr)

        if index % 50 == 0 or index == len(textures):
            print(f"  exported {index}/{len(textures)} textures", flush=True)

    return records


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--data", type=Path, default=DEFAULT_DATA, help="ArkNova.app Resources/Data path")
    parser.add_argument("--output", type=Path, default=Path("steam-assets"), help="output directory")
    parser.add_argument(
        "--group",
        choices=[*BUNDLE_GROUPS, "all"],
        action="append",
        help="asset group to extract; may be repeated (default: cards)",
    )
    args = parser.parse_args()

    groups = args.group or ["cards"]
    data_dir = args.data.expanduser().resolve()
    output = args.output.expanduser().resolve()

    try:
        bundles = find_bundles(data_dir, groups)
    except FileNotFoundError as error:
        parser.error(str(error))
    if not bundles:
        parser.error(f"no matching bundles found below {data_dir}")

    output.mkdir(parents=True, exist_ok=True)
    manifest: list[dict[str, object]] = []
    for bundle in bundles:
        manifest.extend(export_bundle(bundle, output))

    manifest_path = output / "manifest.json"
    manifest_path.write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    print(f"Exported {len(manifest)} textures to {output}")
    print(f"Manifest: {manifest_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
