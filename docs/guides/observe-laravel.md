# Integrar DevHerd Observe en un proyecto Laravel

Como hacer que un Laravel levantado con DevHerd reporte sus errores al collector local,
sin instalar el SDK de Sentry. Verificado sobre Laravel 13 + PHP 8.4 en Docker.

```
excepcion en tu app  →  handler de Laravel  →  DevherdObserveReporter
                     →  POST /api/<proyecto>/event
                     →  collector  →  issue agrupado + logs del contenedor
```

> **Por que no el SDK oficial.** El parser de envelopes del collector **no descomprime
> gzip**, que es el transporte por defecto de `sentry/sentry-laravel`. Un POST propio al
> endpoint simple evita el problema por completo y no agrega dependencias.

## 1. Requisitos de red (leelo antes que nada)

Es donde falla la integracion. DevHerd ya lo resuelve por defecto, pero conviene entender
que esta pasando.

Dentro de un contenedor, `127.0.0.1` es el propio contenedor. Por eso el collector escucha
**a la vez** en loopback y en el gateway de la red compartida `infra_web`, y el DSN que
genera `attach` usa esa segunda direccion:

```bash
devherd observe start
# observe collector: http://127.0.0.1:9777
# observe collector: http://172.18.0.1:9777
# containers on infra_web should use http://172.18.0.1:9777
```

Esa IP es una interfaz del host, asi que el panel te sigue funcionando en
`http://127.0.0.1:9777/observe`, y **no** es ruteable desde tu LAN.

**Si usas `ufw`, hace falta una regla.** El trafico contenedor -> host se descarta en
`INPUT`. Los puertos publicados por Docker funcionan porque sus reglas DNAT preceden a ufw,
pero el collector es un listener normal:

```bash
sudo ufw allow from 172.18.0.0/16 to 172.18.0.1 port 9777 proto tcp \
  comment 'devherd observe collector'
```

`observe status` comprueba esto **desde dentro de un contenedor**, no desde el host, y te
imprime la regla exacta si falla:

```bash
devherd observe status
# container reachability (infra_web): ok at http://172.18.0.1:9777
```

## 2. Aplicar el attach

```bash
devherd observe attach <proyecto> --stack laravel --dry-run
devherd observe attach <proyecto> --stack laravel
devherd up
```

El override inyecta `SENTRY_DSN`, `SENTRY_ENVIRONMENT` y `DEVHERD_OBSERVE=1`, y estampa los
labels `devherd.*` que permiten la correlacion con Docker. El `up` es obligatorio: las
variables y labels solo entran al recrear los contenedores.

`.devherd.observe.override.yml` lo cubre el patron `/.devherd.*.yml` del `.gitignore` que
genera `devherd scaffold`. Si tu repo no lo tiene, agregalo.

## 3. El reporter

Copia esto en `app/Exceptions/DevherdObserveReporter.php`. Es generico: deriva el proyecto
del DSN, asi que sirve igual en cualquier repo.

```php
<?php

namespace App\Exceptions;

use Throwable;

/**
 * Envia excepciones al collector local de DevHerd Observe.
 *
 * Solo actua cuando DEVHERD_OBSERVE y SENTRY_DSN estan presentes en el entorno;
 * las inyecta el override que genera `devherd observe attach`, asi que fuera de
 * desarrollo la clase es inerte. Nunca propaga errores propios: si el collector
 * esta caido o tarda, la excepcion original sigue su curso normal.
 *
 * Se leen las variables con getenv() y no con env() a proposito: env() devuelve
 * null cuando la config esta cacheada (`php artisan config:cache`).
 */
class DevherdObserveReporter
{
    private const CONNECT_TIMEOUT_SECONDS = 1;

    private const TIMEOUT_SECONDS = 2;

    public static function report(Throwable $e): void
    {
        try {
            $endpoint = self::endpoint();

            if ($endpoint === null) {
                return;
            }

            $payload = json_encode(self::payload($e));

            if ($payload === false) {
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

            curl_exec($handle);
            curl_close($handle);
        } catch (Throwable) {
            // Observe jamas debe romper la aplicacion.
        }
    }

    /**
     * Deriva el endpoint de ingesta del DSN local.
     *
     * http://devherd@172.18.0.1:9777/mi-proyecto
     *   -> http://172.18.0.1:9777/api/mi-proyecto/event
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
     * @return array<string, mixed>
     */
    private static function payload(Throwable $e): array
    {
        $environment = getenv('SENTRY_ENVIRONMENT');
        $service = getenv('DEVHERD_SERVICE');
        $context = self::context($e);

        $payload = [
            'message' => $e->getMessage() !== '' ? $e->getMessage() : $e::class,
            'exception_type' => self::shortName($e::class),
            'level' => 'error',
            'platform' => 'php',
            'service' => is_string($service) && $service !== '' ? $service : 'app',
            'container' => (string) gethostname(),
            'culprit' => self::culprit($e),
            'transaction' => self::transaction($context),
            'environment' => is_string($environment) && $environment !== '' ? $environment : 'local',
        ];

        // Se manda ademas en crudo: hoy Observe lo guarda en raw_payload sin
        // exponerlo, pero queda disponible por SQL y si algun dia lo publica en
        // el panel no hay que tocar nada aqui.
        if ($context !== []) {
            $payload['context'] = $context;
        }

        return $payload;
    }

    /**
     * Lee el contexto de la excepcion siguiendo la convencion de Laravel: si la
     * clase define context(): array, el framework lo usa para enriquecer el log.
     *
     * @return array<string, mixed>
     */
    private static function context(Throwable $e): array
    {
        if (! method_exists($e, 'context')) {
            return [];
        }

        try {
            $context = $e->context();
        } catch (Throwable) {
            return [];
        }

        return is_array($context) ? $context : [];
    }

    private static function culprit(Throwable $e): string
    {
        $file = $e->getFile();

        // Se calcula la raiz desde la ubicacion de este archivo (app/Exceptions/)
        // y no con base_path(): ese helper exige la aplicacion ya arrancada, y el
        // reporter debe poder ejecutarse tambien fuera de un contexto Laravel.
        $base = dirname(__DIR__, 2).DIRECTORY_SEPARATOR;

        if (str_starts_with($file, $base)) {
            $file = substr($file, strlen($base));
        }

        return $file.':'.$e->getLine();
    }

    /**
     * El contexto viaja pegado a la transaccion a proposito. Es el unico campo
     * que Observe muestra y que NO entra en el fingerprint (project +
     * exception_type + message + culprit + service), asi que puede variar por
     * ocurrencia sin partir el issue en uno nuevo por cada valor distinto.
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
```

## 4. Cablearlo (Laravel 11+)

En `bootstrap/app.php`:

```php
use App\Exceptions\DevherdObserveReporter;

    ->withExceptions(function (Exceptions $exceptions): void {
        // Inerte salvo que DevHerd Observe este attachado (DEVHERD_OBSERVE=1).
        // No devuelve nada, asi que el logging por defecto de Laravel se conserva.
        $exceptions->report(function (Throwable $e): void {
            DevherdObserveReporter::report($e);
        });
    })->create();
```

Devolver `void` es importante: si el callback devolviera `false`, Laravel dejaria de
escribir la excepcion en `storage/logs`.

En Laravel 10 o anterior el equivalente va en `app/Exceptions/Handler.php`, dentro de
`register()`, con `$this->reportable(fn (Throwable $e) => DevherdObserveReporter::report($e));`.

## 5. Que se captura solo

Cualquier excepcion **no capturada** que llegue al handler: controllers, jobs, comandos,
listeners. No hay que hacer nada.

## 6. Que NO se captura

Laravel mantiene una lista interna (`internalDontReport`) que **nunca** pasa por los
reportable callbacks:

`ValidationException` · `ModelNotFoundException` · `AuthenticationException` ·
`AuthorizationException` · `HttpException` (todos los 404/403) · `HttpResponseException` ·
`TokenMismatchException` · `RecordsNotFoundException` · `MultipleRecordsFoundException` ·
`BackedEnumCaseNotFoundException` · `OriginMismatchException` · `RequestExceptionInterface`

Un 404, un fallo de validacion o un `findOrFail()` vacio **no apareceran en Observe** aunque
los veas en el navegador. Es correcto (son errores del usuario, no del sistema), pero
explica huecos que de otro modo parecen bugs de la integracion.

## 7. Reportar a proposito

El helper `report()` de Laravel pasa por los mismos callbacks, asi que llega a Observe:

```php
try {
    return $this->validarContraSat($cfdi);
} catch (SoapFault $e) {
    report($e);                       // queda registrado en Observe
    return CfdiResult::sinValidar();  // el usuario no ve un 500
}
```

Tambien sirve para senalar algo que no es una excepcion real:

```php
if ($factura->total !== $suma) {
    report(new DomainException("Descuadre en factura {$factura->id}"));
}
```

Donde rinde mas: transiciones de estado y todo lo que hable con un tercero (pasarelas de
pago, APIs fiscales), porque son los fallos que no reproduces en local.

> **Livewire.** Los errores dentro de un componente suelen convertirse en respuesta HTTP y
> pueden no llegar al handler. Si un flujo Livewire te importa, envuelvelo en `try/catch`
> con `report()` explicito en vez de confiar en la captura automatica.

## 8. Contexto por excepcion

Define `context(): array` en tus excepciones de dominio, la convencion que Laravel ya usa
para enriquecer los logs. El reporter la lee sola:

```php
class DescuadreFacturaException extends DomainException
{
    public function __construct(private Factura $factura)
    {
        parent::__construct("Descuadre en factura {$factura->id}");
    }

    public function context(): array
    {
        return ['factura_id' => $this->factura->id, 'total' => $this->factura->total];
    }
}
```

El contexto se ve en `devherd observe timeline <event-id>`:

```
Exception: DescuadreFacturaException
Message: Descuadre en factura 9182

Payload:
- context: {"factura_id":9182,"total":45300.5}
```

y en la API del panel como `payload_extra`.

> **Nota de versiones.** Observe siempre guardo el payload completo en
> `events.raw_payload`, pero durante un tiempo **ninguna consulta leia esa columna**: el
> contexto se escribia y quedaba inaccesible salvo por SQLite directo. Si tu binario es
> anterior a esa correccion, el bloque `Payload:` no aparece; comprueba con
> `devherd observe timeline <id>`. Para inspeccionarlo a mano:
>
> ```bash
> sqlite3 ~/.local/share/devherd/observability/devherd-observe.db \
>   "SELECT json_extract(raw_payload,'\$.context') FROM events WHERE id = 42;"
> ```

**El contexto tambien se refleja en `transaction`.** Es deliberado y sigue siendo util: es
el unico campo que aparece en el listado de `observe events` (el timeline exige abrir un
evento concreto) **y** queda fuera del fingerprint, asi que puede variar en cada ocurrencia
sin partir el issue. Verificado: tres errores con contexto distinto lanzados desde la misma
linea agrupan en un unico issue con `COUNT=3`.

> **Cuidado con `culprit`.** Ese si entra en el fingerprint. Lanzar la *misma* excepcion
> desde lineas distintas produce issues distintos, aunque el mensaje sea identico.

## 9. Verificar la instalacion

```bash
# 1. El contenedor alcanza el collector
devherd observe status

# 2. Las variables llegaron
docker exec <contenedor-app> env | grep -E 'DEVHERD|SENTRY'

# 3. Un error de punta a punta
docker exec <contenedor-app> php -r '
require "/app/vendor/autoload.php";
$app = require "/app/bootstrap/app.php";
$app->make(Illuminate\Contracts\Console\Kernel::class)->bootstrap();
report(new RuntimeException("prueba de integracion"));'

devherd observe issues <proyecto>
```

## 10. Trampas conocidas

**`artisan tinker --execute='throw ...'` no sirve para probar.** Tinker captura y renderiza
la excepcion el mismo, sin pasarla por los reportable callbacks. Usa `report($e)`.

**"Container logs: none captured" casi siempre es correcto.** La ventana es de +-30 s
alrededor del evento y se consulta **una sola vez**, al ingerir. Si la app estaba ociosa no
hay nada que traer, y no se rellena despues.

**Si el collector no corre, el error se pierde.** La ingesta es push sincrono sin cola ni
reintento. El collector es un proceso en foreground: hay que acordarse de levantarlo.

**No uses `base_path()` ni helpers de Laravel dentro del reporter.** Exigen la aplicacion
arrancada; como el `catch` es silencioso por diseno, un helper no disponible hace que el
reporte falle sin dejar rastro.

**Un proyecto llamado `observe` colisiona** con la ruta del panel y su ingesta devuelve 405.

## Referencias

- [observe.md](../observe.md) — arquitectura y referencia completa del modulo
- [IMPROVEMENTS.md](../IMPROVEMENTS.md) — seccion "Hallazgos de campo" (O1-O9)
