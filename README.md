# Alice Panel — Яндекс Станция 1

Панель для **Станции 1** (`tulip-d1` / aarch64).

После установки: `http://<IP-колонки>:8080`

---

## Установка

Одна и та же команда — крутится **на колонке**. `-k` нужен: у curl нет CA.

### SSH

Зашёл на колонку и:

```sh
curl -kfsSL https://raw.githubusercontent.com/initialfox/yandex-station-1-panel/main/install.sh | sh
```

### ADB

С компа:

```bash
adb connect 192.168.1.116:5555
adb shell "curl -kfsSL https://raw.githubusercontent.com/initialfox/yandex-station-1-panel/main/install.sh | sh"
```

Всё. Скрипт сам скачает релиз, распакует в `/data/local/alice-panel` и запустит.

Рестарт: `sh /data/local/alice-panel/start.sh`

---

## Возможности

Обзор · железо · пакеты (`zsh`/`fastfetch`) · веб-терминал · дисплей (снимки — в работе)
