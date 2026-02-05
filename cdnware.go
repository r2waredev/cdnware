package main

import (
    "crypto/md5"
    "encoding/json"
    "fmt"
    "io/fs"
    "io/ioutil"
    "path/filepath"
    "regexp"
    "os"
    "io"
    "strings"
    "flag"
)

func check(err error) {
    if err == nil {
        return
    }
    panic(err)
}

func hashFile(path string) string {
    file, err := os.Open(path)
    check(err)
    defer file.Close()
    hash := md5.New()
    _, err = io.Copy(hash, file)
    check(err)
    hashstr := fmt.Sprintf("%x", hash.Sum(nil))
    hashstr = hashstr[:8]
    return hashstr
}

func copyFile (srcPath string, destPath string) {
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

func revFile(path string, baseDir string, destDir string) string {
    fhash := hashFile(path)
    _, fname := filepath.Split(path)
    parts := strings.Split(fname, ".")
    lindex := len(parts) - 1
    parts = append(parts[:lindex], fhash, parts[lindex])
    hashName := strings.Join(parts, ".")
    hashPath := filepath.Join(baseDir, destDir, hashName)
    copyFile(path, hashPath)
    return hashPath
}

func rev(baseDir string, cdnBaseUrl string, srcDir string, destDir string) map[string]string {
    // Build regex pattern - handle "." baseDir specially since filepath.Walk
    // returns paths without "./" prefix
    var pathPrefix string
    if baseDir == "." {
        pathPrefix = ""
    } else {
        pathPrefix = baseDir + "/"
    }
    lsrcpath := len(pathPrefix)
    repath := regexp.MustCompile(`^` + regexp.QuoteMeta(pathPrefix+srcDir) + `/.+(\.css|\.js|\.jpg|\.png|\.svg|\.ico|\.mp4|\.woff2|\.avif)$`)
    err := os.MkdirAll(filepath.Join(baseDir, destDir), os.ModePerm)
    check(err)
    m := make(map[string]string)
    err = filepath.Walk(baseDir, func(path string, info fs.FileInfo, err error) error {
        check(err)
        matched := repath.MatchString(path)
        if matched == true {
            rev := revFile(path, baseDir, destDir)[lsrcpath:]
            m["/"+path[lsrcpath:]] = fmt.Sprintf("%s/%s", cdnBaseUrl, rev)
        }
        return nil
    })
    check(err)
    return m
}

func repFile(path string, manifest map[string]string, srcDir string) {
    repath := regexp.MustCompile(`["'\(]/` + srcDir + `/.+?(?:\.css|\.js|\.jpg|\.png|\.svg|\.ico|\.mp4|\.woff2|\.avif)["'\)]`)
    input, err := ioutil.ReadFile(path)
    check(err)
    lines := strings.Split(string(input), "\n")
    for i, line := range lines {
        matches := repath.FindAllString(line, -1)
        for _, match := range matches {
            lm := len(match)
            sq := match[0:1]  // start quote ("|')
            eq := match[lm-1:lm]  // end quote ("|')
            orig := match[1:lm-1]
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

func useman(manifest map[string]string, baseDir string, srcDir string) {
    repath := regexp.MustCompile(`^` + baseDir + `/.+(\.css|\.js|\.html|\.webmanifest)$`)
    expath := regexp.MustCompile(`^` + baseDir + `/` + srcDir + `/`)
    err := filepath.Walk(baseDir, func(path string, info fs.FileInfo, err error) error {
        check(err)
        matched := repath.MatchString(path)
        excluded := expath.MatchString(path)
        if matched == true  && excluded != true {
            repFile(path, manifest, srcDir)
        }
        return nil
    })
    check(err)
}

func getUsage() string {
    usage := `Usage of cdnware:

$ cdnware [OPTIONS] [SITEROOT]
`
    return usage
}

func parseFlags() (string, string, string, string) {
    var cdnBaseUrl string
    var srcDir string
    var destDir string
    flag.StringVar(&cdnBaseUrl, "cdn", "", "CDN base url")
    flag.StringVar(&srcDir, "src", "assets", "Source directory for assets (relative to SITEROOT)")
    flag.StringVar(&destDir, "dest", "assets-rev", "Destination directory for revisioned assets (relative to SITEROOT)")

    flag.Usage = func() {
        fmt.Println(getUsage())
        fmt.Println("Options:")
        flag.PrintDefaults()
    }

    flag.Parse()
    
    baseDir := "."
    if flag.NArg() > 0 {
        baseDir = flag.Arg(0)
    }

    return baseDir, cdnBaseUrl, srcDir, destDir
}

func main() {
    baseDir, cdnBaseUrl, srcDir, destDir := parseFlags()
    manifest := rev(baseDir, cdnBaseUrl, srcDir, destDir)
    useman(manifest, baseDir, srcDir)
    jsonData, err := json.MarshalIndent(manifest, "", "  ")
    check(err)
    fmt.Printf("%s\n", jsonData)
}
