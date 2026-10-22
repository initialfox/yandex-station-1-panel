package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	apkgDownloadURL = "https://raw.githubusercontent.com/initialfox/yandex-station-repo/pkg/apkg"
	apkgIndexURL    = "https://raw.githubusercontent.com/initialfox/yandex-station-repo/main/packages.json"
)

// на станциях resolv.conf часто ::1:53 → connection refused
var publicDNS = []string{
	"77.88.8.8:53",
	"8.8.8.8:53",
	"1.1.1.1:53",
	"9.9.9.9:53",
}

var dnsResolver = &net.Resolver{
	PreferGo: true,
	Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		d := net.Dialer{Timeout: 3 * time.Second}
		var last error
		for _, dns := range publicDNS {
			c, err := d.DialContext(ctx, "udp", dns)
			if err == nil {
				return c, nil
			}
			last = err
			c, err = d.DialContext(ctx, "tcp", dns)
			if err == nil {
				return c, nil
			}
			last = err
		}
		return nil, last
	},
}

func lookupPreferV4(ctx context.Context, host string) ([]string, error) {
	addrs, err := dnsResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	var v4, v6 []string
	for _, a := range addrs {
		s := a.IP.String()
		if a.IP.To4() != nil {
			v4 = append(v4, s)
		} else {
			v6 = append(v6, s)
		}
	}
	if len(v4) > 0 {
		return v4, nil
	}
	return v6, nil
}

var apkgHTTP = &http.Client{
	Timeout: 180 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		ForceAttemptHTTP2: false,
		TLSNextProto:      map[string]func(authority string, c *tls.Conn) http.RoundTripper{},
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			d := net.Dialer{Timeout: 15 * time.Second}
			if ip := net.ParseIP(host); ip != nil {
				return d.DialContext(ctx, network, addr)
			}
			ips, err := lookupPreferV4(ctx, host)
			if err != nil || len(ips) == 0 {
				return d.DialContext(ctx, network, addr)
			}
			var last error
			for _, ip := range ips {
				c, err := d.DialContext(ctx, network, net.JoinHostPort(ip, port))
				if err == nil {
					return c, nil
				}
				last = err
			}
			return nil, last
		},
	},
}

func findApkg(bindir string) string {
	cands := []string{
		filepath.Join(bindir, "apkg"),
		"/data/local/bin/apkg",
		"/usr/local/bin/apkg",
		"/opt/bin/apkg",
	}
	if p, err := exec.LookPath("apkg"); err == nil {
		cands = append([]string{p}, cands...)
	}
	for _, p := range cands {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// ensureApkg — если apkg нет, скачивает в bindir.
func ensureApkg(bindir string) (string, error) {
	if p := findApkg(bindir); p != "" {
		return p, nil
	}
	if err := os.MkdirAll(bindir, 0755); err != nil {
		return "", err
	}
	dst := filepath.Join(bindir, "apkg")
	tmp := dst + ".tmp"
	if err := downloadFile(apkgDownloadURL, tmp); err != nil {
		return "", fmt.Errorf("скачивание apkg: %w", err)
	}
	_ = os.Chmod(tmp, 0755)
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(dst)
		if err2 := os.Rename(tmp, dst); err2 != nil {
			return "", err2
		}
	}
	return dst, nil
}

func downloadFile(url, dst string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "alice-panel-apkg")
	res, err := apkgHTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", res.StatusCode)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, res.Body)
	return err
}

func fetchRemotePackages() (*repoMeta, error) {
	req, err := http.NewRequest("GET", apkgIndexURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "alice-panel-apkg")
	res, err := apkgHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("index HTTP %d", res.StatusCode)
	}
	var m repoMeta
	if err := json.NewDecoder(res.Body).Decode(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

func apkgEnv(apkgPath string) []string {
	dir := filepath.Dir(apkgPath)
	path := dir
	if p := os.Getenv("PATH"); p != "" {
		path = dir + ":" + p
	}
	return append(os.Environ(),
		"PATH="+path,
		"ANDROID_DATA=/data",
		"ANDROID_ROOT=/system",
	)
}

func apkgRun(apkgPath string, args ...string) (string, error) {
	cmd := exec.Command(apkgPath, args...)
	cmd.Env = apkgEnv(apkgPath)
	out, err := cmd.CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		if s == "" {
			s = err.Error()
		}
		return s, fmt.Errorf("%s", s)
	}
	return s, nil
}

func apkgUpdate(apkgPath string) (string, error) {
	return apkgRun(apkgPath, "update")
}

func apkgInstall(apkgPath, name string) (string, error) {
	return apkgRun(apkgPath, "install", name)
}

func apkgRemove(apkgPath, name string) (string, error) {
	return apkgRun(apkgPath, "remove", name)
}

func detectFlavor() string {
	if fileExists("/system/bin/linker64") || fileExists("/system/bin/linker") {
		return "android"
	}
	return "buildroot"
}

func defaultBindir(flavor string) string {
	if flavor == "buildroot" {
		for _, d := range []string{"/usr/local/bin", "/opt/bin", "/usr/bin"} {
			if canWriteDir(d) {
				return d
			}
		}
		return "/usr/local/bin"
	}
	return "/data/local/bin"
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func canWriteDir(d string) bool {
	if err := os.MkdirAll(d, 0755); err != nil {
		return false
	}
	f := filepath.Join(d, ".apkg-write-test")
	if err := os.WriteFile(f, []byte("1"), 0644); err != nil {
		return false
	}
	_ = os.Remove(f)
	return true
}

func packageInstalled(p pkgMeta, bindir string) bool {
	bin := p.Bin
	if bin == "" {
		bin = p.Name
	}
	checks := []string{
		filepath.Join(bindir, bin),
		"/data/local/bin/" + bin,
		"/usr/local/bin/" + bin,
		"/system/xbin/" + bin,
	}
	if p.Check != "" {
		checks = append([]string{p.Check}, checks...)
	}
	if p.Kind == "dir" && p.Dest != "" {
		checks = append(checks, filepath.Join(p.Dest, bin), filepath.Join(p.Dest, "bin", bin))
	}
	for _, c := range checks {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}
