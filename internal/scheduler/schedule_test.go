package scheduler

import (
	"testing"
	"time"

	"github.com/smartplug/smartplug/internal/config"
)

func TestScheduler_AddSlot(t *testing.T) {
	cfg := &config.ScheduleConfig{Enabled: true}
	s := NewScheduler(cfg, t.TempDir())

	slot := TimeSlot{
		Start:   "06:00",
		End:     "08:00",
		Days:    []int{1, 2, 3, 4, 5},
		Enabled: true,
	}

	if err := s.AddSlot(slot); err != nil {
		t.Fatalf("Failed to add slot: %v", err)
	}

	slots := s.GetSlots()
	if len(slots) != 1 {
		t.Errorf("Expected 1 slot, got %d", len(slots))
	}

	if slots[0].Start != "06:00" {
		t.Errorf("Expected start time '06:00', got '%s'", slots[0].Start)
	}
}

func TestScheduler_AddSlotValidation(t *testing.T) {
	cfg := &config.ScheduleConfig{Enabled: true}
	s := NewScheduler(cfg, t.TempDir())

	// Invalid start time
	slot := TimeSlot{
		Start: "25:00",
		End:   "08:00",
		Days:  []int{1},
	}

	if err := s.AddSlot(slot); err == nil {
		t.Error("Expected error for invalid start time")
	}

	// Invalid end time
	slot.Start = "06:00"
	slot.End = "invalid"

	if err := s.AddSlot(slot); err == nil {
		t.Error("Expected error for invalid end time")
	}
}

func TestScheduler_MaxSlots(t *testing.T) {
	cfg := &config.ScheduleConfig{Enabled: true}
	s := NewScheduler(cfg, t.TempDir())

	// Add 10 slots (maximum)
	for i := 0; i < 10; i++ {
		slot := TimeSlot{
			Start:   "06:00",
			End:     "08:00",
			Days:    []int{1},
			Enabled: true,
		}
		if err := s.AddSlot(slot); err != nil {
			t.Fatalf("Failed to add slot %d: %v", i, err)
		}
	}

	// Try to add 11th slot
	slot := TimeSlot{
		Start:   "09:00",
		End:     "10:00",
		Days:    []int{1},
		Enabled: true,
	}

	if err := s.AddSlot(slot); err == nil {
		t.Error("Expected error when exceeding max slots")
	}
}

func TestScheduler_DeleteSlot(t *testing.T) {
	cfg := &config.ScheduleConfig{Enabled: true}
	s := NewScheduler(cfg, t.TempDir())

	slot := TimeSlot{
		ID:      "test-slot",
		Start:   "06:00",
		End:     "08:00",
		Days:    []int{1},
		Enabled: true,
	}

	s.AddSlot(slot)

	if err := s.DeleteSlot("test-slot"); err != nil {
		t.Fatalf("Failed to delete slot: %v", err)
	}

	if len(s.GetSlots()) != 0 {
		t.Error("Expected 0 slots after delete")
	}

	// Delete nonexistent
	if err := s.DeleteSlot("nonexistent"); err == nil {
		t.Error("Expected error deleting nonexistent slot")
	}
}

func TestScheduler_EnableDisable(t *testing.T) {
	cfg := &config.ScheduleConfig{Enabled: false}
	s := NewScheduler(cfg, t.TempDir())

	if s.IsEnabled() {
		t.Error("Expected scheduler to be disabled initially")
	}

	s.Enable()
	if !s.IsEnabled() {
		t.Error("Expected scheduler to be enabled after Enable()")
	}

	s.Disable()
	if s.IsEnabled() {
		t.Error("Expected scheduler to be disabled after Disable()")
	}
}

func TestScheduler_EnableDisableSlot(t *testing.T) {
	cfg := &config.ScheduleConfig{Enabled: true}
	s := NewScheduler(cfg, t.TempDir())

	slot := TimeSlot{
		ID:      "test-slot",
		Start:   "06:00",
		End:     "08:00",
		Days:    []int{1},
		Enabled: true,
	}

	s.AddSlot(slot)

	// Disable slot
	if err := s.DisableSlot("test-slot"); err != nil {
		t.Fatalf("Failed to disable slot: %v", err)
	}

	slots := s.GetSlots()
	if slots[0].Enabled {
		t.Error("Expected slot to be disabled")
	}

	// Enable slot
	if err := s.EnableSlot("test-slot"); err != nil {
		t.Fatalf("Failed to enable slot: %v", err)
	}

	slots = s.GetSlots()
	if !slots[0].Enabled {
		t.Error("Expected slot to be enabled")
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		input   string
		minutes int
		err     bool
	}{
		{"00:00", 0, false},
		{"06:30", 390, false},
		{"12:00", 720, false},
		{"23:59", 1439, false},
		{"24:00", 0, true},
		{"12:60", 0, true},
		{"invalid", 0, true},
		{"12", 0, true},
	}

	for _, test := range tests {
		minutes, err := parseTime(test.input)

		if test.err && err == nil {
			t.Errorf("parseTime(%s): expected error", test.input)
		}

		if !test.err && err != nil {
			t.Errorf("parseTime(%s): unexpected error: %v", test.input, err)
		}

		if !test.err && minutes != test.minutes {
			t.Errorf("parseTime(%s) = %d, expected %d", test.input, minutes, test.minutes)
		}
	}
}

func TestScheduler_IsTimeInSlot(t *testing.T) {
	cfg := &config.ScheduleConfig{Enabled: true}
	s := NewScheduler(cfg, t.TempDir())

	// Normal slot (06:00 - 08:00)
	slot := &TimeSlot{
		Start: "06:00",
		End:   "08:00",
	}

	// 07:00 should be in slot
	testTime := time.Date(2024, 1, 1, 7, 0, 0, 0, time.Local)
	if !s.isTimeInSlot(testTime, slot) {
		t.Error("Expected 07:00 to be in slot 06:00-08:00")
	}

	// 05:00 should not be in slot
	testTime = time.Date(2024, 1, 1, 5, 0, 0, 0, time.Local)
	if s.isTimeInSlot(testTime, slot) {
		t.Error("Expected 05:00 to NOT be in slot 06:00-08:00")
	}

	// 08:00 should not be in slot (end exclusive)
	testTime = time.Date(2024, 1, 1, 8, 0, 0, 0, time.Local)
	if s.isTimeInSlot(testTime, slot) {
		t.Error("Expected 08:00 to NOT be in slot 06:00-08:00")
	}
}

func TestScheduler_OvernightSlot(t *testing.T) {
	cfg := &config.ScheduleConfig{Enabled: true}
	s := NewScheduler(cfg, t.TempDir())

	// Overnight slot (23:00 - 01:00)
	slot := &TimeSlot{
		Start: "23:00",
		End:   "01:00",
	}

	// 23:30 should be in slot
	testTime := time.Date(2024, 1, 1, 23, 30, 0, 0, time.Local)
	if !s.isTimeInSlot(testTime, slot) {
		t.Error("Expected 23:30 to be in overnight slot 23:00-01:00")
	}

	// 00:30 should be in slot
	testTime = time.Date(2024, 1, 2, 0, 30, 0, 0, time.Local)
	if !s.isTimeInSlot(testTime, slot) {
		t.Error("Expected 00:30 to be in overnight slot 23:00-01:00")
	}

	// 02:00 should not be in slot
	testTime = time.Date(2024, 1, 2, 2, 0, 0, 0, time.Local)
	if s.isTimeInSlot(testTime, slot) {
		t.Error("Expected 02:00 to NOT be in overnight slot 23:00-01:00")
	}
}

func TestScheduler_ManualVsLearned(t *testing.T) {
	cfg := &config.ScheduleConfig{Enabled: true}
	s := NewScheduler(cfg, t.TempDir())

	// Add manual slot
	s.AddSlot(TimeSlot{
		Start:   "06:00",
		End:     "08:00",
		Days:    []int{1},
		Enabled: true,
		Source:  "manual",
	})

	// Add learned slots
	s.AddLearnedSlots([]TimeSlot{
		{
			Start:   "07:00",
			End:     "08:00",
			Days:    []int{1, 2, 3},
			Enabled: true,
		},
	})

	manualSlots := s.GetManualSlots()
	if len(manualSlots) != 1 {
		t.Errorf("Expected 1 manual slot, got %d", len(manualSlots))
	}

	learnedSlots := s.GetLearnedSlots()
	if len(learnedSlots) != 1 {
		t.Errorf("Expected 1 learned slot, got %d", len(learnedSlots))
	}

	allSlots := s.GetSlots()
	if len(allSlots) != 2 {
		t.Errorf("Expected 2 total slots, got %d", len(allSlots))
	}
}
