import base64
import json
from urllib.parse import parse_qs, urlsplit
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
from types import SimpleNamespace
from xml.etree import ElementTree as ET

from sanitize import NEWZNAB, TORZNAB, Refused, Sanitizer, audit, expectations, parse_xml

SECRET = "FAKE_PRIVATE_KEY_123456789"


def sample(guid="raw-guid-a", title="Example.Movie.2020.1080p"):
    return parse_xml(f'''<rss version="2.0" xmlns:newznab="{NEWZNAB}">
      <channel><title>Indexer</title><link>https://real.example/api?apikey={SECRET}</link>
      <description>Private channel</description><webMaster>person@real.example</webMaster>
      <newznab:response offset="0" total="1"/>
      <item><title>{title}</title><guid isPermaLink="true">https://real.example/details/{guid}</guid>
      <link>https://real.example/api?t=get&amp;id={guid}&amp;apikey={SECRET}</link>
      <comments>https://real.example/comments/{guid}?token={SECRET}</comments>
      <pubDate>Mon, 02 Nov 2026 10:00:00 +0000</pubDate><category>TV &gt; HD</category>
      <description>&lt;a href="https://real.example/?token={SECRET}"&gt;private&lt;/a&gt;</description>
      <enclosure url="https://real.example/api?t=get&amp;id={guid}&amp;apikey={SECRET}" length="1234" type="application/x-nzb"/>
      <newznab:attr name="guid" value="{guid}"/><newznab:attr name="category" value="5000"/>
      <newznab:attr name="category" value="5040"/><newznab:attr name="language" value="English"/>
      <newznab:attr name="language" value="French"/><newznab:attr name="imdb" value="00001234"/>
      <newznab:attr name="season" value="S02"/><newznab:attr name="episode" value="E01"/>
      <newznab:attr name="size" value="1234"/><newznab:attr name="runtime" value="127 min"/>
      </item></channel></rss>'''.encode())


class SanitizerTests(unittest.TestCase):
    def sanitize(self, roots):
        s = Sanitizer({SECRET})
        s.prepare(roots)
        return s, [s.response(r) for r in roots]

    def test_removes_credentials_contacts_and_actionable_urls(self):
        _, outputs = self.sanitize([sample()])
        result = outputs[0]
        self.assertNotIn(SECRET, result)
        self.assertNotIn("real.example", result)
        self.assertNotIn("person@", result)
        self.assertNotIn("raw-guid-a", result)
        self.assertIn("TEST_API_KEY", result)
        self.assertIn("Example.Movie.2020.1080p", result)
        audit(result, [SECRET])

    def test_preserves_repeated_attributes_and_optional_metadata(self):
        _, outputs = self.sanitize([sample()])
        root = ET.fromstring(outputs[0]); attrs = root.findall(".//{" + NEWZNAB + "}attr")
        categories = [a.get("value") for a in attrs if a.get("name") == "category"]
        languages = [a.get("value") for a in attrs if a.get("name") == "language"]
        self.assertEqual(categories, ["5000", "5040"])
        self.assertEqual(languages, ["English", "French"])
        values = {a.get("name"): a.get("value") for a in attrs}
        self.assertEqual((values["imdb"], values["season"], values["episode"]), ("00001234", "S02", "E01"))
        self.assertEqual(expectations(root), {"root": "rss", "item_count": 1,
            "pagination": [{"offset": "0", "total": "1"}],
            "attribute_item_counts": {"category": 1, "episode": 1, "guid": 1, "imdb": 1, "language": 1, "runtime": 1, "season": 1, "size": 1}})

    def test_stable_cross_scenario_identity_and_download_link_equality(self):
        roots = [sample(), sample("raw-guid-b"), sample()]
        _, outputs = self.sanitize(roots)
        parsed = [ET.fromstring(s) for s in outputs]
        guids = [r.findtext("./channel/item/guid") for r in parsed]
        self.assertEqual(guids[0], guids[2])
        self.assertNotEqual(guids[0], guids[1])
        item = parsed[0].find("./channel/item")
        self.assertEqual(item.findtext("link"), item.find("enclosure").get("url"))
        self.assertNotEqual(item.findtext("guid"), item.findtext("link"))
        # Export does not mutate the raw parsed input.
        self.assertIn("real.example", roots[0].findtext("./channel/item/link"))

    def test_synthetic_torznab_magnets_drop_trackers_and_replace_hashes(self):
        root = sample()
        item = root.find("./channel/item")
        item.find("enclosure").set("type", "application/x-bittorrent")
        for attr in item.findall("{" + NEWZNAB + "}attr"):
            attr.tag = "{" + TORZNAB + "}attr"
        source_hash = "a" * 40
        for name, value in (("infohash", source_hash), ("magneturl", f"magnet:?xt=urn:btih:{source_hash}&tr=https://tracker.real.example/{SECRET}"), ("seeders", "3"), ("peers", "8"), ("minimumratio", "0.5")):
            ET.SubElement(item, "{" + TORZNAB + "}attr", name=name, value=value)
        _, outputs = self.sanitize([root])
        self.assertNotIn("tracker.real.example", outputs[0])
        self.assertNotIn(source_hash, outputs[0])
        attrs = {a.get("name"): a.get("value") for a in ET.fromstring(outputs[0]).findall(".//{" + TORZNAB + "}attr")}
        self.assertEqual(parse_qs(urlsplit(attrs["magneturl"]).query), {"xt": ["urn:btih:" + attrs["infohash"]]})
        self.assertEqual((attrs["seeders"], attrs["peers"], attrs["minimumratio"]), ("3", "8", "0.5"))

    def test_guid_attribute_only_items_keep_separate_consistent_identities(self):
        roots = [sample("first-guid"), sample("second-guid")]
        for root in roots:
            item = root.find("./channel/item")
            item.remove(item.find("guid"))
        sanitizer, outputs = self.sanitize(roots)
        self.assertNotIn("", sanitizer.identities)
        identities = []
        for output in outputs:
            item = ET.fromstring(output).find("./channel/item")
            token = next(a.get("value") for a in item.findall("{" + NEWZNAB + "}attr") if a.get("name") == "guid")
            self.assertEqual(token, parse_qs(urlsplit(item.findtext("link")).query)["id"][0])
            identities.append(token)
        self.assertNotEqual(*identities)
        roots = [sample("first-guid"), sample("second-guid")]
        roots[1].find("./channel/item/guid").text = roots[0].findtext("./channel/item/guid")
        with self.assertRaises(Refused):
            self.sanitize(roots)

    def test_request_preserves_query_pairs_and_replaces_key(self):
        s = Sanitizer({SECRET})
        # Construct intentionally fake userinfo at runtime so the negative test
        # is not mistaken for a checked-in credential-bearing URI by scanners.
        userinfo = "fixture-user:fixture-password"
        got = s.request("GET", f"https://{userinfo}@real.example/private/token/api?t=tvsearch&tvdbid=123&cat=5000&cat=5040&season=2&ep=1&apikey={SECRET}")
        self.assertEqual(got, {"method": "GET", "scheme": "https", "host": "indexer.example.invalid", "path": "/api", "query": [
            ["t", "tvsearch"], ["tvdbid", "123"], ["cat", "5000"], ["cat", "5040"], ["season", "2"], ["ep", "1"], ["apikey", "TEST_API_KEY"]]})
        with self.assertRaises(Refused):
            s.request("POST", "https://real.example/api")
        with self.assertRaises(Refused):
            s.request("GET", "https://real.example/api?new_private_parameter=secret")

    def test_known_secret_in_public_title_refuses_export(self):
        with self.assertRaises(Refused):
            self.sanitize([sample(title=SECRET)])
        with self.assertRaises(Refused):
            audit("FAKE%5FPRIVATE%5FKEY%5F123456789", [SECRET])

    def test_unknown_or_malformed_fields_are_not_blindly_copied(self):
        for tag, attrs in (("privateField", {}), (f"{{{NEWZNAB}}}attr", {"name": "uploaderToken", "value": SECRET})):
            root = sample(); ET.SubElement(root.find("./channel/item"), tag, attrs).text = "unsafe"
            with self.assertRaises(Refused):
                self.sanitize([root])
        root = sample(); root.find("./channel/item/enclosure").set("private", SECRET)
        with self.assertRaises(Refused):
            self.sanitize([root])
        root = sample(); root.find("./channel/item/title").text = "https://real.example/?token=unrecognized"
        with self.assertRaises(Refused):
            self.sanitize([root])
        root = sample(); root.find("./channel/item/guid").set("isPermaLink", "unreviewed-private-value")
        with self.assertRaises(Refused):
            self.sanitize([root])

    def test_mixed_content_and_external_entities_rejected(self):
        with self.assertRaises(Refused):
            parse_xml(b'<!DOCTYPE rss [<!ENTITY leak SYSTEM "file:///etc/passwd">]><rss/>')
        with self.assertRaises(Refused):
            parse_xml('<!DOCTYPE rss [<!ENTITY leak "expanded">]><rss>&leak;</rss>'.encode("utf-16"))
        root = sample(); root.find("channel").text = SECRET
        with self.assertRaises(Refused):
            self.sanitize([root])
        with self.assertRaises(Refused):
            parse_xml(b'<rss xmlns:x="http://unreviewed.example"><x:unknown/></rss>')

    def test_capabilities_contacts_scrubbed_parameters_preserved(self):
        root = parse_xml(f'''<caps><server email="private@real.example" url="https://real.example?apikey={SECRET}"/>
          <limits max="100" default="100"/><searching><movie-search available="yes" supportedParams="q,imdbid"/></searching>
          <categories><category id="2000" name="Movies"><subcat id="2040" name="HD"/></category></categories></caps>'''.encode())
        _, outputs = self.sanitize([root]); result = ET.fromstring(outputs[0])
        self.assertEqual(result.find("server").attrib, {"title": "Fixture indexer", "url": "https://indexer.example.invalid"})
        self.assertEqual(result.find("./searching/movie-search").get("supportedParams"), "q,imdbid")
        self.assertNotIn(SECRET, outputs[0])

    def test_nested_capabilities_server_is_rejected(self):
        root = parse_xml(b'<caps><searching><server email="contact@real.example" support="private contact"/></searching></caps>')
        with self.assertRaises(Refused):
            self.sanitize([root])

    def test_credentials_include_userinfo_basic_pairs_and_cookie_values(self):
        import export
        user = "fixture-user"
        password = SECRET
        basic = base64.b64encode(f"{user}:{password}".encode()).decode()
        request = SimpleNamespace(url=f"https://{user}:{password}@real.example/api?t=caps",
            headers=SimpleNamespace(items=lambda **kwargs: [("Authorization", f"Basic {basic}"), ("Cookie", f"session={SECRET}==; other=fixture-cookie")]))
        secrets = export.credentials([SimpleNamespace(request=request)])
        self.assertTrue({user, password, f"{user}:{password}", basic, SECRET + "==", "fixture-cookie"} <= secrets)
        sanitizer = Sanitizer(secrets)
        root = sample(title=password)
        sanitizer.prepare([root])
        with self.assertRaises(Refused):
            sanitizer.response(root)

    def test_listing_never_prints_urls_queries_headers_or_bodies(self):
        import export
        import contextlib
        import io
        request = SimpleNamespace(query={"t": "movie", "q": SECRET}, url=f"https://private.example/?apikey={SECRET}")
        flow = SimpleNamespace(request=request, response=SimpleNamespace(status_code=200, content=SECRET.encode()))
        output = io.StringIO()
        with patch.object(export, "read_flows", return_value=iter([(7, flow)])), patch("sys.argv", ["export.py", "--host", "private.example", "--list"]), contextlib.redirect_stdout(output):
            export.run()
        self.assertNotIn(SECRET, output.getvalue())
        self.assertNotIn("private.example", output.getvalue())
        self.assertEqual(json.loads(output.getvalue()), {"index": 7, "operation": "movie", "query_present": True, "parameter_count": 2, "status": 200, "body_bytes": len(SECRET)})

    def test_cli_refuses_unknown_field_without_publishing_files(self):
        import export
        root = sample(); ET.SubElement(root.find("./channel/item"), "privateField").text = SECRET
        request = SimpleNamespace(method="GET", url=f"https://real.example/api?t=search&apikey={SECRET}", timestamp_start=1793613600,
                                  headers=SimpleNamespace(items=lambda **kwargs: []), query={"t": "search"})
        flow = SimpleNamespace(request=request, response=SimpleNamespace(status_code=200, content=ET.tostring(root), headers={"content-type": "application/xml"}))
        with tempfile.TemporaryDirectory(dir="/fixtures") as directory:
            output = Path(directory) / "not-published"
            argv = ["export.py", "--host", "real.example", "--select", "example=1", "--output", str(output)]
            with patch.object(export, "read_flows", return_value=iter([(1, flow)])), patch("sys.argv", argv):
                with self.assertRaises(Refused):
                    export.run()
            self.assertFalse(output.exists())


if __name__ == "__main__":
    unittest.main()
