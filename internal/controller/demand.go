package controller

import (
	"log"
	"sync"
	"time"

	"github.com/smartplug/smartplug/internal/config"
	"github.com/smartplug/smartplug/internal/hardware"
)

// DemandEvent represents a demand detection event
type DemandEvent struct {
	Timestamp  time.Time
	Duration   time.Duration
	PulseCount int64
	FlowRate   float64
}

// DemandController handles flow-triggered pump activation
type DemandController struct {
	mu sync.RWMutex

	flowMeter *hardware.FlowMeter
	pump      *PumpController
	cfg       *config.FlowMeterConfig

	// State
	demandActive   bool
	demandStart    time.Time
	events         []DemandEvent
	maxEvents      int

	// Callbacks
	callbacks []func(event DemandEvent)

	stopChan chan struct{}
}

// NewDemandController creates a new demand controller
func NewDemandController(
	flowMeter *hardware.FlowMeter,
	pump *PumpController,
	cfg *config.FlowMeterConfig,
) *DemandController {
	return &DemandController{
		flowMeter: flowMeter,
		pump:      pump,
		cfg:       cfg,
		maxEvents: 1000, // Keep last 1000 events
		stopChan:  make(chan struct{}),
	}
}

// Start begins demand detection
func (dc *DemandController) Start() {
	if !dc.cfg.Enabled {
		log.Println("Demand detection disabled")
		return
	}

	// Register for flow events
	dc.flowMeter.OnFlowEvent(dc.onFlowEvent)
	dc.flowMeter.OnDemand(dc.onDemandChange)

	log.Println("Demand controller started")
}

// Stop halts demand detection
func (dc *DemandController) Stop() {
	close(dc.stopChan)
}

// onDemandChange handles demand state changes
func (dc *DemandController) onDemandChange(active bool) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if active && !dc.demandActive {
		dc.demandActive = true
		dc.demandStart = time.Now()
		log.Println("Demand detected - flow started")

		// Trigger pump (pump controller will handle the actual activation)
		// The pump controller is already registered for demand callbacks
	} else if !active && dc.demandActive {
		dc.demandActive = false
		log.Printf("Demand ended - duration: %v", time.Since(dc.demandStart))
	}
}

// onFlowEvent handles flow event completion
func (dc *DemandController) onFlowEvent(event hardware.FlowEvent) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	demandEvent := DemandEvent{
		Timestamp:  event.Timestamp,
		Duration:   event.Duration,
		PulseCount: event.PulseCount,
		FlowRate:   event.FlowRate,
	}

	// Store event
	dc.events = append(dc.events, demandEvent)
	if len(dc.events) > dc.maxEvents {
		dc.events = dc.events[1:]
	}

	// Notify callbacks
	callbacks := dc.callbacks
	dc.mu.Unlock()

	for _, cb := range callbacks {
		cb(demandEvent)
	}

	dc.mu.Lock()

	log.Printf("Flow event: pulses=%d, rate=%.2f L/min, duration=%v",
		event.PulseCount, event.FlowRate, event.Duration)
}

// IsDemandActive returns whether demand is currently detected
func (dc *DemandController) IsDemandActive() bool {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	return dc.demandActive
}

// GetRecentEvents returns recent demand events
func (dc *DemandController) GetRecentEvents(count int) []DemandEvent {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	if count <= 0 || count > len(dc.events) {
		count = len(dc.events)
	}

	start := len(dc.events) - count
	result := make([]DemandEvent, count)
	copy(result, dc.events[start:])

	return result
}

// GetEventCount returns total number of demand events
func (dc *DemandController) GetEventCount() int {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	return len(dc.events)
}

// GetStats returns demand statistics
func (dc *DemandController) GetStats() DemandStats {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	stats := DemandStats{
		TotalEvents:  len(dc.events),
		DemandActive: dc.demandActive,
	}

	if dc.demandActive {
		stats.CurrentDuration = time.Since(dc.demandStart)
	}

	// Calculate averages from recent events
	if len(dc.events) > 0 {
		var totalDuration time.Duration
		var totalFlow float64

		for _, e := range dc.events {
			totalDuration += e.Duration
			totalFlow += e.FlowRate
		}

		stats.AvgDuration = totalDuration / time.Duration(len(dc.events))
		stats.AvgFlowRate = totalFlow / float64(len(dc.events))
	}

	return stats
}

// DemandStats contains demand statistics
type DemandStats struct {
	TotalEvents     int
	DemandActive    bool
	CurrentDuration time.Duration
	AvgDuration     time.Duration
	AvgFlowRate     float64
}

// OnDemandEvent registers a callback for demand events
func (dc *DemandController) OnDemandEvent(callback func(event DemandEvent)) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.callbacks = append(dc.callbacks, callback)
}

// UpdateConfig updates the demand configuration
func (dc *DemandController) UpdateConfig(cfg *config.FlowMeterConfig) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.cfg = cfg
}

// ClearEvents clears the event history
func (dc *DemandController) ClearEvents() {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.events = nil
}
