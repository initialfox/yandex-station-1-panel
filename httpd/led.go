package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	ledUtil     = "/system/vendor/quasar/led_util"
	ledLibs     = "/system/vendor/quasar/libs"
	patternDir  = "/system/vendor/quasar/ledpatterns"
	devPattern  = "/data/quasar/station-led.led"
	devPID      = "/data/quasar/led.pid"
	devLog      = "/data/quasar/led.log"
)

var (
	ledMu      sync.Mutex
	ledLastPath string
	hexColorRe = regexp.MustCompile(`(?i)^[0-9a-f]{6}$`)
	ledNameRe  = regexp.MustCompile(`^[A-Za-z0-9_.-]+\.led$`)
)

func suRun(args ...string) *exec.Cmd {
	// панель обычно уже root; если нет — через su 0
	if os.Geteuid() == 0 {
		return exec.Command(args[0], args[1:]...)
	}
	full := append([]string{"0"}, args...)
	return exec.Command("su", full...)
}

func suShell(cmd string) (string, error) {
	c := suRun("sh", "-c", cmd)
	out, err := c.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func ledReadPID() int {
	b, err := os.ReadFile(devPID)
	if err != nil {
		return 0
	}
	re := regexp.MustCompile(`\d+`)
	m := re.FindString(string(b))
	if m == "" {
		return 0
	}
	n, _ := strconv.Atoi(m)
	return n
}

func ledAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

func ledStopLocked() {
	if pid := ledReadPID(); pid > 0 {
		_ = suRun("kill", strconv.Itoa(pid)).Run()
	}
	if ledLastPath != "" {
		_, _ = suShell(fmt.Sprintf("pkill -f %q 2>/dev/null || true", "led_util "+ledLastPath))
	}
	_ = os.Remove(devPID)
	_ = os.Remove(devLog)
}

func ledStop() {
	ledMu.Lock()
	defer ledMu.Unlock()
	ledStopLocked()
	ledLastPath = ""
}

func ledPlayPath(path string) (int, error) {
	ledMu.Lock()
	defer ledMu.Unlock()
	ledStopLocked()

	_ = os.MkdirAll(filepath.Dir(devPID), 0755)
	inner := fmt.Sprintf(
		"export LD_LIBRARY_PATH=%s; echo $$ > %s; exec %s %s >%s 2>&1",
		ledLibs, devPID, ledUtil, shellQuote(path), devLog,
	)
	cmd := suRun("sh", "-c", inner)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	go func() { _ = cmd.Wait() }()

	time.Sleep(700 * time.Millisecond)
	pid := ledReadPID()
	if pid == 0 || !ledAlive(pid) {
		logb, _ := os.ReadFile(devLog)
		msg := strings.TrimSpace(string(logb))
		if msg == "" {
			msg = "нет вывода"
		}
		return 0, fmt.Errorf("led_util не запустился: %s", msg)
	}
	ledLastPath = path
	return pid, nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

type ledFrame struct {
	Colors []string `json:"colors"`
	MS     int      `json:"ms"`
}

func buildLEDPattern(frames []ledFrame, smooth int) (string, error) {
	var b strings.Builder
	b.WriteString("gammacorrection\n")
	if smooth > 0 {
		fmt.Fprintf(&b, "smooth %d\n", smooth)
	}
	b.WriteString("loop\n")
	for _, f := range frames {
		if len(f.Colors) != 24 {
			return "", fmt.Errorf("в кадре должно быть 24 цвета")
		}
		for _, c := range f.Colors {
			if !hexColorRe.MatchString(c) {
				return "", fmt.Errorf("неверный цвет: %q", c)
			}
		}
		if f.MS < 1 || f.MS > 60000 {
			return "", fmt.Errorf("длительность кадра вне 1..60000 мс")
		}
		b.WriteString(strings.Join(f.Colors, " "))
		fmt.Fprintf(&b, " %d\n", f.MS)
	}
	return b.String(), nil
}

func writeLEDPattern(text string) error {
	if err := os.MkdirAll(filepath.Dir(devPattern), 0755); err != nil {
		return err
	}
	tmp := devPattern + ".tmp"
	if err := os.WriteFile(tmp, []byte(text), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, devPattern)
}

func listLEDPatterns() ([]string, error) {
	ents, err := os.ReadDir(patternDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".led") {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

func registerLED(mux *http.ServeMux) {
	mux.HandleFunc("/api/led/status", func(w http.ResponseWriter, r *http.Request) {
		pid := ledReadPID()
		writeJSON(w, map[string]any{"pid": pid, "alive": ledAlive(pid)})
	})
	mux.HandleFunc("/api/led/patterns", func(w http.ResponseWriter, r *http.Request) {
		names, err := listLEDPatterns()
		if err != nil {
			writeJSON(w, map[string]any{"patterns": []string{}, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"patterns": names})
	})
	mux.HandleFunc("/api/led/play", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", 405)
			return
		}
		var body struct {
			Frames []ledFrame `json:"frames"`
			Smooth int        `json:"smooth"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if n := len(body.Frames); n < 1 || n > 2000 {
			http.Error(w, "нужен список кадров 1..2000", 400)
			return
		}
		if body.Smooth < 0 || body.Smooth > 1000 {
			http.Error(w, "smooth вне 0..1000", 400)
			return
		}
		text, err := buildLEDPattern(body.Frames, body.Smooth)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if err := writeLEDPattern(text); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		pid, err := ledPlayPath(devPattern)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "pid": pid})
	})
	mux.HandleFunc("/api/led/builtin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", 405)
			return
		}
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		name := body.Name
		if !ledNameRe.MatchString(name) || strings.Contains(name, "..") {
			http.Error(w, "неверное имя паттерна", 400)
			return
		}
		path := filepath.Join(patternDir, name)
		if st, err := os.Stat(path); err != nil || st.IsDir() {
			http.Error(w, "паттерн не найден", 404)
			return
		}
		pid, err := ledPlayPath(path)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "pid": pid})
	})
	mux.HandleFunc("/api/led/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", 405)
			return
		}
		ledStop()
		writeJSON(w, map[string]any{"ok": true})
	})
}
