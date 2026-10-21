export type TargetStatus = 'UP' | 'DEGRADED' | 'DOWN' | 'MAINTENANCE' | 'PENDING';

export type DashboardGroup = 'NETWORK' | 'APP_DOMAIN_DB' | 'MAIL_S3_BESZEL';

export type TargetType =
    | 'NETWORK_DEVICE'
    | 'INTERNET'
    | 'WEB_APP'
    | 'DOMAIN'
    | 'DATABASE'
    | 'MAIL_SERVER'
    | 'S3_STORAGE'
    | 'BESZEL_SERVER';

export type ProtocolType =
    | 'ICMP_SNMP'
    | 'ICMP_PING'
    | 'HTTP_HEALTH'
    | 'WHOIS_SSL'
    | 'DB_NATIVE_PING'
    | 'SMTP_DNSBL'
    | 'S3_HEAD'
    | 'BESZEL_API';

export type Environment = 'PRODUCTION' | 'STAGING' | 'DEVELOPMENT' | 'INTERNAL';

export interface MonitoringTarget {
    id: string;
    name: string;
    dashboard_group: DashboardGroup;
    target_type: TargetType;
    environment: Environment;
    host_or_endpoint: string;
    port: number | null;
    protocol_type: ProtocolType;
    config_payload: Record<string, unknown>;
    check_interval_seconds: number;
    timeout_seconds: number;
    retry_threshold: number;
    degraded_threshold_ms: number;
    current_status: TargetStatus;
    is_maintenance: boolean;
    is_active: boolean;
    last_checked_at: string | null;
    next_check_at: string;
    /** Decorated server-side for dashboards */
    state?: TargetState;
    /** Last 60 check outcomes, oldest first. null means no check recorded. */
    heartbeat?: Array<'UP' | 'DEGRADED' | 'DOWN' | null>;
}

export interface TargetState {
    status: TargetStatus;
    last_latency_ms: number | null;
    consecutive_failures: number;
    last_checked_at: string | null;
    summary_metrics: Record<string, unknown>;
}

export interface ProbeResult {
    status: 'UP' | 'DEGRADED' | 'DOWN';
    responseTimeMs: number | null;
    errorMessage: string | null;
    metricsPayload: Record<string, unknown>;
}

export interface Incident {
    id: string;
    target_id: string;
    target_name?: string;
    severity: 'CRITICAL' | 'WARNING';
    status: 'OPEN' | 'ACKNOWLEDGED' | 'RESOLVED';
    root_cause_message: string;
    started_at: string;
    resolved_at: string | null;
    duration_seconds: number | null;
}

export interface NocSummary {
    total: number;
    up: number;
    degraded: number;
    down: number;
    maintenance: number;
    global_sla: number;
}

export interface ReportSchedule {
    id: string;
    name: string;
    period_type: 'DAILY' | 'WEEKLY' | 'MONTHLY' | 'CUSTOM_CRON';
    cron_expression: string;
    timezone: string;
    scope_filter: Record<string, unknown>;
    report_formats: string[];
    recipients_to: string[];
    recipients_cc: string[];
    send_only_on_issues: boolean;
    is_active: boolean;
    last_run_at: string | null;
    next_run_at: string;
}

export interface EmailDeliveryLog {
    id: string;
    email_category: 'SCHEDULED_REPORT' | 'INCIDENT_ALERT' | 'EXPIRY_WARNING';
    recipient_email: string;
    subject: string;
    delivery_status: 'PENDING' | 'SENT' | 'RETRYING' | 'FAILED';
    retry_count: number;
    sent_at: string | null;
    created_at: string;
}

export type UserRole = 'SUPER_ADMIN' | 'SYSADMIN' | 'DEVELOPER' | 'MANAGER';

export interface ManagedUser {
    id: string;
    name: string;
    email: string;
    role: UserRole;
    is_active: boolean;
    created_at: string;
}
