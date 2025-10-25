#!/usr/bin/env python3
"""
ClaraCore Release Script

Build UI, cross-compile Go binaries for:
  - macOS (darwin/arm64)
  - Windows (windows/amd64)
  - Linux (linux/amd64)

Package artifacts and publish a GitHub release with assets.

Requirements:
  - Python 3.8+
  - Go toolchain installed
  - Node.js/npm for UI build
  - Environment variable: GITHUB_TOKEN (or GH_TOKEN)

Usage examples:
  python3 scripts/release.py --tag v0.1.0 --name "ClaraCore v0.1.0"
  python3 scripts/release.py --tag v0.1.0 --prerelease --notes "Beta preview"
"""

import argparse
import json
import os
import platform
import shutil
import subprocess
import sys
import tarfile
import tempfile
import time
from pathlib import Path
from urllib.request import Request, urlopen
from urllib.error import HTTPError
from urllib.parse import quote, urlencode

ROOT = Path(__file__).resolve().parents[1]
DIST = ROOT / "dist"


def log(msg: str) -> None:
    print(msg, flush=True)


def run(cmd: str, cwd: Path | None = None, env: dict | None = None) -> None:
    log(f"$ {cmd}")
    result = subprocess.run(cmd, shell=True, cwd=str(cwd) if cwd else None, env=env)
    if result.returncode != 0:
        raise SystemExit(f"Command failed ({result.returncode}): {cmd}")


def ensure_tools() -> None:
    # npm
    r = subprocess.run(["npm", "--version"], shell=True, capture_output=True, text=True)
    if r.returncode != 0:
        log("WARNING: npm not found; UI build will fail.")
    # go
    r = subprocess.run(["go", "version"], shell=True, capture_output=True, text=True)
    if r.returncode != 0:
        raise SystemExit("Go toolchain not found in PATH")


def build_ui() -> None:
    log("==> Building UI")
    ui_dir = ROOT / "ui"
    if not ui_dir.exists():
        log("UI directory not found; skipping UI build")
        return
    # install deps only if node_modules missing
    if not (ui_dir / "node_modules").exists():
        run("npm install", cwd=ui_dir)
    run("npm run build", cwd=ui_dir)
    out = ROOT / "proxy" / "ui_dist"
    if not out.exists():
        raise SystemExit("UI build output missing at proxy/ui_dist")


def get_repo_slug() -> str:
    # Try to parse from git remote
    try:
        r = subprocess.run(
            ["git", "config", "--get", "remote.origin.url"],
            capture_output=True,
            text=True,
            cwd=str(ROOT),
        )
        if r.returncode == 0:
            url = r.stdout.strip()
            # formats: https://github.com/owner/repo.git or git@github.com:owner/repo.git
            if url.startswith("git@github.com:"):
                slug = url.split(":", 1)[1]
            elif url.startswith("https://github.com/"):
                slug = url.split("https://github.com/", 1)[1]
            else:
                slug = url
            if slug.endswith(".git"):
                slug = slug[:-4]
            if slug.count("/") == 1:
                return slug
    except Exception:
        pass
    # Fallback to env
    slug = os.environ.get("REPO", os.environ.get("GITHUB_REPOSITORY", ""))
    if not slug:
        raise SystemExit("Unable to determine repo slug. Set REPO=owner/repo or configure git remote origin.")
    return slug


def build_target(goos: str, goarch: str, out_dir: Path) -> Path:
    out_dir.mkdir(parents=True, exist_ok=True)
    exe = "claracore.exe" if goos == "windows" else "claracore"
    out_path = out_dir / exe
    env = os.environ.copy()
    env["GOOS"] = goos
    env["GOARCH"] = goarch
    env["CGO_ENABLED"] = "0"  # prefer portable binaries
    # Build from repo root so embed picks up built UI assets
    run(f"go build -o {out_path} .", cwd=ROOT, env=env)
    if not out_path.exists():
        raise SystemExit(f"Expected binary not found: {out_path}")
    return out_path


def stage_and_zip(goos: str, goarch: str, binary_path: Path, tag: str) -> Path:
    # Stage minimal files
    name = f"claracore-{tag}-{goos}-{goarch}"
    stage = DIST / name
    if stage.exists():
        shutil.rmtree(stage)
    stage.mkdir(parents=True)

    # Copy artifacts
    shutil.copy2(binary_path, stage / binary_path.name)
    for fname in ("README.md", "LICENSE.md", "config.example.yaml"):
        src = ROOT / fname
        if src.exists():
            shutil.copy2(src, stage / fname)

    # Zip
    zip_path = DIST / f"{name}.zip"
    if zip_path.exists():
        zip_path.unlink()
    shutil.make_archive(str(zip_path.with_suffix("")), "zip", root_dir=stage)
    return zip_path


def gh_api(token: str, method: str, url: str, body: dict | None = None, headers: dict | None = None) -> dict:
    h = {
        "Accept": "application/vnd.github+json",
        "Authorization": f"Bearer {token}",
        "X-GitHub-Api-Version": "2022-11-28",
        "Content-Type": "application/json",
        "User-Agent": "claracore-release-script",
    }
    if headers:
        h.update(headers)
    data = None
    if body is not None:
        data = json.dumps(body).encode("utf-8")
    req = Request(url, data=data, headers=h, method=method)
    with urlopen(req) as resp:
        text = resp.read().decode("utf-8")
        return json.loads(text) if text else {}


def gh_upload_asset(token: str, upload_url_template: str, asset_path: Path) -> None:
    # upload_url like: https://uploads.github.com/repos/owner/repo/releases/ID/assets{?name,label}
    base = upload_url_template.split("{", 1)[0]
    url = f"{base}?{urlencode({'name': asset_path.name})}"
    headers = {
        "Accept": "application/vnd.github+json",
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/zip",
        "User-Agent": "claracore-release-script",
    }
    data = asset_path.read_bytes()
    req = Request(url, data=data, headers=headers, method="POST")
    try:
        with urlopen(req) as resp:
            _ = resp.read()
            log(f"Uploaded asset: {asset_path.name}")
    except HTTPError as e:
        # If asset with same name exists, surface a helpful message
        raise SystemExit(f"Asset upload failed ({e.code}): {e.read().decode('utf-8', 'ignore')}")


def create_or_get_release(token: str, repo: str, tag: str, name: str, notes: str, draft: bool, prerelease: bool) -> dict:
    # Try get by tag
    try:
        return gh_api(token, "GET", f"https://api.github.com/repos/{repo}/releases/tags/{quote(tag)}")
    except HTTPError:
        pass
    target = subprocess.run(["git", "rev-parse", "HEAD"], capture_output=True, text=True, cwd=str(ROOT))
    target_commitish = (target.stdout or "").strip() or "main"
    payload = {
        "tag_name": tag,
        "name": name or tag,
        "body": notes or "",
        "draft": draft,
        "prerelease": prerelease,
        "target_commitish": target_commitish,
    }
    return gh_api(token, "POST", f"https://api.github.com/repos/{repo}/releases", payload)


def main() -> None:
    parser = argparse.ArgumentParser(description="Build and publish ClaraCore multi-platform release")
    parser.add_argument("--tag", required=True, help="Tag name for the release, e.g. v0.1.0")
    parser.add_argument("--name", default="", help="Release name (defaults to tag)")
    parser.add_argument("--notes", default="", help="Release notes/body")
    parser.add_argument("--draft", action="store_true", help="Create as draft release")
    parser.add_argument("--prerelease", action="store_true", help="Mark release as prerelease")
    parser.add_argument("--skip-upload", action="store_true", help="Build artifacts only, do not publish")
    parser.add_argument("--repo", default="", help="Override repo slug owner/repo (auto-detected if empty)")
    args = parser.parse_args()

    token = os.environ.get("GITHUB_TOKEN") or os.environ.get("GH_TOKEN")
    if not args.skip_upload and not token:
        raise SystemExit("GITHUB_TOKEN (or GH_TOKEN) is required for uploading releases.")

    ensure_tools()

    # Fresh dist dir
    if DIST.exists():
        shutil.rmtree(DIST)
    DIST.mkdir(parents=True)

    # Build UI once (embedded into backend at build time)
    build_ui()

    # Build targets
    builds: list[tuple[str, str, Path]] = []
    targets = [
        ("darwin", "arm64"),
        ("windows", "amd64"),
        ("linux", "amd64"),
    ]
    for goos, goarch in targets:
        out_dir = DIST / f"build-{goos}-{goarch}"
        bin_path = build_target(goos, goarch, out_dir)
        builds.append((goos, goarch, bin_path))

    # Package
    zips: list[Path] = []
    for goos, goarch, bin_path in builds:
        zips.append(stage_and_zip(goos, goarch, bin_path, args.tag))

    if args.skip_upload:
        log("Artifacts built. Skipping GitHub upload as requested.")
        for z in zips:
            log(f" - {z}")
        return

    repo = args.repo or get_repo_slug()
    rel = create_or_get_release(token, repo, args.tag, args.name or args.tag, args.notes, args.draft, args.prerelease)
    upload_url = rel.get("upload_url", "")
    if not upload_url:
        raise SystemExit(f"Could not get upload_url from release response: {json.dumps(rel)}")

    # Upload assets
    for z in zips:
        gh_upload_asset(token, upload_url, z)

    log("\nRelease complete.")
    log(f"Repository: https://github.com/{repo}")
    log(f"Tag: {args.tag}")


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("Interrupted", file=sys.stderr)
        sys.exit(130)



