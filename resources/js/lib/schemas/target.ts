import { z } from 'zod';

/**
 * Dynamic Zod schemas per protocol. Mirrors the config_payload shapes.
 * docs/gemini/erd_skema_database_postgresql.md:162-170
 */

export const networkSchema = z.object({
    sub_type: z.enum(['ROUTER', 'SWITCH', 'AP', 'SERVER']),
    icmp_count: z.coerce.number().int().min(1).max(10).default(4),
    enable_snmp: z.boolean().default(false),
});

export const webAppSchema = z.object({
    method: z.enum(['GET', 'POST', 'HEAD']).default('GET'),
    expected_status: z.coerce.number().int().default(200),
    verify_ssl: z.boolean().default(true),
    expected_json_key: z.string().optional(),
    expected_json_value: z.string().optional(),
});

export const domainSchema = z.object({
    whois_server: z.string().optional(),
    check_ssl_port: z.coerce.number().int().default(443),
    expected_a_records: z.array(z.string()).default([]),
    warn_days_before: z.coerce.number().int().default(14),
});

export const databaseSchema = z.object({
    db_driver: z.enum(['pgsql', 'mysql', 'sqlsrv']).default('pgsql'),
    database_name: z.string().min(1),
    query: z.string().default('SELECT 1'),
    collect_connection_stats: z.boolean().default(true),
    check_replication_lag: z.boolean().default(false),
});

export const mailSchema = z.object({
    check_ports: z.record(z.coerce.number()).default({ smtp: 587, imap: 993 }),
    mail_check_mode: z.enum(['smtp', 'imap']).default('smtp'),
    check_dnsbl: z.boolean().default(true),
    dnsbl_providers: z.array(z.string()).default(['zen.spamcop.net']),
});

export const s3Schema = z.object({
    s3_provider: z.enum(['aws', 'compatible']).default('aws'),
    region: z.string().min(1),
    bucket_name: z.string().min(1),
    use_path_style_endpoint: z.boolean().default(true),
    // Legacy rows from the old form; the S3 URL field is the endpoint now.
    endpoint: z.string().optional(),
});

export const internetSchema = z.object({
    icmp_count: z.coerce.number().int().min(1).max(10).default(4),
});

export const beszelSchema = z.object({
    beszel_system_name: z.string().min(1),
    warn_cpu_pct: z.coerce.number().int().default(85),
    warn_mem_pct: z.coerce.number().int().default(90),
    warn_disk_pct: z.coerce.number().int().default(85),
});

/**
 * Discriminated union on protocol_type. Selects the dynamic panel schema.
 */
export const targetFormSchema = z
    .object({
        name: z.string().min(1, 'Name is required').max(150),
        dashboard_group: z.enum(['NETWORK', 'APP_DOMAIN_DB', 'MAIL_S3_BESZEL']),
        target_type: z.enum([
            'NETWORK_DEVICE',
            'INTERNET',
            'WEB_APP',
            'DOMAIN',
            'DATABASE',
            'MAIL_SERVER',
            'S3_STORAGE',
            'BESZEL_SERVER',
        ]),
        environment: z.enum(['PRODUCTION', 'STAGING', 'DEVELOPMENT', 'INTERNAL']),
        host_or_endpoint: z.string().min(1, 'Host/endpoint is required').max(255),
        // The input feeds "" when untouched; coerce that to null so the optional
        // port does not turn into 0 and fail min(1) on every clean save.
        port: z.preprocess(
            (value) => (value === '' || value === undefined ? null : value),
            z.coerce.number().int().min(1).max(65535).nullable(),
        ),
        protocol_type: z.enum([
            'ICMP_SNMP',
            'ICMP_PING',
            'HTTP_HEALTH',
            'WHOIS_SSL',
            'DB_NATIVE_PING',
            'SMTP_DNSBL',
            'S3_HEAD',
            'BESZEL_API',
        ]),
        check_interval_seconds: z.coerce.number().int().min(10).default(60),
        timeout_seconds: z.coerce.number().int().min(1).max(60).default(5),
        retry_threshold: z.coerce.number().int().min(1).max(10).default(3),
        degraded_threshold_ms: z.coerce.number().int().min(1).default(1000),
        config_payload: z.record(z.unknown()).default({}),
        credentials: z
            .object({
                auth_type: z.enum([
                    'NONE',
                    'DB_AUTH',
                    'SNMP_V2C',
                    'SNMP_V3',
                    'S3_KEY',
                    'BESZEL',
                    'HTTP_BASIC',
                    'HTTP_BEARER',
                    'MAIL_AUTH',
                ]),
                payload: z.record(z.unknown()).default({}),
            })
            .default({ auth_type: 'NONE', payload: {} }),
    })
    .passthrough();

export type TargetFormValues = z.infer<typeof targetFormSchema>;

/** Map target_type -> dynamic sub-schema (used to build config_payload). */
export const configSchemaByType = {
    NETWORK_DEVICE: networkSchema,
    INTERNET: internetSchema,
    WEB_APP: webAppSchema,
    DOMAIN: domainSchema,
    DATABASE: databaseSchema,
    MAIL_SERVER: mailSchema,
    S3_STORAGE: s3Schema,
    BESZEL_SERVER: beszelSchema,
} as const;

export const protocolByTargetType = {
    NETWORK_DEVICE: 'ICMP_SNMP',
    INTERNET: 'ICMP_PING',
    WEB_APP: 'HTTP_HEALTH',
    DOMAIN: 'WHOIS_SSL',
    DATABASE: 'DB_NATIVE_PING',
    MAIL_SERVER: 'SMTP_DNSBL',
    S3_STORAGE: 'S3_HEAD',
    BESZEL_SERVER: 'BESZEL_API',
} as const;

export const dashboardGroupByTargetType = {
    NETWORK_DEVICE: 'NETWORK',
    INTERNET: 'NETWORK',
    WEB_APP: 'APP_DOMAIN_DB',
    DOMAIN: 'APP_DOMAIN_DB',
    DATABASE: 'APP_DOMAIN_DB',
    MAIL_SERVER: 'MAIL_S3_BESZEL',
    S3_STORAGE: 'MAIL_S3_BESZEL',
    BESZEL_SERVER: 'MAIL_S3_BESZEL',
} as const;
