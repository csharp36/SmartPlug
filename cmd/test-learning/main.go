// Test script to simulate usage events and verify learning
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/smartplug/smartplug/internal/config"
	"github.com/smartplug/smartplug/internal/controller"
	"github.com/smartplug/smartplug/internal/scheduler"
)

func main() {
	fmt.Println("=== SmartPlug Learning System Test ===")
	fmt.Println()

	// Create temp directory for test data
	tmpDir, err := os.MkdirTemp("", "smartplug-test")
	if err != nil {
		fmt.Printf("Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	// Create scheduler and learner
	schedCfg := &config.ScheduleConfig{Enabled: true}
	sched := scheduler.NewScheduler(schedCfg, tmpDir)

	learnCfg := &config.LearningConfig{
		Enabled:              true,
		MinDays:              7,
		VacationTimeoutHours: 24,
	}
	learner := scheduler.NewLearner(sched, learnCfg, tmpDir)

	// Simulate 30 days of usage data (need enough for patterns to emerge)
	fmt.Println("Simulating 30 days of hot water usage...")
	fmt.Println()

	now := time.Now()
	eventsAdded := 0

	for day := 0; day < 30; day++ {
		date := now.AddDate(0, 0, -day)
		weekday := date.Weekday()

		// Morning routine: 6:30-7:30 AM on weekdays
		if weekday >= time.Monday && weekday <= time.Friday {
			// Simulate shower at ~7am
			eventTime := time.Date(date.Year(), date.Month(), date.Day(), 7, 0, 0, 0, date.Location())
			learner.RecordEvent(controller.PumpEvent{
				Timestamp:    eventTime,
				Trigger:      controller.TriggerDemand,
				Duration:     8 * time.Minute,
				HotTemp:      120,
				ReturnTemp:   95,
				Differential: 25,
			})
			eventsAdded++

			// Simulate evening usage at ~6pm
			eventTime = time.Date(date.Year(), date.Month(), date.Day(), 18, 15, 0, 0, date.Location())
			learner.RecordEvent(controller.PumpEvent{
				Timestamp:    eventTime,
				Trigger:      controller.TriggerDemand,
				Duration:     5 * time.Minute,
				HotTemp:      118,
				ReturnTemp:   92,
				Differential: 26,
			})
			eventsAdded++
		}

		// Weekend: Later morning at ~9am
		if weekday == time.Saturday || weekday == time.Sunday {
			eventTime := time.Date(date.Year(), date.Month(), date.Day(), 9, 30, 0, 0, date.Location())
			learner.RecordEvent(controller.PumpEvent{
				Timestamp:    eventTime,
				Trigger:      controller.TriggerDemand,
				Duration:     12 * time.Minute,
				HotTemp:      122,
				ReturnTemp:   90,
				Differential: 32,
			})
			eventsAdded++
		}
	}

	fmt.Printf("Added %d simulated events\n", eventsAdded)
	fmt.Println()

	// Check stats
	stats := learner.GetStats()
	fmt.Println("=== Learning Statistics ===")
	fmt.Printf("Total Events:    %d\n", stats.TotalEvents)
	fmt.Printf("Days of Data:    %d\n", stats.DaysOfData)
	fmt.Printf("Patterns Found:  %d\n", stats.PatternCount)
	fmt.Printf("Vacation Mode:   %v\n", stats.VacationMode)
	fmt.Println()

	// Get detected patterns
	patterns := learner.GetPatterns()
	fmt.Println("=== Detected Patterns ===")
	if len(patterns) == 0 {
		fmt.Println("No patterns detected yet")
	}
	for i, p := range patterns {
		dayNames := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
		var days []string
		for _, d := range p.Days {
			days = append(days, dayNames[d])
		}
		fmt.Printf("%d. %02d:00-%02d:00 on %v (count: %d, score: %.2f)\n",
			i+1, p.StartHour, p.EndHour, days, p.Count, p.Score)
	}
	fmt.Println()

	// Get generated schedule slots
	slots := sched.GetLearnedSlots()
	fmt.Println("=== Generated Schedule Slots ===")
	if len(slots) == 0 {
		fmt.Println("No schedule slots generated")
	}
	for _, slot := range slots {
		dayNames := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
		var days []string
		for _, d := range slot.Days {
			days = append(days, dayNames[d])
		}
		fmt.Printf("- %s to %s on %v (enabled: %v)\n",
			slot.Start, slot.End, days, slot.Enabled)
	}
	fmt.Println()

	// Show raw events sample
	events := learner.GetEvents()
	fmt.Println("=== Sample Events (first 5) ===")
	for i, e := range events {
		if i >= 5 {
			break
		}
		dayName := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}[e.Day]
		fmt.Printf("- %s %02d:%02d (trigger: %s, duration: %v)\n",
			dayName, e.Hour, e.Minute, e.Trigger, e.Duration)
	}

	// Save and show the JSON
	if err := learner.Save(); err != nil {
		fmt.Printf("Failed to save: %v\n", err)
	}

	data, _ := os.ReadFile(tmpDir + "/usage_history.json")
	var prettyJSON map[string]interface{}
	json.Unmarshal(data, &prettyJSON)

	fmt.Println()
	fmt.Println("=== Saved Usage History (truncated) ===")
	fmt.Printf("File: %s/usage_history.json\n", tmpDir)
	fmt.Printf("Events saved: %d\n", len(events))
}
