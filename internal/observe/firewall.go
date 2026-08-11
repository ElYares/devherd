package observe

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// FirewallRule describe el permiso que necesita el trafico contenedor -> host
// para llegar al collector por una red concreta.
type FirewallRule struct {
	Network string
	Subnet  string
	Gateway string
	Port    string
}

// FirewallRules deriva una regla por red: cada red tiene su propia subred de
// origen y su propio gateway de destino, asi que una sola regla no vale para
// todas. Es justo el error que hace que un proyecto conectado a una red
// distinta a la del proxy no pueda reportar.
func FirewallRules(infos []NetworkInfo, addr string) []FirewallRule {
	_, port := SplitAddr(addr)
	if port == "" {
		_, port = SplitAddr(DefaultAddr)
	}

	rules := make([]FirewallRule, 0, len(infos))
	for _, info := range infos {
		if info.Subnet == "" || info.Gateway == "" {
			continue
		}

		rules = append(rules, FirewallRule{
			Network: info.Name,
			Subnet:  info.Subnet,
			Gateway: info.Gateway,
			Port:    port,
		})
	}

	return rules
}

// UFWArgs son los argumentos de `ufw` para la regla. `ufw allow` es idempotente:
// repetirla no duplica nada, solo informa de que ya existe.
func (rule FirewallRule) UFWArgs() []string {
	return []string{
		"allow", "from", rule.Subnet,
		"to", rule.Gateway,
		"port", rule.Port,
		"proto", "tcp",
		"comment", rule.comment(),
	}
}

// Command devuelve la regla lista para pegar en una shell: el comentario lleva
// espacios y parentesis, asi que va entrecomillado.
func (rule FirewallRule) Command() string {
	return fmt.Sprintf(
		"sudo ufw allow from %s to %s port %s proto tcp comment '%s'",
		rule.Subnet, rule.Gateway, rule.Port, rule.comment(),
	)
}

func (rule FirewallRule) comment() string {
	return "devherd observe collector (" + rule.Network + ")"
}

// UFWEnabled indica si ufw esta activo leyendo su configuracion: `ufw status`
// exige root y romperia el diagnostico sin privilegios.
func UFWEnabled() bool {
	return ufwEnabled()
}

// UFWAvailable indica si el binario existe, para no ofrecer aplicar reglas en
// hosts que usan otro cortafuegos.
func UFWAvailable() bool {
	_, err := exec.LookPath("ufw")
	return err == nil
}

// ApplyFirewallRules ejecuta las reglas con sudo, avisando antes por stderr como
// hace la sincronizacion de /etc/hosts.
func ApplyFirewallRules(ctx context.Context, rules []FirewallRule) error {
	if len(rules) == 0 {
		return nil
	}
	if !UFWAvailable() {
		return fmt.Errorf("ufw is not installed; apply the equivalent rule with your firewall")
	}

	fmt.Fprintf(os.Stderr, "DevHerd necesita permisos de sudo para anadir %d regla(s) de ufw que permitan el trafico contenedor -> collector.\n", len(rules))

	if err := runInteractive(ctx, "sudo", "-v"); err != nil {
		return err
	}

	for _, rule := range rules {
		args := append([]string{"ufw"}, rule.UFWArgs()...)
		if err := runInteractive(ctx, "sudo", args...); err != nil {
			return fmt.Errorf("apply ufw rule for %s: %w", rule.Network, err)
		}
	}

	return nil
}

func runInteractive(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
