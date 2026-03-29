// Package scheduler implements manual and learned scheduling
package scheduler

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/smartplug/smartplug/internal/config"
)

// TimeSlot represents a scheduled time window
type TimeSlot struct {
	ID      string    `json:"id"`
	Start   string    `json:"start"`   // HH:MM format
	End     string    `json:"end"`     // HH:MM format
	Days    []int     `json:"days"`    // 0=Sunday, 1=Monday, etc.
	Enabled bool      `json:"enabled"`
	Source  string    `json:"source"`  // "manual" or "learned"
}

// ScheduleCallback is called when a schedule triggers
type ScheduleCallback func(slot TimeSlot)

// Scheduler manages time-based pump activation
type Scheduler struct {
	mu sync.RWMutex

	slots      []TimeSlot
	enabled    bool
	callback   ScheduleCallback
	dataFile   string

	// Active slot tracking
	activeSlot *TimeSlot
	inWindow   bool

	stopChan chan struct{}
}

// NewScheduler creates a new scheduler
func NewScheduler(cfg *config.ScheduleConfig, dataDir string) *Scheduler {
	s := &Scheduler{
		enabled:  cfg.Enabled,
		dataFile: filepath.Join(dataDir, "schedules.json"),
		stopChan: make(chan struct{}),
	}

	// Convert config slots to TimeSlot
	for i, slot := range cfg.Slots {
		s.slots = append(s.slots, TimeSlot{
			ID:      fmt.Sprintf("manual-%d", i),
			Start:   slot.Start,
			End:     slot.End,
			Days:    slot.Days,
			Enabled: slot.Enabled,
			Source:  "manual",
		})
	}

	return s
}

// Load loads schedules from disk
func (s *Scheduler) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.dataFile)
	if os.IsNotExist(err) {
		return nil // No saved schedules
	}
	if err != nil {
		return fmt.Errorf("failed to read schedules: %w", err)
	}

	var slots []TimeSlot
	if err := json.Unmarshal(data, &slots); err != nil {
		return fmt.Errorf("failed to parse schedules: %w", err)
	}

	s.slots = slots
	return nil
}

// Save persists schedules to disk
func (s *Scheduler) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Ensure directory exists
	dir := filepath.Dir(s.dataFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	data, err := json.MarshalIndent(s.slots, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schedules: %w", err)
	}

	return os.WriteFile(s.dataFile, data, 0644)
}

// Start begins schedule monitoring
func (s *Scheduler) Start() {
	go s.monitorLoop()
	log.Println("Scheduler started")
}

// Stop halts schedule monitoring
func (s *Scheduler) Stop() {
	close(s.stopChan)
}

// SetCallback sets the callback for schedule triggers
func (s *Scheduler) SetCallback(callback ScheduleCallback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callback = callback
}

// Enable enables the scheduler
func (s *Scheduler) Enable() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = true
}

// Disable disables the scheduler
func (s *Scheduler) Disable() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = false
}

// IsEnabled returns whether the scheduler is enabled
func (s *Scheduler) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

// GetSlots returns all schedule slots
func (s *Scheduler) GetSlots() []TimeSlot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]TimeSlot, len(s.slots))
	copy(result, s.slots)
	return result
}

// AddSlot adds a new schedule slot
func (s *Scheduler) AddSlot(slot TimeSlot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.slots) >= 10 {
		return fmt.Errorf("maximum of 10 schedule slots allowed")
	}

	// Validate time format
	if _, err := parseTime(slot.Start); err != nil {
		return fmt.Errorf("invalid start time: %w", err)
	}
	if _, err := parseTime(slot.End); err != nil {
		return fmt.Errorf("invalid end time: %w", err)
	}

	// Generate ID if not set
	if slot.ID == "" {
		slot.ID = fmt.Sprintf("slot-%d", time.Now().UnixNano())
	}

	if slot.Source == "" {
		slot.Source = "manual"
	}

	s.slots = append(s.slots, slot)
	return nil
}

// UpdateSlot updates an existing slot
func (s *Scheduler) UpdateSlot(id string, slot TimeSlot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, existing := range s.slots {
		if existing.ID == id {
			slot.ID = id
			s.slots[i] = slot
			return nil
		}
	}

	return fmt.Errorf("slot not found: %s", id)
}

// DeleteSlot removes a schedule slot
func (s *Scheduler) DeleteSlot(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, slot := range s.slots {
		if slot.ID == id {
			s.slots = append(s.slots[:i], s.slots[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("slot not found: %s", id)
}

// EnableSlot enables a specific slot
func (s *Scheduler) EnableSlot(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, slot := range s.slots {
		if slot.ID == id {
			s.slots[i].Enabled = true
			return nil
		}
	}

	return fmt.Errorf("slot not found: %s", id)
}

// DisableSlot disables a specific slot
func (s *Scheduler) DisableSlot(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, slot := range s.slots {
		if slot.ID == id {
			s.slots[i].Enabled = false
			return nil
		}
	}

	return fmt.Errorf("slot not found: %s", id)
}

// GetActiveSlot returns the currently active slot, if any
func (s *Scheduler) GetActiveSlot() *TimeSlot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeSlot
}

// IsInScheduledWindow returns whether we're in a scheduled window
func (s *Scheduler) IsInScheduledWindow() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.inWindow
}

// monitorLoop checks schedules periodically
func (s *Scheduler) monitorLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Initial check
	s.checkSchedules()

	for {
		select {
		case <-ticker.C:
			s.checkSchedules()

		case <-s.stopChan:
			return
		}
	}
}

// checkSchedules evaluates current time against all slots
func (s *Scheduler) checkSchedules() {
	s.mu.Lock()

	if !s.enabled {
		s.mu.Unlock()
		return
	}

	now := time.Now()
	currentDay := int(now.Weekday())

	var activeSlot *TimeSlot
	var inWindow bool

	for i := range s.slots {
		slot := &s.slots[i]
		if !slot.Enabled {
			continue
		}

		// Check if current day is in slot's days
		dayMatch := false
		for _, day := range slot.Days {
			if day == currentDay {
				dayMatch = true
				break
			}
		}
		if !dayMatch {
			continue
		}

		// Check if current time is within slot
		if s.isTimeInSlot(now, slot) {
			activeSlot = slot
			inWindow = true
			break
		}
	}

	wasInWindow := s.inWindow
	s.inWindow = inWindow
	s.activeSlot = activeSlot
	callback := s.callback
	s.mu.Unlock()

	// Trigger callback on window entry
	if inWindow && !wasInWindow && callback != nil && activeSlot != nil {
		log.Printf("Entering scheduled window: %s-%s", activeSlot.Start, activeSlot.End)
		callback(*activeSlot)
	}
}

// isTimeInSlot checks if a time falls within a slot
func (s *Scheduler) isTimeInSlot(t time.Time, slot *TimeSlot) bool {
	start, err := parseTime(slot.Start)
	if err != nil {
		return false
	}
	end, err := parseTime(slot.End)
	if err != nil {
		return false
	}

	currentMinutes := t.Hour()*60 + t.Minute()

	// Handle overnight slots (e.g., 23:00 - 01:00)
	if end < start {
		return currentMinutes >= start || currentMinutes < end
	}

	return currentMinutes >= start && currentMinutes < end
}

// parseTime parses HH:MM format to minutes since midnight
func parseTime(s string) (int, error) {
	var hour, minute int
	n, err := fmt.Sscanf(s, "%d:%d", &hour, &minute)
	if err != nil || n != 2 {
		return 0, fmt.Errorf("invalid time format: %s", s)
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, fmt.Errorf("time out of range: %s", s)
	}
	return hour*60 + minute, nil
}

// AddLearnedSlots adds slots from the learning algorithm
func (s *Scheduler) AddLearnedSlots(slots []TimeSlot) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove existing learned slots
	filtered := make([]TimeSlot, 0, len(s.slots))
	for _, slot := range s.slots {
		if slot.Source != "learned" {
			filtered = append(filtered, slot)
		}
	}
	s.slots = filtered

	// Add new learned slots
	for _, slot := range slots {
		slot.Source = "learned"
		s.slots = append(s.slots, slot)
	}
}

// GetManualSlots returns only manual slots
func (s *Scheduler) GetManualSlots() []TimeSlot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []TimeSlot
	for _, slot := range s.slots {
		if slot.Source == "manual" {
			result = append(result, slot)
		}
	}
	return result
}

// GetLearnedSlots returns only learned slots
func (s *Scheduler) GetLearnedSlots() []TimeSlot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []TimeSlot
	for _, slot := range s.slots {
		if slot.Source == "learned" {
			result = append(result, slot)
		}
	}
	return result
}
