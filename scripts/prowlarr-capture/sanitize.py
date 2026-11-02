"""Allowlisted upstream XML fixture export. No network access or credential verification.

Unknown fields are rejected, not copied. Optional descriptions/contact details are
replaced. Public release titles/IDs remain; review these before publishing.
"""
import base64
import copy
import datetime as dt
import email.utils
import hashlib
import html
import json
import re
from urllib.parse import parse_qsl, unquote, urlsplit
from xml.etree import ElementTree as ET

NEWZNAB = "http://www.newznab.com/DTD/2010/feeds/attributes/"
TORZNAB = "http://torznab.com/schemas/2015/feed"
ATOM = "http://www.w3.org/2005/Atom"
for prefix, uri in (("newznab", NEWZNAB), ("torznab", TORZNAB), ("atom", ATOM)):
    ET.register_namespace(prefix, uri)


class Refused(ValueError):
    """A deliberately generic error; never include raw data in its message."""


def local(tag):
    if tag.startswith("{"):
        namespace, name = tag[1:].split("}", 1)
        if namespace not in (NEWZNAB, TORZNAB, ATOM):
            raise Refused("Unknown XML namespace")
        return name
    return tag


def text(value):
    value = value or ""
    if re.search(r"https?://|magnet:|@|-----BEGIN|(?:api[_-]?key|passkey|password|token)\s*[:=]", value, re.I):
        raise Refused("Unexpected URL, contact, or credential in public metadata")
    return value


def integer(value):
    if not re.fullmatch(r"\d+", value or ""):
        raise Refused("Invalid numeric metadata")
    return value


def date(value):
    try:
        email.utils.parsedate_to_datetime(value)
    except (ValueError, TypeError):
        try:
            dt.datetime.fromisoformat(value.replace("Z", "+00:00"))
        except (ValueError, TypeError, AttributeError):
            raise Refused("Invalid date metadata") from None
    return value


def parse_xml(body):
    if len(body) > 8 * 1024 * 1024:
        raise Refused("Oversized XML")
    try:
        decoded = body.decode("utf-8-sig")
    except UnicodeDecodeError:
        raise Refused("Only UTF-8 XML is supported") from None
    if re.search(r"<!\s*(?:DOCTYPE|ENTITY)", decoded, re.I):
        raise Refused("DTD/entity declaration")
    try:
        root = ET.fromstring(body)
    except ET.ParseError:
        raise Refused("Invalid response XML") from None
    # XML fixtures only; reject undocumented attributes/elements below.
    for element in root.iter():
        local(element.tag)
    return root


def audit(content, secrets=()):
    """Check known credentials, including common encoded representations."""
    decoded = content
    for _ in range(3):
        decoded = unquote(html.unescape(decoded))
    if "-----BEGIN" in decoded:
        raise Refused("Private-key material")
    for secret in secrets:
        if not secret:
            continue
        forms = (secret, base64.b64encode(secret.encode()).decode())
        if any(value in content or value in decoded for value in forms):
            raise Refused("Known credential remained after sanitization")
    for match in re.finditer(r"https?://[^\s\"'<>]+", decoded, re.I):
        if match.group() in (NEWZNAB, TORZNAB, ATOM):
            continue  # Namespace identifiers, not fetchable fixture targets.
        parsed = urlsplit(match.group())
        if parsed.username or parsed.password or not (parsed.hostname or "").endswith(".invalid"):
            raise Refused("Actionable external URL remained")
    if re.search(r"magnet:\?", decoded, re.I):
        # Magnet fixtures are rebuilt with synthetic infohashes by this module.
        for match in re.finditer(r"magnet:\?[^\s\"'<>]+", decoded, re.I):
            params = dict(parse_qsl(match.group()[8:]))
            if not re.fullmatch(r"urn:btih:[0-9a-f]{40}", params.get("xt", "")):
                raise Refused("Unexpected magnet format")


class Sanitizer:
    def __init__(self, secrets=()):
        self.secrets = set(secrets)
        self.identities = {}
        self.url_aliases = {}

    @staticmethod
    def identity(item):
        identities = {e.get("value", "") for e in item if local(e.tag) == "attr" and e.get("name") == "guid"}
        if len(identities) > 1:
            raise Refused("Conflicting item identity attributes")
        identity = next(iter(identities), "") or item.findtext("guid", "")
        if not identity:
            raise Refused("Item has no stable identity")
        return identity

    def prepare(self, roots):
        """One shared mapping preserves identity across ALL exported scenarios."""
        for root in roots:
            for item in root.findall("./channel/item"):
                guid = item.findtext("guid", "")
                enclosure = item.find("enclosure")
                download = enclosure.get("url", "") if enclosure is not None else item.findtext("link", "")
                identity = self.identity(item)
                token = self.identities.setdefault(identity, f"release-{len(self.identities) + 1:06d}")
                if guid and self.identities.setdefault(guid, token) != token:
                    raise Refused("Conflicting item identity aliases")
                for old, new in ((guid, f"https://indexer.example.invalid/details/{token}"),
                                 (download, f"https://indexer.example.invalid/api?t=get&id={token}&apikey=TEST_API_KEY")):
                    if old:
                        previous = self.url_aliases.setdefault(old, new)
                        if previous != new:
                            raise Refused("Inconsistent URL identity across cases")

    def request(self, method, url):
        if method != "GET":
            raise Refused("Only GET protocol fixtures are supported")
        query = []
        for key, value in parse_qsl(urlsplit(url).query, keep_blank_values=True):
            key = key.lower()
            if key == "apikey":
                value = "TEST_API_KEY"
            elif key in ("t", "q", "cat", "extended", "limit", "offset", "imdbid", "tvdbid", "tmdbid", "rid", "tvmazeid", "season", "ep", "o", "minage", "maxage", "minsize", "maxsize"):
                value = text(value)
            else:
                raise Refused("Unreviewed request parameter")
            query.append([key, value])
        result = {"method": "GET", "scheme": "https", "host": "indexer.example.invalid", "path": "/api", "query": query}
        audit(json.dumps(result), self.secrets)
        return result

    def caps(self, root):
        allowed = {
            "caps": set(), "limits": {"max", "default"},
            "retention": {"days"}, "registration": {"available", "open"},
            "searching": set(), "search": {"available", "supportedParams", "searchEngine"},
            "movie-search": {"available", "supportedParams"}, "tv-search": {"available", "supportedParams"},
            "audio-search": {"available", "supportedParams"}, "music-search": {"available", "supportedParams"},
            "book-search": {"available", "supportedParams"}, "categories": set(),
            "category": {"id", "name", "description"}, "subcat": {"id", "name", "description"},
            "groups": set(), "group": {"id", "name", "description", "lastupdate"},
            "genres": set(), "genre": {"id", "name", "categoryid"},
        }
        for child in list(root):
            if local(child.tag) == "server":
                child.clear()
                child.attrib.update(title="Fixture indexer", url="https://indexer.example.invalid")
        for e in root.iter():
            name = local(e.tag)
            if name == "server":
                if e not in root:
                    raise Refused("Nested capabilities server")
                continue
            if name not in allowed or set(e.attrib) - allowed[name]:
                raise Refused("Unreviewed capabilities field")
            e.text = text(e.text)
            e.tail = text(e.tail)
            for key, value in e.attrib.items():
                text(value)
        return root

    def rss(self, root):
        if set(root.attrib) - {"version"} or root.get("version", "2.0") != "2.0":
            raise Refused("Unreviewed RSS attributes")
        channel = root.find("channel")
        if channel is None or len(list(root)) != 1 or channel.attrib:
            raise Refused("Unexpected RSS channel")
        for e in list(channel):
            name = local(e.tag)
            if name == "item":
                self.item(e)
            elif name == "response":
                if set(e.attrib) - {"offset", "total"} or len(e):
                    raise Refused("Unexpected pagination fields")
                for v in e.attrib.values():
                    integer(v)
                e.text = None
            elif name == "link" and e.tag == f"{{{ATOM}}}link":
                e.clear()
                e.attrib.update(href="https://indexer.example.invalid/api", rel="self", type="application/rss+xml")
            elif name in ("title", "description", "link", "language", "webMaster", "image", "category"):
                e.clear()
                if name == "link":
                    e.text = "https://indexer.example.invalid/"
                elif name == "image":
                    ET.SubElement(e, "url").text = "https://indexer.example.invalid/logo.png"
                elif name == "webMaster":
                    e.text = "fixture@example.invalid"
                elif name == "language":
                    e.text = "en"
                else:
                    e.text = "Sanitized indexer fixture"
            else:
                raise Refused("Unreviewed RSS channel field")
            e.tail = None
        return root

    def item(self, item):
        if item.attrib:
            raise Refused("Unreviewed item attributes")
        token = self.identities.get(self.identity(item))
        if not token:
            raise Refused("Unprepared item identity")
        for e in item:
            name = local(e.tag)
            if len(e):
                raise Refused("Nested item metadata is not supported")
            if name == "attr":
                if set(e.attrib) != {"name", "value"}:
                    raise Refused("Unexpected release attribute shape")
                self.attribute(e, token)
                e.text = None
            elif name == "enclosure":
                if set(e.attrib) - {"url", "length", "type"}:
                    raise Refused("Unreviewed enclosure attributes")
                if e.get("type") not in ("application/x-nzb", "application/x-bittorrent"):
                    raise Refused("Unsupported enclosure type")
                integer(e.get("length", "0"))
                e.set("url", self.url_aliases[e.get("url", "")])
                e.text = None
            elif name in ("guid", "link", "comments"):
                if set(e.attrib) - ({"isPermaLink"} if name == "guid" else set()):
                    raise Refused("Unreviewed URL attributes")
                if name == "guid" and e.get("isPermaLink", "false") not in ("true", "false"):
                    raise Refused("Invalid GUID permalink flag")
                original = e.text or ""
                if name == "guid" and not urlsplit(original).scheme:
                    e.text = token
                else:
                    e.text = self.url_aliases.get(original, f"https://indexer.example.invalid/{name}/{token}")
            elif name in ("title", "category", "pubDate", "description"):
                if e.attrib:
                    raise Refused("Unreviewed scalar attributes")
                e.text = "Sanitized description" if name == "description" else date(e.text) if name == "pubDate" else text(e.text)
            else:
                raise Refused("Unreviewed release field")
            e.tail = None

    def attribute(self, e, token):
        name = e.get("name", "").lower()
        value = e.get("value", "")
        if name == "guid":
            value = token
        elif name in ("coverurl", "poster"):
            value = f"https://indexer.example.invalid/posters/{token}.jpg"
        elif name == "infohash":
            value = hashlib.sha1(token.encode()).hexdigest()
        elif name == "magneturl":
            value = "magnet:?xt=urn:btih:" + hashlib.sha1(token.encode()).hexdigest()
        elif name in ("category", "size", "rageid", "tvdbid", "tvmazeid", "tmdbid", "traktid", "grabs", "comments", "password", "thumbsup", "thumbsdown", "files", "seeders", "peers", "leechers", "minimumseedtime", "imdbyear"):
            value = integer(value)
        elif name in ("imdb", "imdbid"):
            if not re.fullmatch(r"(?:tt)?\d+", value):
                raise Refused("Unexpected IMDb metadata")
        elif name == "runtime":
            if not re.fullmatch(r"\d+(?: min)?", value):
                raise Refused("Unexpected runtime encoding")
        elif name in ("season", "episode"):
            if not re.fullmatch(r"[SE]?\d+(?:[-, /]?[SE]?\d+)*", value, re.I):
                raise Refused("Unexpected season/episode encoding")
        elif name in ("imdbscore", "downloadvolumefactor", "uploadvolumefactor", "minimumratio"):
            if not re.fullmatch(r"\d+(?:\.\d+)?", value):
                raise Refused("Unexpected decimal metadata")
        elif name == "usenetdate":
            value = date(value)
        elif name in ("imdbplot", "imdbactors", "imdbdirector", "imdbtagline"):
            value = "Sanitized public metadata"
        elif name in ("imdbtitle", "genre", "language", "subs", "releasegroup"):
            value = text(value)
        else:
            raise Refused("Unreviewed release attribute")
        e.set("value", value)

    def response(self, source):
        root = copy.deepcopy(source)
        if root.tag == "caps":
            self.caps(root)
        elif root.tag == "rss":
            self.rss(root)
        else:
            raise Refused("Only successful caps/RSS responses are supported")
        # Text between elements is formatting only; unknown tails must not survive.
        for e in root.iter():
            if len(e) and e.text and e.text.strip():
                raise Refused("Unexpected mixed XML content")
            if e.tail and e.tail.strip():
                raise Refused("Unexpected XML tail content")
        ET.indent(root, space="  ")
        output = ET.tostring(root, encoding="utf-8", xml_declaration=True).decode() + "\n"
        audit(output, self.secrets)
        return output


def expectations(root):
    items = root.findall("./channel/item")
    fields = {}
    for item in items:
        for name in {e.get("name") for e in item if local(e.tag) == "attr"}:
            fields[name] = fields.get(name, 0) + 1
    pagination = [dict(e.attrib) for e in root.iter() if local(e.tag) == "response"]
    return {"root": root.tag, "item_count": len(items), "pagination": pagination, "attribute_item_counts": dict(sorted(fields.items()))}
