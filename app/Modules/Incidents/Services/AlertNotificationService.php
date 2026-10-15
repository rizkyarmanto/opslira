<?php

namespace App\Modules\Incidents\Services;

use App\Modules\Incidents\Models\Incident;
use App\Modules\Reporting\Models\EmailDeliveryLog;
use App\Modules\Targets\Models\MonitoringTarget;
use Illuminate\Support\Facades\Http;
use Illuminate\Support\Facades\Log;
use Illuminate\Support\Facades\Mail;

/**
 * Dispatches state-change notifications (email + Telegram/Slack).
 */
class AlertNotificationService
{
    public function notify(MonitoringTarget $target, string $event, ?string $incidentId = null): void
    {
        $subject = $this->buildSubject($target, $event);
        $body = $this->buildBody($target, $event);

        $recipients = $this->recipients();

        foreach ($recipients as $email) {
            $log = EmailDeliveryLog::create([
                'email_category' => 'INCIDENT_ALERT',
                'incident_id' => $incidentId,
                'recipient_email' => $email,
                'subject' => $subject,
                'delivery_status' => 'PENDING',
            ]);

            try {
                Mail::raw($body, function ($message) use ($email, $subject) {
                    $message->to($email)->subject($subject);
                });

                $log->update(['delivery_status' => 'SENT', 'sent_at' => now()]);
            } catch (\Throwable $e) {
                $log->update([
                    'delivery_status' => 'FAILED',
                    'retry_count' => $log->retry_count + 1,
                    'smtp_response_or_error' => $e->getMessage(),
                ]);
                Log::error('Alert email failed', ['to' => $email, 'error' => $e->getMessage()]);
            }
        }

        $this->sendTelegram($body);
        $this->sendSlack($body);
    }

    private function buildSubject(MonitoringTarget $target, string $event): string
    {
        $prefix = match ($event) {
            'DOWN' => '[CRITICAL] DOWN',
            'RECOVERED' => '[RESOLVED] RECOVERED',
            default => '[ALERT]',
        };

        return "{$prefix}: {$target->name}";
    }

    private function buildBody(MonitoringTarget $target, string $event): string
    {
        $emoji = $event === 'DOWN' ? '🔴' : '🟢';
        $lines = [
            "{$emoji} Status change: {$event}",
            "Target: {$target->name}",
            "Endpoint: {$target->host_or_endpoint}".($target->port ? ":{$target->port}" : ''),
            "Type: {$target->target_type} ({$target->protocol_type})",
            "Environment: {$target->environment}",
            'Time: '.now()->timezone('Asia/Jakarta')->toDateTimeString().' WIB',
        ];

        if ($event === 'DOWN') {
            $incident = Incident::where('target_id', $target->id)->open()->latest('started_at')->first();
            if ($incident) {
                $lines[] = "Cause: {$incident->root_cause_message}";
            }
        }

        return implode("\n", $lines);
    }

    /**
     * @return array<int, string>
     */
    private function recipients(): array
    {
        $configured = env('ALERT_RECIPIENTS');

        return $configured ? array_filter(array_map('trim', explode(',', $configured))) : [];
    }

    private function sendTelegram(string $body): void
    {
        $token = config('services.telegram.bot_token');
        $chatId = config('services.telegram.chat_id');

        if (! $token || ! $chatId) {
            return;
        }

        try {
            Http::timeout(10)->post("https://api.telegram.org/bot{$token}/sendMessage", [
                'chat_id' => $chatId,
                'text' => $body,
            ]);
        } catch (\Throwable $e) {
            Log::warning('Telegram alert failed', ['error' => $e->getMessage()]);
        }
    }

    private function sendSlack(string $body): void
    {
        $webhook = config('services.slack.webhook_url');

        if (! $webhook) {
            return;
        }

        try {
            Http::timeout(10)->post($webhook, ['text' => $body]);
        } catch (\Throwable $e) {
            Log::warning('Slack alert failed', ['error' => $e->getMessage()]);
        }
    }
}
