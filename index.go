package auditlog

// ReportIndex precomputes lookup maps over a [WorkflowReport] for repeated
// O(1) queries. Build it once from a final report, then query cheaply — useful
// when a consumer inspects the same report many times (e.g. in a UI or analysis
// loop).
//
// The index shares the report's backing slices; it is read-only and must not
// outlive mutations to the underlying report. Rebuild the index (call
// [NewReportIndex]) if the report changes.
type ReportIndex struct {
	steps        []StepInfo
	byName       map[string]int // step name → index into steps
	byID         map[int]int    // step id → index into steps
	eventsByName map[string][]Event
	eventsByType map[EventType][]Event
}

// NewReportIndex builds an O(1) lookup index over the given report. The report
// is not retained beyond its slices; callers may discard the WorkflowReport
// value but must not mutate its Steps or Events in place afterward.
func NewReportIndex(r WorkflowReport) *ReportIndex {
	idx := &ReportIndex{
		steps:        r.Steps,
		byName:       make(map[string]int, len(r.Steps)),
		byID:         make(map[int]int, len(r.Steps)),
		eventsByName: make(map[string][]Event),
		eventsByType: make(map[EventType][]Event),
	}

	for i := range r.Steps {
		idx.byName[r.Steps[i].Name] = i

		if r.Steps[i].StepID != 0 {
			idx.byID[r.Steps[i].StepID] = i
		}
	}

	for _, e := range r.Events {
		idx.eventsByName[e.Name] = append(idx.eventsByName[e.Name], e)
		idx.eventsByType[e.EventType] = append(idx.eventsByType[e.EventType], e)
	}

	return idx
}

// StepByName returns a pointer to the first step with the given name, or nil.
// O(1).
func (idx *ReportIndex) StepByName(name string) *StepInfo {
	if i, ok := idx.byName[name]; ok {
		return &idx.steps[i]
	}

	return nil
}

// StepByID returns a pointer to the step with the given [StepInfo.StepID], or
// nil. O(1).
func (idx *ReportIndex) StepByID(id int) *StepInfo {
	if i, ok := idx.byID[id]; ok {
		return &idx.steps[i]
	}

	return nil
}

// EventsByStep returns all events for the given step name (nil if none). O(1).
func (idx *ReportIndex) EventsByStep(name string) []Event {
	return idx.eventsByName[name]
}

// EventsByType returns all events matching the given type (nil if none). O(1).
func (idx *ReportIndex) EventsByType(t EventType) []Event {
	return idx.eventsByType[t]
}
