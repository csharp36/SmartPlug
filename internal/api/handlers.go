// Package api provides REST API handlers
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/smartplug/smartplug/internal/config"
	"github.com/smartplug/smartplug/internal/controller"
	"github.com/smartplug/smartplug/internal/core"
	"github.com/smartplug/smartplug/internal/hardware"
	"github.com/smartplug/smartplug/internal/scheduler"
)

// API handles REST API requests
type API struct {
	pump      *controller.PumpController
	temps     core.TemperatureProvider
	demand    core.DemandDetector
	scheduler *scheduler.Scheduler
	learner   *scheduler.Learner
	config    *config.Manager
}

// NewAPI creates a new API handler using interface-based dependencies.
func NewAPI(
	pump *controller.PumpController,
	temps core.TemperatureProvider,
	demand core.DemandDetector,
	sched *scheduler.Scheduler,
	learner *scheduler.Learner,
	cfg *config.Manager,
) *API {
	return &API{
		pump:      pump,
		temps:     temps,
		demand:    demand,
		scheduler: sched,
		learner:   learner,
		config:    cfg,
	}
}

// RegisterRoutes registers API routes on the given mux
func (a *API) RegisterRoutes(mux *http.ServeMux) {
	// Status endpoints
	mux.HandleFunc("/api/status", a.handleStatus)
	mux.HandleFunc("/api/temperatures", a.handleTemperatures)

	// Pump control endpoints
	mux.HandleFunc("/api/pump/state", a.handlePumpState)
	mux.HandleFunc("/api/pump/heat-now", a.handleHeatNow)
	mux.HandleFunc("/api/pump/stop", a.handlePumpStop)
	mux.HandleFunc("/api/pump/enable", a.handlePumpEnable)
	mux.HandleFunc("/api/pump/disable", a.handlePumpDisable)

	// Schedule endpoints
	mux.HandleFunc("/api/schedule", a.handleSchedule)
	mux.HandleFunc("/api/schedule/slots", a.handleScheduleSlots)

	// Learning endpoints
	mux.HandleFunc("/api/learning/stats", a.handleLearningStats)
	mux.HandleFunc("/api/learning/patterns", a.handleLearningPatterns)
	mux.HandleFunc("/api/learning/clear", a.handleLearningClear)

	// Config endpoints
	mux.HandleFunc("/api/config", a.handleConfig)
	mux.HandleFunc("/api/sensors/discover", a.handleSensorDiscover)
}

// StatusResponse contains full system status
type StatusResponse struct {
	Pump         PumpStatus        `json:"pump"`
	Temperatures TemperatureStatus `json:"temperatures"`
	Flow         FlowStatus        `json:"flow"`
	Schedule     ScheduleStatus    `json:"schedule"`
	Timestamp    time.Time         `json:"timestamp"`
}

type PumpStatus struct {
	State             string  `json:"state"`
	IsRunning         bool    `json:"is_running"`
	Enabled           bool    `json:"enabled"`
	LastTrigger       string  `json:"last_trigger"`
	RuntimeSeconds    float64 `json:"runtime_seconds,omitempty"`
	CooldownRemaining float64 `json:"cooldown_remaining,omitempty"`
}

type TemperatureStatus struct {
	HotOutlet    float64 `json:"hot_outlet"`
	ReturnLine   float64 `json:"return_line"`
	Differential float64 `json:"differential"`
	HotValid     bool    `json:"hot_valid"`
	ReturnValid  bool    `json:"return_valid"`
}

type FlowStatus struct {
	Active     bool  `json:"active"`
	PulseCount int64 `json:"pulse_count"`
	Enabled    bool  `json:"enabled"`
}

type ScheduleStatus struct {
	Enabled    bool   `json:"enabled"`
	InWindow   bool   `json:"in_window"`
	ActiveSlot string `json:"active_slot,omitempty"`
	SlotCount  int    `json:"slot_count"`
}

func (a *API) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pumpStatus := a.pump.GetStatus()
	hot, ret := a.temps.GetCurrentReadings()

	response := StatusResponse{
		Pump: PumpStatus{
			State:       pumpStatus.State.String(),
			IsRunning:   pumpStatus.IsRunning,
			Enabled:     pumpStatus.Enabled,
			LastTrigger: pumpStatus.LastTrigger.String(),
		},
		Temperatures: TemperatureStatus{
			HotOutlet:    hot.Temperature,
			ReturnLine:   ret.Temperature,
			Differential: hot.Temperature - ret.Temperature,
			HotValid:     hot.Valid,
			ReturnValid:  ret.Valid,
		},
		Schedule: ScheduleStatus{
			Enabled:   a.scheduler.IsEnabled(),
			InWindow:  a.scheduler.IsInScheduledWindow(),
			SlotCount: len(a.scheduler.GetSlots()),
		},
		Timestamp: time.Now(),
	}

	if pumpStatus.Runtime > 0 {
		response.Pump.RuntimeSeconds = pumpStatus.Runtime.Seconds()
	}
	if pumpStatus.CooldownRemaining > 0 {
		response.Pump.CooldownRemaining = pumpStatus.CooldownRemaining.Seconds()
	}

	if activeSlot := a.scheduler.GetActiveSlot(); activeSlot != nil {
		response.Schedule.ActiveSlot = activeSlot.ID
	}

	if a.demand != nil {
		response.Flow = FlowStatus{
			Active:  a.demand.IsFlowActive(),
			Enabled: true,
		}

		// Try to get pulse count if the demand detector supports it
		if statsProvider, ok := a.demand.(core.FlowStatsProvider); ok {
			stats := statsProvider.GetStats()
			response.Flow.PulseCount = stats.PulseCount
		}
	}

	jsonResponse(w, response)
}

func (a *API) handleTemperatures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hot, ret := a.temps.GetCurrentReadings()

	response := TemperatureStatus{
		HotOutlet:    hot.Temperature,
		ReturnLine:   ret.Temperature,
		Differential: hot.Temperature - ret.Temperature,
		HotValid:     hot.Valid,
		ReturnValid:  ret.Valid,
	}

	jsonResponse(w, response)
}

func (a *API) handlePumpState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := a.pump.GetStatus()

	response := PumpStatus{
		State:       status.State.String(),
		IsRunning:   status.IsRunning,
		Enabled:     status.Enabled,
		LastTrigger: status.LastTrigger.String(),
	}

	if status.Runtime > 0 {
		response.RuntimeSeconds = status.Runtime.Seconds()
	}
	if status.CooldownRemaining > 0 {
		response.CooldownRemaining = status.CooldownRemaining.Seconds()
	}

	jsonResponse(w, response)
}

func (a *API) handleHeatNow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := a.pump.HeatNow(); err != nil {
		log.Printf("Heat now failed: %v", err)
		jsonError(w, err.Error(), http.StatusConflict)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok", "message": "Pump activated"})
}

func (a *API) handlePumpStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := a.pump.StopManual(); err != nil {
		log.Printf("Pump stop failed: %v", err)
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok", "message": "Pump stopped"})
}

func (a *API) handlePumpEnable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	a.pump.Enable()
	jsonResponse(w, map[string]string{"status": "ok", "message": "Pump enabled"})
}

func (a *API) handlePumpDisable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	a.pump.Disable()
	jsonResponse(w, map[string]string{"status": "ok", "message": "Pump disabled"})
}

func (a *API) handleSchedule(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		response := struct {
			Enabled  bool                 `json:"enabled"`
			InWindow bool                 `json:"in_window"`
			Slots    []scheduler.TimeSlot `json:"slots"`
		}{
			Enabled:  a.scheduler.IsEnabled(),
			InWindow: a.scheduler.IsInScheduledWindow(),
			Slots:    a.scheduler.GetSlots(),
		}
		jsonResponse(w, response)

	case http.MethodPost:
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Enabled {
			a.scheduler.Enable()
		} else {
			a.scheduler.Disable()
		}

		jsonResponse(w, map[string]string{"status": "ok"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) handleScheduleSlots(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		jsonResponse(w, a.scheduler.GetSlots())

	case http.MethodPost:
		var slot scheduler.TimeSlot
		if err := json.NewDecoder(r.Body).Decode(&slot); err != nil {
			jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if err := a.scheduler.AddSlot(slot); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := a.scheduler.Save(); err != nil {
			log.Printf("Failed to save schedules: %v", err)
		}

		jsonResponse(w, map[string]string{"status": "ok", "message": "Slot added"})

	case http.MethodDelete:
		slotID := r.URL.Query().Get("id")
		if slotID == "" {
			jsonError(w, "Missing slot ID", http.StatusBadRequest)
			return
		}

		if err := a.scheduler.DeleteSlot(slotID); err != nil {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}

		if err := a.scheduler.Save(); err != nil {
			log.Printf("Failed to save schedules: %v", err)
		}

		jsonResponse(w, map[string]string{"status": "ok", "message": "Slot deleted"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) handleLearningStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if a.learner == nil {
		jsonError(w, "Learning not enabled", http.StatusNotFound)
		return
	}

	stats := a.learner.GetStats()
	jsonResponse(w, stats)
}

func (a *API) handleLearningPatterns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if a.learner == nil {
		jsonError(w, "Learning not enabled", http.StatusNotFound)
		return
	}

	patterns := a.learner.GetPatterns()
	jsonResponse(w, patterns)
}

func (a *API) handleLearningClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if a.learner == nil {
		jsonError(w, "Learning not enabled", http.StatusNotFound)
		return
	}

	a.learner.ClearHistory()
	jsonResponse(w, map[string]string{"status": "ok", "message": "Learning history cleared"})
}

func (a *API) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := a.config.Get()
		jsonResponse(w, cfg)

	case http.MethodPatch:
		var updates map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Apply updates to config
		err := a.config.Update(func(cfg *config.Config) {
			// Handle pump threshold updates
			if pump, ok := updates["pump"].(map[string]interface{}); ok {
				if v, ok := pump["start_threshold"].(float64); ok {
					cfg.Pump.StartThreshold = v
				}
				if v, ok := pump["stop_threshold"].(float64); ok {
					cfg.Pump.StopThreshold = v
				}
				if v, ok := pump["max_runtime_minutes"].(float64); ok {
					cfg.Pump.MaxRuntimeMinutes = int(v)
				}
				if v, ok := pump["cooldown_minutes"].(float64); ok {
					cfg.Pump.CooldownMinutes = int(v)
				}
			}

			// Handle sensor assignment updates
			if sensors, ok := updates["sensors"].(map[string]interface{}); ok {
				if v, ok := sensors["hot_outlet_id"].(string); ok {
					cfg.Sensors.HotOutletID = v
				}
				if v, ok := sensors["return_line_id"].(string); ok {
					cfg.Sensors.ReturnLineID = v
				}
			}

			// Handle flow meter updates
			if flowmeter, ok := updates["flowmeter"].(map[string]interface{}); ok {
				if v, ok := flowmeter["enabled"].(bool); ok {
					cfg.FlowMeter.Enabled = v
				}
				if v, ok := flowmeter["pulses_per_liter"].(float64); ok {
					cfg.FlowMeter.PulsesPerLiter = int(v)
				}
				if v, ok := flowmeter["trigger_threshold"].(float64); ok {
					cfg.FlowMeter.TriggerThreshold = int(v)
				}
				if v, ok := flowmeter["demand_timeout"].(float64); ok {
					cfg.FlowMeter.DemandTimeout = int(v)
				}
			}

			// Handle MQTT updates
			if mqtt, ok := updates["mqtt"].(map[string]interface{}); ok {
				if v, ok := mqtt["enabled"].(bool); ok {
					cfg.MQTT.Enabled = v
				}
				if v, ok := mqtt["broker"].(string); ok {
					cfg.MQTT.Broker = v
				}
				if v, ok := mqtt["username"].(string); ok {
					cfg.MQTT.Username = v
				}
				if v, ok := mqtt["password"].(string); ok {
					cfg.MQTT.Password = v
				}
				if v, ok := mqtt["ha_discovery"].(bool); ok {
					cfg.MQTT.HADiscovery = v
				}
			}
		})

		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := a.config.Save(); err != nil {
			log.Printf("Failed to save config: %v", err)
		}

		jsonResponse(w, map[string]string{"status": "ok"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) handleSensorDiscover(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Try to use sensor discovery if available
		discoverer, ok := a.temps.(core.SensorDiscoverer)
		if !ok {
			// No discovery available in this mode
			jsonResponse(w, struct {
				Sensors      []string `json:"sensors"`
				HotOutletID  string   `json:"hot_outlet_id"`
				ReturnLineID string   `json:"return_line_id"`
			}{
				Sensors: []string{},
			})
			return
		}

		sensors, err := discoverer.DiscoverSensors()
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		hotID, retID := discoverer.GetSensorIDs()

		response := struct {
			Sensors      []string `json:"sensors"`
			HotOutletID  string   `json:"hot_outlet_id"`
			ReturnLineID string   `json:"return_line_id"`
		}{
			Sensors:      sensors,
			HotOutletID:  hotID,
			ReturnLineID: retID,
		}

		jsonResponse(w, response)

	case http.MethodPost:
		var req struct {
			HotOutletID  string `json:"hot_outlet_id"`
			ReturnLineID string `json:"return_line_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Try to update sensor IDs if discovery is available
		discoverer, ok := a.temps.(core.SensorDiscoverer)
		if !ok {
			jsonError(w, "Sensor configuration not available in this mode", http.StatusBadRequest)
			return
		}

		// Update sensor manager
		discoverer.SetSensorIDs(req.HotOutletID, req.ReturnLineID)

		// Persist to config
		err := a.config.Update(func(cfg *config.Config) {
			cfg.Sensors.HotOutletID = req.HotOutletID
			cfg.Sensors.ReturnLineID = req.ReturnLineID
		})
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := a.config.Save(); err != nil {
			log.Printf("Failed to save config: %v", err)
		}

		jsonResponse(w, map[string]string{"status": "ok", "message": "Sensors assigned"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// GetTemperatureProvider returns the temperature provider for use by other components.
func (a *API) GetTemperatureProvider() core.TemperatureProvider {
	return a.temps
}

// GetDemandDetector returns the demand detector for use by other components.
func (a *API) GetDemandDetector() core.DemandDetector {
	return a.demand
}

// GetLocalFlowMeter returns the underlying flow meter if using local hardware.
// Returns nil if not available (e.g., in controller mode with remote sensors).
func (a *API) GetLocalFlowMeter() *hardware.FlowMeter {
	if detector, ok := a.demand.(*hardware.LocalDemandDetector); ok {
		return detector.Underlying()
	}
	return nil
}

// jsonResponse writes a JSON response
func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// jsonError writes a JSON error response
func jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
