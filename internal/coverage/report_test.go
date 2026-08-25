package coverage

import (
	"strings"
	"testing"
)

// El total se pondera por unidades. Promediar los porcentajes por archivo daria
// 70% donde la realidad es 40,1%, y es el error mas facil de cometer.
func TestPercentIsWeightedByUnitsNotAveraged(t *testing.T) {
	report := Report{Unit: UnitLines, Files: []FileReport{
		{Path: "tiny.go", Total: 3, Covered: 3},
		{Path: "huge.go", Total: 800, Covered: 320},
	}}

	got := report.Percent()
	if got < 40.2 || got > 40.3 {
		t.Fatalf("expected ~40.2%% weighted, got %.2f%%", got)
	}

	average := (report.Files[0].Percent() + report.Files[1].Percent()) / 2
	if average < 69 || average > 71 {
		t.Fatalf("sanity check: the naive average should be ~70%%, got %.2f%%", average)
	}
}

// Ordenar por masa y no por porcentaje es lo que dice donde trabajar.
func TestByUncoveredRanksByMassNotPercentage(t *testing.T) {
	report := Report{Unit: UnitLines, Files: []FileReport{
		{Path: "tiny.go", Total: 3, Covered: 0},     // 0%, 3 sin cubrir
		{Path: "huge.go", Total: 800, Covered: 320}, // 40%, 480 sin cubrir
	}}

	ranked := report.ByUncovered()
	if ranked[0].Path != "huge.go" {
		t.Fatalf("expected the file with the most uncovered mass first, got %q", ranked[0].Path)
	}
}

// El desempate por ruta evita que la salida cambie entre corridas.
func TestByUncoveredBreaksTiesByPath(t *testing.T) {
	report := Report{Unit: UnitLines, Files: []FileReport{
		{Path: "zebra.go", Total: 10, Covered: 5},
		{Path: "alpha.go", Total: 10, Covered: 5},
	}}

	ranked := report.ByUncovered()
	if ranked[0].Path != "alpha.go" || ranked[1].Path != "zebra.go" {
		t.Fatalf("expected a stable path tiebreak, got %q then %q", ranked[0].Path, ranked[1].Path)
	}
}

func TestGroupsAggregateByDirectory(t *testing.T) {
	report := Report{Unit: UnitStatements, Files: []FileReport{
		{Path: "internal/cli/up.go", Total: 100, Covered: 10},
		{Path: "internal/cli/down.go", Total: 100, Covered: 30},
		{Path: "internal/observe/store.go", Total: 200, Covered: 160},
	}}

	groups := report.Groups()
	if len(groups) != 2 {
		t.Fatalf("expected 2 directories, got %d: %#v", len(groups), groups)
	}
	if groups[0].Name != "internal/cli" || groups[0].Total != 200 || groups[0].Covered != 40 {
		t.Fatalf("unexpected first group: %#v", groups[0])
	}
	if groups[0].Files != 2 {
		t.Fatalf("expected 2 files in internal/cli, got %d", groups[0].Files)
	}
	if groups[1].Percent() != 80 {
		t.Fatalf("expected internal/observe at 80%%, got %.1f%%", groups[1].Percent())
	}
}

// El bulto va arriba. En orden alfabetico, un proyecto real con 38 directorios
// obliga a leerlos todos para encontrar donde esta el trabajo.
func TestGroupsRankByUncoveredMass(t *testing.T) {
	report := Report{Unit: UnitStatements, Files: []FileReport{
		// Alfabeticamente iria primero, y no tiene nada sin cubrir.
		{Path: "app/Actions/Companies.php", Total: 23, Covered: 23},
		{Path: "app/Policies/UserPolicy.php", Total: 236, Covered: 105},
		{Path: "app/Livewire/Forms/ToolForm.php", Total: 962, Covered: 421},
	}}

	groups := report.Groups()
	if groups[0].Name != "app/Livewire/Forms" {
		t.Fatalf("expected the largest uncovered mass first, got %q", groups[0].Name)
	}
	if groups[1].Name != "app/Policies" {
		t.Fatalf("expected the second largest next, got %q", groups[1].Name)
	}
	// Lo que esta al 100% se hunde al final: ya no hay nada que hacer ahi.
	if groups[2].Name != "app/Actions" {
		t.Fatalf("expected the fully covered directory last, got %q", groups[2].Name)
	}
}

// Sin desempate estable la salida cambiaria entre corridas por el recorrido del mapa.
func TestGroupsBreakTiesByName(t *testing.T) {
	report := Report{Unit: UnitLines, Files: []FileReport{
		{Path: "zebra/a.go", Total: 10, Covered: 5},
		{Path: "alpha/a.go", Total: 10, Covered: 5},
	}}

	groups := report.Groups()
	if groups[0].Name != "alpha" || groups[1].Name != "zebra" {
		t.Fatalf("expected a stable name tiebreak, got %q then %q", groups[0].Name, groups[1].Name)
	}
}

// Este es el guardarrail central: sentencias y lineas no se suman nunca.
func TestMergeRefusesToMixUnits(t *testing.T) {
	statements := Report{Format: "go", Unit: UnitStatements, Files: []FileReport{{Path: "a.go", Total: 10, Covered: 5}}}
	lines := Report{Format: "lcov", Unit: UnitLines, Files: []FileReport{{Path: "a.ts", Total: 10, Covered: 5}}}

	_, err := statements.Merge(lines)
	if err == nil {
		t.Fatal("merging statements with lines must fail; the resulting number would be meaningless")
	}
	for _, want := range []string{"different units", "statements", "lines"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the error should explain the mismatch; %q missing from %q", want, err)
		}
	}
}

func TestMergeCombinesReportsOfTheSameUnit(t *testing.T) {
	first := Report{Format: "lcov", Unit: UnitLines, Files: []FileReport{
		{Path: "a.ts", Total: 10, Covered: 4},
		{Path: "b.ts", Total: 5, Covered: 5},
	}}
	second := Report{Format: "lcov", Unit: UnitLines, Files: []FileReport{
		{Path: "a.ts", Total: 10, Covered: 7},
		{Path: "c.ts", Total: 2, Covered: 0},
	}}

	merged, err := first.Merge(second)
	if err != nil {
		t.Fatalf("Merge returned error: %v", err)
	}
	if len(merged.Files) != 3 {
		t.Fatalf("expected 3 distinct files, got %#v", merged.Files)
	}

	// El mismo archivo medido dos veces se queda con la mejor medicion, no con la
	// suma: sumar inventaria 20 lineas donde solo hay 10.
	for _, file := range merged.Files {
		if file.Path != "a.ts" {
			continue
		}
		if file.Total != 10 || file.Covered != 7 {
			t.Fatalf("expected a.ts at 7/10, got %d/%d", file.Covered, file.Total)
		}
	}
}

func TestMergeAcceptsAnEmptyOperand(t *testing.T) {
	report := Report{Format: "go", Unit: UnitStatements, Files: []FileReport{{Path: "a.go", Total: 4, Covered: 2}}}

	merged, err := Report{}.Merge(report)
	if err != nil {
		t.Fatalf("merging into an empty report should work: %v", err)
	}
	if merged.Total() != 4 {
		t.Fatalf("expected the non-empty operand to survive, got %d", merged.Total())
	}
}

func TestIsEmptyDistinguishesNoDataFromNothingCovered(t *testing.T) {
	noData := Report{Unit: UnitLines}
	nothingCovered := Report{Unit: UnitLines, Files: []FileReport{{Path: "a.ts", Total: 10, Covered: 0}}}

	if !noData.IsEmpty() {
		t.Fatal("a report without files is empty")
	}
	if nothingCovered.IsEmpty() {
		t.Fatal("a report with 10 uncovered lines has data; it is not empty")
	}
	if nothingCovered.Percent() != 0 {
		t.Fatalf("expected 0%%, got %.1f%%", nothingCovered.Percent())
	}
}

func TestPercentOfAnEmptyReportDoesNotDivideByZero(t *testing.T) {
	if got := (Report{}).Percent(); got != 0 {
		t.Fatalf("expected 0 for an empty report, got %v", got)
	}
	if got := (FileReport{}).Percent(); got != 0 {
		t.Fatalf("expected 0 for an empty file, got %v", got)
	}
}
