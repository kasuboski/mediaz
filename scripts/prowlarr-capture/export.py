#!/usr/bin/env python3
"""Select mitmproxy HTTP flows and publish sanitized upstream XML fixtures."""
import argparse
import base64
import binascii
import datetime as dt
import json
import os
from pathlib import Path
import re
import shutil
import sys
import tempfile
from urllib.parse import parse_qsl, unquote, urlsplit

from sanitize import Refused, Sanitizer, audit, expectations, parse_xml


def read_flows(host):
    from mitmproxy import io
    # Indices are global, ordered by capture filename then flow order. Append-only
    # captures retain their indices; archive complete recordings before exporting.
    index = 0
    for path in sorted(Path("/captures").glob("*.mitm")):
        try:
            with path.open("rb") as source:
                for flow in io.FlowReader(source).stream():
                    request = getattr(flow, "request", None)
                    if request and request.pretty_host == host:
                        yield index, flow
                    index += 1
        except io.FlowReadException:
            raise Refused("Capture incomplete or invalid; stop recording or retry with a complete copy") from None


def credentials(flows):
    secrets = set()
    for flow in flows:
        request = flow.request
        parsed = urlsplit(request.url)
        secrets.update(unquote(value) for value in (parsed.username, parsed.password) if value)
        for key, value in parse_qsl(parsed.query):
            if re.search(r"key|token|pass|auth|session|cookie", key, re.I) and value:
                secrets.add(value)
        for key, value in request.headers.items(multi=True):
            if re.search(r"authorization|api.?key|token|cookie", key, re.I):
                secrets.add(value)
                if "cookie" in key.lower():
                    secrets.update(part.split("=", 1)[1].strip() for part in value.split(";") if "=" in part)
                elif " " in value:
                    scheme, token = value.split(" ", 1)
                    secrets.add(token)
                    if scheme.lower() == "basic":
                        try:
                            decoded = base64.b64decode(token, validate=True).decode("utf-8")
                        except (ValueError, binascii.Error, UnicodeDecodeError):
                            raise Refused("Invalid Basic authorization") from None
                        if ":" not in decoded:
                            raise Refused("Invalid Basic authorization pair")
                        secrets.update(value for value in (decoded, *decoded.split(":", 1)) if value)
    if any(len(s) < 4 for s in secrets):
        raise Refused("Credential too short for reliable residual auditing")
    return secrets


def run():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--host", required=True, help="Exact upstream host; never a Prowlarr host")
    parser.add_argument("--list", action="store_true", help="List flow indices without query values, URLs, or bodies")
    parser.add_argument("--select", action="append", default=[], metavar="SCENARIO=INDEX")
    parser.add_argument("--output", default="/fixtures/selected", help="New directory beneath /fixtures")
    parser.add_argument("--provider", default="custom", help="Public provider label, e.g. nzbgeek")
    parser.add_argument("--protocol", choices=("newznab", "torznab"), default="newznab")
    parser.add_argument("--prowlarr-version", default="unknown")
    args = parser.parse_args()
    flows = dict(read_flows(args.host))
    if args.list:
        for index, flow in flows.items():
            request, response = flow.request, flow.response
            operation = request.query.get("t")
            if operation not in ("caps", "search", "movie", "tvsearch"):
                operation = "other"
            print(json.dumps({"index": index, "operation": operation, "query_present": bool(request.query.get("q")),
                              "parameter_count": len(request.query), "status": response.status_code if response else None,
                              "body_bytes": len(response.content or b"") if response else 0}))
        return
    if not args.select or not re.fullmatch(r"[a-z0-9-]+", args.provider):
        raise Refused("Select named flows and a public provider label")
    if not re.fullmatch(r"[a-zA-Z0-9.-]+", args.prowlarr_version):
        raise Refused("Invalid public version label")
    selected, names = [], set()
    for selection in args.select:
        name, separator, index = selection.partition("=")
        if not separator or not re.fullmatch(r"[a-z][a-z0-9-]*", name) or name in names or not index.isdigit():
            raise Refused("Invalid or duplicate scenario selector")
        flow = flows.get(int(index))
        if flow is None or flow.response is None or flow.response.status_code != 200:
            raise Refused("Selected flow missing or not a successful response")
        if flow.request.query.get("t") not in ("caps", "search", "movie", "tvsearch"):
            raise Refused("Selected flow is not a supported search/caps operation")
        media_type = flow.response.headers.get("content-type", "").split(";", 1)[0].strip().lower()
        if media_type not in ("application/xml", "text/xml", "application/rss+xml", "application/atom+xml"):
            raise Refused("Selected response has an unsupported XML media type")
        selected.append((name, flow, parse_xml(flow.response.content or b"")))
        names.add(name)
    sanitizer = Sanitizer(credentials(flows.values()))
    sanitizer.prepare([root for _, _, root in selected])
    files, manifest = {}, {"format_version": 1, "provider": args.provider, "protocol": args.protocol,
                           "source": "captured upstream indexer HTTP responses, not Prowlarr reserialized output",
                           "prowlarr_version": args.prowlarr_version, "scenarios": []}
    for name, flow, root in selected:
        request = sanitizer.request(flow.request.method, flow.request.url)
        response = sanitizer.response(root)
        clean = parse_xml(response.encode())
        media_type = flow.response.headers.get("content-type").split(";", 1)[0].strip().lower()
        metadata = {"status": flow.response.status_code, "content_type": media_type, "capture_date": dt.datetime.fromtimestamp(flow.request.timestamp_start, dt.timezone.utc).date().isoformat(),
                    "expect": expectations(clean)}
        files[f"{name}/request.json"] = json.dumps(request, indent=2) + "\n"
        files[f"{name}/response.xml"] = response
        files[f"{name}/scenario.json"] = json.dumps(metadata, indent=2) + "\n"
        manifest["scenarios"].append(name)
    files["manifest.json"] = json.dumps(manifest, indent=2) + "\n"
    for content in files.values():
        audit(content, sanitizer.secrets)
    target = Path(args.output).resolve()
    if Path("/fixtures") not in target.parents or target.exists():
        raise Refused("Output must be a new directory below /fixtures")
    target.parent.mkdir(parents=True, exist_ok=True)
    temporary = Path(tempfile.mkdtemp(prefix=".fixture-export-", dir=target.parent))
    try:
        for name, content in files.items():
            dest = temporary / name
            dest.parent.mkdir(parents=True, exist_ok=True)
            dest.write_text(content)
        temporary.rename(target)
    except BaseException:
        shutil.rmtree(temporary, ignore_errors=True)
        raise
    print(f"Exported {len(selected)} sanitized scenarios. Review and secret-scan before copying into Git.")


if __name__ == "__main__":
    os.umask(0o077)
    try:
        run()
    except Refused as error:
        print(f"Export refused: {error}", file=sys.stderr)
        sys.exit(1)
    except Exception:
        # Libraries can embed request data in exception messages. Do not print it.
        print("Export failed; no raw exception details are printed to protect credentials.", file=sys.stderr)
        sys.exit(1)
