<?php

namespace App\Modules\Reporting\Controllers;

use App\Modules\Reporting\Services\ReportDispatchService;
use Illuminate\Http\JsonResponse;

/**
 * Internal endpoint called by the Go runner to generate due reports.
 */
class ReportRunnerController
{
    public function __construct(private readonly ReportDispatchService $service)
    {
    }

    public function dispatch(): JsonResponse
    {
        return response()->json(['dispatched' => $this->service->dispatchDue()]);
    }
}
