package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	launchpadAPIBaseURL = "https://api.launchpad.net/1.0"
	distroName          = "ubuntu"
	distroVersion       = "jammy"
	distroArch          = "amd64"

	pkgsListPath = "./build/pkgs_list"

	requestTimeout = 5 * time.Second
)

type Package struct {
	Name    string
	Version string
}

func GetPublishedPackages(c *http.Client, names []string) ([]Package, error) {
	var pkgs []Package

	for _, pkgName := range names {
		ctx, cancelFn := context.WithTimeout(context.Background(), requestTimeout)
		defer cancelFn()

		url := fmt.Sprintf("%s/%s/+archive/primary", launchpadAPIBaseURL, distroName)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("request creation: %w", err)
		}

		q := req.URL.Query()
		q.Add("ws.op", "getPublishedBinaries")
		q.Add("binary_name", pkgName)
		q.Add("exact_match", "true")
		q.Add("distro_arch_series", fmt.Sprintf("%s/%s/%s/%s", launchpadAPIBaseURL, distroName, distroVersion, distroArch))
		q.Add("status", "Published")
		q.Add("order_by_date", "true")
		req.URL.RawQuery = q.Encode()

		resp, err := c.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		respData := map[string]any{}
		if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		entries, ok := respData["entries"].([]any)
		if !ok {
			return nil, fmt.Errorf("failed to parse response")
		}

		if len(entries) == 0 {
			break
		}

		entry, ok := entries[0].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("failed to parse entry")
		}

		name, ok := entry["binary_package_name"].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse package name")
		}

		version, ok := entry["binary_package_version"].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse package version")
		}

		log.Printf("found %s=%s\n", name, version)

		pkgs = append(pkgs, Package{Name: name, Version: version})
	}

	return pkgs, nil
}

func GenPinnedPackages(pkgsNames []string) error {
	c := &http.Client{}

	pkgs, err := GetPublishedPackages(c, pkgsNames)
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
	list := strings.Split(data, "\n")
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

	if err := GenPinnedPackages(parsePkgsList(string(data))); err != nil {
		log.Fatalf("failed to generate pinned packages: %s", err)
	}

	log.Printf("done")
}
