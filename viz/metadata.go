package viz

// TypeMetadata provides display metadata (icons, labels, colors) for all enum
// types used in the HTML visualization. It is injected into the template as JSON
// so that JavaScript reads from a single Go-authoritative source instead of
// maintaining parallel hardcoded constants.
type TypeMetadata struct {
	Statuses map[string]StatusMeta `json:"statuses"`
	Events   map[string]EventMeta  `json:"events"`
}

// StatusMeta holds display info for a StepStatus.
type StatusMeta struct {
	Icon  string `json:"icon"`
	Label string `json:"label"`
}

// EventMeta holds display info for an EventType.
type EventMeta struct {
	Label string `json:"label"`
	Color string `json:"color"`
}

// BuildTypeMetadata constructs display metadata from the Go enum constants.
// This is the single source of truth — the HTML template's JavaScript reads
// from the injected JSON rather than maintaining parallel constant definitions.
func BuildTypeMetadata() TypeMetadata {
	statuses := make(map[string]StatusMeta, len(AllStepStatuses()))
	for _, status := range AllStepStatuses() {
		statuses[string(status)] = StatusMeta{
			Icon:  status.Icon(),
			Label: status.Label(),
		}
	}

	events := make(map[string]EventMeta, len(AllEventTypes()))
	for _, evt := range AllEventTypes() {
		events[string(evt)] = EventMeta{
			Label: evt.Label(),
			Color: evt.Color(),
		}
	}

	return TypeMetadata{
		Statuses: statuses,
		Events:   events,
	}
}
