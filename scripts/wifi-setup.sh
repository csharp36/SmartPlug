#!/bin/bash
# SmartPlug WiFi Setup Script
# Creates a captive portal for WiFi configuration

set -e

HOTSPOT_SSID="SmartPlug-Setup"
HOTSPOT_PASSWORD="smartplug"
CONFIG_INTERFACE="wlan0"

echo "==================================="
echo "SmartPlug WiFi Setup"
echo "==================================="
echo

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root (sudo ./wifi-setup.sh)"
    exit 1
fi

# Install required packages
echo "Installing required packages..."
apt-get update
apt-get install -y hostapd dnsmasq

# Stop services during configuration
systemctl stop hostapd 2>/dev/null || true
systemctl stop dnsmasq 2>/dev/null || true

# Backup existing configs
cp /etc/dhcpcd.conf /etc/dhcpcd.conf.backup 2>/dev/null || true
cp /etc/hostapd/hostapd.conf /etc/hostapd/hostapd.conf.backup 2>/dev/null || true
cp /etc/dnsmasq.conf /etc/dnsmasq.conf.backup 2>/dev/null || true

# Configure static IP for wlan0
echo "Configuring static IP..."
cat >> /etc/dhcpcd.conf << EOF

# SmartPlug WiFi Setup
interface $CONFIG_INTERFACE
    static ip_address=192.168.4.1/24
    nohook wpa_supplicant
EOF

# Configure hostapd
echo "Configuring hostapd..."
cat > /etc/hostapd/hostapd.conf << EOF
interface=$CONFIG_INTERFACE
driver=nl80211
ssid=$HOTSPOT_SSID
hw_mode=g
channel=7
wmm_enabled=0
macaddr_acl=0
auth_algs=1
ignore_broadcast_ssid=0
wpa=2
wpa_passphrase=$HOTSPOT_PASSWORD
wpa_key_mgmt=WPA-PSK
wpa_pairwise=TKIP
rsn_pairwise=CCMP
EOF

# Point hostapd to config
sed -i 's|#DAEMON_CONF=""|DAEMON_CONF="/etc/hostapd/hostapd.conf"|' /etc/default/hostapd

# Configure dnsmasq
echo "Configuring dnsmasq..."
cat > /etc/dnsmasq.conf << EOF
interface=$CONFIG_INTERFACE
dhcp-range=192.168.4.2,192.168.4.20,255.255.255.0,24h
domain=local
address=/#/192.168.4.1
EOF

# Enable IP forwarding
echo "Enabling IP forwarding..."
sed -i 's/#net.ipv4.ip_forward=1/net.ipv4.ip_forward=1/' /etc/sysctl.conf
sysctl -p

# Unmask and enable services
systemctl unmask hostapd
systemctl enable hostapd
systemctl enable dnsmasq

# Start services
systemctl start hostapd
systemctl start dnsmasq

echo
echo "==================================="
echo "WiFi Setup Portal Active"
echo "==================================="
echo
echo "SSID: $HOTSPOT_SSID"
echo "Password: $HOTSPOT_PASSWORD"
echo "Portal IP: 192.168.4.1"
echo
echo "Connect to the WiFi network and open http://192.168.4.1"
echo
echo "To disable the setup portal and connect to home WiFi:"
echo "1. Configure /etc/wpa_supplicant/wpa_supplicant.conf"
echo "2. Run: sudo ./wifi-setup.sh disable"
