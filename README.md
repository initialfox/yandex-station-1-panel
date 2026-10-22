# Alice Panel — Яндекс Станция 1

Панель для **Станции 1** (`tulip-d1` / aarch64): обзор, железо, пакеты, терминал, дисплей.

После установки открыть: `http://<IP-колонки>:8080`

---

## Установка через ADB (с нуля)

Нужны: `adb` на компе, колонка в той же Wi‑Fi, root ADB.

**1.** Скачай релиз:

- https://github.com/initialfox/yandex-station-1-panel/releases/latest  
- файл `alice-panel-station1.tar`

**2.** Подключись:

```bash
adb connect <IP-колонки>:5555
adb devices
```

**3.** Залей и запусти:

```bash
adb push alice-panel-station1.tar /sdcard/alice-panel-station1.tar
adb shell
```

В шелле колонки:

```sh
pkill alice-httpd 2>/dev/null
rm -rf /data/local/alice-panel
mkdir -p /data/local/alice-panel /data/local/bin
cd /data/local/alice-panel
tar -xf /sdcard/alice-panel-station1.tar
chmod 755 alice-httpd
cat > start.sh <<'EOF'
#!/system/bin/sh
cd /data/local/alice-panel || exit 1
pkill alice-httpd 2>/dev/null
sleep 1
setsid ./alice-httpd -listen 0.0.0.0:8080 -root . -repo ./repo -bindir /data/local/bin </dev/null >httpd.log 2>&1 &
echo $!
EOF
chmod 755 start.sh
sh start.sh
```

Готово. Рестарт позже: `sh /data/local/alice-panel/start.sh`

> `adb push` сразу в `/data/local` на Станции 1 часто падает с Permission denied — поэтому через `/sdcard`.

---

## Установка через SSH (с нуля)

Нужны: SSH-клиент, root на колонке (dropbear, порт **9865**).

На станции **нет нормальных CA** у curl — без `-k` HTTPS не качается. SFTP тоже нет.

### Вариант A — одной командой на колонке

Зайди по SSH и выполни:

```sh
curl -kfsSL https://raw.githubusercontent.com/initialfox/yandex-station-1-panel/main/install.sh | sh
```

Если нет curl:

```sh
wget --no-check-certificate -qO- https://raw.githubusercontent.com/initialfox/yandex-station-1-panel/main/install.sh | sh
```

### Вариант B — залить tar с компа

**1.** Скачай `alice-panel-station1.tar` с [releases](https://github.com/initialfox/yandex-station-1-panel/releases/latest).

**2.** С компа (Mac/Linux/Windows + OpenSSH):

```bash
ssh -p 9865 root@<IP-колонки>
```

В другом терминале отправь архив:

```bash
cat alice-panel-station1.tar | ssh -p 9865 root@<IP-колонки> "cat > /data/local/tmp/alice-panel.tar"
```

**3.** На колонке по SSH:

```sh
pkill alice-httpd 2>/dev/null
rm -rf /data/local/alice-panel
mkdir -p /data/local/alice-panel /data/local/bin
cd /data/local/alice-panel
tar -xf /data/local/tmp/alice-panel.tar
chmod 755 alice-httpd
cat > start.sh <<'EOF'
#!/system/bin/sh
cd /data/local/alice-panel || exit 1
pkill alice-httpd 2>/dev/null
sleep 1
setsid ./alice-httpd -listen 0.0.0.0:8080 -root . -repo ./repo -bindir /data/local/bin </dev/null >httpd.log 2>&1 &
echo $!
EOF
chmod 755 start.sh
sh start.sh
```

---

## Возможности

| Раздел | Что делает |
|--------|------------|
| Обзор | Quasar, RAM, load, uptime, SSH/ADB/Glagol |
| Железо | SoC, I2C, модули, разделы, HDMI |
| Пакеты | Ставит `zsh` / `fastfetch` в `/data/local/bin` |
| Терминал | Веб-шелл (PTY) |
| Дисплей | Снимки HDMI (в работе) |

Файлы на колонке: `/data/local/alice-panel/` · лог: `httpd.log`

---

## Разработка (не для установки на колонку)

```bash
cd httpd && ./build.sh
cd ../web && npm i && npm run build
bash scripts/pack-release.sh
```

Локальный UI: `cd server && npm i && npm run dev` + `cd web && npm run dev`
