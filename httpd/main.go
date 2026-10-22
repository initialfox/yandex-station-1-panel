// alice-httpd — Alice Panel для Яндекс Станции 1 (tulip-d1 / aarch64).
package main

import (
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
	"syscall"
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
	Asset       string `json:"asset"`
	MD5         string `json:"md5"`
	Size        int64  `json:"size"`
	Bin         string `json:"bin"`
	Kind        string `json:"kind"`     // file (default) | dir
	Dest        string `json:"dest"`     // for dir: install prefix, e.g. /data/dropbear
	Check       string `json:"check"`    // path to detect installed
	Category    string `json:"category"` // optional (apkg index)
}

type repoMeta struct {
	Name        string    `json:"name"`
	Arch        string    `json:"arch"`
	Description string    `json:"description"`
	Packages    []pkgMeta `json:"packages"`
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

const defaultUpdateRepo = "initialfox/yandex-station-1-panel"

func main() {
	addr := flag.String("listen", ":8080", "listen address")
	root := flag.String("root", ".", "web root (index.html + assets)")
	profile := flag.String("profile", "", "dir with station-profile.json (optional)")
	// -repo оставлен для совместимости со старыми start.sh → читаем только profile
	_ = flag.String("repo", "", "deprecated: ignored (packages via apkg)")
	bindirFlag := flag.String("bindir", "", "install target (auto: android/buildroot)")
	shellPath := flag.String("shell", "", "shell binary (default: zsh in bindir or /system/bin/sh)")
	flag.Parse()

	panelRoot, _ := filepath.Abs(*root)
	version := readVersion(panelRoot)
	flavor := detectFlavor()

	bindir := *bindirFlag
	if bindir == "" {
		bindir = defaultBindir(flavor)
	}

	profileDir := *profile
	if profileDir == "" {
		for _, c := range []string{
			filepath.Join(*root, "repo"),
			filepath.Join(*root, "profile"),
			*root,
		} {
			if st, err := os.Stat(filepath.Join(c, "station-profile.json")); err == nil && !st.IsDir() {
				profileDir = c
				break
			}
		}
		if profileDir == "" {
			profileDir = *root
		}
	}

	shell := *shellPath
	if shell == "" {
		cand := filepath.Join(bindir, "zsh")
		if _, err := os.Stat(cand); err == nil {
			shell = cand
		} else if flavor == "android" {
			shell = "/system/bin/sh"
		} else {
			shell = "/bin/sh"
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"ok": true, "role": "alice-panel",
			"target": "yandex-station-gen1", "device": "tulip-d1", "platform": "yandexstation",
			"version": version, "flavor": flavor, "bindir": bindir,
			"packages": "apkg",
		})
	})
	mux.HandleFunc("/api/update/check", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, checkUpdate(version))
	})
	mux.HandleFunc("/api/update", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", 405)
			return
		}
		if err := startSelfUpdate(); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "updating": true, "message": "переустановка запущена"})
	})
	mux.HandleFunc("/api/system", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, buildSystem(profileDir))
	})
	mux.HandleFunc("/api/packages", func(w http.ResponseWriter, r *http.Request) {
		apkgPath, _ := ensureApkg(bindir)
		meta, err := fetchRemotePackages()
		if err != nil {
			http.Error(w, "apkg index: "+err.Error(), 502)
			return
		}
		installed := map[string]bool{}
		for _, p := range meta.Packages {
			if packageInstalled(p, bindir) {
				installed[p.Name] = true
			}
		}
		writeJSON(w, map[string]any{
			"repo": meta, "installed": installed, "bindir": bindir,
			"source": "apkg", "apkg": apkgPath, "apkg_present": apkgPath != "",
			"flavor": flavor, "index_url": apkgIndexURL,
		})
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
		apkgPath, err := ensureApkg(bindir)
		if err != nil {
			http.Error(w, "apkg: "+err.Error(), 500)
			return
		}
		upd, _ := apkgUpdate(apkgPath)
		out, err := apkgInstall(apkgPath, name)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]any{
			"ok": true, "name": name, "path": bindir,
			"via": "apkg", "log": out, "update": upd, "apkg": apkgPath,
		})
	})
	mux.HandleFunc("/api/remove/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", 405)
			return
		}
		name := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/remove/"), "/")
		if name == "" || strings.Contains(name, "..") || strings.Contains(name, "/") {
			http.Error(w, "bad name", 400)
			return
		}
		apkgPath, err := ensureApkg(bindir)
		if err != nil {
			http.Error(w, "apkg: "+err.Error(), 500)
			return
		}
		out, err := apkgRemove(apkgPath, name)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "name": name, "via": "apkg", "log": out})
	})

	mux.HandleFunc("/ws/shell", func(w http.ResponseWriter, r *http.Request) {
		handleShell(w, r, shell)
	})

	mux.HandleFunc("/api/display", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, readDisplayInfo())
	})
	mux.HandleFunc("/api/display/screenshot", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			http.Error(w, "GET/POST only", 405)
			return
		}
		png, err := takeScreenshot()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Disposition", `inline; filename="station-screen.png"`)
		_, _ = w.Write(png)
	})

	registerLED(mux)

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

	log.Printf("Alice Panel · Станция 1 v%s flavor=%s listen=%s root=%s bindir=%s shell=%s packages=apkg",
		version, flavor, *addr, *root, bindir, shell)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func readVersion(root string) string {
	for _, p := range []string{
		filepath.Join(root, "VERSION"),
		filepath.Join(root, "..", "VERSION"),
	} {
		b, err := os.ReadFile(p)
		if err == nil {
			v := strings.TrimSpace(string(b))
			if v != "" {
				return strings.TrimPrefix(v, "v")
			}
		}
	}
	return "0.0.0"
}

func normalizeVer(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

func checkUpdate(current string) map[string]any {
	current = normalizeVer(current)
	out := map[string]any{
		"current":   current,
		"latest":    current,
		"update":    false,
		"tag":       "",
		"url":       "",
		"checkedAt": time.Now().Unix(),
	}
	repo := os.Getenv("PANEL_REPO")
	if repo == "" {
		repo = defaultUpdateRepo
	}
	apiURL := "https://api.github.com/repos/" + repo + "/releases/latest"
	body, err := httpGetInsecure(apiURL)
	if err != nil {
		out["error"] = err.Error()
		return out
	}
	var rel struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		out["error"] = err.Error()
		return out
	}
	latest := normalizeVer(rel.TagName)
	out["latest"] = latest
	out["tag"] = rel.TagName
	out["url"] = rel.HTMLURL
	out["update"] = versionLess(current, latest)
	return out
}

func versionLess(a, b string) bool {
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x < y {
			return true
		}
		if x > y {
			return false
		}
	}
	return false
}

func httpGetInsecure(url string) ([]byte, error) {
	// на станции curl без CA
	cmd := exec.Command("curl", "-kfsSL", "--max-time", "20", url)
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		return out, nil
	}
	// fallback: обычный http.Client (локальная сборка / если curl нет)
	client := &http.Client{Timeout: 20 * time.Second}
	req, err2 := http.NewRequest(http.MethodGet, url, nil)
	if err2 != nil {
		return nil, fmt.Errorf("curl: %v; req: %v", err, err2)
	}
	req.Header.Set("User-Agent", "alice-panel")
	res, err2 := client.Do(req)
	if err2 != nil {
		return nil, fmt.Errorf("curl: %v; http: %v", err, err2)
	}
	defer res.Body.Close()
	b, err2 := io.ReadAll(res.Body)
	if err2 != nil {
		return nil, err2
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("github HTTP %d", res.StatusCode)
	}
	return b, nil
}

func startSelfUpdate() error {
	script := `#!/system/bin/sh
export ANDROID_DATA=/data
export ANDROID_ROOT=/system
setenforce 0 2>/dev/null || true
sleep 2
curl -kfsSL https://cdn.jsdelivr.net/gh/initialfox/yandex-station-1-panel@main/install.sh | sh > /data/local/tmp/alice-panel-update.log 2>&1
`
	path := "/data/local/tmp/alice-panel-do-update.sh"
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		// локальная разработка
		path = filepath.Join(os.TempDir(), "alice-panel-do-update.sh")
		if err2 := os.WriteFile(path, []byte(script), 0755); err2 != nil {
			return err
		}
	}
	cmd := exec.Command("setsid", "sh", path)
	cmd.Dir = "/"
	if err := cmd.Start(); err != nil {
		// без setsid
		cmd = exec.Command("sh", path)
		if err2 := cmd.Start(); err2 != nil {
			return fmt.Errorf("start update: %v / %v", err, err2)
		}
	}
	go func() { _ = cmd.Wait() }()
	return nil
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
		"storage":    readStorage(),
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

// readStorage — ПЗУ: размер eMMC + занятость ключевых разделов (/data, /system, /cache).
func readStorage() map[string]any {
	out := map[string]any{}
	if b := strings.TrimSpace(readFile("/sys/block/mmcblk0/size")); b != "" {
		if sectors, err := strconv.ParseInt(b, 10, 64); err == nil && sectors > 0 {
			totalB := sectors * 512
			out["emmc_total_kb"] = totalB / 1024
			out["emmc_total_human"] = fmtBytes(totalB)
			out["device"] = "mmcblk0"
		}
	}
	mounts := []map[string]any{}
	var dataUsedPct float64
	var dataTotalKB, dataAvailKB int64
	for _, mp := range []string{"/data", "/system", "/cache", "/sdcard"} {
		m := readMount(mp)
		if m == nil {
			continue
		}
		mounts = append(mounts, m)
		if mp == "/data" {
			dataUsedPct, _ = m["used_pct"].(float64)
			dataTotalKB, _ = m["total_kb"].(int64)
			dataAvailKB, _ = m["available_kb"].(int64)
		}
	}
	out["mounts"] = mounts
	if dataTotalKB > 0 {
		out["data_total_kb"] = dataTotalKB
		out["data_available_kb"] = dataAvailKB
		out["data_used_pct"] = dataUsedPct
	}
	return out
}

func readMount(path string) map[string]any {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return nil
	}
	bsize := int64(st.Bsize)
	if bsize <= 0 {
		return nil
	}
	total := int64(st.Blocks) * bsize
	avail := int64(st.Bavail) * bsize
	used := total - avail
	if used < 0 {
		used = 0
	}
	pct := 0.0
	if total > 0 {
		pct = float64(used) * 100 / float64(total)
	}
	return map[string]any{
		"path":          path,
		"total_kb":      total / 1024,
		"used_kb":       used / 1024,
		"available_kb":  avail / 1024,
		"used_pct":      float64(int(pct*10)) / 10,
		"total_human":   fmtBytes(total),
		"used_human":    fmtBytes(used),
		"avail_human":   fmtBytes(avail),
	}
}

func fmtBytes(b int64) string {
	if b >= 1024*1024*1024 {
		return fmt.Sprintf("%.1f ГБ", float64(b)/(1024*1024*1024))
	}
	if b >= 1024*1024 {
		return fmt.Sprintf("%.0f МБ", float64(b)/(1024*1024))
	}
	if b >= 1024 {
		return fmt.Sprintf("%.0f КБ", float64(b)/1024)
	}
	return fmt.Sprintf("%d Б", b)
}

func readFile(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}

func takeScreenshot() ([]byte, error) {
	// Android screencap → PNG на stdout
	screencap := "/system/bin/screencap"
	if _, err := os.Stat(screencap); err != nil {
		screencap = "screencap"
	}
	cmd := exec.Command(screencap, "-p")
	out, err := cmd.Output()
	if err != nil {
		// fallback: файл во /sdcard
		tmp := fmt.Sprintf("/sdcard/alice-panel-shot-%d.png", time.Now().UnixNano())
		cmd2 := exec.Command(screencap, "-p", tmp)
		if err2 := cmd2.Run(); err2 != nil {
			return nil, fmt.Errorf("screencap: %v / %v", err, err2)
		}
		b, rerr := os.ReadFile(tmp)
		_ = os.Remove(tmp)
		if rerr != nil {
			return nil, rerr
		}
		if len(b) < 8 {
			return nil, fmt.Errorf("пустой скриншот")
		}
		return b, nil
	}
	if len(out) < 8 {
		return nil, fmt.Errorf("пустой скриншот")
	}
	return out, nil
}

func readDisplayInfo() map[string]any {
	info := map[string]any{
		"width":  1920,
		"height": 1080,
		"hdmi":   "1920x1080",
		"fps":    60,
		"state":  "unknown",
		"source": "fallback",
	}
	// wm size
	if out, err := exec.Command("wm", "size").CombinedOutput(); err == nil {
		s := strings.TrimSpace(string(out))
		info["wm_size"] = s
		// Physical size: 1920x1080
		for _, part := range strings.Fields(s) {
			if strings.Contains(part, "x") && !strings.Contains(part, ":") {
				xy := strings.Split(part, "x")
				if len(xy) == 2 {
					if w, e := strconv.Atoi(xy[0]); e == nil {
						info["width"] = w
					}
					if h, e := strconv.Atoi(xy[1]); e == nil {
						info["height"] = h
					}
					info["hdmi"] = part
					info["source"] = "wm"
				}
			}
		}
	}
	dump := ""
	if out, err := exec.Command("dumpsys", "display").CombinedOutput(); err == nil {
		dump = string(out)
	}
	if dump != "" {
		info["source"] = "dumpsys"
		if strings.Contains(dump, "state ON") || strings.Contains(dump, "mGlobalDisplayState=ON") {
			info["state"] = "ON"
		} else if strings.Contains(dump, "state OFF") || strings.Contains(dump, "mGlobalDisplayState=OFF") {
			info["state"] = "OFF"
		}
		for _, line := range strings.Split(dump, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "mBrightness=") {
				info["brightness"] = strings.TrimPrefix(line, "mBrightness=")
			}
			if strings.Contains(line, "PhysicalDisplayInfo{") {
				// PhysicalDisplayInfo{1920 x 1080, 60.000004 fps, ...
				rest := strings.TrimPrefix(line, "PhysicalDisplayInfo{")
				fields := strings.Split(rest, ",")
				if len(fields) > 0 {
					wh := strings.Fields(fields[0])
					if len(wh) >= 3 && wh[1] == "x" {
						if w, e := strconv.Atoi(wh[0]); e == nil {
							info["width"] = w
						}
						if h, e := strconv.Atoi(wh[2]); e == nil {
							info["height"] = h
						}
						info["hdmi"] = fmt.Sprintf("%vx%v", info["width"], info["height"])
					}
				}
				if len(fields) > 1 && strings.Contains(fields[1], "fps") {
					info["fps"] = strings.TrimSpace(strings.Replace(fields[1], "fps", "", 1))
				}
			}
		}
	}
	info["screencap"] = fileExists("/system/bin/screencap")
	info["ts"] = time.Now().Unix()
	return info
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
