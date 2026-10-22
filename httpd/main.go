// alice-httpd — Alice Panel для Яндекс Станции 1 (tulip-d1 / aarch64).
package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

type pkgMeta struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Arch        string `json:"arch"`
	Description string `json:"description"`
	URL         string `json:"url"`
	MD5         string `json:"md5"`
	Size        int64  `json:"size"`
	Bin         string `json:"bin"`
}

type repoMeta struct {
	Name        string    `json:"name"`
	Arch        string    `json:"arch"`
	Description string    `json:"description"`
	Packages    []pkgMeta `json:"packages"`
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func main() {
	addr := flag.String("listen", ":8080", "listen address")
	root := flag.String("root", ".", "web root (index.html + assets)")
	repo := flag.String("repo", "", "repo dir with packages.json")
	bindir := flag.String("bindir", "/data/local/bin", "install target")
	shellPath := flag.String("shell", "", "shell binary (default: zsh in bindir or /system/bin/sh)")
	flag.Parse()

	repoDir := *repo
	if repoDir == "" {
		if st, err := os.Stat(filepath.Join(*root, "repo", "packages.json")); err == nil && !st.IsDir() {
			repoDir = filepath.Join(*root, "repo")
		} else {
			repoDir = *root
		}
	}

	shell := *shellPath
	if shell == "" {
		cand := filepath.Join(*bindir, "zsh")
		if _, err := os.Stat(cand); err == nil {
			shell = cand
		} else {
			shell = "/system/bin/sh"
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"ok": true, "role": "alice-panel",
			"target": "yandex-station-gen1", "device": "tulip-d1", "platform": "yandexstation",
		})
	})
	mux.HandleFunc("/api/system", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, buildSystem(repoDir))
	})
	mux.HandleFunc("/api/packages", func(w http.ResponseWriter, r *http.Request) {
		meta, err := loadRepo(repoDir)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		installed := map[string]bool{}
		for _, p := range meta.Packages {
			bin := p.Bin
			if bin == "" {
				bin = p.Name
			}
			if _, err := os.Stat(filepath.Join(*bindir, bin)); err == nil {
				installed[p.Name] = true
			}
		}
		writeJSON(w, map[string]any{"repo": meta, "installed": installed, "bindir": *bindir})
	})
	mux.HandleFunc("/api/install/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", 405)
			return
		}
		name := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/install/"), "/")
		if name == "" || strings.Contains(name, "..") || strings.Contains(name, "/") {
			http.Error(w, "bad name", 400)
			return
		}
		meta, err := loadRepo(repoDir)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		var pkg *pkgMeta
		for i := range meta.Packages {
			if meta.Packages[i].Name == name {
				pkg = &meta.Packages[i]
				break
			}
		}
		if pkg == nil {
			http.Error(w, "package not found", 404)
			return
		}
		src := filepath.Join(repoDir, filepath.FromSlash(pkg.URL))
		bin := pkg.Bin
		if bin == "" {
			bin = pkg.Name
		}
		dst := filepath.Join(*bindir, bin)
		if err := os.MkdirAll(*bindir, 0755); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		sum, size, err := copyFile(src, dst)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if pkg.MD5 != "" && !strings.EqualFold(sum, pkg.MD5) {
			_ = os.Remove(dst)
			http.Error(w, fmt.Sprintf("md5 mismatch: got %s want %s", sum, pkg.MD5), 500)
			return
		}
		_ = os.Chmod(dst, 0755)
		tryXbinLink(dst, bin)
		writeJSON(w, map[string]any{
			"ok": true, "name": name, "path": dst, "md5": sum, "size": size, "version": pkg.Version,
		})
	})

	mux.HandleFunc("/ws/shell", func(w http.ResponseWriter, r *http.Request) {
		handleShell(w, r, shell)
	})

	mux.Handle("/packages/", http.StripPrefix("/packages/", http.FileServer(http.Dir(filepath.Join(repoDir, "packages")))))
	fileServer := http.FileServer(http.Dir(*root))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Join(*root, filepath.Clean("/"+r.URL.Path))
		if r.URL.Path != "/" {
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		http.ServeFile(w, r, filepath.Join(*root, "index.html"))
	})

	log.Printf("Alice Panel · Станция 1 listen=%s root=%s repo=%s shell=%s", *addr, *root, repoDir, shell)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func handleShell(w http.ResponseWriter, r *http.Request, shell string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "HOME=/data/local", "PATH=/data/local/bin:/system/xbin:/system/bin:/sbin")
	cmd.Dir = "/data/local"
	ptmx, err := pty.Start(cmd)
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("pty error: "+err.Error()+"\r\n"))
		return
	}
	defer func() { _ = ptmx.Close(); _ = cmd.Process.Kill() }()

	_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[32m# Alice Panel · Станция 1\x1b[0m "+shell+"\r\n"))

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				_ = conn.WriteMessage(websocket.BinaryMessage, buf[:n])
			}
			if err != nil {
				_ = conn.Close()
				return
			}
		}
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		s := string(msg)
		var j struct {
			Type string `json:"type"`
			Data string `json:"data"`
			Cols int    `json:"cols"`
			Rows int    `json:"rows"`
		}
		if json.Unmarshal(msg, &j) == nil && j.Type != "" {
			switch j.Type {
			case "input":
				_, _ = ptmx.Write([]byte(j.Data))
			case "resize":
				if j.Cols > 0 && j.Rows > 0 {
					_ = pty.Setsize(ptmx, &pty.Winsize{Cols: uint16(j.Cols), Rows: uint16(j.Rows)})
				}
			}
			continue
		}
		_, _ = ptmx.Write([]byte(s))
	}
}

func buildSystem(repoDir string) map[string]any {
	station := map[string]any{}
	if b, err := os.ReadFile(filepath.Join(repoDir, "station-profile.json")); err == nil {
		_ = json.Unmarshal(b, &station)
	}
	live := map[string]any{
		"host":       readHostname(),
		"platform":   "android",
		"arch":       "arm64",
		"uptime_sec": readUptime(),
		"memory":     readMem(),
		"loadavg":    readLoad(),
		"cpus":       4,
		"ts":         time.Now().Unix(),
	}
	// refresh quasar versions live if possible
	if v := strings.TrimSpace(readFile("/system/vendor/quasar/version_full")); v != "" {
		if q, ok := station["quasar"].(map[string]any); ok {
			q["software_version"] = v
		}
	}
	return map[string]any{"station": station, "live": live, "source": "device"}
}

func readHostname() string {
	b, _ := os.ReadFile("/proc/sys/kernel/hostname")
	return strings.TrimSpace(string(b))
}

func readUptime() float64 {
	parts := strings.Fields(readFile("/proc/uptime"))
	if len(parts) == 0 {
		return 0
	}
	f, _ := strconv.ParseFloat(parts[0], 64)
	return f
}

func readLoad() map[string]float64 {
	parts := strings.Fields(readFile("/proc/loadavg"))
	out := map[string]float64{"1": 0, "5": 0, "15": 0}
	if len(parts) >= 3 {
		out["1"], _ = strconv.ParseFloat(parts[0], 64)
		out["5"], _ = strconv.ParseFloat(parts[1], 64)
		out["15"], _ = strconv.ParseFloat(parts[2], 64)
	}
	return out
}

func readMem() map[string]any {
	total, free, cached, buffers := 0, 0, 0, 0
	for _, line := range strings.Split(readFile("/proc/meminfo"), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		v, _ := strconv.Atoi(f[1])
		switch f[0] {
		case "MemTotal:":
			total = v
		case "MemFree:":
			free = v
		case "Cached:":
			cached = v
		case "Buffers:":
			buffers = v
		}
	}
	avail := free + cached + buffers
	usedPct := 0.0
	if total > 0 {
		usedPct = float64(total-avail) * 100 / float64(total)
	}
	return map[string]any{"total_kb": total, "available_kb": avail, "used_pct": float64(int(usedPct*10)) / 10}
}

func readFile(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}

func loadRepo(dir string) (*repoMeta, error) {
	b, err := os.ReadFile(filepath.Join(dir, "packages.json"))
	if err != nil {
		return nil, err
	}
	var m repoMeta
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func copyFile(src, dst string) (string, int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", 0, err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return "", 0, err
	}
	h := md5.New()
	n, err := io.Copy(io.MultiWriter(out, h), in)
	cerr := out.Close()
	if err != nil {
		_ = os.Remove(tmp)
		return "", 0, err
	}
	if cerr != nil {
		_ = os.Remove(tmp)
		return "", 0, cerr
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func tryXbinLink(dst, bin string) {
	xbin := filepath.Join("/system/xbin", bin)
	_ = exec.Command("mount", "-o", "remount,rw", "/system").Run()
	_ = os.Remove(xbin)
	_ = os.Symlink(dst, xbin)
	_ = exec.Command("mount", "-o", "remount,ro", "/system").Run()
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
