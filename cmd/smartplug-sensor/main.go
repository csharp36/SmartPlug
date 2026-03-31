// SmartPlug Sensor Node - Publishes sensor data over MQTT
// This is a lightweight binary for running on a sensor-only Raspberry Pi.
// It reads from DS18B20 temperature sensors and an optional flow meter,
// then publishes the data to an MQTT broker for consumption by a controller.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/smartplug/smartplug/internal/config"
	"github.com/smartplug/smartplug/internal/hardware"
	"github.com/smartplug/smartplug/internal/mqtt"
	"github.com/smartplug/smartplug/internal/remote"
)

var (
	version    = "1.0.0"
	configFile = flag.String("config", "/etc/smartplug/sensor.yaml", "Path to configuration file")
	mockMode   = flag.Bool("mock", false, "Run in mock mode (no hardware)")
	nodeID     = flag.String("node-id", "", "Node ID (overrides config)")
	broker     = flag.String("broker", "", "MQTT broker URL (overrides config)")
)

func main() {
	flag.Parse()

	log.Printf("SmartPlug Sensor Node v%s starting...", version)

	// Load configuration
	cfgManager := config.NewManager(*configFile)
	if err := cfgManager.Load(); err != nil {
		log.Printf("Warning: Failed to load config from %s: %v", *configFile, err)
		log.Println("Using default configuration")
	}
	cfg := cfgManager.Get()

	// Override from command line if specified
	if *nodeID != "" {
		cfg.Deployment.NodeID = *nodeID
	}
	if *broker != "" {
		cfg.MQTT.Broker = *broker
		cfg.MQTT.Enabled = true
	}

	// Validate configuration
	if cfg.Deployment.NodeID == "" {
		log.Fatal("node_id must be configured (via config file or --node-id flag)")
	}
	if !cfg.MQTT.Enabled || cfg.MQTT.Broker == "" {
		log.Fatal("MQTT must be enabled with a valid broker URL")
	}

	log.Printf("Node ID: %s", cfg.Deployment.NodeID)
	log.Printf("MQTT Broker: %s", cfg.MQTT.Broker)

	// Initialize GPIO
	var gpio hardware.GPIOController
	if *mockMode {
		log.Println("Running in mock mode")
		gpio = hardware.NewMockGPIO()
	} else {
		gpio = hardware.NewLinuxGPIO()
	}

	// Initialize sensors
	sensors := hardware.NewSensorManager(
		cfg.Sensors.HotOutletID,
		cfg.Sensors.ReturnLineID,
		cfg.Sensors.PollInterval,
	)

	var flowMeter *hardware.FlowMeter
	if cfg.FlowMeter.Enabled {
		flowMeter = hardware.NewFlowMeter(
			gpio,
			cfg.Hardware.FlowMeterGPIO,
			cfg.FlowMeter.PulsesPerLiter,
			cfg.FlowMeter.TriggerThreshold,
			cfg.FlowMeter.DebounceMs,
			cfg.FlowMeter.DemandTimeout,
		)
	}

	// Enable mock mode
	if *mockMode {
		sensors.EnableMockMode(120.0, 95.0)
		if flowMeter != nil {
			flowMeter.EnableMockMode()
		}
	}

	// Start sensors
	if err := sensors.Start(); err != nil {
		log.Fatalf("Failed to start sensors: %v", err)
	}
	log.Println("Sensors started")

	if flowMeter != nil {
		if err := flowMeter.Start(); err != nil {
			log.Printf("Warning: Failed to start flow meter: %v", err)
		} else {
			log.Println("Flow meter started")
		}
	}

	// Create MQTT client
	mqttClient := createMQTTClient(&cfg.MQTT, cfg.Deployment.NodeID)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("Failed to connect to MQTT: %v", token.Error())
	}
	log.Println("Connected to MQTT broker")

	// Create topics helper
	topics := mqtt.NewTopics(cfg.MQTT.TopicPrefix)

	// Create and start sensor publisher
	publisher := remote.NewSensorPublisher(
		mqttClient,
		topics,
		cfg.Deployment.NodeID,
		sensors,
		flowMeter,
		version,
	)

	if err := publisher.Start(); err != nil {
		log.Fatalf("Failed to start sensor publisher: %v", err)
	}

	log.Printf("SmartPlug sensor node ready (publishing to %s)", cfg.MQTT.TopicPrefix)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down sensor node...")

	publisher.Stop()

	if flowMeter != nil {
		flowMeter.Stop()
	}

	sensors.Stop()
	mqttClient.Disconnect(1000)

	log.Println("Sensor node stopped")
}

// createMQTTClient creates and configures a paho MQTT client.
func createMQTTClient(cfg *config.MQTTConfig, nodeID string) pahomqtt.Client {
	opts := pahomqtt.NewClientOptions()
	opts.AddBroker(cfg.Broker)

	if cfg.ClientID != "" {
		opts.SetClientID(cfg.ClientID)
	} else {
		opts.SetClientID(fmt.Sprintf("smartplug-sensor-%s-%d", nodeID, time.Now().UnixNano()%10000))
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
