import cors from "cors";
import express from "express";
import fs from "fs";
import path from "path";
import crypto from "crypto";
import os from "os";
import { fileURLToPath } from "url";
import { WebSocketServer } from "ws";
import http from "http";
import { spawn } from "child_process";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, "..");
const REPO = path.join(ROOT, "repo");
const WEB_DIST = path.join(ROOT, "web", "dist");
const BINDIR = process.env.BINDIR || path.join(ROOT, ".local-bin");
const PORT = Number(process.env.PORT || 8080);
const PROFILE = path.join(REPO, "station-profile.json");

const app = express();
app.use(cors());
app.use(express.json());

function loadRepo() {
  return JSON.parse(fs.readFileSync(path.join(REPO, "packages.json"), "utf8"));
}

function loadProfile() {
  return JSON.parse(fs.readFileSync(PROFILE, "utf8"));
}

function md5File(file) {
  const h = crypto.createHash("md5");
  h.update(fs.readFileSync(file));
  return h.digest("hex");
}

function hostStats() {
  const total = os.totalmem();
  const free = os.freemem();
  const load = os.loadavg();
  return {
    host: os.hostname(),
    platform: os.platform(),
    arch: os.arch(),
    uptime_sec: os.uptime(),
    memory: {
      total_kb: Math.round(total / 1024),
      available_kb: Math.round(free / 1024),
      used_pct: Math.round(100 * (1 - free / total) * 10) / 10,
    },
    loadavg: { "1": load[0], "5": load[1], "15": load[2] },
    cpus: os.cpus().length,
  };
}

function readVersion() {
  try {
    return fs.readFileSync(path.join(ROOT, "VERSION"), "utf8").trim().replace(/^v/, "");
  } catch {
    return "0.0.0-dev";
  }
}

app.get("/api/health", (_req, res) =>
  res.json({
    ok: true,
    role: "alice-panel",
    target: "yandex-station-gen1",
    device: "tulip-d1",
    platform: "yandexstation",
    version: readVersion(),
  })
);

app.get("/api/update/check", async (_req, res) => {
  const current = readVersion();
  try {
    const r = await fetch("https://api.github.com/repos/initialfox/yandex-station-1-panel/releases/latest", {
      headers: { "User-Agent": "alice-panel" },
    });
    const j = await r.json();
    const latest = String(j.tag_name || "").replace(/^v/, "");
    const update = latest.localeCompare(current, undefined, { numeric: true }) > 0;
    res.json({ current, latest, update, tag: j.tag_name, url: j.html_url });
  } catch (e) {
    res.json({ current, latest: current, update: false, error: String(e.message || e) });
  }
});

app.post("/api/update", (_req, res) => {
  res.status(501).json({ error: "обновление только на колонке (alice-httpd)" });
});

app.get("/api/display", (_req, res) => {
  res.json({
    width: 1920,
    height: 1080,
    hdmi: "1920x1080",
    fps: 60,
    state: "local-dev",
    screencap: false,
    source: "local",
  });
});

app.get("/api/display/screenshot", (_req, res) => {
  res.status(501).json({ error: "скриншот только на колонке" });
});

app.get("/api/system", (_req, res) => {
  const station = loadProfile();
  const live = hostStats();
  // панель показывает профиль Станции; live — метрики машины, где крутится API (локально Mac)
  res.json({
    station,
    live,
    source: "station-profile.json + local host",
  });
});

app.get("/api/packages", (_req, res) => {
  const repo = loadRepo();
  const installed = {};
  for (const p of repo.packages) {
    const check =
      p.check ||
      (p.kind === "dir" && p.dest ? path.join(p.dest, p.bin || p.name) : path.join(BINDIR, p.bin || p.name));
    installed[p.name] = fs.existsSync(check);
  }
  res.json({ repo, installed, bindir: BINDIR });
});

app.post("/api/install/:name", (req, res) => {
  const name = req.params.name;
  if (!name || name.includes("..") || name.includes("/")) {
    return res.status(400).json({ error: "bad name" });
  }
  const repo = loadRepo();
  const pkg = repo.packages.find((p) => p.name === name);
  if (!pkg) return res.status(404).json({ error: "package not found" });

  const src = path.join(REPO, pkg.url);
  const bin = pkg.bin || pkg.name;

  if (pkg.kind === "dir") {
    const dest = pkg.dest || path.join(BINDIR, bin);
    fs.mkdirSync(dest, { recursive: true });
    for (const f of fs.readdirSync(src)) {
      if (f.endsWith("-ssh")) continue;
      const from = path.join(src, f);
      if (fs.statSync(from).isDirectory()) continue;
      fs.copyFileSync(from, path.join(dest, f));
      fs.chmodSync(path.join(dest, f), 0o755);
    }
    const wrap = path.join(src, `${bin}-ssh`);
    if (fs.existsSync(wrap)) {
      fs.mkdirSync(BINDIR, { recursive: true });
      fs.copyFileSync(wrap, path.join(BINDIR, bin));
      fs.chmodSync(path.join(BINDIR, bin), 0o755);
    }
    return res.json({ ok: true, name, path: dest, kind: "dir", version: pkg.version });
  }

  const dst = path.join(BINDIR, bin);
  fs.mkdirSync(BINDIR, { recursive: true });
  fs.copyFileSync(src, dst);
  fs.chmodSync(dst, 0o755);
  const sum = md5File(dst);
  if (pkg.md5 && sum !== pkg.md5) {
    fs.unlinkSync(dst);
    return res.status(500).json({ error: `md5 mismatch got ${sum}` });
  }
  res.json({ ok: true, name, path: dst, md5: sum, size: fs.statSync(dst).size, version: pkg.version });
});

app.use("/packages", express.static(path.join(REPO, "packages")));

if (fs.existsSync(WEB_DIST)) {
  app.use(express.static(WEB_DIST));
  app.get("*", (req, res, next) => {
    if (req.path.startsWith("/api") || req.path.startsWith("/packages")) return next();
    res.sendFile(path.join(WEB_DIST, "index.html"));
  });
}

fs.mkdirSync(BINDIR, { recursive: true });
const server = http.createServer(app);

const wss = new WebSocketServer({ server, path: "/ws/shell" });
wss.on("connection", (ws) => {
  const shell = process.env.SHELL || "/bin/zsh";
  const child = spawn(shell, ["-l"], {
    cwd: os.homedir(),
    env: { ...process.env, TERM: "xterm-256color", COLORTERM: "truecolor" },
  });
  ws.send(`\r\n\x1b[32m# Alice Panel · Станция 1 (local shell)\x1b[0m ${shell}\r\n`);
  child.stdout.on("data", (d) => ws.readyState === 1 && ws.send(d.toString("utf8")));
  child.stderr.on("data", (d) => ws.readyState === 1 && ws.send(d.toString("utf8")));
  child.on("close", () => {
    try {
      ws.close();
    } catch {}
  });
  ws.on("message", (msg) => {
    const s = msg.toString();
    try {
      const j = JSON.parse(s);
      if (j.type === "input") child.stdin.write(j.data);
      return;
    } catch {}
    child.stdin.write(s);
  });
  ws.on("close", () => child.kill());
});

server.listen(PORT, () => {
  console.log(`Alice Panel · Станция 1  http://127.0.0.1:${PORT}`);
  console.log(`target=yandex-station-gen1 (tulip-d1 / yandexstation)`);
  console.log(`repo=${REPO}`);
  console.log(`profile=${PROFILE}`);
  console.log(`bindir=${BINDIR}`);
  console.log(`shell ws=/ws/shell`);
});
