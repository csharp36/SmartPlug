// Package web provides the web server and UI
package web

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/smartplug/smartplug/internal/api"
	"github.com/smartplug/smartplug/internal/config"
	"github.com/smartplug/smartplug/internal/controller"
	"github.com/smartplug/smartplug/internal/core"
	"github.com/smartplug/smartplug/internal/scheduler"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

// Server is the web server
type Server struct {
	cfg       *config.WebConfig
	api       *api.API
	pump      *controller.PumpController
	temps     core.TemperatureProvider
	demand    core.DemandDetector
	scheduler *scheduler.Scheduler
	learner   *scheduler.Learner
	templates *template.Template
	server    *http.Server
}

// Template functions
var templateFuncs = template.FuncMap{
	"dayName": func(day int) string {
		days := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
		if day >= 0 && day < len(days) {
			return days[day]
		}
		return ""
	},
}

// NewServer creates a new web server using interface-based dependencies.
func NewServer(
	cfg *config.WebConfig,
	apiHandler *api.API,
	pump *controller.PumpController,
	temps core.TemperatureProvider,
	demand core.DemandDetector,
	sched *scheduler.Scheduler,
	learner *scheduler.Learner,
) (*Server, error) {
	// Parse templates with custom functions
	tmpl, err := template.New("").Funcs(templateFuncs).ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	return &Server{
		cfg:       cfg,
		api:       apiHandler,
		pump:      pump,
		temps:     temps,
		demand:    demand,
		scheduler: sched,
		learner:   learner,
		templates: tmpl,
	}, nil
}

// Start starts the web server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Static files
	staticHandler := http.FileServer(http.FS(staticFS))
	mux.Handle("/static/", staticHandler)

	// API routes
	s.api.RegisterRoutes(mux)

	// Web UI routes
	mux.HandleFunc("/", s.handleDashboard)
	mux.HandleFunc("/schedule", s.handleSchedulePage)
	mux.HandleFunc("/settings", s.handleSettingsPage)

	s.server = &http.Server{
		Addr:         s.cfg.Address,
		Handler:      s.loggingMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Web server starting on %s", s.cfg.Address)

	if s.cfg.HTTPSEnabled {
		return s.server.ListenAndServeTLS(s.cfg.CertFile, s.cfg.KeyFile)
	}

	return s.server.ListenAndServe()
}

// Stop stops the web server
func (s *Server) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

// loggingMiddleware logs HTTP requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
	})
}

// DashboardData contains data for the dashboard template
type DashboardData struct {
	PumpState         string
	PumpRunning       bool
	PumpEnabled       bool
	HotTemperature    float64
	ReturnTemperature float64
	Differential      float64
	HotValid          bool
	ReturnValid       bool
	FlowActive        bool
	FlowEnabled       bool
	InSchedule        bool
	ScheduleEnabled   bool
	LastTrigger       string
	RuntimeSeconds    float64
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	pumpStatus := s.pump.GetStatus()
	hot, ret := s.temps.GetCurrentReadings()

	data := DashboardData{
		PumpState:         pumpStatus.State.String(),
		PumpRunning:       pumpStatus.IsRunning,
		PumpEnabled:       pumpStatus.Enabled,
		HotTemperature:    hot.Temperature,
		ReturnTemperature: ret.Temperature,
		Differential:      hot.Temperature - ret.Temperature,
		HotValid:          hot.Valid,
		ReturnValid:       ret.Valid,
		InSchedule:        s.scheduler.IsInScheduledWindow(),
		ScheduleEnabled:   s.scheduler.IsEnabled(),
		LastTrigger:       pumpStatus.LastTrigger.String(),
		RuntimeSeconds:    pumpStatus.Runtime.Seconds(),
	}

	if s.demand != nil {
		data.FlowEnabled = true
		data.FlowActive = s.demand.IsFlowActive()
	}

	if err := s.templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// ScheduleData contains data for the schedule template
type ScheduleData struct {
	Enabled         bool
	Slots           []scheduler.TimeSlot
	LearnedSlots    []scheduler.TimeSlot
	LearningEnabled bool
	LearningStats   *scheduler.LearnerStats
}

func (s *Server) handleSchedulePage(w http.ResponseWriter, r *http.Request) {
	data := ScheduleData{
		Enabled:      s.scheduler.IsEnabled(),
		Slots:        s.scheduler.GetManualSlots(),
		LearnedSlots: s.scheduler.GetLearnedSlots(),
	}

	if s.learner != nil {
		data.LearningEnabled = true
		stats := s.learner.GetStats()
		data.LearningStats = &stats
	}

	if err := s.templates.ExecuteTemplate(w, "schedule.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// SettingsData contains data for the settings template
type SettingsData struct {
	Config            config.Config
	DiscoveredSensors []string
}

func (s *Server) handleSettingsPage(w http.ResponseWriter, r *http.Request) {
	var sensors []string

	// Try to discover sensors if the temperature provider supports it
	if discoverer, ok := s.temps.(core.SensorDiscoverer); ok {
		sensors, _ = discoverer.DiscoverSensors()
	}

	data := SettingsData{
		// Config will be loaded from manager
		DiscoveredSensors: sensors,
	}

	if err := s.templates.ExecuteTemplate(w, "settings.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
