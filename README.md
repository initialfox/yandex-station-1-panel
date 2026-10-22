# Alice Panel — Яндекс Станция 1

Веб-панель для **Яндекс Станции 1 поколения** (`tulip-d1` / `yandexstation` / aarch64).

Один бинарь `alice-httpd` на колонке (`:8080`), без Node. UI в браузере в той же Wi‑Fi сети:

```text
http://<IP-колонки>:8080
```

---

## Возможности

### Обзор
- Модель и железо: Allwinner R18, Android 6.0.1, HDMI 1920×1080
- Прошивка Quasar: software / vendor / revision, APK, roles, spotter
- Живые метрики хоста: loadavg, RAM, uptime
- Сеть и доступ: Glagol, SSH (dropbear), ADB

### Железо
- SoC, CPU (Cortex-A53 ×4), частоты
- I2C (усилители tas5751m и др.)
- Модули ядра, разделы накопителя, inputs
- Thermal / SELinux / locale

### Пакеты
- Установка aarch64-бинарников на колонку в `/data/local/bin`
- Сейчас в репо: **zsh**, **fastfetch**
- Кнопка «Установить / Переустановить» из UI

### Терминал
- Веб-шелл прямо в браузере (xterm + PTY на колонке)
- По умолчанию zsh из `/data/local/bin`, иначе `/system/bin/sh`

### Дисплей
- Управление HDMI-выходом
- Сейчас: **снимки экрана** (framebuffer) — в работе

Файлы на устройстве: `/data/local/alice-panel/`  
Лог: `httpd.log` · рестарт: `sh /data/local/alice-panel/start.sh`

---

## Установка

Одна команда **на колонке**. `-k` обязателен: у системного curl нет CA-сертификатов.

### SSH (уже root)

```sh
curl -kfsSL https://raw.githubusercontent.com/initialfox/yandex-station-1-panel/main/install.sh | sh
```

### ADB

```bash
adb connect 192.168.1.116:5555
adb shell "su 0 sh -c 'curl -kfsSL https://raw.githubusercontent.com/initialfox/yandex-station-1-panel/main/install.sh | sh'"
```

`su 0` нужен: обычный `adb shell` на Станции 1 — не root.

Скрипт скачает релиз `alice-panel-station1.tar`, распакует в `/data/local/alice-panel` и запустит панель.

---

## Разработка

```bash
cd httpd && ./build.sh
cd ../web && npm i && npm run build
bash scripts/pack-release.sh
```

Локальный тест UI:

```bash
cd server && npm i && npm run dev   # :8080
cd web && npm i && npm run dev      # :5173
```

### Структура

```
httpd/       Go alice-httpd (бинарь на станцию)
web/         React UI
server/      Express (только локальный тест)
repo/        packages.json + бинарники + station-profile
install.sh   установщик на колонке (curl|sh)
```
