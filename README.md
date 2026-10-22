# Alice Panel — Яндекс Станция 1

Веб-панель для **Яндекс Станции 1 поколения** (`tulip-d1` / `yandexstation` / aarch64).

Один бинарь `alice-httpd` на колонке, без Node.

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
- **zsh** ставится сразу при установке панели
- В UI также: **fastfetch** и переустановка пакетов

### Терминал
- Веб-шелл прямо в браузере (xterm + PTY на колонке)
- По умолчанию zsh из `/data/local/bin`, иначе `/system/bin/sh`

### Дисплей
- Управление HDMI-выходом
- Сейчас: **снимки экрана** (framebuffer) — в работе

Файлы на устройстве: `/data/local/alice-panel/`  
Лог: `httpd.log` · рестарт: `sh /data/local/alice-panel/start.sh`

---

## Root

Нужен **root** на колонке. Как получить на Станции 1:

https://t.me/c/4457409211/1710/6152

---

## Установка (SSH)

Одна команда **на колонке под root**. `-k` обязателен: у системного curl нет CA.

```sh
curl -kfsSL https://cdn.jsdelivr.net/gh/initialfox/yandex-station-1-panel@main/install.sh | sh
```

Скрипт скачает релиз, распакует в `/data/local/alice-panel`, поставит **zsh** в `/data/local/bin` и запустит панель.

> Через ADB нормально не ставится: `adb su` на Станции 1 без полных capabilities.

### После установки

В конце скрипт сам напишет адрес, например:

```text
==> панель запущена
    открой в браузере (та же Wi‑Fi, что и колонка):
    http://192.168.1.116:8080
```

Открой этот URL в браузере: **IP колонки + порт 8080** (та же Wi‑Fi).

Если IP в выводе не появился — посмотри IP в роутере / `ip addr` и зайди на `http://<IP>:8080`.

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
