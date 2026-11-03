FROM php:8.4-fpm-alpine

# System dependencies + probe binaries (iputils-ping, net-snmp, whois, openssl)
RUN apk add --no-cache \
    bash \
    git \
    curl \
    unzip \
    icu-dev \
    libzip-dev \
    postgresql-dev \
    oniguruma-dev \
    $PHPIZE_DEPS \
    iputils-ping \
    net-snmp-tools \
    whois \
    openssl \
    nginx \
    supervisor \
    nodejs \
    npm

# PHP extensions required by the app + probe adapters
RUN docker-php-ext-install \
    pdo_pgsql \
    pgsql \
    pdo_mysql \
    sockets \
    bcmath \
    intl \
    zip \
    opcache \
    pcntl \
    gd

# SQL Server target support. pdo_sqlsrv links against the Microsoft ODBC driver,
# which Microsoft publishes as an Alpine package outside the Alpine repository,
# so it is fetched and installed with --allow-untrusted. The driver version is
# pinned because the download URL carries it.
ARG MSODBCSQL_VERSION=18.7.1.1-1
RUN set -eux; \
    case "$(uname -m)" in \
        x86_64) arch="amd64" ;; \
        aarch64) arch="arm64" ;; \
        *) echo "no SQL Server ODBC package for $(uname -m)" >&2; exit 1 ;; \
    esac; \
    apk add --no-cache unixodbc-dev; \
    curl -fsSL -o /tmp/msodbcsql18.apk \
        "https://download.microsoft.com/download/ade174b7-8cea-4543-91a6-c33ae320c2f0/msodbcsql18_${MSODBCSQL_VERSION}_${arch}.apk"; \
    apk add --no-cache --allow-untrusted /tmp/msodbcsql18.apk; \
    rm -f /tmp/msodbcsql18.apk; \
    pecl install sqlsrv pdo_sqlsrv; \
    docker-php-ext-enable sqlsrv pdo_sqlsrv

# Redis extension
RUN pecl install redis && docker-php-ext-enable redis

# Composer
COPY --from=composer:2 /usr/bin/composer /usr/bin/composer

WORKDIR /var/www/html

# Install PHP dependencies first (layer caching)
COPY composer.json composer.lock* ./
RUN composer install --no-dev --no-scripts --no-interaction --prefer-dist || composer install --no-dev --no-scripts --no-interaction

# Install JS dependencies + build frontend
COPY package.json package-lock.json* ./
RUN npm install

COPY . .

RUN npm run build || true
RUN composer dump-autoload --optimize

# Storage permissions
RUN chown -R www-data:www-data storage bootstrap/cache \
    && chmod -R ug+rwx storage bootstrap/cache

# Nginx + php-fpm supervised together (web container serves both)
COPY docker/app/nginx.conf /etc/nginx/nginx.conf
COPY docker/app/default.conf /etc/nginx/http.d/default.conf
COPY docker/app/supervisord.conf /etc/supervisord.conf
COPY docker/app/php-fpm.conf /usr/local/etc/php-fpm.d/zz-opslira.conf

# Optional artisan worker programs. The entrypoint renders the template into
# /etc/supervisor/conf.d only when RUN_ARTISAN_WORKERS=true; otherwise the
# container runs nginx + php-fpm alone and leaves periodic work to the runner.
COPY docker/app/supervisor-artisan.conf.tpl /etc/opslira/supervisor-artisan.conf.tpl
COPY docker/app/entrypoint.sh /usr/local/bin/opslira-entrypoint
RUN chmod +x /usr/local/bin/opslira-entrypoint \
    && mkdir -p /etc/supervisor/conf.d

EXPOSE 80 9000
ENTRYPOINT ["/usr/local/bin/opslira-entrypoint"]
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf"]
