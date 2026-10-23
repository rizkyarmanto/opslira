; Optional artisan worker programs.
;
; This file is a template. The container entrypoint renders it into
; /etc/supervisor/conf.d/artisan.conf only when RUN_ARTISAN_WORKERS=true.
;
; WARNING: the monitoring-runner service already owns probe dispatch, report
; dispatch, rollup and partition prune. Starting these programs while the Go
; runner is alive makes every one of them run twice, which doubles probe load
; and duplicate reports. Enable this only on a deployment without the runner.
;
; monitoring:reset-data is intentionally absent: it is a destructive manual
; command, never a daemon.

[program:artisan-probes]
command=/bin/sh -c 'while true; do php /var/www/html/artisan monitoring:dispatch-probes; sleep __PROBE_SECONDS__; done'
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

[program:artisan-reports]
command=/bin/sh -c 'while true; do php /var/www/html/artisan monitoring:dispatch-reports; sleep __REPORT_SECONDS__; done'
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

[program:artisan-rollup]
command=/bin/sh -c 'while true; do php /var/www/html/artisan monitoring:rollup; sleep __ROLLUP_SECONDS__; done'
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

[program:artisan-prune]
command=/bin/sh -c 'while true; do php /var/www/html/artisan monitoring:prune-partitions; sleep __PRUNE_SECONDS__; done'
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
