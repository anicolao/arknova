import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("verify_preview_artifact.py")
SPEC = importlib.util.spec_from_file_location("verify_preview_artifact", SCRIPT)
verify_artifact = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(verify_artifact)


class VerifyPreviewArtifactTests(unittest.TestCase):
    sha = "a" * 40

    def tar_bytes(self, files):
        output = io.BytesIO()
        with tarfile.open(fileobj=output, mode="w:gz") as archive:
            for name, content in files:
                member = tarfile.TarInfo(name)
                member.size = len(content)
                archive.addfile(member, io.BytesIO(content))
        return output.getvalue()

    def artifact(self, directory, release_files=None):
        build = {
            "repository": "github.com/anicolao/arknova",
            "commit": self.sha,
            "goVersion": "1.26.4",
            "bunVersion": "1.3.13",
            "contentVersion": "none",
            "artifactFormatVersion": 1,
        }
        if release_files is None:
            release_files = [
                ("./bin/arknova", b"elf"),
                ("./web/index.html", b"table"),
                ("./build.json", json.dumps(build).encode()),
            ]
        payload = self.tar_bytes(release_files)
        (directory / "release.tar.gz").write_bytes(payload)
        envelope = {
            "artifactFormatVersion": 1,
            "pullRequest": 7,
            "pullRequestTitle": "Candidate",
            "sourceRunId": 1234,
            "sourceRunAttempt": 1,
            "headRepository": "anicolao/arknova",
            "commit": self.sha,
            "packagingTimestamp": "2026-08-16T00:00:00Z",
            "payloadFile": "release.tar.gz",
            "payloadSha256": hashlib.sha256(payload).hexdigest(),
            "payloadSize": len(payload),
        }
        (directory / "deployment.json").write_text(json.dumps(envelope))

    def test_accepts_exact_run_identity(self):
        with tempfile.TemporaryDirectory() as root:
            directory = Path(root)
            self.artifact(directory)
            result = verify_artifact.verify(
                directory, 7, 1234, 1, "anicolao/arknova", self.sha
            )
            self.assertEqual(result["commit"], self.sha)

    def test_rejects_mismatched_run_and_traversal(self):
        with tempfile.TemporaryDirectory() as root:
            directory = Path(root)
            self.artifact(directory)
            with self.assertRaises(verify_artifact.VerificationError):
                verify_artifact.verify(
                    directory, 7, 9999, 1, "anicolao/arknova", self.sha
                )
            self.artifact(directory, [("../escape", b"bad")])
            with self.assertRaises(verify_artifact.VerificationError):
                verify_artifact.verify(
                    directory, 7, 1234, 1, "anicolao/arknova", self.sha
                )

    def test_rejects_extra_artifact_entry(self):
        with tempfile.TemporaryDirectory() as root:
            directory = Path(root)
            self.artifact(directory)
            (directory / "extra").write_text("unexpected")
            with self.assertRaises(verify_artifact.VerificationError):
                verify_artifact.verify(
                    directory, 7, 1234, 1, "anicolao/arknova", self.sha
                )

    def test_rejects_weakly_typed_identity(self):
        with tempfile.TemporaryDirectory() as root:
            directory = Path(root)
            self.artifact(directory)
            envelope_path = directory / "deployment.json"
            envelope = json.loads(envelope_path.read_text())
            envelope["sourceRunAttempt"] = 1.0
            envelope_path.write_text(json.dumps(envelope))
            with self.assertRaises(verify_artifact.VerificationError):
                verify_artifact.verify(
                    directory, 7, 1234, 1, "anicolao/arknova", self.sha
                )

    def test_rejects_required_member_that_is_not_a_file(self):
        with tempfile.TemporaryDirectory() as root:
            directory = Path(root)
            build = {
                "repository": "github.com/anicolao/arknova",
                "commit": self.sha,
                "goVersion": "go1.26.4",
                "bunVersion": "1.3.13",
                "contentVersion": "none",
                "artifactFormatVersion": 1,
            }
            self.artifact(
                directory,
                [
                    ("bin/arknova", b"elf"),
                    ("web/index.html", b"table"),
                    ("build.json", json.dumps(build).encode()),
                ],
            )
            payload = io.BytesIO()
            with tarfile.open(fileobj=payload, mode="w:gz") as archive:
                binary = tarfile.TarInfo("bin/arknova")
                binary.type = tarfile.DIRTYPE
                archive.addfile(binary)
                for name, content in [
                    ("web/index.html", b"table"),
                    ("build.json", json.dumps(build).encode()),
                ]:
                    member = tarfile.TarInfo(name)
                    member.size = len(content)
                    archive.addfile(member, io.BytesIO(content))
            payload_bytes = payload.getvalue()
            (directory / "release.tar.gz").write_bytes(payload_bytes)
            envelope_path = directory / "deployment.json"
            envelope = json.loads(envelope_path.read_text())
            envelope["payloadSize"] = len(payload_bytes)
            envelope["payloadSha256"] = hashlib.sha256(payload_bytes).hexdigest()
            envelope_path.write_text(json.dumps(envelope))
            with self.assertRaises(verify_artifact.VerificationError):
                verify_artifact.verify(
                    directory, 7, 1234, 1, "anicolao/arknova", self.sha
                )


if __name__ == "__main__":
    unittest.main()
