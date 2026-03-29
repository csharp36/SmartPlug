#!/bin/bash
# SmartPlug Installation Script
# Run as root or with sudo

set -e

INSTALL_DIR="/opt/smartplug"
CONFIG_DIR="/etc/smartplug"
DATA_DIR="/var/lib/smartplug"
LOG_DIR="/var/log"
SERVICE_USER="smartplug"

echo "==================================="
echo "SmartPlug Installation Script"
echo "==================================="
echo

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root (sudo ./install.sh)"
    exit 1
fi

# Check architecture
ARCH=$(uname -m)
case $ARCH in
    aarch64|arm64)
        BINARY_ARCH="arm64"
        ;;
    armv7l|armhf)
        BINARY_ARCH="arm"
        ;;
    x86_64)
        BINARY_ARCH="amd64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

echo "Detected architecture: $ARCH ($BINARY_ARCH)"

# Create service user
echo "Creating service user..."
if ! id "$SERVICE_USER" &>/dev/null; then
    useradd --system --no-create-home --shell /bin/false "$SERVICE_USER"
fi

# Add user to gpio group for hardware access
usermod -a -G gpio "$SERVICE_USER" 2>/dev/null || true

# Create directories
echo "Creating directories..."
mkdir -p "$INSTALL_DIR"
mkdir -p "$CONFIG_DIR"
mkdir -p "$DATA_DIR"

# Copy binary
echo "Installing binary..."
if [ -f "./smartplug" ]; then
    cp ./smartplug "$INSTALL_DIR/smartplug"
elif [ -f "./cmd/smartplug/smartplug" ]; then
    cp ./cmd/smartplug/smartplug "$INSTALL_DIR/smartplug"
else
    echo "Binary not found. Please build first: go build -o smartplug ./cmd/smartplug"
    exit 1
fi

chmod +x "$INSTALL_DIR/smartplug"

# Copy default config if not exists
if [ ! -f "$CONFIG_DIR/smartplug.yaml" ]; then
    echo "Installing default configuration..."
    if [ -f "./configs/smartplug.yaml" ]; then
        cp ./configs/smartplug.yaml "$CONFIG_DIR/smartplug.yaml"
    fi
fi

# Set permissions
echo "Setting permissions..."
chown -R "$SERVICE_USER:$SERVICE_USER" "$DATA_DIR"
chown root:root "$CONFIG_DIR/smartplug.yaml"
chmod 644 "$CONFIG_DIR/smartplug.yaml"

# Enable 1-Wire interface
echo "Enabling 1-Wire interface..."
if ! grep -q "dtoverlay=w1-gpio" /boot/config.txt 2>/dev/null; then
    echo "dtoverlay=w1-gpio" >> /boot/config.txt
    echo "1-Wire overlay added to /boot/config.txt"
    REBOOT_REQUIRED=true
fi

# Load 1-Wire modules
modprobe w1-gpio 2>/dev/null || true
modprobe w1-therm 2>/dev/null || true

# Install systemd service
echo "Installing systemd service..."
cat > /etc/systemd/system/smartplug.service << 'EOF'
[Unit]
Description=SmartPlug Hot Water Recirculation Controller
After=network.target

[Service]
Type=simple
User=smartplug
Group=smartplug
ExecStart=/opt/smartplug/smartplug --config /etc/smartplug/smartplug.yaml
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/smartplug
PrivateTmp=true

# Hardware access
SupplementaryGroups=gpio

[Install]
WantedBy=multi-user.target
EOF

# Reload systemd
systemctl daemon-reload

# Enable and start service
echo "Enabling service..."
systemctl enable smartplug.service

echo
echo "==================================="
echo "Installation complete!"
echo "==================================="
echo
echo "Configuration file: $CONFIG_DIR/smartplug.yaml"
echo "Data directory: $DATA_DIR"
echo "Service: smartplug.service"
echo
echo "Commands:"
echo "  Start:   sudo systemctl start smartplug"
echo "  Stop:    sudo systemctl stop smartplug"
echo "  Status:  sudo systemctl status smartplug"
echo "  Logs:    sudo journalctl -u smartplug -f"
echo
echo "Web interface: http://$(hostname -I | awk '{print $1}'):8080"
echo

if [ "$REBOOT_REQUIRED" = true ]; then
    echo "IMPORTANT: A reboot is required to enable 1-Wire support."
    echo "Run: sudo reboot"
fi
