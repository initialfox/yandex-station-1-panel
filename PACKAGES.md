# Пакеты Alice Panel (Станция 1 / aarch64)

Актуальный каталог для установщика панели. Источник правды: [`repo/packages.json`](repo/packages.json).

| Пакет | Версия | Описание |
|-------|--------|----------|
| **zsh** | 5.9.2 | Z shell (static musl), ставится вместе с панелью |
| **fastfetch** | 2.70.0 | Системная информация в терминале |
| **dropbear** | 2022.82 | SSH-сервер → `/data/dropbear` |
| **curl** | 8.14.1 | HTTP-клиент (static musl) |
| **miniserve** | 0.29.0 | Раздача файлов по HTTP |
| **btop** | 1.4.7 | Мониторинг процессов |
| **ffmpeg** | 7.0.2 | Медиаконвертер (static glibc — на Android может не стартовать) |
| **nano** | 8.0 | Редактор (Alpine musl → `/data/local/opt/nano`) |
| **iperf3** | 3.17.1 | Измерение пропускной способности |
| **tcpdump** | 4.99.4 | Сниффер пакетов |
| **nmap** | 7.95 | Сканер сети (+ share) |

Ставятся из вкладки **Packages** в панели или при установке (zsh — сразу).

Релизы: https://github.com/initialfox/yandex-station-1-panel/releases
