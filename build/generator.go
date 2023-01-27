package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

const (
	apiBaseURL   = "https://packages.debian.org/search"
	suiteVersion = "sid"

	pkgsListPath = "./build/pkgs_list"

	requestTimeout = 5 * time.Second
)

type Package struct {
	Name    string
	Version string
}

func GetPublishedPackages(c *http.Client, names []string, arch string) ([]Package, error) {
	var pkgs []Package

	for _, pkgName := range names {
		ctx, cancelFn := context.WithTimeout(context.Background(), requestTimeout)
		defer cancelFn()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBaseURL, nil)
		if err != nil {
			return nil, fmt.Errorf("request creation: %w", err)
		}

		q := req.URL.Query()
		q.Add("keywords", pkgName)
		q.Add("searchon", "names")
		q.Add("exact", "1")
		q.Add("suite", suiteVersion)
		q.Add("section", "all")
		q.Add("arch", arch)
		req.URL.RawQuery = q.Encode()

		resp, err := c.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		var version string
		suffixes := []string{fmt.Sprintf(": %s", arch), ": all"}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasSuffix(line, suffixes[0]) || strings.HasSuffix(line, suffixes[1]) {
				line = strings.TrimSpace(line)
				version = strings.TrimPrefix(line, "<br>")
				version = strings.TrimSuffix(version, suffixes[0])
				version = strings.TrimSuffix(version, suffixes[1])
				break
			}
		}

		if version == "" || strings.Contains(version, " ") {
			return nil, fmt.Errorf("failed to parse package version for %s", pkgName)
		}

		log.Printf("found %s=%s\n", pkgName, version)

		pkgs = append(pkgs, Package{Name: pkgName, Version: version})
	}

	return pkgs, nil
}

func GenPinnedPackages(pkgsNames []string, arch string) error {
	c := &http.Client{}

	pkgs, err := GetPublishedPackages(c, pkgsNames, arch)
	if err != nil {
		return fmt.Errorf("failed to get packages: %w", err)
	}

	outFile, err := os.OpenFile(pkgsListPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
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
	data, err := os.ReadFile(pkgsListPath)
	if err != nil {
		log.Fatalf("failed to read packages file: %s", err)
	}

	log.Printf("arch=%s", runtime.GOARCH)

	if err := GenPinnedPackages(parsePkgsList(string(data)), runtime.GOARCH); err != nil {
		log.Fatalf("failed to generate pinned packages: %s", err)
	}

	log.Printf("done")
}
