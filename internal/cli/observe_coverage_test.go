package cli

import (
	"testing"
	"time"
)

func TestShortDurationReadsLikeAPerson(t *testing.T) {
	cases := map[time.Duration]string{
		45 * time.Second: "45s",
		5 * time.Minute:  "5m",
		90 * time.Minute: "1h 30m",
		2 * time.Hour:    "2h",
		25 * time.Hour:   "1d 1h",
		48 * time.Hour:   "2d",
	}
	for d, want := range cases {
		if got := shortDuration(d); got != want {
			t.Errorf("shortDuration(%s) = %q, want %q", d, got, want)
		}
	}
}

// Un hueco que cruza la medianoche sale como "22:34 to 16:34" si el final solo
// lleva hora, y se lee como si fuera hacia atras.
func TestFormatGapRangeShowsTheDateWhenTheGapCrossesDays(t *testing.T) {
	sameDay := formatGapRange(
		time.Date(2026, 8, 26, 10, 0, 0, 0, time.Local),
		time.Date(2026, 8, 26, 14, 0, 0, 0, time.Local),
	)
	if sameDay != "2026-08-26 10:00 to 14:00" {
		t.Errorf("within one day the end needs no date, got %q", sameDay)
	}

	crossing := formatGapRange(
		time.Date(2026, 8, 25, 22, 34, 0, 0, time.Local),
		time.Date(2026, 8, 26, 16, 34, 0, 0, time.Local),
	)
	if crossing != "2026-08-25 22:34 to 2026-08-26 16:34" {
		t.Errorf("across days the end needs its date, got %q", crossing)
	}
}
