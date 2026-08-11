package observe

import "testing"

func fingerprintOf(t *testing.T, project, payload string) Event {
	t.Helper()

	event, err := NormalizeEvent(project, []byte(payload))
	if err != nil {
		t.Fatalf("NormalizeEvent returned error: %v", err)
	}

	return event
}

func TestFingerprintMasksVariableData(t *testing.T) {
	// El caso que mas duele: un mismo bug con datos distintos por ocurrencia
	// abria un issue por cada valor.
	cases := []struct {
		name string
		a    string
		b    string
	}{
		{"numbers", "Login failed for user 42", "Login failed for user 4321"},
		{"emails", "No account for juan@example.com", "No account for ana@otra.mx"},
		{"uuids", "Order 550e8400-e29b-41d4-a716-446655440000 not found", "Order 6ba7b810-9dad-11d1-80b4-00c04fd430c8 not found"},
		{"hashes", "Token 9f86d081884c7d65 expired", "Token 2c26b46b68ffc68f expired"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			first := fingerprintOf(t, "demo", `{"message":`+quote(tc.a)+`,"exception_type":"LoginException"}`)
			second := fingerprintOf(t, "demo", `{"message":`+quote(tc.b)+`,"exception_type":"LoginException"}`)

			if first.Fingerprint != second.Fingerprint {
				t.Errorf("messages %q and %q produced different issues", tc.a, tc.b)
			}
		})
	}
}

func TestFingerprintStillSeparatesDifferentFailures(t *testing.T) {
	// El enmascarado no debe fundir errores que son de verdad distintos.
	unknown := fingerprintOf(t, "aang-server", `{"message":"Login rejected: unknown account","exception_type":"LoginUnknownAccountException"}`)
	badPassword := fingerprintOf(t, "aang-server", `{"message":"Login rejected: bad password","exception_type":"LoginBadPasswordException"}`)

	if unknown.Fingerprint == badPassword.Fingerprint {
		t.Error("two different login failures must not share an issue")
	}
}

func TestExplicitFingerprintWins(t *testing.T) {
	// Mensajes distintos, misma clave: un solo issue.
	first := fingerprintOf(t, "demo", `{"message":"algo fallo aqui","fingerprint":"login-unknown-account"}`)
	second := fingerprintOf(t, "demo", `{"message":"otra cosa fallo alla","fingerprint":"login-unknown-account"}`)

	if first.Fingerprint != second.Fingerprint {
		t.Fatal("an explicit fingerprint must group both events")
	}

	// Y no se mezcla con el fingerprint derivado del mensaje.
	derived := fingerprintOf(t, "demo", `{"message":"algo fallo aqui"}`)
	if first.Fingerprint == derived.Fingerprint {
		t.Error("an explicit fingerprint must not collide with the derived one")
	}
}

func TestExplicitFingerprintIsScopedByProject(t *testing.T) {
	first := fingerprintOf(t, "aang-server", `{"message":"x","fingerprint":"login-failed"}`)
	second := fingerprintOf(t, "tl-mas-server", `{"message":"x","fingerprint":"login-failed"}`)

	if first.Fingerprint == second.Fingerprint {
		t.Error("the same key in two projects must not share an issue")
	}
}

func TestExplicitFingerprintAcceptsSDKLists(t *testing.T) {
	// Los SDK tipo Sentry lo mandan como lista.
	list := fingerprintOf(t, "demo", `{"message":"x","fingerprint":["login","unknown-account"]}`)
	joined := fingerprintOf(t, "demo", `{"message":"y","fingerprint":"login|unknown-account"}`)

	if list.Fingerprint != joined.Fingerprint {
		t.Error("a fingerprint list must group like its joined form")
	}
}

func TestEmptyFingerprintFallsBackToTheDerivedOne(t *testing.T) {
	explicit := fingerprintOf(t, "demo", `{"message":"boom","fingerprint":"   "}`)
	derived := fingerprintOf(t, "demo", `{"message":"boom"}`)

	if explicit.Fingerprint != derived.Fingerprint {
		t.Error("a blank fingerprint must not change grouping")
	}
}

func quote(value string) string {
	return `"` + value + `"`
}
