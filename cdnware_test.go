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

	hash := hashFile(testFile)

	// Hash should be 8 characters
	if len(hash) != 8 {
		t.Errorf("expected hash length 8, got %d", len(hash))
	}

	// Hash should be consistent
	hash2 := hashFile(testFile)
	if hash != hash2 {
		t.Errorf("hash not consistent: %s != %s", hash, hash2)
	}

	// Different content should produce different hash
	testFile2 := filepath.Join(tmpDir, "test2.txt")
	if err := os.WriteFile(testFile2, []byte("different content"), 0644); err != nil {
		t.Fatal(err)
	}
	hash3 := hashFile(testFile2)
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

	result := revFile(srcPath, tmpDir, destDir)

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

	manifest := rev(tmpDir, cdnBaseUrl, srcDir, destDir)

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

	manifest := rev(tmpDir, cdnBaseUrl, srcDir, destDir)

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

	repFile(htmlPath, manifest, srcDir)

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

	repFile(jsPath, manifest, srcDir)

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

	repFile(cssPath, manifest, srcDir)

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

	useman(manifest, tmpDir, srcDir)

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
	manifest := rev(tmpDir, cdnBaseUrl, srcDir, destDir)
	useman(manifest, tmpDir, srcDir)

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
