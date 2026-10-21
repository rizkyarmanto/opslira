<?php

namespace App\Modules\Probes\Controllers;

use App\Modules\Probes\Contracts\ProbeResult;
use App\Modules\Probes\Services\ProbeExecutionService;
use App\Modules\Targets\Models\MonitoringTarget;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Artisan;
use Illuminate\Support\Facades\DB;

/**
 * Internal endpoints called by the Go runner.
 *
 * next_check_at advances inside the claim transaction while the rows are locked,
 * which is what keeps two runners from probing the same target twice. The cost
 * is explicit: a probe that dies mid-flight leaves next_check_at already pushed
 * forward, so that target waits one interval before it is tried again. Accepted,
 * because a crashed probe writes no log either way.
 */
class RunnerController
{
    /**
     * Hand out due targets and push their next_check_at forward.
     */
    public function claim(Request $request): JsonResponse
    {
        $validated = $request->validate([
            'limit' => ['required', 'integer', 'min:1', 'max:100'],
        ]);

        $targets = DB::transaction(function () use ($validated) {
            $due = MonitoringTarget::dueForCheck()
                ->orderBy('next_check_at')
                ->limit($validated['limit'])
                ->lockForUpdate()
                ->get();

            foreach ($due as $target) {
                $target->update([
                    'next_check_at' => now()->addSeconds($target->check_interval_seconds),
                ]);
            }

            return $due;
        });

        return response()->json([
            'targets' => $targets
                ->map(fn (MonitoringTarget $target) => [
                    'id' => $target->id,
                    'name' => $target->name,
                ])
                ->values(),
        ]);
    }

    /**
     * Run one probe through the existing PHP adapter pipeline.
     */
    public function probe(string $target): JsonResponse
    {
        $model = MonitoringTarget::findOrFail($target);

        if (! $model->is_active || $model->is_maintenance) {
            return response()->json(['message' => 'Target is not due'], 409);
        }

        /** @var ProbeResult $result */
        $result = app(ProbeExecutionService::class)->execute($model);

        return response()->json([
            'status' => $result->status,
            'response_time_ms' => $result->responseTimeMs,
            'error_message' => $result->errorMessage,
        ]);
    }

    public function rollup(): JsonResponse
    {
        return $this->runMaintenance('monitoring:rollup');
    }

    public function prune(): JsonResponse
    {
        return $this->runMaintenance('monitoring:prune-partitions');
    }

    private function runMaintenance(string $command): JsonResponse
    {
        $exit = Artisan::call($command);
        $output = Artisan::output();

        if ($exit !== 0) {
            return response()->json(['ok' => false, 'output' => $output], 500);
        }

        return response()->json(['ok' => true, 'output' => $output]);
    }
}
