import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";

async function api(path, opts) {
  const res = await fetch(path, opts);
  const text = await res.text();
  let data;
  try {
    data = JSON.parse(text);
  } catch {
    data = { raw: text };
  }
  if (!res.ok) throw new Error(data.error || data.raw || res.statusText);
  return data;
}

function fmtBytesKb(kb) {
  if (kb == null) return "—";
  if (kb > 1024 * 1024) return `${(kb / 1024 / 1024).toFixed(1)} ГБ`;
  if (kb > 1024) return `${(kb / 1024).toFixed(0)} МБ`;
  return `${kb} КБ`;
}

function fmtUptime(sec) {
  if (!sec && sec !== 0) return "—";
  const s = Math.floor(sec);
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  if (d) return `${d}д ${h}ч ${m}м`;
  if (h) return `${h}ч ${m}м`;
  return `${m}м`;
}

function Overview({ system, onRefresh }) {
  const st = system?.station;
  const live = system?.live;
  if (!system) return <p className="muted">Загрузка…</p>;
  if (!st) return <p className="muted">Нет данных станции…</p>;

  const memPct = live?.memory?.used_pct ?? st.memory?.used_pct ?? 0;
  const load1 = live?.loadavg?.["1"] ?? 0;
  const storage = live?.storage;
  const romPct = storage?.data_used_pct ?? 0;

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Обзор</h1>
          <p>
            Панель для <strong>Яндекс Станции 1 поколения</strong> ({st.device}) · Android{" "}
            {st.android} · {st.soc}
          </p>
        </div>
        <button type="button" className="btn ghost" onClick={onRefresh}>
          Обновить
        </button>
      </div>

      <div className="grid stats">
        <div className="card">
          <h3>Нагрузка (хост API)</h3>
          <div className="stat-value">
            {Number(load1).toFixed(2)}
            <small>loadavg 1 / 5 / 15: {live?.loadavg?.["1"]?.toFixed?.(2)} · {live?.loadavg?.["5"]?.toFixed?.(2)} · {live?.loadavg?.["15"]?.toFixed?.(2)}</small>
          </div>
        </div>
        <div className="card">
          <h3>RAM хоста</h3>
          <div className="stat-value">
            {memPct}%
            <small>
              свободно {fmtBytesKb(live?.memory?.available_kb)} / {fmtBytesKb(live?.memory?.total_kb)}
            </small>
          </div>
          <div className="bar">
            <i style={{ width: `${Math.min(100, memPct)}%` }} />
          </div>
        </div>
        <div className="card">
          <h3>ПЗУ (/data)</h3>
          <div className="stat-value">
            {romPct ? `${romPct}%` : "—"}
            <small>
              свободно {fmtBytesKb(storage?.data_available_kb)} / {fmtBytesKb(storage?.data_total_kb)}
              {storage?.emmc_total_human ? ` · eMMC ${storage.emmc_total_human}` : ""}
            </small>
          </div>
          <div className="bar">
            <i style={{ width: `${Math.min(100, romPct || 0)}%` }} />
          </div>
        </div>
        <div className="card">
          <h3>Uptime хоста</h3>
          <div className="stat-value">
            {fmtUptime(live?.uptime_sec)}
            <small>{live?.host} · {live?.arch}</small>
          </div>
        </div>
      </div>

      <div className="grid two" style={{ marginTop: "0.9rem" }}>
        <div className="card">
          <h2>Прошивка / Quasar</h2>
          <div className="kv">
            <div className="kv-row">
              <span>Quasar software</span>
              <span>{st.quasar?.software_version}</span>
            </div>
            <div className="kv-row">
              <span>Vendor VERSION</span>
              <span>{st.quasar?.vendor_version}</span>
            </div>
            <div className="kv-row">
              <span>Revision</span>
              <span>{st.quasar?.software_revision?.slice?.(0, 12)}…</span>
            </div>
            <div className="kv-row">
              <span>APK</span>
              <span>{st.quasar?.apk_package}</span>
            </div>
            <div className="kv-row">
              <span>APK path</span>
              <span>{st.quasar?.apk_path}</span>
            </div>
            <div className="kv-row">
              <span>Roles</span>
              <span>{(st.quasar?.roles || []).join(" ")}</span>
            </div>
            <div className="kv-row">
              <span>Spotter</span>
              <span>{st.quasar?.spotter}</span>
            </div>
            <div className="kv-row">
              <span>Build / display</span>
              <span>{st.display_id}</span>
            </div>
          </div>
        </div>

        <div className="card">
          <h2>Железо</h2>
          <div className="kv">
            <div className="kv-row">
              <span>SoC</span>
              <span>{st.soc}</span>
            </div>
            <div className="kv-row">
              <span>CPU</span>
              <span>
                {st.cpu?.cores}× {st.cpu?.name} ({st.cpu?.arch})
              </span>
            </div>
            <div className="kv-row">
              <span>Freq</span>
              <span>
                {(st.cpu?.freqs_khz || []).map((f) => `${f / 1000}`).join("/")} МГц
              </span>
            </div>
            <div className="kv-row">
              <span>Усилители</span>
              <span>{st.amplifiers}</span>
            </div>
            <div className="kv-row">
              <span>Sound</span>
              <span>{(st.sound_cards || []).join(", ")}</span>
            </div>
            <div className="kv-row">
              <span>HDMI</span>
              <span>{st.hdmi}</span>
            </div>
            <div className="kv-row">
              <span>Thermal SoC</span>
              <span>{st.thermal?.soc_thermal_c}°C</span>
            </div>
            <div className="kv-row">
              <span>Kernel</span>
              <span>3.10.65 · SELinux {st.selinux}</span>
            </div>
          </div>
        </div>
      </div>

      <div className="grid two" style={{ marginTop: "0.9rem" }}>
        <div className="card">
          <h2>Сеть / доступ</h2>
          <div className="kv">
            <div className="kv-row">
              <span>Glagol</span>
              <span>
                {st.glagol?.websocket_active ? "active" : "—"} · {st.glagol?.local_ip}:{st.glagol?.port}
              </span>
            </div>
            <div className="kv-row">
              <span>SSH</span>
              <span>
                :{st.ssh?.port} ({st.ssh?.banner})
              </span>
            </div>
            <div className="kv-row">
              <span>ADB</span>
              <span>:{st.adb?.port}</span>
            </div>
            <div className="kv-row">
              <span>Locale / TZ</span>
              <span>
                {st.locale} · {st.timezone}
              </span>
            </div>
          </div>
        </div>

        <div className="card">
          <h2>Бинарники на колонке</h2>
          <div className="kv">
            {Object.entries(st.bins || {}).map(([k, v]) => (
              <div className="kv-row" key={k}>
                <span>{k}</span>
                <span>{v}</span>
              </div>
            ))}
            <div className="kv-row">
              <span>Platform</span>
              <span>{st.platform}</span>
            </div>
            <div className="kv-row">
              <span>ABI</span>
              <span>{st.abi}</span>
            </div>
          </div>
        </div>
      </div>
    </>
  );
}

function Packages() {
  const [repo, setRepo] = useState(null);
  const [installed, setInstalled] = useState({});
  const [bindir, setBindir] = useState("");
  const [apkgPath, setApkgPath] = useState("");
  const [flavor, setFlavor] = useState("");
  const [busy, setBusy] = useState(null);
  const [error, setError] = useState("");
  const [log, setLog] = useState("");

  const refresh = useCallback(async () => {
    const data = await api("/api/packages");
    setRepo(data.repo);
    setInstalled(data.installed || {});
    setBindir(data.bindir || "");
    setApkgPath(data.apkg || "");
    setFlavor(data.flavor || "");
  }, []);

  useEffect(() => {
    refresh().catch((e) => setError(String(e.message || e)));
  }, [refresh]);

  async function install(name) {
    setBusy(name);
    setError("");
    setLog("");
    try {
      const data = await api(`/api/install/${encodeURIComponent(name)}`, { method: "POST" });
      setLog(`apkg install ${data.name}\n${data.log || ""}`.trim());
      await refresh();
    } catch (e) {
      setError(String(e.message || e));
    } finally {
      setBusy(null);
    }
  }

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Пакеты</h1>
          <p>
            Репозиторий <span className="mono">apkg</span>
            {flavor ? ` · ${flavor}` : ""}
            {bindir ? ` · ${bindir}` : ""}
            {!apkgPath ? " · apkg скачается при установке" : ""}
          </p>
        </div>
        <button type="button" className="btn ghost" onClick={() => refresh().catch((e) => setError(String(e.message || e)))}>
          Обновить
        </button>
      </div>
      {error && <div className="banner err">{error}</div>}
      {log && <div className="banner ok" style={{ whiteSpace: "pre-wrap" }}>{log}</div>}
      <div className="pkg-list">
        {repo?.packages?.map((p) => {
          const on = !!installed[p.name];
          const mb = p.size ? (p.size / 1024 / 1024).toFixed(1) : "?";
          return (
            <article key={p.name} className="pkg">
              <div>
                <h2>{p.name}</h2>
                <p>
                  {p.description}
                  <br />
                  <span className="muted mono">
                    v{p.version}
                    {p.category ? ` · ${p.category}` : ""} · {mb} МБ
                    {on ? " · установлено" : ""}
                  </span>
                </p>
              </div>
              <button type="button" className="btn" disabled={!!busy} onClick={() => install(p.name)}>
                {busy === p.name ? "apkg…" : on ? "Переустановить" : "Установить"}
              </button>
            </article>
          );
        })}
      </div>
    </>
  );
}

function Shell() {
  const ref = useRef(null);
  const termRef = useRef(null);

  useEffect(() => {
    if (!ref.current || termRef.current) return;
    const term = new Terminal({
      cursorBlink: true,
      fontFamily: '"IBM Plex Mono", ui-monospace, monospace',
      fontSize: 13,
      theme: {
        background: "#070b09",
        foreground: "#d7ebe0",
        cursor: "#5dcea0",
        selectionBackground: "#2a4a38",
      },
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(ref.current);
    fit.fit();
    termRef.current = term;

    const proto = location.protocol === "https:" ? "wss" : "ws";
    // на станции панель и API с одного origin; в vite-dev — напрямую :8080
    const host = import.meta.env.DEV ? "127.0.0.1:8080" : location.host;
    const ws = new WebSocket(`${proto}://${host}/ws/shell`);
    ws.binaryType = "arraybuffer";
    ws.onopen = () => {
      fit.fit();
      ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
    };
    ws.onmessage = (ev) => {
      term.write(typeof ev.data === "string" ? ev.data : new TextDecoder().decode(ev.data));
    };
    ws.onerror = () => term.writeln("\r\n\x1b[31m[ws error]\x1b[0m");
    ws.onclose = () => term.writeln("\r\n\x1b[31m[disconnected]\x1b[0m");
    term.onData((data) => {
      if (ws.readyState === 1) ws.send(JSON.stringify({ type: "input", data }));
    });
    const onResize = () => {
      fit.fit();
      if (ws.readyState === 1) {
        ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
      }
    };
    window.addEventListener("resize", onResize);
    return () => {
      window.removeEventListener("resize", onResize);
      ws.close();
      term.dispose();
      termRef.current = null;
    };
  }, []);

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Терминал</h1>
          <p>Веб-шелл на колонке</p>
        </div>
      </div>
      <div className="term-wrap">
        <div className="term-bar">
          <span className="term-host">ws://…/ws/shell</span>
          <span>xterm + zsh/bash хоста</span>
        </div>
        <div id="terminal" ref={ref} className="xterm-root" />
      </div>
    </>
  );
}

function Hardware({ system }) {
  const st = system?.station;
  const storage = system?.live?.storage;
  if (!st) return null;
  return (
    <>
      <div className="page-head">
        <div>
          <h1>Железо</h1>
          <p>Станция 1 (Allwinner R18 / tulip-d1) — снимок YSE с твоей колонки</p>
        </div>
      </div>
      <div className="grid two">
        <div className="card">
          <h2>ПЗУ (eMMC)</h2>
          <div className="kv">
            <div className="kv-row">
              <span>Устройство</span>
              <span>{storage?.device || "mmcblk0"}</span>
            </div>
            <div className="kv-row">
              <span>Ёмкость</span>
              <span>{storage?.emmc_total_human || fmtBytesKb(storage?.emmc_total_kb) || "—"}</span>
            </div>
          </div>
          {(storage?.mounts || []).length > 0 ? (
            <table className="proc-table" style={{ marginTop: "0.75rem" }}>
              <thead>
                <tr>
                  <th>раздел</th>
                  <th>занято</th>
                  <th>свободно</th>
                  <th>всего</th>
                </tr>
              </thead>
              <tbody>
                {storage.mounts.map((m) => (
                  <tr key={m.path}>
                    <td className="name">{m.path}</td>
                    <td>{m.used_pct}%</td>
                    <td>{m.avail_human || fmtBytesKb(m.available_kb)}</td>
                    <td>{m.total_human || fmtBytesKb(m.total_kb)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <p className="muted" style={{ marginTop: "0.75rem" }}>
              Нет живых данных о монтировании
            </p>
          )}
        </div>
        <div className="card">
          <h2>I2C</h2>
          <table className="proc-table">
            <thead>
              <tr>
                <th>addr</th>
                <th>name</th>
              </tr>
            </thead>
            <tbody>
              {(st.i2c || []).map((x) => (
                <tr key={x.addr}>
                  <td>{x.addr}</td>
                  <td className="name">{x.name}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="card">
          <h2>Модули ядра</h2>
          <p className="mono" style={{ margin: 0, lineHeight: 1.6, color: "var(--ink-dim)" }}>
            {(st.modules || []).join(" · ")}
          </p>
        </div>
        <div className="card">
          <h2>Имена разделов</h2>
          <p className="mono" style={{ margin: 0, lineHeight: 1.6, color: "var(--ink-dim)" }}>
            {(st.storage_partitions || []).join(" · ")}
          </p>
        </div>
        <div className="card">
          <h2>Inputs</h2>
          <p style={{ margin: 0, lineHeight: 1.6, color: "var(--ink-dim)" }}>
            {(st.inputs || []).join(" · ")}
          </p>
        </div>
      </div>
    </>
  );
}

function Display() {
  const [info, setInfo] = useState(null);
  const [shot, setShot] = useState(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const refreshInfo = useCallback(async () => {
    const data = await api("/api/display");
    setInfo(data);
  }, []);

  useEffect(() => {
    refreshInfo().catch((e) => setError(String(e.message || e)));
  }, [refreshInfo]);

  async function capture() {
    setBusy(true);
    setError("");
    try {
      const res = await fetch(`/api/display/screenshot?t=${Date.now()}`);
      if (!res.ok) throw new Error(await res.text());
      const blob = await res.blob();
      if (shot) URL.revokeObjectURL(shot);
      setShot(URL.createObjectURL(blob));
      await refreshInfo();
    } catch (e) {
      setError(String(e.message || e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Дисплей</h1>
          <p>HDMI Станции 1 — снимки экрана</p>
        </div>
        <div className="btn-row">
          <button type="button" className="btn ghost" onClick={() => refreshInfo().catch((e) => setError(String(e.message || e)))}>
            Обновить info
          </button>
          <button type="button" className="btn" disabled={busy} onClick={capture}>
            {busy ? "Снимаю…" : "Снимок экрана"}
          </button>
        </div>
      </div>
      {error && <div className="banner err">{error}</div>}
      <div className="grid two" style={{ marginBottom: "0.9rem" }}>
        <div className="card">
          <h2>HDMI</h2>
          <div className="kv">
            <div className="kv-row">
              <span>Разрешение</span>
              <span className="mono">{info?.hdmi || "—"}</span>
            </div>
            <div className="kv-row">
              <span>Состояние</span>
              <span>{info?.state || "—"}</span>
            </div>
            <div className="kv-row">
              <span>FPS</span>
              <span>{info?.fps ?? "—"}</span>
            </div>
            <div className="kv-row">
              <span>Яркость</span>
              <span>{info?.brightness ?? "—"}</span>
            </div>
            <div className="kv-row">
              <span>screencap</span>
              <span>{info?.screencap ? "да" : "нет"}</span>
            </div>
          </div>
        </div>
        <div className="card">
          <h2>Как это работает</h2>
          <p className="muted" style={{ margin: 0, lineHeight: 1.55 }}>
            Снимок берётся через <span className="mono">/system/bin/screencap</span> — framebuffer HDMI
            1920×1080. Управление питанием экрана — позже.
          </p>
        </div>
      </div>
      {shot && (
        <div className="shot-wrap">
          <img src={shot} alt="Снимок HDMI" className="shot-img" />
          <a className="btn ghost" href={shot} download="station-screen.png">
            Скачать PNG
          </a>
        </div>
      )}
    </>
  );
}

function UpdateBanner({ update, onUpdated }) {
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState("");

  if (!update?.update) return null;

  async function doUpdate() {
    setBusy(true);
    setMsg("Ставлю обновление… панель перезапустится");
    try {
      await api("/api/update", { method: "POST" });
      // ждём новый health
      const want = update.latest;
      for (let i = 0; i < 40; i++) {
        await new Promise((r) => setTimeout(r, 2000));
        try {
          const h = await api("/api/health");
          if (h.version && h.version !== update.current) {
            setMsg(`Готово · v${h.version}`);
            onUpdated?.();
            setTimeout(() => window.location.reload(), 800);
            return;
          }
          if (want && h.version === want) {
            setMsg(`Готово · v${h.version}`);
            onUpdated?.();
            setTimeout(() => window.location.reload(), 800);
            return;
          }
        } catch {
          /* ещё поднимается */
        }
      }
      setMsg("Таймаут ожидания — обнови страницу вручную");
    } catch (e) {
      setMsg(String(e.message || e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="banner update">
      <div>
        <strong>Доступно обновление</strong>
        <span className="muted">
          {" "}
          v{update.current} → v{update.latest}
        </span>
        {msg && (
          <>
            <br />
            <span className="mono">{msg}</span>
          </>
        )}
      </div>
      <button type="button" className="btn" disabled={busy} onClick={doUpdate}>
        {busy ? "Обновляю…" : "Обновить"}
      </button>
    </div>
  );
}

export default function App() {
  const [tab, setTab] = useState("overview");
  const [system, setSystem] = useState(null);
  const [error, setError] = useState("");
  const [update, setUpdate] = useState(null);
  const [version, setVersion] = useState("");

  const refresh = useCallback(async () => {
    setError("");
    const data = await api("/api/system");
    setSystem(data);
  }, []);

  const checkUpdates = useCallback(async () => {
    try {
      const h = await api("/api/health");
      if (h.version) setVersion(h.version);
      const u = await api("/api/update/check");
      setUpdate(u);
    } catch {
      /* офлайн / нет сети до GitHub */
    }
  }, []);

  useEffect(() => {
    refresh().catch((e) => setError(String(e.message || e)));
    checkUpdates();
  }, [refresh, checkUpdates]);

  const title = useMemo(
    () =>
      ({
        overview: "Обзор",
        packages: "Пакеты",
        shell: "Терминал",
        hw: "Железо",
        display: "Дисплей",
      })[tab],
    [tab]
  );

  return (
    <div className="app">
      <aside className="nav">
        <div className="brand">
          <div className="brand-mark">
            Alice <span>Panel</span>
          </div>
          <div className="brand-sub">Станция 1 · tulip-d1{version ? ` · v${version}` : ""}</div>
        </div>
        <nav className="nav-links">
          <button type="button" className={tab === "overview" ? "active" : ""} onClick={() => setTab("overview")}>
            Обзор
          </button>
          <button type="button" className={tab === "hw" ? "active" : ""} onClick={() => setTab("hw")}>
            Железо
          </button>
          <button type="button" className={tab === "display" ? "active" : ""} onClick={() => setTab("display")}>
            Дисплей
          </button>
          <button type="button" className={tab === "packages" ? "active" : ""} onClick={() => setTab("packages")}>
            Пакеты
          </button>
          <button type="button" className={tab === "shell" ? "active" : ""} onClick={() => setTab("shell")}>
            Терминал
          </button>
        </nav>
        <div className="nav-foot">
          {system?.station?.quasar?.software_version || "…"}
          <br />
          {title}
        </div>
      </aside>
      <main className="main">
        <UpdateBanner update={update} onUpdated={checkUpdates} />
        {error && <div className="banner err">{error}</div>}
        {tab === "overview" && <Overview system={system} onRefresh={() => refresh().catch((e) => setError(String(e.message || e)))} />}
        {tab === "hw" && <Hardware system={system} />}
        {tab === "display" && <Display />}
        {tab === "packages" && <Packages />}
        {tab === "shell" && <Shell />}
      </main>
    </div>
  );
}
