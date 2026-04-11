#!/usr/bin/env python3
# =========================================================
# Script Name: Clone-Index.py
# Description: Mirror Rust crates from crates.io to a local directory.
# Author: APTlantis Team
# Creation Date: 2024-11-15
#
# Dependencies:
# - git
#
# Usage:
#   python Clone-Index.py [options]
# =========================================================

import argparse
import logging
import shutil
import subprocess
import sys
import time
from pathlib import Path

DEFAULT_ROOT = Path.home() / "Rust-Crates"
DEFAULT_INDEX_DIR = DEFAULT_ROOT / "crates.io-index"
DEFAULT_OUTPUT_DIR = DEFAULT_ROOT / "crates"
DEFAULT_LOG_PATH = DEFAULT_ROOT / "crate-download.log"
DEFAULT_MANIFEST_PATH = DEFAULT_ROOT / "manifest.jsonl"
DEFAULT_BUNDLES_DIR = DEFAULT_ROOT / "bundles"
DEFAULT_THREADS = 128
DEFAULT_LISTEN_ADDR = ":9090"

def parse_arguments():
    """Parse command line arguments.

    Returns:
        argparse.Namespace: Parsed command line arguments
    """
    parser = argparse.ArgumentParser(description="Mirror Rust crates from crates.io")
    parser.add_argument(
        "--index-dir",
        type=str,
        default=str(DEFAULT_INDEX_DIR),
        help="Path to local crates.io index (default: %(default)s)",
    )
    parser.add_argument(
        "--output-dir",
        type=str,
        default=str(DEFAULT_OUTPUT_DIR),
        help="Directory where .crate files will be saved (default: %(default)s)",
    )
    parser.add_argument(
        "--log-path",
        type=str,
        default=str(DEFAULT_LOG_PATH),
        help="Path to log file for this wrapper (default: %(default)s)",
    )
    parser.add_argument(
        "--threads",
        type=int,
        default=DEFAULT_THREADS,
        help="Number of download threads (mapped to -concurrency for Download-Crates) (default: %(default)s)",
    )
    parser.add_argument(
        "--rate-limit",
        type=float,
        default=0.5,
        help="Deprecated: no direct equivalent in Download-Crates; kept for compatibility",
    )
    parser.add_argument(
        "--resume",
        action="store_true",
        help="Deprecated: no direct equivalent; kept for compatibility",
    )
    parser.add_argument(
        "--verify",
        action="store_true",
        help="Deprecated: verification handled by Download-Crates; kept for compatibility",
    )
    parser.add_argument(
        "--skip-index-update",
        action="store_true",
        help="Skip updating the crates.io index",
    )
    parser.add_argument(
        "--non-interactive",
        "--yes",
        dest="non_interactive",
        action="store_true",
        help="Do not prompt; proceed automatically (CI-friendly)",
    )
    parser.add_argument(
        "--log-level",
        type=str,
        default="info",
        choices=["debug", "info", "warning", "error"],
        help="Logging level for this wrapper (default: info)",
    )
    parser.add_argument(
        "--downloader-path",
        type=str,
        default="",
        help="Path to Download-Crates binary; if empty, auto-detect or fallback to 'go run'",
    )
    parser.add_argument(
        "--include-yanked",
        action="store_true",
        help="Include yanked crate versions when building the download list",
    )
    parser.add_argument(
        "--verify-existing",
        action="store_true",
        help="Re-hash and verify existing crate files instead of trusting them during update runs",
    )
    parser.add_argument(
        "--bundle",
        action="store_true",
        help="Enable rolling tar.zst bundle output while downloading",
    )
    parser.add_argument(
        "--bundle-mode",
        type=str,
        default="only",
        choices=["only", "add"],
        help="How bundled downloads are stored when --bundle is enabled (default: %(default)s)",
    )
    parser.add_argument(
        "--bundle-size-gb",
        type=int,
        default=8,
        help="Target bundle size in GB when --bundle is enabled (default: %(default)s)",
    )
    parser.add_argument(
        "--bundles-out",
        type=str,
        default=str(DEFAULT_BUNDLES_DIR),
        help="Directory for bundle archives and bundle manifests (default: %(default)s)",
    )
    parser.add_argument(
        "--manifest",
        type=str,
        default=str(DEFAULT_MANIFEST_PATH),
        help="Path for the downloader JSONL audit log (default: %(default)s)",
    )
    parser.add_argument(
        "--listen",
        type=str,
        default=DEFAULT_LISTEN_ADDR,
        help="Metrics listener address for the downloader (default: %(default)s; use empty string to disable)",
    )
    parser.add_argument(
        "--progress-interval",
        type=str,
        default="5s",
        help="Periodic downloader progress logging interval, e.g. 5s (default: %(default)s)",
    )
    parser.add_argument(
        "--progress-every",
        type=int,
        default=0,
        help="Log downloader progress every N processed records (default: %(default)s, disabled)",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Validate inputs and show the effective downloader configuration without downloading",
    )
    return parser.parse_args()


def setup_logging(level_str: str, log_path: str = ""):
    level = {
        "debug": logging.DEBUG,
        "info": logging.INFO,
        "warning": logging.WARNING,
        "error": logging.ERROR,
    }.get(str(level_str).lower(), logging.INFO)

    logger = logging.getLogger()
    logger.handlers.clear()
    logger.setLevel(level)
    fmt = logging.Formatter("%(asctime)s %(levelname)s %(message)s")

    sh = logging.StreamHandler(sys.stdout)
    sh.setFormatter(fmt)
    logger.addHandler(sh)

    if log_path:
        try:
            Path(log_path).expanduser().resolve().parent.mkdir(parents=True, exist_ok=True)
        except Exception as exc:
            logging.getLogger(__name__).warning(
                "Failed to create log directory %s: %s", log_path, exc
            )
        try:
            fh = logging.FileHandler(log_path, encoding="utf-8")
            fh.setFormatter(fmt)
            logger.addHandler(fh)
        except Exception as e:
            # Fallback to console-only if file handler fails
            logging.getLogger(__name__).warning(
                "Failed to open log file %s: %s", log_path, e
            )


def find_downloader(provided_path: str):
    """Find the Download-Crates executable or fall back to `go run`.

    Returns:
        tuple[str, list[str]] | None: (mode, base_cmd). mode is 'binary' or 'go-run'.
    """
    script_dir = Path(__file__).parent.absolute()

    if provided_path:
        p = Path(provided_path)
        if p.is_file():
            return ("binary", [str(p)])
        logging.warning(
            "--downloader-path %s does not exist; attempting auto-detect.",
            provided_path,
        )

    candidates = [
        script_dir / "Download-Crates.exe",
        script_dir / "Download-Crates",
        script_dir / "download-crates.exe",
        script_dir / "download-crates",
    ]
    for c in candidates:
        if c.is_file():
            return ("binary", [str(c)])

    for name in ("Download-Crates", "download-crates"):
        which_bin = shutil.which(name)
        if which_bin:
            return ("binary", [which_bin])

    go_bin = shutil.which("go")
    if go_bin:
        return ("go-run", [go_bin, "run", str(script_dir / "cmd" / "download-crates")])

    return None


def ensure_directory(path):
    """Ensure that a directory exists.

    Args:
        path: Path to the directory to create
    """
    Path(path).mkdir(parents=True, exist_ok=True)
    logging.info(f"Ensured directory exists: {path}")


def git_head(index_path: Path):
    try:
        result = subprocess.run(
            ["git", "rev-parse", "HEAD"],
            cwd=str(index_path),
            check=True,
            capture_output=True,
            text=True,
        )
    except subprocess.CalledProcessError:
        return ""
    return result.stdout.strip()


def clone_or_update_index(index_dir, skip_update=False):
    """Clone or update the crates.io index repository.

    Args:
        index_dir: Path to the crates.io index directory
        skip_update: Whether to skip updating the index if it already exists

    Returns:
        bool: True if successful, False otherwise
    """
    index_path = Path(index_dir)

    if index_path.exists() and (index_path / ".git").exists():
        logging.info(f"Crates.io index already exists at {index_dir}")
        if skip_update:
            logging.info("Skipping index update as requested")
            return True

        previous_head = git_head(index_path)
        logging.info("Updating the crates.io index...")
        try:
            result = subprocess.run(
                ["git", "pull"],
                cwd=str(index_path),
                check=True,
                capture_output=True,
                text=True,
            )
            current_head = git_head(index_path)
            summary = result.stdout.strip() or result.stderr.strip()
            if previous_head and current_head:
                if previous_head == current_head:
                    logging.info("Index update complete: already current")
                else:
                    logging.info("Index update complete: new commit fetched")
                    logging.info("Index commit changed: %s -> %s", previous_head, current_head)
            if summary:
                logging.debug(summary)
            return True
        except subprocess.CalledProcessError as e:
            logging.error("Error updating crates.io index: %s", e)
            logging.error("Error output: %s", e.stderr)
            return False
    else:
        logging.info(f"Cloning crates.io index to {index_dir}...")
        try:
            # Create parent directory if it doesn't exist
            index_path.parent.mkdir(parents=True, exist_ok=True)

            # Clone the repository
            result = subprocess.run(
                [
                    "git",
                    "clone",
                    "https://github.com/rust-lang/crates.io-index.git",
                    str(index_dir),
                ],
                check=True,
                capture_output=True,
                text=True,
            )
            current_head = git_head(index_path)
            logging.info("Index clone complete%s", f": {current_head}" if current_head else "")
            logging.debug(result.stdout.strip())
            return True
        except subprocess.CalledProcessError as e:
            logging.error("Error cloning crates.io index: %s", e)
            logging.error("Error output: %s", e.stderr)
            return False


def run_mirror_crates(args):
    """Invoke the Go Download-Crates tool with mapped arguments.

    Returns:
        bool: True if successful, False otherwise
    """
    found = find_downloader(args.downloader_path)
    if not found:
        logging.error(
            "Could not find Download-Crates binary and no Go toolchain available for 'go run'."
        )
        return False

    mode, base = found
    cmd = list(base)
    # Map known arguments to Download-Crates flags
    cmd.extend(
        [
            "-index-dir",
            str(args.index_dir),
            "-out",
            str(args.output_dir),
            "-concurrency",
            str(args.threads),
            "-manifest",
            str(args.manifest),
        ]
    )
    if args.include_yanked:
        cmd.append("-include-yanked")
    if args.verify_existing or args.verify:
        cmd.append("-verify-existing")
    if args.bundle:
        cmd.extend(
            [
                "-bundle",
                "-bundle-mode",
                str(args.bundle_mode),
                "-bundle-size-gb",
                str(args.bundle_size_gb),
                "-bundles-out",
                str(args.bundles_out),
            ]
        )
    if args.progress_interval:
        cmd.extend(["-progress-interval", str(args.progress_interval)])
    if args.progress_every > 0:
        cmd.extend(["-progress-every", str(args.progress_every)])
    if args.listen != "":
        cmd.extend(["-listen", str(args.listen)])
    else:
        cmd.extend(["-listen", ""])
    if args.dry_run:
        cmd.append("-dry-run")

    logging.info("Starting downloader (%s): %s", mode, " ".join(cmd))
    try:
        subprocess.run(cmd, check=True)
        return True
    except subprocess.CalledProcessError as e:
        logging.error("Downloader exited with error: %s", e)
        return False


def main():
    """Main function to mirror Rust crates from crates.io."""
    args = parse_arguments()
    setup_logging(args.log_level, args.log_path)
    started = time.monotonic()

    logging.info(
        "Wrapper starting: index=%s out=%s threads=%s include_yanked=%s verify_existing=%s bundle=%s bundle_mode=%s manifest=%s listen=%s",
        args.index_dir,
        args.output_dir,
        args.threads,
        args.include_yanked,
        args.verify_existing or args.verify,
        args.bundle,
        args.bundle_mode,
        args.manifest,
        args.listen if args.listen != "" else "(disabled)",
    )

    # Ensure output directory exists
    ensure_directory(args.output_dir)
    if args.bundle:
        ensure_directory(args.bundles_out)
    Path(args.manifest).expanduser().resolve().parent.mkdir(parents=True, exist_ok=True)

    # Clone or update the crates.io index
    if not args.skip_index_update:
        if not clone_or_update_index(
            args.index_dir, skip_update=args.skip_index_update
        ):
            logging.error("Failed to clone or update crates.io index. Exiting.")
            return 1

    # Start download decision
    if not args.non_interactive:
        while True:
            reply = input("Start downloading crates now? [y/n]: ").strip().lower()
            if reply in ("y", "yes"):
                break
            elif reply in ("n", "no"):
                logging.info("Download aborted by user. Exiting without downloading.")
                return 0
            else:
                logging.warning("Please enter 'y' or 'n'.")
    else:
        logging.info("Non-interactive mode: proceeding without prompt.")

    # Run the Go Download-Crates tool
    if not run_mirror_crates(args):
        logging.error("Failed to run Download-Crates. Exiting.")
        return 1

    elapsed = time.monotonic() - started
    logging.info(
        "Rust crates mirroring completed successfully in %.1fs. Output=%s manifest=%s bundles=%s",
        elapsed,
        args.output_dir,
        args.manifest,
        args.bundles_out if args.bundle else "(disabled)",
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
