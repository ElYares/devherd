# Diagramas de DevHerd

Diagramas en [Mermaid](https://mermaid.js.org/), renderizables en GitHub, GoLand (plugin
Mermaid) y cualquier visor Markdown moderno. Complementan a
[ARCHITECTURE.md](ARCHITECTURE.md) (detalle de paquetes) y
[SYSTEM-OVERVIEW.md](SYSTEM-OVERVIEW.md) (estado real del sistema).

---

## 1. Arquitectura de componentes

Qué paquete depende de qué, y contra qué recurso externo actúa cada uno.

```mermaid
flowchart TD
    MAIN["cmd/devherd/main.go<br/>Execute() + exit 1"]

    subgraph CLI["internal/cli — 18 comandos raíz (Cobra)"]
        ROOT["root.go<br/>flags --verbose / --log-json"]
        CTX["app_context.go<br/>loadAppContext(): paths + config + SQLite"]
    end

    subgraph CORE["Núcleo transversal"]
        CONFIG["config<br/>rutas XDG + config.json"]
        DB["database<br/>SQLite migrada"]
        RUNNER["runner<br/>seam de exec + timeouts"]
    end

    subgraph DOMAIN["Dominio"]
        DETECTOR["detector<br/>stacks y frameworks"]
        COMPOSE["compose<br/>resolve, naming, args, logs"]
        SCAFFOLD["scaffold<br/>genera docker-compose"]
        PREFLIGHT["preflight<br/>colisiones puertos/nombres"]
        DOCTOR["doctor<br/>prerequisitos del host"]
    end

    subgraph NET["Red y publicación"]
        PROXY["proxy<br/>caddy (host) / caddy-docker-external"]
        DNS["dns<br/>bloque administrado /etc/hosts"]
        SERVICES["services<br/>redis + mailpit"]
    end

    OBSERVE["observe<br/>collector HTTP + SQLite propia<br/>panel, alertas, correlación Docker"]

    MAIN --> ROOT
    ROOT --> CTX
    CTX --> CONFIG
    CTX --> DB

    ROOT --> DETECTOR
    ROOT --> COMPOSE
    ROOT --> SCAFFOLD
    ROOT --> PREFLIGHT
    ROOT --> DOCTOR
    ROOT --> PROXY
    ROOT --> SERVICES
    ROOT --> OBSERVE

    COMPOSE --> RUNNER
    SERVICES --> RUNNER
    SCAFFOLD --> COMPOSE
    PREFLIGHT --> COMPOSE
    PROXY --> DNS

    DOCKER[("Docker daemon")]
    HOSTS[("/etc/hosts — sudo")]
    SQLITE1[("~/.local/share/devherd/devherd.db")]
    SQLITE2[("…/observability/devherd-observe.db")]

    RUNNER --> DOCKER
    PROXY --> DOCKER
    DOCTOR --> DOCKER
    OBSERVE --> DOCKER
    DNS --> HOSTS
    DB --> SQLITE1
    OBSERVE --> SQLITE2
```

> `internal/api`, `internal/logs`, `internal/runtimes` e `internal/sentry` son sólo
> `doc.go` sin implementación; no aparecen en el diagrama a propósito.

---

## 2. Flujo típico de usuario

```mermaid
flowchart LR
    INIT["devherd init<br/>config.json + SQLite"]
    DOCTOR["devherd doctor<br/>¿el host está listo?"]
    PARK["devherd park ~/dev<br/>descubre proyectos"]
    SCAF["devherd scaffold<br/>(sólo si no hay compose)"]
    PLAN["devherd plan / inspect<br/>preflight sin efectos"]
    UP["devherd up<br/>docker compose up -d"]
    APPLY["devherd proxy apply<br/>Caddy + /etc/hosts"]
    OPEN["devherd open<br/>abre proyecto.test"]

    INIT --> DOCTOR --> PARK --> SCAF --> PLAN --> UP --> APPLY --> OPEN

    UP -.-> SVC["devherd service start redis|mailpit"]
    UP -.-> OBS["devherd observe attach<br/>+ observe start"]

    SERVE["devherd serve = up + proxy apply + open"]
    SERVE -.encadena.-> UP
```

---

## 3. `devherd up` paso a paso

```mermaid
sequenceDiagram
    autonumber
    actor U as Usuario
    participant CLI as internal/cli/up.go
    participant CTX as loadAppContext
    participant CMP as compose.ResolveProject
    participant PF as preflight.Inspect
    participant RUN as runner.Cmd
    participant D as Docker

    U->>CLI: devherd up [path]
    CLI->>CTX: resolver rutas XDG, config.json, abrir SQLite
    CTX-->>CLI: appContext (Paths, Config, DB)
    CLI->>CMP: resolver proyecto
    Note over CMP: .devherd.yml si existe,<br/>si no autodetecta compose.<br/>ProjectName = devherd-SLUG-SHA1[:8]
    CMP-->>CLI: Project{Root, ComposeFiles, EnvFile, Proxy}
    CLI->>CLI: appendObserveOverride + override de proxy
    alt sin --no-inspect
        CLI->>PF: puertos, container_name, volúmenes, env Laravel, redes
        PF-->>CLI: Report (ok / warn / fail)
        CLI-->>U: aborta si hay fail y no hay --force
    end
    CLI->>RUN: docker compose --project-name … up --build -d
    RUN->>D: ejecuta
    D-->>RUN: salida combinada stdout+stderr
    RUN-->>CLI: resultado (o el mensaje real de Docker como error)
    CLI-->>U: estado del stack por stdout
```

---

## 4. Proxy: driver `caddy-docker-external`

El camino recomendado. DevHerd administra un stack propio (`local_proxy`) cuyo
contenedor es `infra_caddy`.

```mermaid
flowchart TB
    BROWSER["Navegador<br/>https://proyecto.test"]
    HOSTS["/etc/hosts<br/>bloque # devherd start/end → 127.0.0.1"]

    subgraph HOST["Host"]
        subgraph LP["stack local_proxy (~/.local/share/devherd/local_proxy)"]
            CADDY["infra_caddy<br/>Caddyfile con bloques administrados"]
        end

        subgraph WEB["red infra_web"]
            SVCWEB["servicio publicado del proyecto<br/>(alias por servicio)"]
        end

        subgraph NET["red infra_net"]
            REDIS["infra_redis :6379"]
            MAIL["infra_mailpit :1025 / :8025"]
        end

        subgraph PRIV["red privada del proyecto"]
            APP["app"]
            QUEUE["queue"]
            DBC["db"]
        end
    end

    BROWSER --> HOSTS --> CADDY
    CADDY -->|reverse_proxy alias:puerto| SVCWEB
    SVCWEB --- APP
    APP --- QUEUE
    APP --- DBC
    APP -.-> REDIS
    APP -.-> MAIL
```

Secuencia de `devherd proxy apply` con este driver:

```mermaid
sequenceDiagram
    autonumber
    participant CLI as cli/proxy.go
    participant EXT as proxy/external.go
    participant BS as proxy/bootstrap.go
    participant D as Docker
    participant DNS as dns.SyncHosts

    CLI->>EXT: BuildExternalProject(cfg, project)
    Note over EXT: dominio efectivo + prefijo de alias.<br/>Usa proxy.service/proxy.port del manifiesto;<br/>si no, regla especial vue+flask
    CLI->>EXT: EnsureComposeOverride
    Note over EXT: escribe .devherd.proxy.override.yml<br/>conectando servicios a infra_web
    CLI->>EXT: ConnectProject
    EXT->>D: crea red si falta + docker network connect --alias
    CLI->>EXT: ApplyExternalProxy
    EXT->>BS: BootstrapExternalProxy (plantillas go:embed)
    BS-->>EXT: compose.yml, Caddyfile, .env en local_proxy
    EXT->>EXT: mergeExternalProxyConfig (bloques administrados)
    EXT->>D: docker compose up -d
    EXT->>D: exec infra_caddy: caddy validate && caddy reload
    CLI->>DNS: syncManagedDomains
    DNS->>DNS: sudo cp del /etc/hosts con el bloque reescrito
```

> Fricción conocida: el Caddyfile se escribe **antes** de validarse y no hay rollback
> si la validación falla (`ARCHITECTURE.md` §17).

---

## 5. Observe: ingesta de eventos

```mermaid
flowchart LR
    subgraph CONT["Contenedor del proyecto"]
        SDK["SDK Sentry / reporter generado<br/>SENTRY_DSN inyectado por<br/>.devherd.observe.override.yml"]
    end

    subgraph HOSTP["Host — devherd observe start"]
        LISTEN["Listeners: 127.0.0.1:9777<br/>+ gateway de cada red relevante"]
        SRV["observe/server.go<br/>handleAPI"]
        NORM["NormalizeEvent()<br/>título, culprit, tags, timestamp"]
        FP["Fingerprint()<br/>explícito o derivado"]
        STORE["Store.StoreEvent()"]
        PANEL["Panel web<br/>http://127.0.0.1:9777/observe"]
        ALERTS["Alertas locales<br/>new-issue, error-rate,<br/>container-exit, container-restart"]
    end

    ODB[("SQLite separada<br/>devherd-observe.db")]
    DOCKER[("Docker: containers, exits, restarts")]

    SDK -->|HTTP: endpoint simple o envelope Sentry| LISTEN
    LISTEN --> SRV --> NORM --> FP --> STORE --> ODB
    STORE --> ALERTS
    ODB --> PANEL
    DOCKER -->|correlación| STORE
```

El detalle que suele morder: dentro de un contenedor `127.0.0.1` es el propio
contenedor. Por eso el collector escucha también en los gateways de `infra_web`,
`infra_net` y las redes de los contenedores ya observados, y `attach`/`dsn` eligen la
red DevHerd con mayor cobertura de contenedores del proyecto.

```mermaid
flowchart TD
    START["devherd observe start"] --> LSN["¿alcanzable desde el contenedor?"]
    LSN -->|no, ufw activo| FW["devherd observe firewall --apply<br/>una regla por red"]
    LSN -->|sí| ST["devherd observe status PROYECTO<br/>sonda dentro de la red del proyecto"]
    FW --> ST
    ST --> DAEMON["devherd observe daemon install<br/>unidad systemd --user"]
```

---

## 6. Modelo de datos

Dos bases SQLite independientes, con dos sistemas de esquema distintos.

```mermaid
erDiagram
    PARKS ||--o{ PROJECTS : contiene
    PROJECTS ||--o{ PROJECT_DOMAINS : publica

    PARKS {
        int id PK
        string path
    }
    PROJECTS {
        int id PK
        int park_id FK
        string path
        string name
        string framework
        string stack
    }
    PROJECT_DOMAINS {
        int id PK
        int project_id FK
        string domain
        bool primary
    }
```

> La baseline `0001_init.sql` declara además `settings`, `runtime_preferences`,
> `services`, `sentry_configs` y `events`, pero **ningún código Go las usa** hoy.
> La base de Observe (issues, eventos, contenedores, alertas, deliveries) vive aparte
> y **no tiene migraciones versionadas**: reejecuta `schema.sql` en cada invocación.

---

## 7. Estados de un proyecto

```mermaid
stateDiagram-v2
    [*] --> Detectado: devherd park
    Detectado --> Contenerizado: devherd scaffold
    Detectado --> Contenerizado: ya traía compose
    Contenerizado --> Arriba: devherd up
    Arriba --> Publicado: devherd proxy apply
    Publicado --> Arriba: devherd down (quita bloques + override + red)
    Arriba --> Parado: devherd stop
    Parado --> Arriba: devherd up
    Arriba --> [*]: devherd down
    Publicado --> Observado: devherd observe attach
    Observado --> Publicado: devherd observe detach
```

---

## Cómo regenerar / editar

- Estos diagramas son **texto**: se editan aquí y se versionan con el código.
- Para exportar PNG/SVG: [mermaid.live](https://mermaid.live) o
  `npx @mermaid-js/mermaid-cli -i docs/diagrams.md -o out.png`.
- El grafo navegable autogenerado del repo está en `graphify-out/graph.html`
  (703 nodos, 45 comunidades) y su índice en `graphify-out/wiki/index.md`.
