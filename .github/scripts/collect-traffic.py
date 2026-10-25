#!/usr/bin/env python3
"""Save GitHub traffic and download stats to CSV files.

GitHub keeps repository traffic (views, clones, referrers, popular pages) for
only 14 days. Run this daily to keep the history:

  - views.csv / clones.csv   one row per day, merged by date (newest values win)
  - referrers.csv / paths.csv  daily snapshot of the 14-day top lists
  - downloads.csv            daily snapshot of cumulative release downloads
  - repo.csv                 daily snapshot of stars, forks, watchers, issues

Traffic endpoints need a token that can read repository administration
(TRAFFIC_TOKEN). Without it, only downloads and repo stats are saved.

Usage: collect-traffic.py <owner/repo> <output-dir>
Stdlib only.
"""

import csv
import datetime as dt
import json
import os
import sys
import urllib.error
import urllib.request

API = "https://api.github.com"


def api_get(path, token=None):
    req = urllib.request.Request(API + path, headers={
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
        "User-Agent": "klew-traffic-stats",
    })
    if token:
        req.add_header("Authorization", "Bearer " + token)
    with urllib.request.urlopen(req, timeout=30) as resp:
        return json.load(resp)


def read_csv(path):
    if not os.path.exists(path):
        return []
    with open(path, newline="") as f:
        return list(csv.DictReader(f))


def write_csv(path, fields, rows):
    with open(path, "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=fields, lineterminator="\n")
        w.writeheader()
        for r in rows:
            w.writerow({k: r.get(k, "") for k in fields})


def merge_daily(existing, fresh, key="date"):
    """Merge rows by date; fresh rows replace existing ones for the same day."""
    by_key = {r[key]: r for r in existing}
    for r in fresh:
        by_key[r[key]] = r
    return [by_key[k] for k in sorted(by_key)]


def replace_snapshot(existing, fresh, today, key="snapshot_date"):
    """Replace today's snapshot rows (re-runs on the same day don't duplicate)."""
    kept = [r for r in existing if r.get(key) != today]
    return kept + fresh


def daily_rows(payload, items_key):
    """Traffic API per-day series → [{date, count, uniques}]."""
    out = []
    for item in payload.get(items_key, []):
        out.append({
            "date": item["timestamp"][:10],
            "count": str(item.get("count", 0)),
            "uniques": str(item.get("uniques", 0)),
        })
    return out


def download_rows(releases, today):
    rows = []
    for rel in releases:
        for asset in rel.get("assets", []):
            rows.append({
                "snapshot_date": today,
                "tag": rel.get("tag_name", ""),
                "asset": asset.get("name", ""),
                "downloads": str(asset.get("download_count", 0)),
            })
    return rows


def main(argv):
    if len(argv) != 3:
        print(__doc__, file=sys.stderr)
        return 2
    repo, out_dir = argv[1], argv[2]
    os.makedirs(out_dir, exist_ok=True)
    token = os.environ.get("TRAFFIC_TOKEN", "").strip()
    today = dt.datetime.now(dt.timezone.utc).date().isoformat()

    # Public data — no special token needed.
    releases = api_get(f"/repos/{repo}/releases?per_page=100", token or None)
    path = os.path.join(out_dir, "downloads.csv")
    write_csv(path, ["snapshot_date", "tag", "asset", "downloads"],
              replace_snapshot(read_csv(path), download_rows(releases, today), today))

    info = api_get(f"/repos/{repo}", token or None)
    path = os.path.join(out_dir, "repo.csv")
    write_csv(path, ["snapshot_date", "stars", "forks", "watchers", "open_issues"],
              replace_snapshot(read_csv(path), [{
                  "snapshot_date": today,
                  "stars": str(info.get("stargazers_count", 0)),
                  "forks": str(info.get("forks_count", 0)),
                  "watchers": str(info.get("subscribers_count", 0)),
                  "open_issues": str(info.get("open_issues_count", 0)),
              }], today))

    if not token:
        print("::warning::TRAFFIC_TOKEN is not set — saved downloads and repo stats only, not traffic.")
        return 0

    try:
        views = api_get(f"/repos/{repo}/traffic/views?per=day", token)
        clones = api_get(f"/repos/{repo}/traffic/clones?per=day", token)
        referrers = api_get(f"/repos/{repo}/traffic/popular/referrers", token)
        paths = api_get(f"/repos/{repo}/traffic/popular/paths", token)
    except urllib.error.HTTPError as err:
        print(f"::error::Traffic API returned HTTP {err.code}. TRAFFIC_TOKEN needs read access "
              "to repository Administration for this repo.")
        return 1

    for name, payload, items in (("views.csv", views, "views"), ("clones.csv", clones, "clones")):
        path = os.path.join(out_dir, name)
        write_csv(path, ["date", "count", "uniques"], merge_daily(read_csv(path), daily_rows(payload, items)))

    path = os.path.join(out_dir, "referrers.csv")
    write_csv(path, ["snapshot_date", "referrer", "count_14d", "uniques_14d"],
              replace_snapshot(read_csv(path), [{
                  "snapshot_date": today, "referrer": r.get("referrer", ""),
                  "count_14d": str(r.get("count", 0)), "uniques_14d": str(r.get("uniques", 0)),
              } for r in referrers], today))

    path = os.path.join(out_dir, "paths.csv")
    write_csv(path, ["snapshot_date", "path", "title", "count_14d", "uniques_14d"],
              replace_snapshot(read_csv(path), [{
                  "snapshot_date": today, "path": p.get("path", ""), "title": p.get("title", ""),
                  "count_14d": str(p.get("count", 0)), "uniques_14d": str(p.get("uniques", 0)),
              } for p in paths], today))

    print(f"Saved traffic for {repo}: {views.get('count', 0)} views / {views.get('uniques', 0)} unique visitors (14 days).")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
