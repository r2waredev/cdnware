package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHashFile(t *testing.T) {
	// Create a temp file with known content
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("hello world")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	hash := hashFile(testFile, 8)

	// Hash should be 8 characters
	if len(hash) != 8 {
		t.Errorf("expected hash length 8, got %d", len(hash))
	}

	// Hash should be consistent
	hash2 := hashFile(testFile, 8)
	if hash != hash2 {
		t.Errorf("hash not consistent: %s != %s", hash, hash2)
	}

	// Different content should produce different hash
	testFile2 := filepath.Join(tmpDir, "test2.txt")
	if err := os.WriteFile(testFile2, []byte("different content"), 0644); err != nil {
		t.Fatal(err)
	}
	hash3 := hashFile(testFile2, 8)
	if hash == hash3 {
		t.Errorf("different content produced same hash")
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")
	content := []byte("test content for copy")

	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	copyFile(srcPath, destPath)

	// Verify dest file exists and has same content
	destContent, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read dest file: %v", err)
	}
	if string(destContent) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", destContent, content)
	}
}

func TestRevFile(t *testing.T) {
	tmpDir := t.TempDir()
	destDir := "assets-rev"

	// Create dest directory
	if err := os.MkdirAll(filepath.Join(tmpDir, destDir), 0755); err != nil {
		t.Fatal(err)
	}

	// Create source file
	srcPath := filepath.Join(tmpDir, "styles.css")
	if err := os.WriteFile(srcPath, []byte("body { color: red; }"), 0644); err != nil {
		t.Fatal(err)
	}

	result := revFile(srcPath, tmpDir, destDir, 8)

	// Result should be in destDir with hash in filename
	if !strings.Contains(result, destDir) {
		t.Errorf("result path should contain destDir: %s", result)
	}
	if !strings.HasSuffix(result, ".css") {
		t.Errorf("result should end with .css: %s", result)
	}
	// Should have format: styles.<hash>.css
	basename := filepath.Base(result)
	parts := strings.Split(basename, ".")
	if len(parts) != 3 {
		t.Errorf("expected 3 parts (name.hash.ext), got %d: %s", len(parts), basename)
	}
	if parts[0] != "styles" {
		t.Errorf("expected base name 'styles', got %s", parts[0])
	}
	if len(parts[1]) != 8 {
		t.Errorf("expected 8-char hash, got %d chars: %s", len(parts[1]), parts[1])
	}

	// Verify file was copied
	if _, err := os.Stat(result); os.IsNotExist(err) {
		t.Errorf("revisioned file was not created: %s", result)
	}
}

func TestRev(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := "assets"
	destDir := "assets-rev"
	cdnBaseUrl := "https://cdn.example.com"

	// Create assets directory with test files
	assetsDir := filepath.Join(tmpDir, srcDir)
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test assets
	testFiles := map[string]string{
		"styles.css": "body { color: red; }",
		"script.js":  "console.log('hello');",
		"image.png":  "fake png content",
	}
	for name, content := range testFiles {
		path := filepath.Join(assetsDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	manifest := rev(tmpDir, cdnBaseUrl, srcDir, destDir, defaultAssetExts, 8)

	// Should have 3 entries
	if len(manifest) != 3 {
		t.Errorf("expected 3 manifest entries, got %d", len(manifest))
	}

	// Check manifest entries
	for origPath, cdnUrl := range manifest {
		// Original path should start with /assets/
		if !strings.HasPrefix(origPath, "/"+srcDir+"/") {
			t.Errorf("original path should start with /%s/: %s", srcDir, origPath)
		}
		// CDN URL should start with cdnBaseUrl
		if !strings.HasPrefix(cdnUrl, cdnBaseUrl) {
			t.Errorf("CDN URL should start with %s: %s", cdnBaseUrl, cdnUrl)
		}
		// CDN URL should contain destDir
		if !strings.Contains(cdnUrl, "/"+destDir+"/") {
			t.Errorf("CDN URL should contain /%s/: %s", destDir, cdnUrl)
		}
	}

	// Verify assets-rev directory was created
	if _, err := os.Stat(filepath.Join(tmpDir, destDir)); os.IsNotExist(err) {
		t.Errorf("destDir was not created")
	}
}

func TestRevWithCustomDirs(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := "static"
	destDir := "static-rev"
	cdnBaseUrl := "https://cdn.example.com"

	// Create custom source directory
	staticDir := filepath.Join(tmpDir, srcDir)
	if err := os.MkdirAll(staticDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test asset
	if err := os.WriteFile(filepath.Join(staticDir, "app.js"), []byte("var x = 1;"), 0644); err != nil {
		t.Fatal(err)
	}

	manifest := rev(tmpDir, cdnBaseUrl, srcDir, destDir, defaultAssetExts, 8)

	if len(manifest) != 1 {
		t.Errorf("expected 1 manifest entry, got %d", len(manifest))
	}

	for origPath, cdnUrl := range manifest {
		if !strings.HasPrefix(origPath, "/"+srcDir+"/") {
			t.Errorf("original path should use custom srcDir: %s", origPath)
		}
		if !strings.Contains(cdnUrl, "/"+destDir+"/") {
			t.Errorf("CDN URL should use custom destDir: %s", cdnUrl)
		}
	}
}

func TestRepFile(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := "assets"

	htmlPath := filepath.Join(tmpDir, "index.html")
	htmlContent := `<html>
<head>
  <link rel="stylesheet" href="/assets/styles.css">
</head>
<body>
  <img src="/assets/image.png">
  <script src="/assets/script.js"></script>
</body>
</html>`
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0644); err != nil {
		t.Fatal(err)
	}

	manifest := map[string]string{
		"/assets/styles.css": "https://cdn.example.com/assets-rev/styles.abc12345.css",
		"/assets/image.png":  "https://cdn.example.com/assets-rev/image.def67890.png",
		"/assets/script.js":  "https://cdn.example.com/assets-rev/script.ghi11111.js",
	}

	repFile(htmlPath, manifest, srcDir, defaultAssetExts)

	// Read the modified file
	result, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	resultStr := string(result)

	// Check that all references were replaced
	if strings.Contains(resultStr, "/assets/styles.css") {
		t.Errorf("styles.css reference was not replaced")
	}
	if strings.Contains(resultStr, "/assets/image.png") {
		t.Errorf("image.png reference was not replaced")
	}
	if strings.Contains(resultStr, "/assets/script.js") {
		t.Errorf("script.js reference was not replaced")
	}

	// Check that CDN URLs are present
	if !strings.Contains(resultStr, "https://cdn.example.com/assets-rev/styles.abc12345.css") {
		t.Errorf("CDN URL for styles.css not found")
	}
	if !strings.Contains(resultStr, "https://cdn.example.com/assets-rev/image.def67890.png") {
		t.Errorf("CDN URL for image.png not found")
	}
	if !strings.Contains(resultStr, "https://cdn.example.com/assets-rev/script.ghi11111.js") {
		t.Errorf("CDN URL for script.js not found")
	}
}

func TestRepFileWithSingleQuotes(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := "assets"

	jsPath := filepath.Join(tmpDir, "app.js")
	jsContent := `var img = '/assets/logo.png';
var css = "/assets/styles.css";`
	if err := os.WriteFile(jsPath, []byte(jsContent), 0644); err != nil {
		t.Fatal(err)
	}

	manifest := map[string]string{
		"/assets/logo.png":   "https://cdn.example.com/assets-rev/logo.abc12345.png",
		"/assets/styles.css": "https://cdn.example.com/assets-rev/styles.def67890.css",
	}

	repFile(jsPath, manifest, srcDir, defaultAssetExts)

	result, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatal(err)
	}
	resultStr := string(result)

	// Single quotes should be preserved
	if !strings.Contains(resultStr, "'https://cdn.example.com/assets-rev/logo.abc12345.png'") {
		t.Errorf("single-quoted reference not properly replaced: %s", resultStr)
	}
	// Double quotes should be preserved
	if !strings.Contains(resultStr, "\"https://cdn.example.com/assets-rev/styles.def67890.css\"") {
		t.Errorf("double-quoted reference not properly replaced: %s", resultStr)
	}
}

func TestRepFileWithUrlInCSS(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := "assets"

	cssPath := filepath.Join(tmpDir, "styles.css")
	cssContent := `.hero {
  background: url(/assets/hero.jpg);
}
.logo {
  background-image: url("/assets/logo.png");
}`
	if err := os.WriteFile(cssPath, []byte(cssContent), 0644); err != nil {
		t.Fatal(err)
	}

	manifest := map[string]string{
		"/assets/hero.jpg": "https://cdn.example.com/assets-rev/hero.abc12345.jpg",
		"/assets/logo.png": "https://cdn.example.com/assets-rev/logo.def67890.png",
	}

	repFile(cssPath, manifest, srcDir, defaultAssetExts)

	result, err := os.ReadFile(cssPath)
	if err != nil {
		t.Fatal(err)
	}
	resultStr := string(result)

	if !strings.Contains(resultStr, "url(https://cdn.example.com/assets-rev/hero.abc12345.jpg)") {
		t.Errorf("url() reference not properly replaced: %s", resultStr)
	}
}

func TestUseman(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := "assets"

	// Create assets directory (should be excluded)
	assetsDir := filepath.Join(tmpDir, srcDir)
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a CSS file in assets (should NOT be processed)
	assetsCss := filepath.Join(assetsDir, "source.css")
	assetsCssContent := `@import "/assets/other.css";`
	if err := os.WriteFile(assetsCss, []byte(assetsCssContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create HTML file at root (should be processed)
	htmlPath := filepath.Join(tmpDir, "index.html")
	htmlContent := `<link href="/assets/styles.css">`
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0644); err != nil {
		t.Fatal(err)
	}

	manifest := map[string]string{
		"/assets/styles.css": "https://cdn.example.com/assets-rev/styles.abc12345.css",
		"/assets/other.css":  "https://cdn.example.com/assets-rev/other.def67890.css",
	}

	useman(manifest, tmpDir, srcDir, defaultSourceExts, defaultAssetExts)

	// HTML file should be modified
	htmlResult, _ := os.ReadFile(htmlPath)
	if strings.Contains(string(htmlResult), "/assets/styles.css") {
		t.Errorf("index.html should have been processed")
	}

	// CSS in assets directory should NOT be modified
	cssResult, _ := os.ReadFile(assetsCss)
	if !strings.Contains(string(cssResult), "/assets/other.css") {
		t.Errorf("CSS in assets directory should not be processed")
	}
}

func TestGetUsage(t *testing.T) {
	usage := getUsage()
	if !strings.Contains(usage, "cdnware") {
		t.Errorf("usage should contain 'cdnware'")
	}
	if !strings.Contains(usage, "SITEROOT") {
		t.Errorf("usage should contain 'SITEROOT'")
	}
}

func TestIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := "assets"
	destDir := "assets-rev"
	cdnBaseUrl := "https://cdn.example.com"

	// Setup directory structure
	assetsDir := filepath.Join(tmpDir, srcDir)
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create assets
	if err := os.WriteFile(filepath.Join(assetsDir, "styles.css"), []byte("body{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "app.js"), []byte("var x=1;"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create HTML file
	htmlContent := `<!DOCTYPE html>
<html>
<head>
  <link rel="stylesheet" href="/assets/styles.css">
</head>
<body>
  <script src="/assets/app.js"></script>
</body>
</html>`
	if err := os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte(htmlContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Run the full pipeline
	manifest := rev(tmpDir, cdnBaseUrl, srcDir, destDir, defaultAssetExts, 8)
	useman(manifest, tmpDir, srcDir, defaultSourceExts, defaultAssetExts)

	// Verify manifest has correct entries
	if len(manifest) != 2 {
		t.Errorf("expected 2 manifest entries, got %d", len(manifest))
	}

	// Verify HTML was updated
	result, _ := os.ReadFile(filepath.Join(tmpDir, "index.html"))
	resultStr := string(result)

	if strings.Contains(resultStr, "/assets/styles.css") {
		t.Errorf("HTML still contains original asset reference")
	}
	if strings.Contains(resultStr, "/assets/app.js") {
		t.Errorf("HTML still contains original asset reference")
	}
	if !strings.Contains(resultStr, cdnBaseUrl) {
		t.Errorf("HTML should contain CDN URL")
	}

	// Verify revisioned files exist
	files, _ := os.ReadDir(filepath.Join(tmpDir, destDir))
	if len(files) != 2 {
		t.Errorf("expected 2 files in %s, got %d", destDir, len(files))
	}
}

func TestRevWithCustomAssetExts(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := "assets"
	destDir := "assets-rev"
	cdnBaseUrl := "https://cdn.example.com"

	assetsDir := filepath.Join(tmpDir, srcDir)
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "styles.css"), []byte("body{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "image.png"), []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	manifest := rev(tmpDir, cdnBaseUrl, srcDir, destDir, []string{"css"}, 8)

	if len(manifest) != 1 {
		t.Errorf("expected only .css to be revved, got %d entries: %v", len(manifest), manifest)
	}
	if _, ok := manifest["/assets/styles.css"]; !ok {
		t.Errorf("expected /assets/styles.css in manifest: %v", manifest)
	}
	if _, ok := manifest["/assets/image.png"]; ok {
		t.Errorf(".png should have been skipped: %v", manifest)
	}
}

func TestUsemanWithCustomSourceExts(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := "assets"

	if err := os.MkdirAll(filepath.Join(tmpDir, srcDir), 0755); err != nil {
		t.Fatal(err)
	}

	mdPath := filepath.Join(tmpDir, "doc.md")
	if err := os.WriteFile(mdPath, []byte(`![](/assets/image.png)`), 0644); err != nil {
		t.Fatal(err)
	}
	htmlPath := filepath.Join(tmpDir, "index.html")
	if err := os.WriteFile(htmlPath, []byte(`<img src="/assets/image.png">`), 0644); err != nil {
		t.Fatal(err)
	}

	manifest := map[string]string{
		"/assets/image.png": "https://cdn.example.com/assets-rev/image.abc12345.png",
	}

	useman(manifest, tmpDir, srcDir, []string{"md"}, defaultAssetExts)

	mdResult, _ := os.ReadFile(mdPath)
	if strings.Contains(string(mdResult), "/assets/image.png") {
		t.Errorf(".md should have been rewritten: %s", mdResult)
	}
	htmlResult, _ := os.ReadFile(htmlPath)
	if !strings.Contains(string(htmlResult), "/assets/image.png") {
		t.Errorf(".html should NOT have been rewritten: %s", htmlResult)
	}
}

func TestHashLen(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "x.txt")
	if err := os.WriteFile(p, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{4, 16, 32} {
		h := hashFile(p, n)
		if len(h) != n {
			t.Errorf("hashFile(%d) length = %d, want %d", n, len(h), n)
		}
	}
}

func TestParseExtsNormalization(t *testing.T) {
	in := []string{"PNG", ".jpg", " js ", "", ".WEBP"}
	want := []string{"png", "jpg", "js", "webp"}
	got := parseExts(in)
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d]=%q, want %q", i, got[i], want[i])
		}
	}
}

func TestLoadConfigTOML(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "cdnware.toml")
	body := `cdn = "https://x.example.com"
src = "static"
dest = "static-rev"
asset_exts = ["css", "js"]
source_exts = ["html"]
hash_len = 10
`
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfigFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Cdn == nil || *cfg.Cdn != "https://x.example.com" {
		t.Errorf("cdn mismatch: %+v", cfg.Cdn)
	}
	if cfg.Src == nil || *cfg.Src != "static" {
		t.Errorf("src mismatch")
	}
	if cfg.HashLen == nil || *cfg.HashLen != 10 {
		t.Errorf("hash_len mismatch")
	}
	if len(cfg.AssetExts) != 2 || cfg.AssetExts[0] != "css" {
		t.Errorf("asset_exts mismatch: %v", cfg.AssetExts)
	}
}

func TestLoadConfigYAML(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "cdnware.yml")
	body := `cdn: https://y.example.com
src: static
dest: static-rev
asset_exts: [css, js]
source_exts: [html]
hash_len: 12
`
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfigFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Cdn == nil || *cfg.Cdn != "https://y.example.com" {
		t.Errorf("cdn mismatch")
	}
	if cfg.HashLen == nil || *cfg.HashLen != 12 {
		t.Errorf("hash_len mismatch")
	}
	if len(cfg.SourceExts) != 1 || cfg.SourceExts[0] != "html" {
		t.Errorf("source_exts mismatch: %v", cfg.SourceExts)
	}
}

func TestLoadConfigJSON(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "cdnware.json")
	body := `{
  "cdn": "https://j.example.com",
  "asset_exts": ["css", "js"],
  "source_exts": ["html"],
  "hash_len": 6
}`
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfigFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Cdn == nil || *cfg.Cdn != "https://j.example.com" {
		t.Errorf("cdn mismatch")
	}
	if cfg.HashLen == nil || *cfg.HashLen != 6 {
		t.Errorf("hash_len mismatch")
	}
}

func TestConfigDiscoveryOrder(t *testing.T) {
	tmpDir := t.TempDir()
	// Place .json and .toml; .toml should win (it's first in the candidate list).
	if err := os.WriteFile(filepath.Join(tmpDir, "cdnware.toml"), []byte(`cdn = "from-toml"`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "cdnware.json"), []byte(`{"cdn": "from-json"}`), 0644); err != nil {
		t.Fatal(err)
	}
	p := discoverConfig(tmpDir)
	if !strings.HasSuffix(p, "cdnware.toml") {
		t.Errorf("expected cdnware.toml to win, got %s", p)
	}
}

func TestConfigDiscoveryNone(t *testing.T) {
	tmpDir := t.TempDir()
	// Make CWD also a place without config so this test is hermetic.
	prev, _ := os.Getwd()
	defer os.Chdir(prev)
	emptyCwd := t.TempDir()
	os.Chdir(emptyCwd)
	p := discoverConfig(tmpDir)
	if p != "" {
		t.Errorf("expected no config discovered, got %s", p)
	}
}

func TestMergePrecedence(t *testing.T) {
	// File sets cdn=file; flag sets cdn=flag; flag wins.
	cdnFile := "from-file"
	hashLenFile := 12
	fileCfg := &Config{Cdn: &cdnFile, HashLen: &hashLenFile}
	fs := &flagSet{cdn: "from-flag", hashLen: 16, assetExts: "css,js", sourceExts: "html"}
	explicit := map[string]bool{"cdn": true}

	s, err := mergeSettings(".", fileCfg, fs, explicit)
	if err != nil {
		t.Fatal(err)
	}
	if s.Cdn != "from-flag" {
		t.Errorf("flag should win for cdn: got %q", s.Cdn)
	}
	if s.HashLen != 12 {
		t.Errorf("file should win for hash_len when flag not explicit: got %d", s.HashLen)
	}
	if s.Src != defaultSrc {
		t.Errorf("default should win for src: got %q", s.Src)
	}
}

func TestInvalidHashLen(t *testing.T) {
	for _, n := range []int{0, 33, -1} {
		fs := &flagSet{hashLen: n, assetExts: "css", sourceExts: "html"}
		explicit := map[string]bool{"hash-len": true}
		if _, err := mergeSettings(".", nil, fs, explicit); err == nil {
			t.Errorf("expected error for hash_len=%d", n)
		}
	}
}

func TestInvalidEmptyExts(t *testing.T) {
	fs := &flagSet{assetExts: "", sourceExts: ""}
	explicit := map[string]bool{"asset-exts": true, "source-exts": true}
	if _, err := mergeSettings(".", nil, fs, explicit); err == nil {
		t.Errorf("expected error for empty asset_exts/source_exts")
	}
}

func TestLoadSettingsEndToEnd(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cdnware.toml")
	body := `cdn = "https://cfg.example.com"
asset_exts = ["css", "png"]
source_exts = ["html"]
hash_len = 6
`
	if err := os.WriteFile(cfgPath, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	// Flag explicitly overrides cdn.
	s, err := loadSettings([]string{"-config", cfgPath, "-cdn", "https://flag.example.com", tmpDir})
	if err != nil {
		t.Fatal(err)
	}
	if s.Cdn != "https://flag.example.com" {
		t.Errorf("flag cdn should win: %q", s.Cdn)
	}
	if s.HashLen != 6 {
		t.Errorf("hash_len from file: %d", s.HashLen)
	}
	if len(s.AssetExts) != 2 || s.AssetExts[1] != "png" {
		t.Errorf("asset_exts from file: %v", s.AssetExts)
	}
}
