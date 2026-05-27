# cdnware

Rev your static assets and update all references to them to prep your site
for deploy to a CDN (or anywhere else) with version mapping.

## Install

<!-- Pre-built binaries are distributed in [releases](https://github.com/reidransom/cdnware/releases). -->

You can build from source,

```
git clone https://github.com/reidransom/cdnware
cd cdnware
go build 
sudo mv cdnware /usr/local/bin # optional
```

## Usage

Suppose you built your jekyll site to the standard `_site` folder.

```
$ cdnware -cdn https://cdn.example.com/some-path _site
```

All of the assets in `_site/assets` will be revved and moved to `_site/assets-rev`
then a json hash map will be printed to standard out.

Ex:

```
$ tree _site
_site
├── assets
│   ├── image.png
│   ├── script.js
│   └── styles.css
└── index.html

2 directories, 4 files
$ cat _site/index.html 
<html>
  <head>
    <link rel="stylesheet" href="/assets/styles.css">
  </head>
  <img src="/assets/image.png">
  <script src="/assets/script.js"></script>
</html>
$ cdnware -cdn https://cdn.example.com/some-path _site
{
  "/assets/image.png": "https://cdn.example.com/some-path/assets-rev/image.e15d28a4.png",
  "/assets/script.js": "https://cdn.example.com/some-path/assets-rev/script.dada41ac.js",
  "/assets/styles.css": "https://cdn.example.com/some-path/assets-rev/styles.1f81c53a.css"
}
$ tree _site
_site
├── assets
│   ├── image.png
│   ├── script.js
│   └── styles.css
├── assets-rev
│   ├── image.e15d28a4.png
│   ├── script.dada41ac.js
│   └── styles.1f81c53a.css
└── index.html

3 directories, 7 files
$ cat _site/index.html 
<html>
  <head>
    <link rel="stylesheet" href="https://cdn.example.com/some-path/assets-rev/styles.1f81c53a.css">
  </head>
  <img src="https://cdn.example.com/some-path/assets-rev/image.e15d28a4.png">
  <script src="https://cdn.example.com/some-path/assets-rev/script.dada41ac.js"></script>
</html>
```

## Configuration

All settings can be set via CLI flag or a config file. Precedence is
**flag > config file > built-in default**.

### Flags

| Flag | Default | Purpose |
| --- | --- | --- |
| `-cdn` | `""` | CDN base url |
| `-src` | `assets` | Source asset directory (relative to SITEROOT) |
| `-dest` | `assets-rev` | Destination directory for revisioned assets |
| `-asset-exts` | `css,js,jpg,png,webp,svg,ico,mp4,woff2,avif` | Extensions to rev |
| `-source-exts` | `css,js,html,toml,webmanifest` | File types to scan and rewrite |
| `-hash-len` | `8` | Hex chars of MD5 hash to embed (1–32) |
| `-config` | _auto_ | Path to config file; use `-` to disable auto-discovery |

Extensions may be written with or without a leading dot (`png` and `.png` both work).

### Config file

If `-config` is not given, cdnware looks in SITEROOT then CWD for, in order:
`cdnware.toml`, `cdnware.yaml`, `cdnware.yml`, `cdnware.json`. First match wins.

```toml
# cdnware.toml
cdn = "https://cdn.example.com/v3"
asset_exts = ["css", "js", "png", "webp", "avif"]
source_exts = ["html", "xml"]
hash_len = 10
```

```yaml
# cdnware.yml
cdn: https://cdn.example.com/v3
asset_exts: [css, js, png, webp, avif]
source_exts: [html, xml]
hash_len: 10
```

```json
{
  "cdn": "https://cdn.example.com/v3",
  "asset_exts": ["css", "js", "png", "webp", "avif"],
  "source_exts": ["html", "xml"],
  "hash_len": 10
}
```

## Philosophy

This tool is meant to be simple to use and adaptable for many different workflows.
While it was designed specifically for use with Jekyll, it works completely outside
the built-in jekyll build command and it can be incorporated easily for other
web build stacks.
