package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"pault.ag/go/debian/control"
)

const (
	baseURL      = "http://ftp.debian.org/debian/dists"
	suiteVersion = "sid"

	pkgsListPath = "./pkgs_list"

	requestTimeout = 30 * time.Second
)

type Package struct {
	Name    string
	Version string
}

func GetPublishedPackages(c *http.Client, names []string, arch string) ([]Package, error) {
	var pkgs []Package

	ctx, cancelFn := context.WithTimeout(context.Background(), requestTimeout)
	defer cancelFn()

	releaseFileURL := fmt.Sprintf("%s/%s/InRelease", baseURL, suiteVersion)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseFileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("request creation: %w", err)
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	defer resp.Body.Close()

	var controlFileHash string
	var controlFileSize int64
	controlFilePath := fmt.Sprintf("main/binary-%s/Packages.gz", arch)
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, controlFilePath) {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				controlFileHash = fields[0]
				controlFileSize, _ = strconv.ParseInt(fields[1], 10, 64)
			}
			break
		}
	}

	if controlFileHash == "" {
		return nil, fmt.Errorf("control file hash sum not found")
	}

	if controlFileSize <= 0 {
		return nil, fmt.Errorf("control file size not found")
	}

	controlFileURL := fmt.Sprintf("%s/%s/%s", baseURL, suiteVersion, controlFilePath)
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, controlFileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("request creation: %w", err)
	}

	resp, err = c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.ContentLength != controlFileSize {
		return nil, fmt.Errorf("control file response content length mismatch: %d vs %d", resp.ContentLength, controlFileSize)
	}

	h := md5.New()
	dataRd := io.TeeReader(io.LimitReader(resp.Body, controlFileSize), h)

	gzipRd, err := gzip.NewReader(dataRd)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}

	ctrl, err := control.ParseControl(bufio.NewReader(gzipRd), "")
	if err != nil {
		return nil, fmt.Errorf("failed to parse control file data: %w", err)
	}

	actualHash := hex.EncodeToString(h.Sum(nil))
	if controlFileHash != actualHash {
		return nil, fmt.Errorf("control file hash mismatch: %q vs %q", controlFileHash, actualHash)
	}

	for _, binary := range ctrl.Binaries {
		for _, pkgName := range names {
			if binary.Package == pkgName {
				version := binary.Values["Version"]
				log.Printf("found %s=%s\n", pkgName, version)
				pkgs = append(pkgs, Package{Name: pkgName, Version: version})
				break
			}
		}
	}

	return pkgs, nil
}

func GenPinnedPackages(pkgsNames []string, arch string) error {
	c := &http.Client{}

	pkgs, err := GetPublishedPackages(c, pkgsNames, arch)
	if err != nil {
		return fmt.Errorf("failed to get packages: %w", err)
	}

	outFile, err := os.OpenFile(pkgsListPath+"_"+arch, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("failed to open output file: %w", err)
	}
	defer outFile.Close()

	for _, pkg := range pkgs {
		fmt.Fprintf(outFile, "%s=%s\n", pkg.Name, pkg.Version)
	}

	return nil
}

func parsePkgsList(data string) []string {
	var pkgs []string
	list := strings.Split(strings.TrimSuffix(data, "\n"), "\n")
	for _, el := range list {
		name, _, _ := strings.Cut(el, "=")
		pkgs = append(pkgs, name)
	}
	return pkgs
}

func main() {
	arch := runtime.GOARCH
	if len(os.Args) > 1 {
		arch = os.Args[1]
	}

	data, err := os.ReadFile(pkgsListPath + "_" + arch)
	if err != nil {
		log.Fatalf("failed to read packages file: %s", err)
	}

	log.Printf("arch=%s", arch)

	if err := GenPinnedPackages(parsePkgsList(string(data)), arch); err != nil {
		log.Fatalf("failed to generate pinned packages: %s", err)
	}

	log.Printf("done")
}
