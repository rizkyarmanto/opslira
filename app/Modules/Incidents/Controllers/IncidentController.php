<?php

namespace App\Modules\Incidents\Controllers;

use App\Modules\Incidents\Models\Incident;
use Illuminate\Http\JsonResponse;

class IncidentController
{
    /**
     * Close an incident by hand.
     *
     * The probe owns target status, so this writes the incident row only and
     * leaves the target's current_status where the last probe set it. A second
     * call on an already resolved incident changes nothing and still returns
     * 200, so the button is safe to press twice. Auto-recovery email stays
     * probe-only: a manual resolve sends no notification.
     */
    public function resolve(Incident $incident): JsonResponse
    {
        $incident->loadMissing('target:id,name');

        if ($incident->status !== 'RESOLVED') {
            $resolvedAt = now();
            $incident->update([
                'status' => 'RESOLVED',
                'resolved_at' => $resolvedAt,
                // Whole seconds between start and resolve. Carbon 3's
                // diffInSeconds returns a signed float (negative when the
                // argument is the earlier date), so the difference comes from
                // the epoch timestamps and the integer column takes an int.
                'duration_seconds' => max(0, $resolvedAt->getTimestamp() - $incident->started_at->getTimestamp()),
            ]);
        }

        return response()->json(['incident' => $incident->toAlertsRow()]);
    }
}
