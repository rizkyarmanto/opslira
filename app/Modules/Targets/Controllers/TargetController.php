<?php

namespace App\Modules\Targets\Controllers;

use App\Modules\Probes\Contracts\ProbeResult;
use App\Modules\Probes\Services\ProbeAdapterFactory;
use App\Modules\Probes\Services\ProbeFailureLogger;
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
        private readonly ProbeFailureLogger $failures,
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

        // Default list is active targets: disabled rows are kept out until the
        // operator asks for them, so a soft-deleted target stops cluttering the
        // list the moment it is switched off.
        $state = $request->query('state', 'active');
        if ($state === 'disabled') {
            $query->where('is_active', false);
        } elseif ($state !== 'all') {
            $query->where('is_active', true);
        }

        // Default hides rows in maintenance: a paused target is not something the
        // operator acts on, and `maintenance=all` shows both again.
        $maintenance = $request->query('maintenance', 'no');
        if ($maintenance === 'yes') {
            $query->where('is_maintenance', true);
        } elseif ($maintenance === 'no') {
            $query->where('is_maintenance', false);
        }

        return Inertia::render('Targets/TargetIndex', [
            'targets' => $query->orderBy('name')->get(),
            'filters' => $request->only([
                'dashboard_group', 'target_type', 'environment', 'status', 'state', 'maintenance',
            ]),
        ]);
    }

    public function store(StoreTargetRequest $request): JsonResponse
    {
        $data = $this->normalizeTargetPayload($request->validated());

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
        $escaped = str_replace(['%', '_'], ['\\%', '\\_'], $term);
        $like = '%'.$escaped.'%';
        $prefix = $escaped.'%';

        $targets = MonitoringTarget::query()
            ->where('is_active', true)
            ->when($term !== '', function ($query) use ($like) {
                $query->where(function ($q) use ($like) {
                    $q->where('name', 'ilike', $like)
                        ->orWhere('host_or_endpoint', 'ilike', $like)
                        ->orWhere('target_type', 'ilike', $like)
                        ->orWhere('protocol_type', 'ilike', $like)
                        ->orWhere('environment', 'ilike', $like);
                });
            })
            // A name or host that starts with what was typed beats a match in the
            // middle of a word, so typing "8.8" surfaces the host first.
            ->orderByRaw(
                'CASE WHEN name ILIKE ? THEN 0 WHEN host_or_endpoint ILIKE ? THEN 1 ELSE 2 END',
                [$prefix, $prefix]
            )
            ->orderBy('name')
            ->limit(12)
            ->get([
                'id', 'name', 'host_or_endpoint', 'port', 'dashboard_group', 'target_type',
                'protocol_type', 'current_status', 'is_maintenance',
            ]);

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
        $oldHub = (string) ($target->config_payload['hub_url'] ?? '');
        $oldCreds = [];
        if ($credential = $target->credentials) {
            $oldCreds = $this->encryption->decrypt(
                $credential->encrypted_payload,
                $credential->encryption_iv,
                $credential->encryption_tag,
            );
        }
        $oldEmail = (string) ($oldCreds['email'] ?? '');
        $data = $this->normalizeTargetPayload($request->validated());
        unset($data['credentials']);
        $target->update($data);
        if (array_key_exists('credentials', $request->all())) {
            $this->persistCredentials($target, $request->input('credentials'));
        }
        if ($target->target_type === 'BESZEL_SERVER') {
            $newHub = (string) ($target->config_payload['hub_url'] ?? '');
            $newCreds = $request->input('credentials.payload', []);
            $newEmail = (string) ($newCreds['email'] ?? $oldEmail);
            if ($oldHub !== '' && $oldEmail !== '') {
                \App\Modules\Probes\Adapters\BeszelApiProbeAdapter::forgetToken($oldHub, $oldEmail);
            }
            if ($newHub !== '' && $newEmail !== '' && ($newHub !== $oldHub || is_array($newCreds))) {
                \App\Modules\Probes\Adapters\BeszelApiProbeAdapter::forgetToken($newHub, $newEmail);
            }
        }
        return response()->json(['target' => $target->fresh()]);
    }

    public function destroy(MonitoringTarget $target): JsonResponse
    {
        // Soft disable, never hard delete
        $target->update(['is_active' => false]);

        return response()->json(['ok' => true]);
    }

    public function enable(MonitoringTarget $target): JsonResponse
    {
        $target->update(['is_active' => true]);

        return response()->json(['ok' => true, 'is_active' => $target->is_active]);
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
            $this->failures->record($target, $result, 'test');

            return response()->json($result->toArray());
        } catch (\Throwable $e) {
            $this->failures->record($target, ProbeResult::down($e->getMessage()), 'test');

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
        // Not saved yet: the logger still needs name, type, and host to say which
        // check failed, so it gets a transient target with the form values.
        $transient = new MonitoringTarget([
            'name' => (string) ($data['name'] ?? 'Unsaved target test'),
            'dashboard_group' => (string) ($data['dashboard_group'] ?? 'NETWORK'),
            'target_type' => (string) ($data['target_type'] ?? 'NETWORK_DEVICE'),
            'protocol_type' => (string) ($data['protocol_type'] ?? 'ICMP_SNMP'),
            'host_or_endpoint' => (string) ($data['host_or_endpoint'] ?? ''),
        ]);

        try {
            $data = $this->normalizeTargetPayload($data);
            $adapter = $this->factory->make($data['protocol_type']);
            $config = [
                ...($data['config_payload'] ?? []),
                'host' => $data['host_or_endpoint'] ?? null,
                'port' => $data['port'] ?? null,
            ];
            $credentials = [];
            if (! empty($data['credentials']['auth_type']) && $data['credentials']['auth_type'] !== 'NONE') {
                $credentials = $data['credentials']['payload'] ?? [];
            }
            $adapter->withTimeoutSeconds((int) ($data['timeout_seconds'] ?? 5))->configure($config, $credentials);

            /** @var ProbeResult $result */
            $result = $adapter->execute();
            $this->failures->record($transient, $result, 'test');

            return response()->json($result->toArray());
        } catch (\Throwable $e) {
            $this->failures->record($transient, ProbeResult::down($e->getMessage()), 'test');

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

    /**
     * INTERNET targets are ping only. The protocol is forced and the ping-only
     * flag is set server-side, so a client cannot switch the target back onto
     * TCP fallback or SNMP; any other type must not inherit the flag.
     *
     * @param  array<string, mixed>  $data
     * @return array<string, mixed>
     */
    private function normalizeTargetPayload(array $data): array
    {
        if (($data['target_type'] ?? null) === 'INTERNET') {
            $data['protocol_type'] = 'ICMP_PING';
            $data['config_payload'] = [...($data['config_payload'] ?? []), 'icmp_only' => true];

            return $data;
        }

        if (is_array($data['config_payload'] ?? null)) {
            unset($data['config_payload']['icmp_only']);
            if (($data['target_type'] ?? null) === 'BESZEL_SERVER') {
                if (! isset($data['config_payload']['beszel_system_name']) && isset($data['config_payload']['beszel_system_id'])) {
                    $data['config_payload']['beszel_system_name'] = $data['config_payload']['beszel_system_id'];
                }
                unset($data['config_payload']['beszel_system_id']);
            }
        }

        return $data;
    }

    private function persistCredentials(MonitoringTarget $target, ?array $credentials): void
    {
        if (empty($credentials) || empty($credentials['auth_type']) || $credentials['auth_type'] === 'NONE') {
            $target->credentials()->delete();

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
