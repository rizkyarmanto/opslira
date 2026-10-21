<?php

namespace App\Http\Middleware;

use Closure;
use Illuminate\Http\Request;
use Symfony\Component\HttpFoundation\Response;

/**
 * Guards the internal runner endpoints.
 *
 * The Go runner authenticates with a shared secret in X-Opslira-Runner instead
 * of a session, so these routes never carry the web middleware: no cookie, no
 * CSRF token. An unset server-side token fails closed rather than opening the
 * endpoints to anyone who guesses the path.
 */
class EnsureRunnerToken
{
    public function handle(Request $request, Closure $next): Response
    {
        $configured = (string) config('monitoring.runner_token', '');
        $provided = (string) $request->header('X-Opslira-Runner', '');

        if ($configured === '' || $provided === '' || ! hash_equals($configured, $provided)) {
            abort(response()->json(['message' => 'Invalid runner token'], 401));
        }

        return $next($request);
    }
}
