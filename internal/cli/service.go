package cli

import (
	"fmt"
	"os"

	"github.com/devherd/devherd/internal/config"
	"github.com/devherd/devherd/internal/observe"
	"github.com/devherd/devherd/internal/services"
	"github.com/spf13/cobra"
)

func newServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage shared local development services",
	}

	cmd.AddCommand(
		newServiceActionCmd("start"),
		newServiceActionCmd("stop"),
		newServiceActionCmd("status"),
	)

	return cmd
}

func newServiceActionCmd(action string) *cobra.Command {
	args := cobra.ExactArgs(1)
	if action == "status" {
		args = cobra.MaximumNArgs(1)
	}

	var force bool

	cmd := &cobra.Command{
		Use:   action + " [service]",
		Short: serviceActionShort(action),
		Args:  args,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}

			if err := paths.Ensure(); err != nil {
				return fmt.Errorf("create local directories: %w", err)
			}

			manager := services.NewManager(paths)
			service := ""
			if len(args) == 1 {
				service = args[0]
			}

			output, files, err := runServiceAction(cmd, manager, action, service, force)
			if err == nil && action == "start" {
				reportServiceAccess(cmd, manager, service)
			}
			// El aviso de configuracion va por stderr y **antes** de la salida de
			// docker: escribir dentro del directorio de alguien sin decirlo es como
			// se pierde una tarde buscando por que un servicio no toma sus ajustes.
			if notice := services.DescribeFileResults(files); notice != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), notice)
			}
			if output != "" {
				fmt.Fprintln(cmd.OutOrStdout(), output)
			}

			return err
		},
	}

	if action == "start" {
		cmd.Flags().BoolVar(&force, "force", false,
			"Restore managed configuration files from the DevHerd template, keeping a .bak copy")
	}

	return cmd
}

func runServiceAction(
	cmd *cobra.Command,
	manager services.Manager,
	action, service string,
	force bool,
) (string, []services.FileResult, error) {
	switch action {
	case "start":
		// Grafana sin Prometheus arranca y muestra paneles vacios, que es la peor
		// forma de fallar: parece que funciona. Se dice antes, no despues de que el
		// usuario abra el tablero y no entienda nada.
		if warning := missingDependencyWarning(cmd, manager, service); warning != "" {
			fmt.Fprintln(cmd.ErrOrStderr(), warning)
		}

		opts := services.StartOptions{Force: force}
		if services.NeedsWorkspace(service) {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", nil, fmt.Errorf("resolve the home directory: %w", err)
			}
			opts.Workspace = services.DefaultWorkspace(home)
			opts.UID = os.Getuid()
			opts.GID = os.Getgid()
		}
		if services.NeedsCollector(service) {
			addr, warning := collectorAddrForService(cmd)
			// El aviso va **antes** de arrancar: un target caido se descubre media
			// hora despues, cuando ya nadie recuerda que corrio este comando.
			if warning != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), warning)
			}
			opts.CollectorAddr = addr
		}

		return manager.Start(cmd.Context(), service, opts)
	case "stop":
		output, err := manager.Stop(cmd.Context(), service)

		return output, nil, err
	case "status":
		output, err := manager.Status(cmd.Context(), service)

		return output, nil, err
	default:
		return "", nil, fmt.Errorf("unsupported service action %q", action)
	}
}

func serviceActionShort(action string) string {
	switch action {
	case "start":
		return "Start a shared service"
	case "stop":
		return "Stop a shared service"
	case "status":
		return "Show status for a shared service"
	default:
		return "Manage a shared service"
	}
}

// collectorAddrForService resuelve la direccion del collector que un contenedor
// puede alcanzar, y devuelve un aviso cuando no la hay.
//
// Reusa planObserveAddrs en vez de recalcular: esa funcion ya resolvio que la red
// DevHerd se prefiere sobre la privada del proyecto, porque Docker le cambia la
// subred a esta al recrearla. Recalcularlo aqui seria repetir un bug que ya costo
// un falso positivo.
func collectorAddrForService(cmd *cobra.Command) (addr string, warning string) {
	plan := planObserveAddrs(cmd.Context(), observeAddrOptions{
		ProxyNetwork: observeSharedNetwork(cmd.Context()),
	})

	return collectorAddrFromPlan(plan, func(network, target string) observe.Reachability {
		return observe.ProbeFromContainer(cmd.Context(), plan.runner, network, target)
	})
}

// collectorAddrFromPlan es la decision, separada de como se obtiene el plan y de
// como se sondea, para poder probarla sin Docker.
func collectorAddrFromPlan(
	plan observeAddrPlan,
	probe func(network, addr string) observe.Reachability,
) (addr string, warning string) {

	// **La red que importa es la del servicio, no la del proyecto.** El DSN del
	// plan elige la red que mas contenedores del proyecto comparten, que es lo
	// correcto para un reporter dentro de ese proyecto. Prometheus corre en
	// infra_net, y el gateway que tiene que alcanzar es el de esa red: medido, el
	// plan devolvia el gateway de infra_web (172.18.0.1) mientras Prometheus vivia
	// en infra_net (172.20.0.1).
	for _, network := range plan.Networks {
		if network.Name != services.NetworkName || network.Gateway == "" {
			continue
		}

		addr := observe.WithHost(plan.DSN, network.Gateway)

		// Que exista el gateway no significa que se llegue: el cortafuegos del host
		// filtra el trafico de los contenedores y es la barrera que de verdad deja
		// el target caido. Se comprueba **desde dentro** de un contenedor, porque
		// probarlo desde el host da un falso positivo: el host alcanza su propio
		// loopback y atraviesa su propio cortafuegos.
		result := probe(services.NetworkName, addr)
		switch {
		case result.Reachable, result.Skipped:
			// Skipped es "no habia imagen local para sondear", no "no se llega".
			// Tratarlo como fallo asustaria sin motivo.
			return addr, ""
		}

		warning := "WARNING: the Observe collector did not answer at " + addr + " from inside a container.\n" +
			"  Prometheus will start, but the devherd-observe target will stay down."
		if hint := observe.FirewallHint(network, addr); hint != "" {
			warning += "\n  " + hint
		}
		warning += "\n  Fix it, then rerun with --force to rewrite the target."

		return addr, warning
	}

	// Sin gateway de infra_net no hay direccion que sirva: 127.0.0.1 dentro de un
	// contenedor es el propio contenedor, y escribirlo igual dejaria el target en
	// `down` sin ninguna explicacion.
	reason := plan.Reason
	if reason == "" {
		reason = "the " + services.NetworkName + " network has no gateway yet"
	}

	return plan.DSN, "WARNING: the Observe collector is not reachable from a container (" + reason + ").\n" +
		"  The devherd-observe target will stay down until it is.\n" +
		"  Start a shared service first so " + services.NetworkName + " exists, then\n" +
		"  `devherd service start prometheus --force` to rewrite the target."
}

// missingDependencyWarning avisa si falta el servicio del que este depende. No
// falla ni lo arranca solo: levantar contenedores que nadie pidio es peor que
// avisar, y el usuario puede tener su propio Prometheus fuera de DevHerd.
func missingDependencyWarning(cmd *cobra.Command, manager services.Manager, service string) string {
	dependency := services.DependsOn(service)
	if dependency == "" {
		return ""
	}

	running, err := manager.IsRunning(cmd.Context(), dependency)
	if err != nil || running {
		// Un fallo al consultar docker no se convierte en un aviso: el propio
		// `service start` va a fallar a continuacion con un error de verdad.
		return ""
	}

	return dependencyWarning(service, dependency)
}

// dependencyWarning arma el mensaje. Separado de la consulta a docker para poder
// probar el texto, que es lo unico que ve el usuario.
func dependencyWarning(service, dependency string) string {
	return "WARNING: " + dependency + " is not running, so " + service + " will show empty panels.\n" +
		"  Start it first:  devherd service start " + dependency + "\n" +
		"  Or point " + service + " at your own " + dependency + " by editing its datasource."
}

// reportServiceAccess dice como se entra al servicio recien arrancado. Para
// Jupyter no es cortesia: sin el token la URL no sirve, y buscarlo en los logs de
// un contenedor es exactamente la friccion que este comando existe para quitar.
func reportServiceAccess(cmd *cobra.Command, manager services.Manager, service string) {
	url, err := manager.AccessURL(service)
	if err != nil || url == "" {
		return
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\n%s: %s\n", service, url)
}
