<?php

namespace App\Modules\Targets\Controllers;

use App\Modules\Probes\Contracts\ProbeResult;
use App\Modules\Probes\Services\ProbeAdapterFactory;
use App\Modules\Probes\Services\TargetStateService;
use App\Modules\Targets\Models\MonitoringTarget;
use App\Modules\Targets\Models\TargetCredential;
use App\Modules\Targets\Requests\StoreTargetRequest;
use App\Modules\Targets\Requests\UpdateTargetRequest;
use App\Modules\Targets\Services\CredentialEncryptionService;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Inertia\Inertia;
use Inertia\Response;

class TargetController
{
    public function __construct(
        private readonly CredentialEncryptionService $encryption,
        private readonly ProbeAdapterFactory $factory,
        private readonly TargetStateService $state,
    ) {
    }

    public function index(Request $request): Response
    {
        $query = MonitoringTarget::query();

        if ($group = $request->query('dashboard_group')) {
            $query->where('dashboard_group', $group);
        }
        if ($type = $request->query('target_type')) {
            $query->where('target_type', $type);
        }
        if ($env = $request->query('environment')) {
            $query->where('environment', $env);
        }
        if ($status = $request->query('status')) {
            $query->where('current_status', $status);
        }

        return Inertia::render('Targets/TargetIndex', [
            'targets' => $query->orderBy('name')->get(),
            'filters' => $request->only(['dashboard_group', 'target_type', 'environment', 'status']),
        ]);
    }

    public function store(StoreTargetRequest $request): JsonResponse
    {
        $data = $request->validated();

        $target = MonitoringTarget::create([
            ...$data,
            'created_by' => $request->user()?->id,
            'next_check_at' => now(),
        ]);

        $this->persistCredentials($target, $request->input('credentials'));

        return response()->json(['target' => $target->fresh()], 201);
    }

    /**
     * JSON list for the command palette, which needs data rather than an Inertia
     * page. Matches on name, endpoint, type, or protocol, and caps the result set
     * (R-26: the search control must actually search).
     */
    public function search(Request $request): JsonResponse
    {
        $term = trim((string) $request->query('q', ''));

        $targets = MonitoringTarget::query()
            ->where('is_active', true)
            ->when($term !== '', function ($query) use ($term) {
                $like = '%'.str_replace(['%', '_'], ['\\%', '\\_'], $term).'%';
                $query->where(function ($q) use ($like) {
                    $q->where('name', 'ilike', $like)
                        ->orWhere('host_or_endpoint', 'ilike', $like)
                        ->orWhere('target_type', 'ilike', $like)
                        ->orWhere('protocol_type', 'ilike', $like);
                });
            })
            ->orderBy('name')
            ->limit(12)
            ->get(['id', 'name', 'host_or_endpoint', 'target_type', 'protocol_type', 'current_status']);

        return response()->json(['data' => $targets]);
    }

    public function show(MonitoringTarget $target): Response
    {
        $logs = $target->logs()->orderByDesc('checked_at')->limit(50)->get();

        return Inertia::render('Targets/TargetDetail', [
            'target' => [
                ...$target->toArray(),
                'state' => $this->state->get($target->id),
                'has_credentials' => $target->credentials()->exists(),
            ],
            'logs' => $logs,
            'incidents' => $target->incidents()->orderByDesc('started_at')->limit(20)->get(),
        ]);
    }

    public function update(UpdateTargetRequest $request, MonitoringTarget $target): JsonResponse
    {
        $data = $request->validated();
        unset($data['credentials']);

        $target->update($data);
        $this->persistCredentials($target, $request->input('credentials'));

        return response()->json(['target' => $target->fresh()]);
    }

    public function destroy(MonitoringTarget $target): JsonResponse
    {
        // Soft disable, never hard delete
        $target->update(['is_active' => false]);

        return response()->json(['ok' => true]);
    }

    /**
     * Synchronous connection test. Does not persist a log.
     */
    public function testConnection(MonitoringTarget $target): JsonResponse
    {
        try {
            $adapter = $this->factory->make($target->protocol_type);

            $credentials = [];
            if ($credential = $target->credentials) {
                $credentials = $this->encryption->decrypt(
                    $credential->encrypted_payload,
                    $credential->encryption_iv,
                    $credential->encryption_tag,
                );
            }

            $config = [...($target->config_payload ?? []), 'host' => $target->host_or_endpoint, 'port' => $target->port];
            $adapter->forTarget($target)->configure($config, $credentials);

            /** @var ProbeResult $result */
            $result = $adapter->execute();

            return response()->json($result->toArray());
        } catch (\Throwable $e) {
            return response()->json([
                'status' => 'DOWN',
                'responseTimeMs' => null,
                'errorMessage' => $e->getMessage(),
                'metricsPayload' => [],
            ]);
        }
    }

    /**
     * Test connection from raw form payload (before a target exists).
     */
    public function testConnectionPayload(Request $request): JsonResponse
    {
        $data = $request->all();

        try {
            $adapter = $this->factory->make($data['protocol_type']);
            $config = [
                ...($data['config_payload'] ?? []),
                'host' => $data['host_or_endpoint'] ?? null,
                'port' => $data['port'] ?? null,
            ];
            $adapter->configure($config, []);

            /** @var ProbeResult $result */
            $result = $adapter->execute();

            return response()->json($result->toArray());
        } catch (\Throwable $e) {
            return response()->json([
                'status' => 'DOWN',
                'responseTimeMs' => null,
                'errorMessage' => $e->getMessage(),
                'metricsPayload' => [],
            ]);
        }
    }

    /**
     * Toggle maintenance mode.
     *
     * Turning it on sets the status to MAINTENANCE so the dashboards stop treating
     * the target as failing. Turning it off resets the status to PENDING rather than
     * keeping MAINTENANCE, so the next probe decides the real value. Keeping the old
     * MAINTENANCE value here left the target permanently mislabelled (R-17).
     */
    public function toggleMaintenance(MonitoringTarget $target): JsonResponse
    {
        $enabled = ! $target->is_maintenance;

        $target->update([
            'is_maintenance' => $enabled,
            'current_status' => $enabled ? 'MAINTENANCE' : 'PENDING',
        ]);

        return response()->json([
            'is_maintenance' => $enabled,
            'current_status' => $target->current_status,
        ]);
    }

    private function persistCredentials(MonitoringTarget $target, ?array $credentials): void
    {
        if (empty($credentials) || empty($credentials['auth_type']) || $credentials['auth_type'] === 'NONE') {
            return;
        }

        $payload = $credentials['payload'] ?? [];
        $encrypted = $this->encryption->encrypt($payload);

        TargetCredential::updateOrCreate(
            ['target_id' => $target->id],
            [
                'auth_type' => $credentials['auth_type'],
                'encrypted_payload' => $encrypted['encrypted_payload'],
                'encryption_iv' => $encrypted['encryption_iv'],
                'encryption_tag' => $encrypted['encryption_tag'],
                'updated_at' => now(),
            ],
        );
    }
}
