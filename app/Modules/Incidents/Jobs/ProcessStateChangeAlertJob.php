<?php

namespace App\Modules\Incidents\Jobs;

use App\Modules\Incidents\Services\AlertNotificationService;
use App\Modules\Targets\Models\MonitoringTarget;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;

/**
 * Sends DOWN/RECOVERED notifications. Dispatched to queue:alerts (priority 1).
 * docs/gemini/dokumen_arsitektur_aplikasi_web.md:103, 167
 */
class ProcessStateChangeAlertJob implements ShouldQueue
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;

    public int $tries = 3;

    public int $timeout = 15;

    /**
     * @param  string  $event  DOWN|RECOVERED
     */
    public function __construct(
        public string $targetId,
        public string $event,
        public ?string $incidentId = null,
    ) {
        $this->onQueue('alerts');
    }

    public function handle(AlertNotificationService $service): void
    {
        $target = MonitoringTarget::find($this->targetId);

        if (! $target) {
            return;
        }

        $service->notify($target, $this->event, $this->incidentId);
    }
}
