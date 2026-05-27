package main

import (
	"crypto/md5"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// Config holds all settings. Pointer fields distinguish "absent" from "zero"
// so file values can be overridden by explicitly-set CLI flags.
type Config struct {
	Cdn        *string  `toml:"cdn" yaml:"cdn" json:"cdn,omitempty"`
	Src        *string  `toml:"src" yaml:"src" json:"src,omitempty"`
	Dest       *string  `toml:"dest" yaml:"dest" json:"dest,omitempty"`
	AssetExts  []string `toml:"asset_exts" yaml:"asset_exts" json:"asset_exts,omitempty"`
	SourceExts []string `toml:"source_exts" yaml:"source_exts" json:"source_exts,omitempty"`
	HashLen    *int     `toml:"hash_len" yaml:"hash_len" json:"hash_len,omitempty"`
}

// Settings is the resolved, validated configuration used at runtime.
type Settings struct {
	BaseDir    string
	Cdn        string
	Src        string
	Dest       string
	AssetExts  []string
	SourceExts []string
	HashLen    int
}

var defaultAssetExts = []string{"css", "js", "jpg", "png", "webp", "svg", "ico", "mp4", "woff2", "avif"}
var defaultSourceExts = []string{"css", "js", "html", "toml", "webmanifest"}

const defaultSrc = "assets"
const defaultDest = "assets-rev"
const defaultHashLen = 8

func check(err error) {
	if err == nil {
		return
	}
	panic(err)
}

func hashFile(path string, hashLen int) string {
	file, err := os.Open(path)
	check(err)
	defer file.Close()
	hash := md5.New()
	_, err = io.Copy(hash, file)
	check(err)
	hashstr := fmt.Sprintf("%x", hash.Sum(nil))
	hashstr = hashstr[:hashLen]
	return hashstr
}

func copyFile(srcPath string, destPath string) {
	srcFile, err := os.Open(srcPath)
	check(err)
	defer srcFile.Close()

	destFile, err := os.Create(destPath)
	check(err)
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	check(err)

	err = destFile.Sync()
	check(err)
}

func revFile(path string, baseDir string, destDir string, hashLen int) string {
	fhash := hashFile(path, hashLen)
	_, fname := filepath.Split(path)
	parts := strings.Split(fname, ".")
	lindex := len(parts) - 1
	parts = append(parts[:lindex], fhash, parts[lindex])
	hashName := strings.Join(parts, ".")
	hashPath := filepath.Join(baseDir, destDir, hashName)
	copyFile(path, hashPath)
	return hashPath
}

// parseExts normalizes a list of extensions: trims whitespace, strips leading
// dots, lowercases, and drops empties. Order is preserved.
func parseExts(in []string) []string {
	out := make([]string, 0, len(in))
	for _, e := range in {
		e = strings.TrimSpace(e)
		e = strings.TrimPrefix(e, ".")
		e = strings.ToLower(e)
		if e == "" {
			continue
		}
		out = append(out, e)
	}
	return out
}

// buildExtAlt turns ["css","js"] into `\.css|\.js`, regex-escaped.
func buildExtAlt(exts []string) string {
	parts := make([]string, len(exts))
	for i, e := range exts {
		parts[i] = `\.` + regexp.QuoteMeta(e)
	}
	return strings.Join(parts, "|")
}

func rev(baseDir string, cdnBaseUrl string, srcDir string, destDir string, assetExts []string, hashLen int) map[string]string {
	// Build regex pattern - handle "." baseDir specially since filepath.Walk
	// returns paths without "./" prefix
	var pathPrefix string
	if baseDir == "." {
		pathPrefix = ""
	} else {
		pathPrefix = baseDir + "/"
	}
	lsrcpath := len(pathPrefix)
	extAlt := buildExtAlt(assetExts)
	repath := regexp.MustCompile(`^` + regexp.QuoteMeta(pathPrefix+srcDir) + `/.+(` + extAlt + `)$`)
	err := os.MkdirAll(filepath.Join(baseDir, destDir), os.ModePerm)
	check(err)
	m := make(map[string]string)
	err = filepath.Walk(baseDir, func(path string, info fs.FileInfo, err error) error {
		check(err)
		matched := repath.MatchString(path)
		if matched == true {
			rev := revFile(path, baseDir, destDir, hashLen)[lsrcpath:]
			m["/"+path[lsrcpath:]] = fmt.Sprintf("%s/%s", cdnBaseUrl, rev)
		}
		return nil
	})
	check(err)
	return m
}

func repFile(path string, manifest map[string]string, srcDir string, assetExts []string) {
	extAlt := buildExtAlt(assetExts)
	repath := regexp.MustCompile(`["'\(]/` + regexp.QuoteMeta(srcDir) + `/.+?(?:` + extAlt + `)["'\)]`)
	input, err := ioutil.ReadFile(path)
	check(err)
	lines := strings.Split(string(input), "\n")
	for i, line := range lines {
		matches := repath.FindAllString(line, -1)
		for _, match := range matches {
			lm := len(match)
			sq := match[0:1]     // start quote ("|')
			eq := match[lm-1 : lm] // end quote ("|')
			orig := match[1 : lm-1]
			rev, ok := manifest[orig]
			if ok {
				rep := fmt.Sprintf("%s%s%s", sq, rev, eq)
				line = strings.Replace(line, match, rep, 1)
			}
		}
		lines[i] = line
	}
	output := strings.Join(lines, "\n")
	err = ioutil.WriteFile(path, []byte(output), 0644)
	check(err)
}

func useman(manifest map[string]string, baseDir string, srcDir string, sourceExts []string, assetExts []string) {
	extAlt := buildExtAlt(sourceExts)
	repath := regexp.MustCompile(`^` + regexp.QuoteMeta(baseDir) + `/.+(` + extAlt + `)$`)
	expath := regexp.MustCompile(`^` + regexp.QuoteMeta(baseDir) + `/` + regexp.QuoteMeta(srcDir) + `/`)
	err := filepath.Walk(baseDir, func(path string, info fs.FileInfo, err error) error {
		check(err)
		matched := repath.MatchString(path)
		excluded := expath.MatchString(path)
		if matched == true && excluded != true {
			repFile(path, manifest, srcDir, assetExts)
		}
		return nil
	})
	check(err)
}

// loadConfigFile decodes a config file based on its extension. Returns
// (nil, nil) if path is empty.
func loadConfigFile(path string) (*Config, error) {
	if path == "" {
		return nil, nil
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	cfg := &Config{}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".toml":
		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing TOML %s: %w", path, err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing YAML %s: %w", path, err)
		}
	case ".json":
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing JSON %s: %w", path, err)
		}
	default:
		return nil, fmt.Errorf("unknown config format for %s (expected .toml/.yaml/.yml/.json)", path)
	}
	return cfg, nil
}

// discoverConfig searches SITEROOT then CWD for a config file. Returns the
// first existing path or "" if none found.
func discoverConfig(siteroot string) string {
	names := []string{"cdnware.toml", "cdnware.yaml", "cdnware.yml", "cdnware.json"}
	dirs := []string{siteroot}
	if cwd, err := os.Getwd(); err == nil && cwd != siteroot {
		dirs = append(dirs, cwd)
	}
	for _, dir := range dirs {
		for _, name := range names {
			p := filepath.Join(dir, name)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

// flagSet wraps flag definitions so we can detect which flags were explicitly
// passed (via flag.Visit) for merge precedence.
type flagSet struct {
	fs           *flag.FlagSet
	cdn          string
	src          string
	dest         string
	assetExts    string
	sourceExts   string
	hashLen      int
	configPath   string
}

func splitCsv(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// mergeSettings applies precedence: defaults < file < explicitly-set flags.
func mergeSettings(baseDir string, fileCfg *Config, fs *flagSet, explicit map[string]bool) (Settings, error) {
	s := Settings{
		BaseDir:    baseDir,
		Cdn:        "",
		Src:        defaultSrc,
		Dest:       defaultDest,
		AssetExts:  append([]string(nil), defaultAssetExts...),
		SourceExts: append([]string(nil), defaultSourceExts...),
		HashLen:    defaultHashLen,
	}

	if fileCfg != nil {
		if fileCfg.Cdn != nil {
			s.Cdn = *fileCfg.Cdn
		}
		if fileCfg.Src != nil {
			s.Src = *fileCfg.Src
		}
		if fileCfg.Dest != nil {
			s.Dest = *fileCfg.Dest
		}
		if fileCfg.AssetExts != nil {
			s.AssetExts = fileCfg.AssetExts
		}
		if fileCfg.SourceExts != nil {
			s.SourceExts = fileCfg.SourceExts
		}
		if fileCfg.HashLen != nil {
			s.HashLen = *fileCfg.HashLen
		}
	}

	if explicit["cdn"] {
		s.Cdn = fs.cdn
	}
	if explicit["src"] {
		s.Src = fs.src
	}
	if explicit["dest"] {
		s.Dest = fs.dest
	}
	if explicit["asset-exts"] {
		s.AssetExts = splitCsv(fs.assetExts)
	}
	if explicit["source-exts"] {
		s.SourceExts = splitCsv(fs.sourceExts)
	}
	if explicit["hash-len"] {
		s.HashLen = fs.hashLen
	}

	s.AssetExts = parseExts(s.AssetExts)
	s.SourceExts = parseExts(s.SourceExts)

	if len(s.AssetExts) == 0 {
		return s, fmt.Errorf("asset_exts is empty after normalization")
	}
	if len(s.SourceExts) == 0 {
		return s, fmt.Errorf("source_exts is empty after normalization")
	}
	if s.HashLen < 1 || s.HashLen > 32 {
		return s, fmt.Errorf("hash_len must be between 1 and 32, got %d", s.HashLen)
	}

	return s, nil
}

func getUsage() string {
	usage := `Usage of cdnware:

$ cdnware [OPTIONS] [SITEROOT]
`
	return usage
}

// loadSettings parses CLI flags, discovers/loads the config file, merges, and
// validates. Returns the resolved Settings.
func loadSettings(args []string) (Settings, error) {
	fs := &flagSet{fs: flag.NewFlagSet("cdnware", flag.ContinueOnError)}
	fs.fs.StringVar(&fs.cdn, "cdn", "", "CDN base url")
	fs.fs.StringVar(&fs.src, "src", defaultSrc, "Source directory for assets (relative to SITEROOT)")
	fs.fs.StringVar(&fs.dest, "dest", defaultDest, "Destination directory for revisioned assets (relative to SITEROOT)")
	fs.fs.StringVar(&fs.assetExts, "asset-exts", strings.Join(defaultAssetExts, ","), "Comma-separated asset extensions to rev")
	fs.fs.StringVar(&fs.sourceExts, "source-exts", strings.Join(defaultSourceExts, ","), "Comma-separated source-file extensions to rewrite")
	fs.fs.IntVar(&fs.hashLen, "hash-len", defaultHashLen, "Hex chars of MD5 hash to embed in filename (1-32)")
	fs.fs.StringVar(&fs.configPath, "config", "", `Path to config file (.toml/.yaml/.yml/.json). Use "-" to disable auto-discovery.`)

	fs.fs.Usage = func() {
		fmt.Println(getUsage())
		fmt.Println("Options:")
		fs.fs.PrintDefaults()
	}

	if err := fs.fs.Parse(args); err != nil {
		return Settings{}, err
	}

	baseDir := "."
	if fs.fs.NArg() > 0 {
		baseDir = fs.fs.Arg(0)
	}

	explicit := map[string]bool{}
	fs.fs.Visit(func(f *flag.Flag) {
		explicit[f.Name] = true
	})

	var fileCfg *Config
	var err error
	switch fs.configPath {
	case "":
		if p := discoverConfig(baseDir); p != "" {
			fileCfg, err = loadConfigFile(p)
			if err != nil {
				return Settings{}, err
			}
		}
	case "-":
		// explicit opt-out of auto-discovery
	default:
		fileCfg, err = loadConfigFile(fs.configPath)
		if err != nil {
			return Settings{}, err
		}
	}

	return mergeSettings(baseDir, fileCfg, fs, explicit)
}

func main() {
	settings, err := loadSettings(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	manifest := rev(settings.BaseDir, settings.Cdn, settings.Src, settings.Dest, settings.AssetExts, settings.HashLen)
	useman(manifest, settings.BaseDir, settings.Src, settings.SourceExts, settings.AssetExts)
	jsonData, err := json.MarshalIndent(manifest, "", "  ")
	check(err)
	fmt.Printf("%s\n", jsonData)
}
