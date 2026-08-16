#!/usr/bin/env python3
"""Verify an untrusted CI preview artifact without extracting or executing it."""

from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import sys
import tarfile


MAX_PAYLOAD_SIZE = 512 * 1024 * 1024
MAX_RELEASE_SIZE = 1024 * 1024 * 1024
MAX_ARCHIVE_MEMBERS = 50_000
SHA_PATTERN = re.compile(r"^[0-9a-f]{40}$")
DIGEST_PATTERN = re.compile(r"^[0-9a-f]{64}$")
TIMESTAMP_PATTERN = re.compile(
    r"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$"
)


class VerificationError(RuntimeError):
    pass


def normalized_member_name(name: str) -> str:
    while name.startswith("./"):
        name = name[2:]
    path = PurePosixPath(name)
    if not name or path.is_absolute() or ".." in path.parts or "" in path.parts:
        raise VerificationError(f"unsafe archive member {name!r}")
    return str(path)


def release_members(payload: Path) -> dict[str, tarfile.TarInfo]:
    result = {}
    total_size = 0
    with tarfile.open(payload, mode="r:gz") as archive:
        for count, member in enumerate(archive, start=1):
            if count > MAX_ARCHIVE_MEMBERS:
                raise VerificationError("release contains too many archive members")
            if member.name in (".", "./"):
                continue
            name = normalized_member_name(member.name)
            if name.split("/", 1)[0] not in {"bin", "web", "content", "build.json"}:
                raise VerificationError(f"unexpected release member {name!r}")
            if not (member.isdir() or member.isreg()) or member.size < 0:
                raise VerificationError(
                    f"release member {name!r} is not a regular file or directory"
                )
            if name in result:
                raise VerificationError(f"duplicate release member {name!r}")
            total_size += member.size
            if total_size > MAX_RELEASE_SIZE:
                raise VerificationError("release expands beyond the size limit")
            result[name] = member
    return result


def read_release_json(payload: Path, name: str, maximum: int = 64 * 1024) -> dict:
    with tarfile.open(payload, mode="r:gz") as archive:
        for member in archive:
            if (
                member.name not in (".", "./")
                and normalized_member_name(member.name) == name
            ):
                if not member.isreg() or member.size > maximum:
                    raise VerificationError(f"invalid {name}")
                source = archive.extractfile(member)
                if source is None:
                    raise VerificationError(f"cannot read {name}")
                return json.loads(source.read(maximum + 1))
    raise VerificationError(f"release is missing {name}")


def verify(
    directory: Path, pr: int, run_id: int, run_attempt: int, repository: str, sha: str
) -> dict:
    if (
        isinstance(pr, bool)
        or isinstance(run_id, bool)
        or isinstance(run_attempt, bool)
        or pr < 1
        or run_id < 1
        or run_attempt < 1
        or repository != "anicolao/arknova"
        or not SHA_PATTERN.fullmatch(sha)
    ):
        raise VerificationError("expected identity metadata is invalid")
    entries = list(directory.iterdir())
    if {entry.name for entry in entries} != {"deployment.json", "release.tar.gz"}:
        raise VerificationError(
            "artifact must contain exactly deployment.json and release.tar.gz"
        )
    if any(not entry.is_file() or entry.is_symlink() for entry in entries):
        raise VerificationError("artifact entries must be regular files")

    envelope_path = directory / "deployment.json"
    if envelope_path.stat().st_size > 64 * 1024:
        raise VerificationError("deployment.json exceeds the size limit")
    envelope = json.loads(envelope_path.read_text())
    if set(envelope) != {
        "artifactFormatVersion",
        "pullRequest",
        "pullRequestTitle",
        "sourceRunId",
        "sourceRunAttempt",
        "headRepository",
        "commit",
        "packagingTimestamp",
        "payloadFile",
        "payloadSha256",
        "payloadSize",
    }:
        raise VerificationError("deployment.json has an unexpected schema")
    if not all(
        type(envelope[name]) is int
        for name in (
            "artifactFormatVersion",
            "pullRequest",
            "sourceRunId",
            "sourceRunAttempt",
            "payloadSize",
        )
    ):
        raise VerificationError("deployment envelope integer fields are invalid")
    if (
        envelope["artifactFormatVersion"] != 1
        or envelope["pullRequest"] != pr
        or envelope["sourceRunId"] != run_id
        or envelope["sourceRunAttempt"] != run_attempt
        or envelope["headRepository"] != repository
        or envelope["commit"] != sha
        or envelope["payloadFile"] != "release.tar.gz"
    ):
        raise VerificationError(
            "deployment envelope does not match the trusted workflow event"
        )
    if (
        not isinstance(envelope["pullRequestTitle"], str)
        or len(envelope["pullRequestTitle"]) > 256
    ):
        raise VerificationError("pull request title is invalid")
    if not isinstance(
        envelope["packagingTimestamp"], str
    ) or not TIMESTAMP_PATTERN.fullmatch(envelope["packagingTimestamp"]):
        raise VerificationError("packaging timestamp is invalid")
    if not isinstance(envelope["payloadSha256"], str) or not DIGEST_PATTERN.fullmatch(
        envelope["payloadSha256"]
    ):
        raise VerificationError("payload digest is invalid")

    payload = directory / "release.tar.gz"
    payload_size = payload.stat().st_size
    if (
        payload_size < 1
        or payload_size > MAX_PAYLOAD_SIZE
        or envelope["payloadSize"] != payload_size
    ):
        raise VerificationError("payload size does not match deployment.json")
    digest_builder = hashlib.sha256()
    with payload.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest_builder.update(chunk)
    digest = digest_builder.hexdigest()
    if envelope["payloadSha256"] != digest:
        raise VerificationError("payload digest does not match deployment.json")

    members = release_members(payload)
    if not {"bin/arknova", "web/index.html", "build.json"}.issubset(members):
        raise VerificationError("release is incomplete")
    if any(
        not members[name].isreg()
        for name in ("bin/arknova", "web/index.html", "build.json")
    ):
        raise VerificationError("required release members must be regular files")
    build = read_release_json(payload, "build.json")
    if set(build) != {
        "repository",
        "commit",
        "goVersion",
        "bunVersion",
        "contentVersion",
        "artifactFormatVersion",
    }:
        raise VerificationError("build.json has an unexpected schema")
    if (
        type(build["artifactFormatVersion"]) is not int
        or any(
            not isinstance(build[name], str) or not build[name]
            for name in (
                "repository",
                "commit",
                "goVersion",
                "bunVersion",
                "contentVersion",
            )
        )
        or build["repository"] != "github.com/anicolao/arknova"
        or build["commit"] != sha
        or build["artifactFormatVersion"] != 1
    ):
        raise VerificationError(
            "release build metadata does not match the trusted workflow event"
        )
    return envelope


def main(arguments: list[str]) -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("directory", type=Path)
    parser.add_argument("--pr", required=True, type=int)
    parser.add_argument("--run-id", required=True, type=int)
    parser.add_argument("--run-attempt", required=True, type=int)
    parser.add_argument("--repository", required=True)
    parser.add_argument("--sha", required=True)
    values = parser.parse_args(arguments)
    envelope = verify(
        values.directory,
        values.pr,
        values.run_id,
        values.run_attempt,
        values.repository,
        values.sha,
    )
    print(
        json.dumps(
            {
                "status": "verified",
                "pullRequest": envelope["pullRequest"],
                "commit": envelope["commit"],
            },
            sort_keys=True,
        )
    )
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main(sys.argv[1:]))
    except (
        VerificationError,
        OSError,
        ValueError,
        json.JSONDecodeError,
        tarfile.TarError,
    ) as error:
        print(f"preview artifact verification failed: {error}", file=sys.stderr)
        raise SystemExit(1)
