# Alice Panel — Яндекс Станция 1

Панель для **Станции 1 поколения** (`tulip-d1` / `yandexstation` / aarch64).

## Возможности

- **Обзор** — прошивка Quasar, нагрузка, RAM, uptime, сеть (SSH/ADB/Glagol)
- **Железо** — SoC Allwinner R18, I2C, модули ядра, разделы, HDMI
- **Пакеты** — установка aarch64-бинарников на колонку (`zsh`, `fastfetch`, …) в `/data/local/bin`
- **Терминал** — веб-шелл (PTY) прямо в браузере
- **Дисплей** — снимки HDMI-экрана (в работе)

API на колонке — один бинарь `alice-httpd` (`:8080`), без Node.

---

## Установка

Нужен **root**. Станция без CA-бандла у `curl` → для HTTPS нужен `-k`.  
Прямой `adb push` в `/data/local` часто даёт Permission denied — скрипт кладёт tar в `/sdcard`, дальше root shell.

### 1) С колонки (curl / wget → GitHub)

```sh
curl -kfsSL https://raw.githubusercontent.com/initialfox/yandex-station-1-panel/main/install.sh | sh
```

```sh
wget --no-check-certificate -qO- https://raw.githubusercontent.com/initialfox/yandex-station-1-panel/main/install.sh | sh
```

Фиксированный тег: `PANEL_TAG=v0.1.0` перед `| sh`.

### 2) С Mac — ADB push

Колонка в той же сети, ADB `:5555`:

```bash
adb connect 192.168.1.116:5555
bash scripts/pack-release.sh   # если ещё нет alice-panel-station1.tar
./scripts/install-adb.sh
# или: ADB_SERIAL=192.168.1.116:5555 ./scripts/install-adb.sh
```

### 3) С Mac — SSH (без SFTP)

На станции нет SFTP — tar уходит потоком через `ssh cat`. Порт dropbear обычно `9865`:

```bash
bash scripts/pack-release.sh
SSH_HOST=192.168.1.116 SSH_PORT=9865 SSH_PASS='…' ./scripts/install-ssh.sh
# нужен: brew install sshpass   (или ssh-ключ)
```

Через публичный IP то же самое: `SSH_HOST=91.x.x.x`.

### 4) Вручную одной командой (SSH)

```bash
cat alice-panel-station1.tar | ssh -p 9865 root@192.168.1.116 \
  'cat > /data/local/tmp/p.tar && cd /data/local && rm -rf alice-panel && mkdir alice-panel && cd alice-panel && tar -xf /data/local/tmp/p.tar && sh start.sh || (chmod 755 alice-httpd && ./alice-httpd -listen 0.0.0.0:8080 -root . -repo ./repo -bindir /data/local/bin &)'
```

После установки:

```text
http://<IP-колонки>:8080
```

Файлы: `/data/local/alice-panel/`. Лог: `httpd.log`. Рестарт: `sh /data/local/alice-panel/start.sh`.

---

## Локальная разработка

```bash
cd server && npm i && npm run dev   # :8080
cd web && npm i && npm run dev      # :5173
```

Сборка релиза:

```bash
cd httpd && ./build.sh
cd ../web && npm run build
bash scripts/pack-release.sh
```

## Структура

```
httpd/              Go alice-httpd (бинарь на станцию)
web/                React UI
server/             Express (только локальный тест)
repo/               packages.json + бинарники + station-profile
install.sh          установщик на колонке (curl|sh)
scripts/install-adb.sh   ADB push с Mac
scripts/install-ssh.sh   SSH stream с Mac
scripts/pack-release.sh  сборка alice-panel-station1.tar
```
