<?php

namespace App\Exceptions;

use Throwable;

/**
 * Envia excepciones y eventos de dominio al collector local de DevHerd Observe.
 *
 * Generado por `devherd observe attach --reporter`. Puedes editarlo: DevHerd no
 * lo sobrescribe salvo que pases --force.
 *
 * Solo actua cuando DEVHERD_OBSERVE y SENTRY_DSN estan presentes en el entorno;
 * las inyecta el override que genera `devherd observe attach`, asi que fuera de
 * desarrollo la clase es inerte. Nunca propaga errores propios: si el collector
 * esta caido o tarda, el flujo original sigue su curso normal.
 *
 * Se leen las variables con getenv() y no con env() a proposito: env() devuelve
 * null cuando la config esta cacheada (`php artisan config:cache`).
 *
 * Cableado en bootstrap/app.php:
 *
 *     ->withExceptions(function (Exceptions $exceptions): void {
 *         $exceptions->report(function (Throwable $e): void {
 *             DevherdObserveReporter::report($e);
 *         });
 *     })
 */
class DevherdObserveReporter
{
    private const CONNECT_TIMEOUT_SECONDS = 1;

    private const TIMEOUT_SECONDS = 2;

    /**
     * Cuanto del cuerpo de una respuesta de error se copia al log.
     */
    private const SNIPPET_LENGTH = 200;

    /**
     * Reporta una excepcion.
     *
     * La excepcion puede afinar el evento definiendo metodos opcionales:
     * context(): array (convencion de Laravel), level(): string y
     * fingerprint(): string.
     */
    public static function report(Throwable $e): void
    {
        self::send([
            'exception_type' => self::shortName($e::class),
            'message' => $e->getMessage() !== '' ? $e->getMessage() : self::shortName($e::class),
            'level' => self::optionalString($e, 'level') ?? 'error',
            'culprit' => self::relativePath($e->getFile()).':'.$e->getLine(),
            'context' => self::optionalArray($e, 'context'),
            'fingerprint' => self::optionalString($e, 'fingerprint'),
        ]);
    }

    /**
     * Reporta algo que NO es una excepcion: un login rechazado, un pago
     * denegado, cualquier evento de dominio que quieras seguir como issue.
     *
     * El mensaje debe ser constante y los datos que cambian en cada ocurrencia
     * van en $context. El collector enmascara numeros, correos e identificadores
     * antes de agrupar, pero un mensaje estable se lee mucho mejor en el
     * listado. Con $fingerprint decides tu la agrupacion y el mensaje deja de
     * influir en ella.
     *
     *     DevherdObserveReporter::capture(
     *         'LoginUnknownAccount',
     *         'Login rejected: unknown account',
     *         ['email_domain' => 'example.com', 'ip' => $request->ip()],
     *         'warning',
     *     );
     *
     * @param  array<string, mixed>  $context
     */
    public static function capture(
        string $type,
        string $message,
        array $context = [],
        string $level = 'error',
        ?string $fingerprint = null,
    ): void {
        self::send([
            'exception_type' => $type,
            'message' => $message,
            'level' => $level,
            'culprit' => self::caller(),
            'context' => $context,
            'fingerprint' => $fingerprint,
        ]);
    }

    /**
     * @param  array<string, mixed>  $event
     */
    private static function send(array $event): void
    {
        try {
            $endpoint = self::endpoint();

            if ($endpoint === null) {
                self::trace('no endpoint: OBSERVE_DSN is missing or malformed');

                return;
            }

            $payload = json_encode(self::payload($event));

            if ($payload === false) {
                self::trace('payload could not be encoded: '.json_last_error_msg());

                return;
            }

            $handle = curl_init($endpoint);

            curl_setopt_array($handle, [
                CURLOPT_POST => true,
                CURLOPT_POSTFIELDS => $payload,
                CURLOPT_HTTPHEADER => ['Content-Type: application/json'],
                CURLOPT_RETURNTRANSFER => true,
                CURLOPT_CONNECTTIMEOUT => self::CONNECT_TIMEOUT_SECONDS,
                CURLOPT_TIMEOUT => self::TIMEOUT_SECONDS,
            ]);

            $body = curl_exec($handle);
            $status = (int) curl_getinfo($handle, CURLINFO_RESPONSE_CODE);
            $error = curl_error($handle);
            curl_close($handle);

            // El collector caido y el collector contestando 500 son problemas
            // distintos, y hasta ahora los dos se veian igual: como nada.
            if ($body === false || $error !== '') {
                self::trace('transport failed: '.($error !== '' ? $error : 'unknown curl error'));

                return;
            }

            if ($status >= 400) {
                self::trace('collector rejected the event with HTTP '.$status.self::snippet($body));
            }
        } catch (Throwable $t) {
            // Observe jamas debe romper la aplicacion, pero callarse del todo
            // convierte un reporter roto en una app sin errores.
            self::trace('reporter threw '.self::shortName($t::class).': '.$t->getMessage());
        }
    }

    /**
     * Deja rastro de un envio que no llego.
     *
     * Sin esto, un `observe issues` vacio es ambiguo: no se puede distinguir una
     * aplicacion sana de un reporter que lleva horas fallando en silencio. Va a
     * `error_log()` y no al logger de Laravel a proposito: el logger puede ser
     * justo lo que esta roto, y este codigo tiene que funcionar incluso mientras
     * el framework se cae.
     */
    private static function trace(string $reason): void
    {
        try {
            error_log('[devherd-observe] '.$reason);
        } catch (Throwable) {
            // Si ni error_log funciona, no queda nada que hacer: lo que no puede
            // pasar es que el intento de avisar tumbe la aplicacion.
        }
    }

    /**
     * Recorta el cuerpo de una respuesta de error para el log.
     *
     * Un collector devolviendo HTML de error puede escupir kilobytes, y un log
     * de aplicacion no es sitio para eso.
     */
    private static function snippet(string|bool $body): string
    {
        if (! is_string($body)) {
            return '';
        }

        $trimmed = trim($body);

        if ($trimmed === '') {
            return '';
        }

        if (mb_strlen($trimmed) > self::SNIPPET_LENGTH) {
            $trimmed = mb_substr($trimmed, 0, self::SNIPPET_LENGTH).'...';
        }

        return ': '.$trimmed;
    }

    /**
     * Deriva el endpoint de ingesta del DSN local.
     *
     * http://devherd@172.20.0.1:9777/aang-server
     *   -> http://172.20.0.1:9777/api/aang-server/event
     */
    private static function endpoint(): ?string
    {
        if (getenv('DEVHERD_OBSERVE') !== '1') {
            return null;
        }

        $dsn = getenv('SENTRY_DSN');

        if (! is_string($dsn) || $dsn === '') {
            return null;
        }

        $parts = parse_url($dsn);
        $project = trim($parts['path'] ?? '', '/');

        if (empty($parts['scheme']) || empty($parts['host']) || $project === '') {
            return null;
        }

        $authority = $parts['host'];

        if (! empty($parts['port'])) {
            $authority .= ':'.$parts['port'];
        }

        return sprintf('%s://%s/api/%s/event', $parts['scheme'], $authority, rawurlencode($project));
    }

    /**
     * @param  array<string, mixed>  $event
     * @return array<string, mixed>
     */
    private static function payload(array $event): array
    {
        $environment = getenv('SENTRY_ENVIRONMENT');
        $service = getenv('DEVHERD_SERVICE');
        $context = is_array($event['context'] ?? null) ? $event['context'] : [];

        $payload = [
            'message' => $event['message'],
            'exception_type' => $event['exception_type'],
            'level' => $event['level'],
            'platform' => 'php',
            'service' => is_string($service) && $service !== '' ? $service : 'app',
            'container' => (string) gethostname(),
            'culprit' => $event['culprit'],
            'transaction' => self::transaction($context),
            'environment' => is_string($environment) && $environment !== '' ? $environment : 'local',
        ];

        // El contexto viaja tambien en crudo: el collector lo guarda en
        // raw_payload y lo muestra en `devherd observe timeline`.
        if ($context !== []) {
            $payload['context'] = $context;
        }

        // Con fingerprint explicito el collector ignora mensaje y culprit para
        // agrupar, asi que dos ocurrencias distintas caen en el mismo issue.
        if (is_string($event['fingerprint'] ?? null) && $event['fingerprint'] !== '') {
            $payload['fingerprint'] = $event['fingerprint'];
        }

        return $payload;
    }

    /**
     * Lee un metodo opcional que devuelve array, siguiendo la convencion de
     * Laravel para context().
     *
     * @return array<string, mixed>
     */
    private static function optionalArray(Throwable $e, string $method): array
    {
        if (! method_exists($e, $method)) {
            return [];
        }

        try {
            $value = $e->{$method}();
        } catch (Throwable) {
            return [];
        }

        return is_array($value) ? $value : [];
    }

    private static function optionalString(Throwable $e, string $method): ?string
    {
        if (! method_exists($e, $method)) {
            return null;
        }

        try {
            $value = $e->{$method}();
        } catch (Throwable) {
            return null;
        }

        return is_string($value) && $value !== '' ? $value : null;
    }

    /**
     * Ubicacion desde la que se llamo a capture(), para que dos capturas de
     * sitios distintos no se confundan.
     */
    private static function caller(): string
    {
        $frames = debug_backtrace(DEBUG_BACKTRACE_IGNORE_ARGS, 3);
        $frame = $frames[1] ?? null;

        if (! is_array($frame) || ! isset($frame['file'], $frame['line'])) {
            return 'unknown';
        }

        return self::relativePath((string) $frame['file']).':'.$frame['line'];
    }

    /**
     * La raiz se calcula desde la ubicacion de este archivo (app/Exceptions/) y
     * no con base_path(): ese helper exige la aplicacion ya arrancada, y el
     * reporter debe poder ejecutarse tambien fuera de un contexto Laravel.
     */
    private static function relativePath(string $file): string
    {
        $base = dirname(__DIR__, 2).DIRECTORY_SEPARATOR;

        if (str_starts_with($file, $base)) {
            return substr($file, strlen($base));
        }

        return $file;
    }

    /**
     * El contexto viaja pegado a la transaccion a proposito: es el unico campo
     * que Observe muestra en el listado de eventos y que NO entra en el
     * fingerprint, asi que puede variar por ocurrencia sin partir el issue.
     *
     * @param  array<string, mixed>  $context
     */
    private static function transaction(array $context): string
    {
        $base = PHP_SAPI === 'cli'
            ? 'cli:'.implode(' ', array_slice($_SERVER['argv'] ?? [], 1, 3))
            : trim(($_SERVER['REQUEST_METHOD'] ?? '').' '.strtok($_SERVER['REQUEST_URI'] ?? '', '?'));

        if ($context === []) {
            return $base;
        }

        $pairs = [];

        foreach ($context as $key => $value) {
            $pairs[] = $key.'='.self::stringify($value);
        }

        return mb_substr(trim($base.' {'.implode(', ', $pairs).'}'), 0, 300);
    }

    private static function stringify(mixed $value): string
    {
        $rendered = match (true) {
            is_bool($value) => $value ? 'true' : 'false',
            $value === null => 'null',
            is_scalar($value) => (string) $value,
            default => json_encode($value) ?: '?',
        };

        return mb_substr($rendered, 0, 60);
    }

    private static function shortName(string $class): string
    {
        $position = strrpos($class, '\\');

        return $position === false ? $class : substr($class, $position + 1);
    }
}
