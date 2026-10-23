#!/bin/sh
#
# Entrypoint for the web container.
#
# The image ships the artisan worker programs as a template, not as an active
# supervisor config, because the Go runner owns the same periodic work. This
# script renders them into conf.d only when the operator asks for them, then
# hands PID 1 to supervisord.
#
# Every rendered file is deleted on start, so a container restarted with a
# different environment never keeps a program the operator just switched off.
set -eu

CONF_DIR=/etc/supervisor/conf.d
TEMPLATE=/etc/opslira/supervisor-artisan.conf.tpl

mkdir -p "$CONF_DIR"
rm -f "$CONF_DIR/artisan.conf" "$CONF_DIR/artisan-queue.conf"

is_positive_int() {
    case "$1" in
        '' | *[!0-9]*) return 1 ;;
        *) [ "$1" -ge 1 ] ;;
    esac
}

# Prints the interval to use, or exits when the operator supplied something
# that is not a plain positive integer. Refusing beats a sed that silently
# writes a broken supervisor command.
resolve_seconds() {
    value=$1
    name=$2
    fallback=$3

    if [ -z "$value" ]; then
        printf '%s' "$fallback"
        return 0
    fi

    if ! is_positive_int "$value"; then
        echo "opslira: $name must be a positive integer, got '$value'" >&2
        exit 1
    fi

    printf '%s' "$value"
}

if [ "${RUN_ARTISAN_WORKERS:-false}" = "true" ]; then
    probe_seconds=$(resolve_seconds "${ARTISAN_PROBE_SECONDS:-}" ARTISAN_PROBE_SECONDS 60)
    report_seconds=$(resolve_seconds "${ARTISAN_REPORT_SECONDS:-}" ARTISAN_REPORT_SECONDS 60)
    rollup_seconds=$(resolve_seconds "${ARTISAN_ROLLUP_SECONDS:-}" ARTISAN_ROLLUP_SECONDS 3600)
    prune_seconds=$(resolve_seconds "${ARTISAN_PRUNE_SECONDS:-}" ARTISAN_PRUNE_SECONDS 86400)

    sed \
        -e "s|__PROBE_SECONDS__|$probe_seconds|g" \
        -e "s|__REPORT_SECONDS__|$report_seconds|g" \
        -e "s|__ROLLUP_SECONDS__|$rollup_seconds|g" \
        -e "s|__PRUNE_SECONDS__|$prune_seconds|g" \
        "$TEMPLATE" > "$CONF_DIR/artisan.conf"

    echo "opslira: artisan workers enabled (probes ${probe_seconds}s, reports ${report_seconds}s, rollup ${rollup_seconds}s, prune ${prune_seconds}s)" >&2
    echo "opslira: keep monitoring-runner stopped, or every job runs twice" >&2
else
    echo "opslira: artisan workers disabled (set RUN_ARTISAN_WORKERS=true to enable)" >&2
fi

if [ "${RUN_ARTISAN_QUEUE:-false}" = "true" ]; then
    queue_names=${ARTISAN_QUEUE_NAMES:-default}

    cat > "$CONF_DIR/artisan-queue.conf" <<EOF
[program:artisan-queue]
command=php /var/www/html/artisan queue:work --queue=${queue_names} --sleep=3 --tries=3 --max-time=3600
directory=/var/www/html
autostart=true
autorestart=true
startsecs=5
startretries=3
stopasgroup=true
killasgroup=true
stdout_logfile=/dev/stdout
stdout_logfile_maxbytes=0
stderr_logfile=/dev/stderr
stderr_logfile_maxbytes=0
EOF

    echo "opslira: queue worker enabled (queue=${queue_names})" >&2
fi

exec "$@"
