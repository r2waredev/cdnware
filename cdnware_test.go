package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func destinationPath(t *testing.T, root, url string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(url, "/")))
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected destination %s: %v", path, err)
	}
	return path
}

func assertFilenameMatchesContent(t *testing.T, path string) {
	t.Helper()
	name := filepath.Base(path)
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	separator := strings.LastIndexByte(stem, '.')
	if separator == -1 {
		t.Fatalf("revisioned filename has no hash: %s", name)
	}
	want := hashFile(path)
	if got := stem[separator+1:]; got != want {
		t.Fatalf("filename hash %s does not match content hash %s for %s", got, want, path)
	}
}

func TestHashFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "asset.txt")
	writeTestFile(t, path, "hello world")

	first := hashFile(path)
	second := hashFile(path)
	if len(first) != hashLength {
		t.Fatalf("hash length = %d, want %d", len(first), hashLength)
	}
	if first != second {
		t.Fatalf("hash is not deterministic: %s != %s", first, second)
	}

	writeTestFile(t, path, "changed")
	if changed := hashFile(path); changed == first {
		t.Fatal("different content produced the same hash")
	}
}

func TestRevPreservesNestedPathsAndRevisionsEveryFile(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "assets/images/patrons/person.webp"), "patron")
	writeTestFile(t, filepath.Join(root, "assets/images/team/person.webp"), "team")
	writeTestFile(t, filepath.Join(root, "assets/js/runtime.wasm"), "wasm")
	writeTestFile(t, filepath.Join(root, "assets/data/catalog.bin"), "binary")

	manifest := rev(root, "", "assets", "assets-rev")
	if len(manifest) != 4 {
		t.Fatalf("manifest entries = %d, want 4", len(manifest))
	}

	patronURL := manifest["/assets/images/patrons/person.webp"]
	teamURL := manifest["/assets/images/team/person.webp"]
	if patronURL == teamURL {
		t.Fatalf("same basenames in different directories collided at %s", patronURL)
	}
	if !strings.HasPrefix(patronURL, "/assets-rev/images/patrons/person.") {
		t.Fatalf("patron URL did not preserve directories: %s", patronURL)
	}
	if !strings.HasPrefix(teamURL, "/assets-rev/images/team/person.") {
		t.Fatalf("team URL did not preserve directories: %s", teamURL)
	}
	if !strings.HasSuffix(manifest["/assets/js/runtime.wasm"], ".wasm") {
		t.Fatal("wasm asset was not revisioned")
	}
	if !strings.Contains(manifest["/assets/data/catalog.bin"], ".bin") {
		t.Fatal("unlisted binary extension was not revisioned")
	}

	for _, url := range manifest {
		assertFilenameMatchesContent(t, destinationPath(t, root, url))
	}
}

func TestRevRewritesAssetGraphBeforeHashing(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "assets/js/app.js"), `import "./lib/helper.js";
const wasm = "/assets/js/lib/runtime.wasm";
`)
	writeTestFile(t, filepath.Join(root, "assets/js/lib/helper.js"), `export const value = 1;
`)
	writeTestFile(t, filepath.Join(root, "assets/js/lib/runtime.wasm"), "wasm")

	manifest := rev(root, "https://cdn.example.com/static", "assets", "assets-rev")
	appPath := destinationPath(t, root, strings.TrimPrefix(manifest["/assets/js/app.js"], "https://cdn.example.com/static"))
	content, err := os.ReadFile(appPath)
	if err != nil {
		t.Fatal(err)
	}
	app := string(content)
	if !strings.Contains(app, manifest["/assets/js/lib/helper.js"]) {
		t.Fatalf("relative dependency was not rewritten: %s", app)
	}
	if !strings.Contains(app, manifest["/assets/js/lib/runtime.wasm"]) {
		t.Fatalf("absolute dependency was not rewritten: %s", app)
	}
	assertFilenameMatchesContent(t, appPath)
}

func TestRevRejectsAssetReferenceCycles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "assets/a.js"), `import "./b.js";`)
	writeTestFile(t, filepath.Join(root, "assets/b.js"), `import "./a.js";`)

	revisioner, err := newRevisioner(root, "", "assets", "assets-rev")
	if err != nil {
		t.Fatal(err)
	}
	_, err = revisioner.revise()
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected asset reference cycle error, got %v", err)
	}
}

func TestRevClearsStaleDestinationFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "assets/app.js"), "first")
	stale := filepath.Join(root, "assets-rev/app.stale.js")
	writeTestFile(t, stale, "stale")

	rev(root, "", "assets", "assets-rev")
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale destination survived revisioning: %v", err)
	}
}

func TestUsemanRewritesHTMLJavaScriptAndSrcsetReferences(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "assets/images/small.webp"), "small")
	writeTestFile(t, filepath.Join(root, "assets/images/large.webp"), "large")
	writeTestFile(t, filepath.Join(root, "assets/js/app.js"), "app")
	indexPath := filepath.Join(root, "index.html")
	writeTestFile(t, indexPath, `<source srcset="/assets/images/small.webp 480w, /assets/images/large.webp 960w">
<script>new URL("/assets/js/app.js", document.baseURI)</script>`)

	manifest := rev(root, "", "assets", "assets-rev")
	if err := useman(manifest, root, "assets", "assets-rev"); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	result := string(content)
	for sourceURL, revisionedURL := range manifest {
		if strings.Contains(result, sourceURL) {
			t.Fatalf("source URL was not replaced: %s", sourceURL)
		}
		if !strings.Contains(result, revisionedURL) {
			t.Fatalf("revisioned URL is missing: %s", revisionedURL)
		}
	}
}

func TestUsemanLeavesSourceAssetsUnchanged(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "assets/app.js")
	writeTestFile(t, sourcePath, `const icon = "/assets/icon.svg";`)
	writeTestFile(t, filepath.Join(root, "assets/icon.svg"), `<svg/>`)

	manifest := rev(root, "", "assets", "assets-rev")
	if err := useman(manifest, root, "assets", "assets-rev"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "/assets/icon.svg") {
		t.Fatal("source asset was modified")
	}
}

func TestPublicURL(t *testing.T) {
	if got := publicURL("", "assets-rev/app.12345678.js"); got != "/assets-rev/app.12345678.js" {
		t.Fatalf("same-origin URL = %s", got)
	}
	if got := publicURL("https://cdn.example.com/root/", "assets-rev/app.12345678.js"); got != "https://cdn.example.com/root/assets-rev/app.12345678.js" {
		t.Fatalf("CDN URL = %s", got)
	}
}

func TestSourceAndDestinationMustDiffer(t *testing.T) {
	root := t.TempDir()
	if _, err := newRevisioner(root, "", "assets", "assets"); err == nil {
		t.Fatal("expected matching source and destination directories to fail")
	}
}

func TestGetUsage(t *testing.T) {
	usage := getUsage()
	if !strings.Contains(usage, "cdnware") || !strings.Contains(usage, "SITEROOT") {
		t.Fatalf("unexpected usage: %s", usage)
	}
}

func TestLoadConfigTOML(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "cdnware.toml")
	writeTestFile(t, path, `cdn = "https://x.example.com"
src = "static"
dest = "static-rev"
`)

	cfg, err := loadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Cdn == nil || *cfg.Cdn != "https://x.example.com" {
		t.Errorf("cdn mismatch: %v", cfg.Cdn)
	}
	if cfg.Src == nil || *cfg.Src != "static" {
		t.Errorf("src mismatch: %v", cfg.Src)
	}
	if cfg.Dest == nil || *cfg.Dest != "static-rev" {
		t.Errorf("dest mismatch: %v", cfg.Dest)
	}
}

func TestLoadConfigYAML(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "cdnware.yml")
	writeTestFile(t, path, `cdn: https://y.example.com
src: static
dest: static-rev
`)

	cfg, err := loadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Cdn == nil || *cfg.Cdn != "https://y.example.com" {
		t.Errorf("cdn mismatch: %v", cfg.Cdn)
	}
	if cfg.Src == nil || *cfg.Src != "static" {
		t.Errorf("src mismatch: %v", cfg.Src)
	}
	if cfg.Dest == nil || *cfg.Dest != "static-rev" {
		t.Errorf("dest mismatch: %v", cfg.Dest)
	}
}

func TestLoadConfigJSON(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "cdnware.json")
	writeTestFile(t, path, `{
  "cdn": "https://j.example.com",
  "src": "static",
  "dest": "static-rev"
}`)

	cfg, err := loadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Cdn == nil || *cfg.Cdn != "https://j.example.com" {
		t.Errorf("cdn mismatch: %v", cfg.Cdn)
	}
	if cfg.Src == nil || *cfg.Src != "static" {
		t.Errorf("src mismatch: %v", cfg.Src)
	}
	if cfg.Dest == nil || *cfg.Dest != "static-rev" {
		t.Errorf("dest mismatch: %v", cfg.Dest)
	}
}

func TestConfigDiscoveryOrder(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "cdnware.toml"), `cdn = "from-toml"`)
	writeTestFile(t, filepath.Join(root, "cdnware.json"), `{"cdn": "from-json"}`)

	if path := discoverConfig(root); !strings.HasSuffix(path, "cdnware.toml") {
		t.Errorf("expected cdnware.toml to win, got %s", path)
	}
}

func TestConfigDiscoveryNone(t *testing.T) {
	root := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	if path := discoverConfig(root); path != "" {
		t.Errorf("expected no config discovered, got %s", path)
	}
}

func TestMergePrecedence(t *testing.T) {
	cdnFile := "from-file"
	destFile := "file-rev"
	fileCfg := &Config{Cdn: &cdnFile, Dest: &destFile}
	fs := &flagSet{cdn: "from-flag"}

	settings := mergeSettings(".", fileCfg, fs, map[string]bool{"cdn": true})
	if settings.Cdn != "from-flag" {
		t.Errorf("flag should win for cdn: got %q", settings.Cdn)
	}
	if settings.Dest != "file-rev" {
		t.Errorf("file should win for dest: got %q", settings.Dest)
	}
	if settings.Src != defaultSrc {
		t.Errorf("default should win for src: got %q", settings.Src)
	}
}

func TestLoadSettingsEndToEnd(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "cdnware.toml")
	writeTestFile(t, path, `cdn = "https://cfg.example.com"
src = "static"
dest = "static-rev"
`)

	settings, err := loadSettings([]string{"-config", path, "-cdn", "https://flag.example.com", root})
	if err != nil {
		t.Fatal(err)
	}
	if settings.Cdn != "https://flag.example.com" {
		t.Errorf("flag cdn should win: %q", settings.Cdn)
	}
	if settings.Src != "static" {
		t.Errorf("src from file: %q", settings.Src)
	}
	if settings.Dest != "static-rev" {
		t.Errorf("dest from file: %q", settings.Dest)
	}
}
