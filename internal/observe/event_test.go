package observe

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestExtraPayload(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    map[string]any
	}{
		{
			name:    "payload vacio",
			payload: "",
			want:    nil,
		},
		{
			name:    "objeto vacio",
			payload: "{}",
			want:    nil,
		},
		{
			name:    "json ilegible no revienta",
			payload: "{no soy json",
			want:    nil,
		},
		{
			name:    "solo claves con columna propia",
			payload: `{"message":"boom","exception_type":"DemoError","service":"web","culprit":"app.php:1"}`,
			want:    nil,
		},
		{
			name:    "context sobrevive",
			payload: `{"message":"boom","context":{"factura_id":9182}}`,
			want:    map[string]any{"context": map[string]any{"factura_id": float64(9182)}},
		},
		{
			name:    "tags y claves desconocidas sobreviven",
			payload: `{"level":"error","tags":{"modulo":"pagos"},"breadcrumbs":[1,2]}`,
			want: map[string]any{
				"tags":        map[string]any{"modulo": "pagos"},
				"breadcrumbs": []any{float64(1), float64(2)},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtraPayload(tc.payload)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ExtraPayload(%q) = %#v, want %#v", tc.payload, got, tc.want)
			}
		})
	}
}

// El payload crudo se guarda tal cual llega, de modo que ExtraPayload aplicado a
// lo que produce NormalizeEvent debe seguir viendo el contexto del SDK.
func TestExtraPayloadOverNormalizedEvent(t *testing.T) {
	payload := []byte(`{"message":"boom","service":"web","context":{"pago_id":42},"release":"v1"}`)

	event, err := NormalizeEvent("demo", payload)
	if err != nil {
		t.Fatalf("NormalizeEvent returned error: %v", err)
	}

	extra := ExtraPayload(event.RawPayload)
	if len(extra) != 1 {
		t.Fatalf("ExtraPayload = %#v, want solo la clave context", extra)
	}

	encoded, err := json.Marshal(extra["context"])
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if string(encoded) != `{"pago_id":42}` {
		t.Fatalf("context = %s, want {\"pago_id\":42}", encoded)
	}
}
