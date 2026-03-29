package scheduler

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/smartplug/smartplug/internal/config"
	"github.com/smartplug/smartplug/internal/controller"
)

// UsageEvent represents a recorded pump usage event
type UsageEvent struct {
	Timestamp time.Time            `json:"timestamp"`
	Day       int                  `json:"day"`       // 0=Sunday
	Hour      int                  `json:"hour"`
	Minute    int                  `json:"minute"`
	Trigger   controller.TriggerSource `json:"trigger"`
	Duration  time.Duration        `json:"duration"`
}

// UsagePattern represents a detected usage pattern
type UsagePattern struct {
	Days      []int   `json:"days"`       // Days of week
	StartHour int     `json:"start_hour"` // Hour of day
	EndHour   int     `json:"end_hour"`   // Hour of day
	Count     int     `json:"count"`      // Number of events in pattern
	Score     float64 `json:"score"`      // Confidence score
}

// Learner implements adaptive schedule learning
type Learner struct {
	mu sync.RWMutex

	events       []UsageEvent
	patterns     []UsagePattern
	scheduler    *Scheduler
	cfg          *config.LearningConfig

	dataFile     string
	lastActivity time.Time
	vacationMode bool

	stopChan chan struct{}
}

// NewLearner creates a new schedule learner
func NewLearner(scheduler *Scheduler, cfg *config.LearningConfig, dataDir string) *Learner {
	return &Learner{
		scheduler:    scheduler,
		cfg:          cfg,
		dataFile:     filepath.Join(dataDir, "usage_history.json"),
		lastActivity: time.Now(),
		stopChan:     make(chan struct{}),
	}
}

// Start begins the learning system
func (l *Learner) Start() {
	if !l.cfg.Enabled {
		log.Println("Learning disabled")
		return
	}

	// Load existing data
	if err := l.Load(); err != nil {
		log.Printf("Failed to load usage history: %v", err)
	}

	go l.monitorLoop()
	log.Println("Learning system started")
}

// Stop halts the learning system
func (l *Learner) Stop() {
	close(l.stopChan)

	// Save data
	if err := l.Save(); err != nil {
		log.Printf("Failed to save usage history: %v", err)
	}
}

// Load loads usage history from disk
func (l *Learner) Load() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := os.ReadFile(l.dataFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read usage history: %w", err)
	}

	var history struct {
		Events       []UsageEvent   `json:"events"`
		LastActivity time.Time      `json:"last_activity"`
	}

	if err := json.Unmarshal(data, &history); err != nil {
		return fmt.Errorf("failed to parse usage history: %w", err)
	}

	l.events = history.Events
	l.lastActivity = history.LastActivity

	return nil
}

// Save persists usage history to disk
func (l *Learner) Save() error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	dir := filepath.Dir(l.dataFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	history := struct {
		Events       []UsageEvent   `json:"events"`
		LastActivity time.Time      `json:"last_activity"`
	}{
		Events:       l.events,
		LastActivity: l.lastActivity,
	}

	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal usage history: %w", err)
	}

	return os.WriteFile(l.dataFile, data, 0644)
}

// RecordEvent records a pump usage event
func (l *Learner) RecordEvent(event controller.PumpEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()

	usageEvent := UsageEvent{
		Timestamp: event.Timestamp,
		Day:       int(event.Timestamp.Weekday()),
		Hour:      event.Timestamp.Hour(),
		Minute:    event.Timestamp.Minute(),
		Trigger:   event.Trigger,
		Duration:  event.Duration,
	}

	l.events = append(l.events, usageEvent)
	l.lastActivity = time.Now()
	l.vacationMode = false

	// Keep last 30 days of data
	cutoff := time.Now().AddDate(0, 0, -30)
	filtered := make([]UsageEvent, 0, len(l.events))
	for _, e := range l.events {
		if e.Timestamp.After(cutoff) {
			filtered = append(filtered, e)
		}
	}
	l.events = filtered

	// Check if we should regenerate patterns
	l.checkPatternGeneration()
}

// GetEvents returns all recorded events
func (l *Learner) GetEvents() []UsageEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]UsageEvent, len(l.events))
	copy(result, l.events)
	return result
}

// GetPatterns returns detected patterns
func (l *Learner) GetPatterns() []UsagePattern {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]UsagePattern, len(l.patterns))
	copy(result, l.patterns)
	return result
}

// GetStats returns learning statistics
func (l *Learner) GetStats() LearnerStats {
	l.mu.RLock()
	defer l.mu.RUnlock()

	stats := LearnerStats{
		TotalEvents:    len(l.events),
		PatternCount:   len(l.patterns),
		LastActivity:   l.lastActivity,
		VacationMode:   l.vacationMode,
		DaysOfData:     l.getDaysOfData(),
	}

	if len(l.events) > 0 {
		stats.FirstEvent = l.events[0].Timestamp
		stats.LatestEvent = l.events[len(l.events)-1].Timestamp
	}

	return stats
}

// LearnerStats contains learning statistics
type LearnerStats struct {
	TotalEvents  int
	PatternCount int
	FirstEvent   time.Time
	LatestEvent  time.Time
	LastActivity time.Time
	VacationMode bool
	DaysOfData   int
}

// IsVacationMode returns whether vacation mode is active
func (l *Learner) IsVacationMode() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.vacationMode
}

// monitorLoop runs periodic checks
func (l *Learner) monitorLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.checkVacationMode()
			l.checkPatternGeneration()

		case <-l.stopChan:
			return
		}
	}
}

// checkVacationMode checks for extended inactivity
func (l *Learner) checkVacationMode() {
	l.mu.Lock()
	defer l.mu.Unlock()

	timeout := time.Duration(l.cfg.VacationTimeoutHours) * time.Hour

	if time.Since(l.lastActivity) > timeout && !l.vacationMode {
		l.vacationMode = true
		log.Println("Vacation mode activated - no activity detected")

		// Disable learned schedules
		l.scheduler.mu.Lock()
		for i := range l.scheduler.slots {
			if l.scheduler.slots[i].Source == "learned" {
				l.scheduler.slots[i].Enabled = false
			}
		}
		l.scheduler.mu.Unlock()
	}
}

// checkPatternGeneration checks if patterns should be regenerated
func (l *Learner) checkPatternGeneration() {
	daysOfData := l.getDaysOfData()

	if daysOfData >= l.cfg.MinDays {
		l.generatePatterns()
	}
}

// getDaysOfData returns the number of days with recorded events
func (l *Learner) getDaysOfData() int {
	if len(l.events) == 0 {
		return 0
	}

	days := make(map[string]bool)
	for _, e := range l.events {
		day := e.Timestamp.Format("2006-01-02")
		days[day] = true
	}

	return len(days)
}

// generatePatterns analyzes usage and generates schedule patterns
func (l *Learner) generatePatterns() {
	// Group events by hour (across all days)
	type hourBucket struct {
		hour  int
		count int
		days  map[int]int // day -> count for that day
	}

	buckets := make(map[int]*hourBucket)

	for _, e := range l.events {
		if buckets[e.Hour] == nil {
			buckets[e.Hour] = &hourBucket{
				hour: e.Hour,
				days: make(map[int]int),
			}
		}
		buckets[e.Hour].count++
		buckets[e.Hour].days[e.Day]++
	}

	// Convert to slice and sort by total count
	var sortedBuckets []*hourBucket
	for _, b := range buckets {
		sortedBuckets = append(sortedBuckets, b)
	}
	sort.Slice(sortedBuckets, func(i, j int) bool {
		return sortedBuckets[i].count > sortedBuckets[j].count
	})

	// Generate patterns from top buckets
	daysOfData := l.getDaysOfData()
	// Require at least 3 events at this hour, or 1/4 of days (whichever is higher)
	minCount := daysOfData / 4
	if minCount < 3 {
		minCount = 3
	}

	var patterns []UsagePattern

	for _, b := range sortedBuckets {
		if b.count < minCount {
			continue
		}

		// Collect days that have this hour pattern
		var days []int
		for day := range b.days {
			days = append(days, day)
		}
		sort.Ints(days)

		pattern := UsagePattern{
			Days:      days,
			StartHour: b.hour,
			EndHour:   b.hour + 1,
			Count:     b.count,
			Score:     float64(b.count) / float64(daysOfData),
		}

		patterns = append(patterns, pattern)

		if len(patterns) >= 5 {
			break // Max 5 learned patterns
		}
	}

	l.patterns = patterns

	// Convert patterns to schedule slots
	var slots []TimeSlot
	for i, p := range patterns {
		slot := TimeSlot{
			ID:      fmt.Sprintf("learned-%d", i),
			Start:   fmt.Sprintf("%02d:00", p.StartHour),
			End:     fmt.Sprintf("%02d:00", p.EndHour),
			Days:    p.Days,
			Enabled: true,
			Source:  "learned",
		}
		slots = append(slots, slot)
	}

	l.scheduler.AddLearnedSlots(slots)

	if len(patterns) > 0 {
		log.Printf("Generated %d learned schedule patterns", len(patterns))
	}
}

// ClearHistory clears all usage history
func (l *Learner) ClearHistory() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.events = nil
	l.patterns = nil
	l.scheduler.AddLearnedSlots(nil) // Remove learned slots
}

// ForceRelearn triggers immediate pattern regeneration
func (l *Learner) ForceRelearn() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.generatePatterns()
}
