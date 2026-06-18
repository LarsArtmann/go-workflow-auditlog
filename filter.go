package auditlog

import (
	"cmp"
	"slices"
	"time"
)

// ReportOption configures a filter for WorkflowReport.Filtered.
type ReportOption func(*reportFilter)

type reportFilter struct {
	stepNames  map[string]struct{}
	statuses   map[StepStatus]struct{}
	eventTypes map[EventType]struct{}
	timeFrom   *time.Time
	timeTo     *time.Time
}

// WithStepsByName filters to only steps matching any of the given names.
func WithStepsByName(names ...string) ReportOption {
	return func(f *reportFilter) {
		if f.stepNames == nil {
			f.stepNames = make(map[string]struct{})
		}

		for _, name := range names {
			f.stepNames[name] = struct{}{}
		}
	}
}

// WithStepsByStatus filters to only steps matching any of the given statuses.
func WithStepsByStatus(statuses ...StepStatus) ReportOption {
	return func(f *reportFilter) {
		if f.statuses == nil {
			f.statuses = make(map[StepStatus]struct{})
		}

		for _, s := range statuses {
			f.statuses[s] = struct{}{}
		}
	}
}

// WithEventsByType filters to only events matching the given type.
func WithEventsByType(eventType EventType) ReportOption {
	return func(f *reportFilter) {
		if f.eventTypes == nil {
			f.eventTypes = make(map[EventType]struct{})
		}

		f.eventTypes[eventType] = struct{}{}
	}
}

// WithTimeRange filters events to those within the given time range (inclusive).
func WithTimeRange(from, to time.Time) ReportOption {
	return func(f *reportFilter) {
		f.timeFrom = &from
		f.timeTo = &to
	}
}

// Filtered returns a new report containing only the steps and events that
// match all of the given filter options. Aggregate counts are recomputed.
//
// With no options, returns a copy of the report.
func (r WorkflowReport) Filtered(opts ...ReportOption) WorkflowReport {
	filter := reportFilter{}

	for _, opt := range opts {
		opt(&filter)
	}

	var filteredSteps []StepInfo

	for _, step := range r.Steps {
		if !filter.matchStep(step) {
			continue
		}

		filteredSteps = append(filteredSteps, step)
	}

	var filteredEvents []Event

	for _, evt := range r.Events {
		if !filter.matchEvent(evt) {
			continue
		}

		filteredEvents = append(filteredEvents, evt)
	}

	// Also filter events to only those referencing filtered steps.
	if len(filter.stepNames) > 0 || len(filter.statuses) > 0 {
		filteredEvents = filterEventsToSteps(filteredEvents, filteredSteps)
	}

	slices.SortFunc(filteredSteps, func(a, b StepInfo) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return buildReportFromCore(
		r.Version,
		r.WorkflowID,
		time.Now(),
		r.DroppedEventCount,
		filteredEvents,
		filteredSteps,
	)
}

func (f *reportFilter) matchStep(step StepInfo) bool {
	if len(f.stepNames) > 0 {
		if _, ok := f.stepNames[step.Name]; !ok {
			return false
		}
	}

	if len(f.statuses) > 0 {
		if _, ok := f.statuses[step.Status]; !ok {
			return false
		}
	}

	return true
}

func (f *reportFilter) matchEvent(evt Event) bool {
	if len(f.eventTypes) > 0 {
		if _, ok := f.eventTypes[evt.EventType]; !ok {
			return false
		}
	}

	if f.timeFrom != nil && evt.Timestamp.Before(*f.timeFrom) {
		return false
	}

	if f.timeTo != nil && evt.Timestamp.After(*f.timeTo) {
		return false
	}

	return true
}

// filterEventsToSteps keeps only events whose step name appears in the
// filtered steps list.
func filterEventsToSteps(events []Event, steps []StepInfo) []Event {
	names := make(map[string]struct{}, len(steps))
	for _, s := range steps {
		names[s.Name] = struct{}{}
	}

	var result []Event

	for _, evt := range events {
		if _, ok := names[evt.Name]; ok {
			result = append(result, evt)
		}
	}

	return result
}

// ReportFiltered returns a filtered report. Convenience method on Auditor.
func (a *Auditor) ReportFiltered(opts ...ReportOption) WorkflowReport {
	return a.Report().Filtered(opts...)
}
