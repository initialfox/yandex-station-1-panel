# Alice Panel — Яндекс Станция 1

Панель для **Станции 1 поколения** (`tulip-d1` / `yandexstation` / aarch64).

Обзор · железо · пакеты · веб-терминал.

## Установка на колонку

Нужен root (SSH/ADB):

```sh
curl -fsSL https://raw.githubusercontent.com/initialfox/yandex-station-1-panel/main/install.sh | sh
```

или:

```sh
wget -qO- https://raw.githubusercontent.com/initialfox/yandex-station-1-panel/main/install.sh | sh
```

Открыть в браузере (та же Wi‑Fi сеть):

```text
http://<IP-колонки>:8080
```

Файлы лежат в `/data/local/alice-panel/`. Лог: `httpd.log`. Перезапуск: `sh /data/local/alice-panel/start.sh`.

## Локальная разработка

```bash
cd server && npm i && npm run dev   # :8080
cd web && npm i && npm run dev      # :5173
```

Сборка релиза для колонки:

```bash
cd httpd && ./build.sh
cd ../web && npm run build
# см. scripts/pack-release.sh
```

## Структура

```
httpd/     Go alice-httpd (бинарь на станцию)
web/       React UI
server/    Express (только локальный тест)
repo/      packages.json + бинарники + station-profile
install.sh установщик на колонку
```
