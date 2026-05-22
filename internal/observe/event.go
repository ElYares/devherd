package observe

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Event struct {
	Project       string
	EventID       string
	Timestamp     string
	Level         string
	Platform      string
	Service       string
	Container     string
	ExceptionType string
	Message       string
	Culprit       string
	Transaction   string
	Environment   string
	Release       string
	Title         string
	Fingerprint   string
	RawPayload    string
}

func NormalizeEvent(project string, payload []byte) (Event, error) {
	project = strings.TrimSpace(project)
	if project == "" {
		return Event{}, fmt.Errorf("project is required")
	}

	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return Event{}, fmt.Errorf("parse observe event: %w", err)
	}

	event := Event{
		Project:       project,
		EventID:       stringValue(raw, "event_id"),
		Timestamp:     timestampValue(raw["timestamp"]),
		Level:         firstNonEmpty(stringValue(raw, "level"), "error"),
		Platform:      stringValue(raw, "platform"),
		Service:       firstNonEmpty(stringValue(raw, "service"), tagValue(raw, "service")),
		Container:     firstNonEmpty(stringValue(raw, "container"), tagValue(raw, "container")),
		ExceptionType: stringValue(raw, "exception_type"),
		Message:       firstNonEmpty(stringValue(raw, "message"), logEntryMessage(raw)),
		Culprit:       stringValue(raw, "culprit"),
		Transaction:   stringValue(raw, "transaction"),
		Environment:   firstNonEmpty(stringValue(raw, "environment"), "local"),
		Release:       stringValue(raw, "release"),
	}

	exceptionType, exceptionMessage, exceptionCulprit := exceptionDetails(raw)
	event.ExceptionType = firstNonEmpty(event.ExceptionType, exceptionType)
	event.Message = firstNonEmpty(event.Message, exceptionMessage)
	event.Culprit = firstNonEmpty(event.Culprit, exceptionCulprit)
	event.Title = eventTitle(event)
	event.Fingerprint = Fingerprint(event)

	compacted, err := compactJSON(payload)
	if err != nil {
		return Event{}, err
	}
	event.RawPayload = compacted

	if event.EventID == "" {
		event.EventID = newEventID()
	}
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}

	return event, nil
}

func Fingerprint(event Event) string {
	parts := []string{
		strings.ToLower(strings.TrimSpace(event.Project)),
		strings.ToLower(strings.TrimSpace(event.ExceptionType)),
		normalizeMessage(event.Message),
		strings.ToLower(strings.TrimSpace(event.Culprit)),
		strings.ToLower(strings.TrimSpace(event.Service)),
	}
	sum := sha1.Sum([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

func eventTitle(event Event) string {
	message := strings.TrimSpace(event.Message)
	exceptionType := strings.TrimSpace(event.ExceptionType)

	switch {
	case exceptionType != "" && message != "":
		return exceptionType + ": " + message
	case message != "":
		return message
	case exceptionType != "":
		return exceptionType
	default:
		return "Unknown error"
	}
}

func compactJSON(payload []byte) (string, error) {
	var raw any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return "", fmt.Errorf("parse observe event: %w", err)
	}

	compacted, err := json.Marshal(raw)
	if err != nil {
		return "", fmt.Errorf("encode observe event: %w", err)
	}

	return string(compacted), nil
}

func normalizeMessage(message string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(message)))
	if len(fields) == 0 {
		return ""
	}

	return strings.Join(fields, " ")
}

func newEventID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}

	sum := sha1.Sum([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	return hex.EncodeToString(sum[:])[:32]
}

func stringValue(raw map[string]any, key string) string {
	value, ok := raw[key]
	if !ok {
		return ""
	}

	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func timestampValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		seconds := int64(typed)
		nanos := int64((typed - float64(seconds)) * 1_000_000_000)
		return time.Unix(seconds, nanos).UTC().Format(time.RFC3339Nano)
	default:
		return ""
	}
}

func logEntryMessage(raw map[string]any) string {
	logEntry, ok := raw["logentry"].(map[string]any)
	if !ok {
		return ""
	}

	message, ok := logEntry["message"].(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(message)
}

func exceptionDetails(raw map[string]any) (string, string, string) {
	exception, ok := raw["exception"].(map[string]any)
	if !ok {
		return "", "", ""
	}

	values, ok := exception["values"].([]any)
	if !ok || len(values) == 0 {
		return "", "", ""
	}

	value, ok := values[len(values)-1].(map[string]any)
	if !ok {
		return "", "", ""
	}

	exceptionType := mapString(value, "type")
	message := mapString(value, "value")
	culprit := stackCulprit(value)

	return exceptionType, message, culprit
}

func stackCulprit(exception map[string]any) string {
	stacktrace, ok := exception["stacktrace"].(map[string]any)
	if !ok {
		return ""
	}

	frames, ok := stacktrace["frames"].([]any)
	if !ok || len(frames) == 0 {
		return ""
	}

	frame, ok := frames[len(frames)-1].(map[string]any)
	if !ok {
		return ""
	}

	filename := firstNonEmpty(mapString(frame, "filename"), mapString(frame, "abs_path"), mapString(frame, "module"))
	function := mapString(frame, "function")
	line := mapString(frame, "lineno")

	switch {
	case filename != "" && function != "" && line != "":
		return filename + ":" + line + " " + function
	case filename != "" && line != "":
		return filename + ":" + line
	case filename != "":
		return filename
	case function != "":
		return function
	default:
		return ""
	}
}

func tagValue(raw map[string]any, key string) string {
	tags, ok := raw["tags"]
	if !ok {
		return ""
	}

	switch typed := tags.(type) {
	case map[string]any:
		return mapString(typed, key)
	case []any:
		for _, item := range typed {
			tag, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if mapString(tag, "key") == key {
				return mapString(tag, "value")
			}
		}
	}

	return ""
}

func mapString(raw map[string]any, key string) string {
	value, ok := raw[key]
	if !ok {
		return ""
	}

	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}

	return ""
}
