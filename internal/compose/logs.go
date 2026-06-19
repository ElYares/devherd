package compose

import (
	"context"
	"io"
	"os/exec"
)

// LogsOptions configura la salida de `docker compose logs`.
type LogsOptions struct {
	Follow   bool
	Tail     string
	Services []string
}

// LogsArgs construye los argumentos de `docker compose ... logs ...` para un
// proyecto. Se mantiene como función pura para poder testearla sin Docker.
func LogsArgs(project Project, opts LogsOptions) []string {
	args := composeArgs(project)
	args = append(args, "logs")

	if opts.Follow {
		args = append(args, "--follow")
	}
	if opts.Tail != "" {
		args = append(args, "--tail", opts.Tail)
	}
	args = append(args, opts.Services...)

	return args
}

// LogsProject transmite los logs del proyecto a los writers indicados.
// A diferencia de run(), no almacena la salida en buffer: la conecta
// directamente para soportar `--follow` (streaming en vivo).
func LogsProject(ctx context.Context, project Project, opts LogsOptions, stdout, stderr io.Writer) error {
	args := LogsArgs(project, opts)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = project.Root
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return cmd.Run()
}

// Logs resuelve el proyecto desde la ruta dada y transmite sus logs.
func Logs(ctx context.Context, projectPath string, opts LogsOptions, stdout, stderr io.Writer) error {
	project, err := ResolveProject(projectPath)
	if err != nil {
		return err
	}

	return LogsProject(ctx, project, opts, stdout, stderr)
}
