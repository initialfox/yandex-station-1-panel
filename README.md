# Alice Panel — Яндекс Станция 1

Панель для **Станции 1** (`tulip-d1` / aarch64): обзор · железо · пакеты · терминал · дисплей.

После установки: `http://<IP-колонки>:8080`

---

## С компа — одна команда по ссылке

Скрипт сам качает релиз и ставит панель.

### ADB

```bash
curl -fsSL https://raw.githubusercontent.com/initialfox/yandex-station-1-panel/main/install-remote.sh | bash -s -- adb 192.168.1.116
```

Нужен `adb`. Подставь LAN IP колонки.

### SSH

```bash
curl -fsSL https://raw.githubusercontent.com/initialfox/yandex-station-1-panel/main/install-remote.sh | SSH_PASS='пароль' bash -s -- ssh
```

По умолчанию `root@192.168.1.116:9865`. Иначе:

```bash
curl -fsSL https://raw.githubusercontent.com/initialfox/yandex-station-1-panel/main/install-remote.sh \
  | SSH_HOST=91.x.x.x SSH_PORT=9865 SSH_PASS='пароль' bash -s -- ssh
```

Нужен `sshpass` (`brew install sshpass`) или ssh-ключ (тогда без `SSH_PASS`).

---

## Прямо на колонке

```sh
curl -kfsSL https://raw.githubusercontent.com/initialfox/yandex-station-1-panel/main/install.sh | sh
```

---

## Возможности

Обзор · железо · пакеты (`zsh`/`fastfetch`) · веб-терминал · дисплей (снимки — в работе)

Файлы: `/data/local/alice-panel/` · рестарт: `sh /data/local/alice-panel/start.sh`
