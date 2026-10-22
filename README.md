# Alice Panel — Яндекс Станция 1

Панель для **Станции 1** (`tulip-d1` / aarch64).

После установки: `http://<IP-колонки>:8080`

---

## Установка

Одна команда на колонке. `-k` — у curl нет CA.

### SSH (уже root)

```sh
curl -kfsSL https://raw.githubusercontent.com/initialfox/yandex-station-1-panel/main/install.sh | sh
```

### ADB

```bash
adb connect 192.168.1.116:5555
adb shell "su 0 sh -c 'curl -kfsSL https://raw.githubusercontent.com/initialfox/yandex-station-1-panel/main/install.sh | sh'"
```

(`su 0` — adb по умолчанию не root на Станции 1.)

Рестарт: `sh /data/local/alice-panel/start.sh`

---

## Возможности

Обзор · железо · пакеты (`zsh`/`fastfetch`) · веб-терминал · дисплей (снимки — в работе)
