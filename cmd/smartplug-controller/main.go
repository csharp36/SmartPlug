// SmartPlug Controller - Receives sensor data via MQTT and controls the pump
// This binary runs on a controller device (e.g., Home Assistant add-on, separate Pi)
// and subscribes to sensor data from remote sensor nodes, then controls the pump
// via MQTT commands to a smart plug or actuator node.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/smartplug/smartplug/internal/api"
	"github.com/smartplug/smartplug/internal/config"
	"github.com/smartplug/smartplug/internal/controller"
	"github.com/smartplug/smartplug/internal/core"
	"github.com/smartplug/smartplug/internal/hardware"
	"github.com/smartplug/smartplug/internal/mqtt"
	"github.com/smartplug/smartplug/internal/remote"
	"github.com/smartplug/smartplug/internal/scheduler"
	"github.com/smartplug/smartplug/internal/web"
)

var (
	version      = "1.0.0"
	configFile   = flag.String("config", "/etc/smartplug/controller.yaml", "Path to configuration file")
	mockMode     = flag.Bool("mock", false, "Run in mock mode (no hardware)")
	broker       = flag.String("broker", "", "MQTT broker URL (overrides config)")
	sensorNodes  = flag.String("sensor-nodes", "", "Comma-separated sensor node IDs (overrides config)")
	actuatorType = flag.String("actuator", "", "Actuator type: 'mqtt' or 'local' (overrides config)")
)

func main() {
	flag.Parse()

	log.Printf("SmartPlug Controller v%s starting...", version)

	// Load configuration
	cfgManager := config.NewManager(*configFile)
	if err := cfgManager.Load(); err != nil {
		log.Printf("Warning: Failed to load config from %s: %v", *configFile, err)
		log.Println("Using default configuration")
	}
	cfg := cfgManager.Get()

	// Override from command line if specified
	if *broker != "" {
		cfg.MQTT.Broker = *broker
		cfg.MQTT.Enabled = true
	}
	if *sensorNodes != "" {
		cfg.Deployment.SensorNodeIDs = strings.Split(*sensorNodes, ",")
	}
	if *actuatorType != "" {
		cfg.Deployment.ActuatorType = *actuatorType
	}

	// Validate configuration
	if !cfg.MQTT.Enabled || cfg.MQTT.Broker == "" {
		log.Fatal("MQTT must be enabled with a valid broker URL")
	}
	if len(cfg.Deployment.SensorNodeIDs) == 0 {
		log.Fatal("sensor_node_ids must be configured (via config file or --sensor-nodes flag)")
	}

	log.Printf("MQTT Broker: %s", cfg.MQTT.Broker)
	log.Printf("Sensor Nodes: %v", cfg.Deployment.SensorNodeIDs)
	log.Printf("Actuator Type: %s", cfg.Deployment.ActuatorType)

	// Create MQTT client
	mqttClient := createMQTTClient(&cfg.MQTT)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("Failed to connect to MQTT: %v", token.Error())
	}
	log.Println("Connected to MQTT broker")

	// Create topics helper
	topics := mqtt.NewTopics(cfg.MQTT.TopicPrefix)

	// Create timeout duration
	timeout := time.Duration(cfg.Deployment.DataTimeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Create remote temperature provider
	tempProvider := remote.NewMQTTTemperatureProvider(
		mqttClient,
		topics,
		cfg.Deployment.SensorNodeIDs,
		timeout,
	)

	// Create remote demand detector
	demandDetector := remote.NewMQTTDemandDetector(
		mqttClient,
		topics,
		cfg.Deployment.SensorNodeIDs,
		timeout,
	)

	// Create pump actuator based on configuration
	var pumpActuator core.PumpActuator
	switch cfg.Deployment.ActuatorType {
	case "mqtt", "":
		log.Println("Using MQTT pump actuator")
		pumpActuator = remote.NewMQTTPumpActuator(mqttClient, topics, 5*time.Second)
	case "local":
		log.Println("Using local GPIO pump actuator")
		var gpio hardware.GPIOController
		if *mockMode {
			gpio = hardware.NewMockGPIO()
		} else {
			gpio = hardware.NewLinuxGPIO()
		}
		relay := hardware.NewRelayController(gpio, cfg.Hardware.RelayGPIO)
		if *mockMode {
			relay.EnableMockMode()
		}
		if err := relay.Initialize(); err != nil {
			log.Fatalf("Failed to initialize relay: %v", err)
		}
		pumpActuator = hardware.NewLocalPumpActuator(relay)
	default:
		log.Fatalf("Unknown actuator_type: %s", cfg.Deployment.ActuatorType)
	}

	// Initialize actuator
	if err := pumpActuator.Initialize(); err != nil {
		log.Fatalf("Failed to initialize pump actuator: %v", err)
	}

	// Start providers
	if err := tempProvider.Start(); err != nil {
		log.Fatalf("Failed to start temperature provider: %v", err)
	}
	log.Println("Temperature provider started")

	if err := demandDetector.Start(); err != nil {
		log.Fatalf("Failed to start demand detector: %v", err)
	}
	log.Println("Demand detector started")

	// Create pump controller
	pumpController := controller.NewPumpController(
		pumpActuator,
		tempProvider,
		demandDetector,
		&cfg.Pump,
	)

	// Initialize scheduler
	sched := scheduler.NewScheduler(&cfg.Schedule, cfg.System.DataDir)
	if err := sched.Load(); err != nil {
		log.Printf("Warning: Failed to load schedules: %v", err)
	}

	sched.SetCallback(func(slot scheduler.TimeSlot) {
		log.Printf("Schedule triggered: %s-%s", slot.Start, slot.End)
		if err := pumpController.TriggerSchedule(); err != nil {
			log.Printf("Failed to trigger pump from schedule: %v", err)
		}
	})

	// Initialize learner
	var learner *scheduler.Learner
	if cfg.Schedule.Learning.Enabled {
		learner = scheduler.NewLearner(sched, &cfg.Schedule.Learning, cfg.System.DataDir)
		pumpController.OnEvent(func(event controller.PumpEvent) {
			learner.RecordEvent(event)
		})
	}

	// Initialize API
	apiHandler := api.NewAPI(
		pumpController,
		tempProvider,
		demandDetector,
		sched,
		learner,
		cfgManager,
	)

	// Initialize web server
	webServer, err := web.NewServer(
		&cfg.Web,
		apiHandler,
		pumpController,
		tempProvider,
		demandDetector,
		sched,
		learner,
	)
	if err != nil {
		log.Fatalf("Failed to create web server: %v", err)
	}

	// Start all components
	pumpController.Start()
	log.Println("Pump controller started")

	sched.Start()
	log.Println("Scheduler started")

	if learner != nil {
		learner.Start()
		log.Println("Learning engine started")
	}

	go func() {
		if err := webServer.Start(); err != nil {
			log.Printf("Web server error: %v", err)
		}
	}()

	log.Printf("SmartPlug controller ready (web UI at %s)", cfg.Web.Address)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down controller...")

	webServer.Stop()

	if learner != nil {
		learner.Stop()
	}

	sched.Stop()
	pumpController.Stop()

	if err := pumpActuator.Close(); err != nil {
		log.Printf("Error closing actuator: %v", err)
	}

	tempProvider.Stop()
	demandDetector.Stop()
	mqttClient.Disconnect(1000)

	log.Println("Controller stopped")
}

// createMQTTClient creates and configures a paho MQTT client.
func createMQTTClient(cfg *config.MQTTConfig) pahomqtt.Client {
	opts := pahomqtt.NewClientOptions()
	opts.AddBroker(cfg.Broker)

	if cfg.ClientID != "" {
		opts.SetClientID(cfg.ClientID)
	} else {
		opts.SetClientID(fmt.Sprintf("smartplug-controller-%d", time.Now().UnixNano()%10000))
	}

	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
		opts.SetPassword(cfg.Password)
	}

	opts.SetAutoReconnect(true)
	opts.SetConnectTimeout(5 * time.Second)

	opts.SetOnConnectHandler(func(c pahomqtt.Client) {
		log.Println("MQTT connected")
	})

	opts.SetConnectionLostHandler(func(c pahomqtt.Client, err error) {
		log.Printf("MQTT connection lost: %v", err)
	})

	return pahomqtt.NewClient(opts)
}
