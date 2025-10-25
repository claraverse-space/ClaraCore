#!/bin/bash

# ClaraCore Service Management Script
# Cross-platform service control for Linux and macOS

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Detect OS
OS="$(uname)"
if [[ "$OS" == "Linux" ]]; then
    SERVICE_MANAGER="systemd"
    SERVICE_NAME="claracore"
    SERVICE_FILE="/etc/systemd/system/claracore.service"
elif [[ "$OS" == "Darwin" ]]; then
    SERVICE_MANAGER="launchd"
    SERVICE_NAME="com.claracore.server"
    SERVICE_FILE="/Library/LaunchDaemons/com.claracore.server.plist"
else
    echo -e "${RED}Error: Unsupported operating system: $OS${NC}"
    echo "This script supports Linux and macOS only."
    echo "For Windows, use PowerShell: Get-Service ClaraCore, Start-Service ClaraCore, Stop-Service ClaraCore"
    exit 1
fi

print_header() {
    echo ""
    echo -e "${BLUE}╔══════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║  $1${NC}"
    echo -e "${BLUE}╚══════════════════════════════════════╝${NC}"
    echo ""
}

check_admin() {
    if [[ $EUID -ne 0 ]]; then
        echo -e "${RED}Error: This script must be run as root (use sudo)${NC}"
        exit 1
    fi
}

service_status() {
    echo -e "${BLUE}Checking ClaraCore service status...${NC}"
    echo ""
    
    if [[ "$SERVICE_MANAGER" == "systemd" ]]; then
        if systemctl is-active --quiet "$SERVICE_NAME"; then
            echo -e "${GREEN}✓ Service is running${NC}"
            STATUS="running"
        else
            echo -e "${YELLOW}✗ Service is not running${NC}"
            STATUS="stopped"
        fi
        
        if systemctl is-enabled --quiet "$SERVICE_NAME"; then
            echo -e "${GREEN}✓ Service is enabled (auto-start)${NC}"
            ENABLED="yes"
        else
            echo -e "${YELLOW}✗ Service is disabled${NC}"
            ENABLED="no"
        fi
        
        echo ""
        echo "Detailed status:"
        systemctl status "$SERVICE_NAME" --no-pager -l
        
    elif [[ "$SERVICE_MANAGER" == "launchd" ]]; then
        if launchctl list | grep -q "$SERVICE_NAME"; then
            echo -e "${GREEN}✓ Service is loaded${NC}"
            STATUS="running"
            
            # Check if it's actually running
            PID=$(launchctl list | grep "$SERVICE_NAME" | awk '{print $1}')
            if [[ "$PID" == "-" ]]; then
                echo -e "${YELLOW}✗ Service is loaded but not running${NC}"
                STATUS="loaded"
            else
                echo -e "${GREEN}✓ Service is running (PID: $PID)${NC}"
            fi
        else
            echo -e "${YELLOW}✗ Service is not loaded${NC}"
            STATUS="stopped"
        fi
        
        if [[ -f "$SERVICE_FILE" ]]; then
            echo -e "${GREEN}✓ Service is installed${NC}"
            ENABLED="yes"
        else
            echo -e "${YELLOW}✗ Service is not installed${NC}"
            ENABLED="no"
        fi
    fi
}

service_start() {
    echo -e "${BLUE}Starting ClaraCore service...${NC}"
    
    if [[ "$SERVICE_MANAGER" == "systemd" ]]; then
        systemctl start "$SERVICE_NAME"
        if systemctl is-active --quiet "$SERVICE_NAME"; then
            echo -e "${GREEN}✓ Service started successfully${NC}"
        else
            echo -e "${RED}✗ Failed to start service${NC}"
            systemctl status "$SERVICE_NAME" --no-pager -l
            exit 1
        fi
    elif [[ "$SERVICE_MANAGER" == "launchd" ]]; then
        launchctl load "$SERVICE_FILE"
        sleep 2
        if launchctl list | grep -q "$SERVICE_NAME"; then
            echo -e "${GREEN}✓ Service started successfully${NC}"
        else
            echo -e "${RED}✗ Failed to start service${NC}"
            exit 1
        fi
    fi
}

service_stop() {
    echo -e "${BLUE}Stopping ClaraCore service...${NC}"
    
    if [[ "$SERVICE_MANAGER" == "systemd" ]]; then
        systemctl stop "$SERVICE_NAME"
        if ! systemctl is-active --quiet "$SERVICE_NAME"; then
            echo -e "${GREEN}✓ Service stopped successfully${NC}"
        else
            echo -e "${RED}✗ Failed to stop service${NC}"
            exit 1
        fi
    elif [[ "$SERVICE_MANAGER" == "launchd" ]]; then
        launchctl unload "$SERVICE_FILE"
        sleep 2
        if ! launchctl list | grep -q "$SERVICE_NAME"; then
            echo -e "${GREEN}✓ Service stopped successfully${NC}"
        else
            echo -e "${RED}✗ Failed to stop service${NC}"
            exit 1
        fi
    fi
}

service_restart() {
    echo -e "${BLUE}Restarting ClaraCore service...${NC}"
    
    if [[ "$SERVICE_MANAGER" == "systemd" ]]; then
        systemctl restart "$SERVICE_NAME"
        if systemctl is-active --quiet "$SERVICE_NAME"; then
            echo -e "${GREEN}✓ Service restarted successfully${NC}"
        else
            echo -e "${RED}✗ Failed to restart service${NC}"
            systemctl status "$SERVICE_NAME" --no-pager -l
            exit 1
        fi
    elif [[ "$SERVICE_MANAGER" == "launchd" ]]; then
        launchctl unload "$SERVICE_FILE" 2>/dev/null || true
        sleep 1
        launchctl load "$SERVICE_FILE"
        sleep 2
        if launchctl list | grep -q "$SERVICE_NAME"; then
            echo -e "${GREEN}✓ Service restarted successfully${NC}"
        else
            echo -e "${RED}✗ Failed to restart service${NC}"
            exit 1
        fi
    fi
}

service_enable() {
    echo -e "${BLUE}Enabling ClaraCore service for auto-start...${NC}"
    
    if [[ "$SERVICE_MANAGER" == "systemd" ]]; then
        systemctl enable "$SERVICE_NAME"
        echo -e "${GREEN}✓ Service enabled for auto-start${NC}"
    elif [[ "$SERVICE_MANAGER" == "launchd" ]]; then
        # launchd services in /Library/LaunchDaemons are automatically enabled
        echo -e "${GREEN}✓ Service is automatically enabled (launchd)${NC}"
    fi
}

service_disable() {
    echo -e "${BLUE}Disabling ClaraCore service auto-start...${NC}"
    
    if [[ "$SERVICE_MANAGER" == "systemd" ]]; then
        systemctl disable "$SERVICE_NAME"
        echo -e "${GREEN}✓ Service disabled from auto-start${NC}"
    elif [[ "$SERVICE_MANAGER" == "launchd" ]]; then
        echo -e "${YELLOW}Note: launchd services cannot be disabled without removal${NC}"
        echo "To disable, you need to unload and remove the service file."
    fi
}

show_logs() {
    echo -e "${BLUE}Showing ClaraCore service logs...${NC}"
    echo ""
    
    if [[ "$SERVICE_MANAGER" == "systemd" ]]; then
        journalctl -u "$SERVICE_NAME" -f --no-pager
    elif [[ "$SERVICE_MANAGER" == "launchd" ]]; then
        echo "Checking system logs for ClaraCore..."
        tail -f /var/log/system.log | grep -i claracore
    fi
}

show_help() {
    cat << EOF
ClaraCore Service Management Script

USAGE:
    sudo $0 <command>

COMMANDS:
    status      Show service status and information
    start       Start the ClaraCore service
    stop        Stop the ClaraCore service
    restart     Restart the ClaraCore service
    enable      Enable service for auto-start on boot
    disable     Disable service auto-start
    logs        Show service logs (follow mode)
    help        Show this help message

EXAMPLES:
    sudo $0 status          # Check if service is running
    sudo $0 restart         # Restart the service
    sudo $0 logs            # Watch service logs

SYSTEM INFORMATION:
    OS: $OS
    Service Manager: $SERVICE_MANAGER
    Service Name: $SERVICE_NAME
    Service File: $SERVICE_FILE

For Windows service management, use PowerShell:
    Get-Service ClaraCore
    Start-Service ClaraCore
    Stop-Service ClaraCore
    Restart-Service ClaraCore

EOF
}

# Main script logic
case "${1:-}" in
    "status")
        print_header "ClaraCore Service Status"
        service_status
        ;;
    "start")
        check_admin
        print_header "Starting ClaraCore Service"
        service_start
        ;;
    "stop")
        check_admin
        print_header "Stopping ClaraCore Service"
        service_stop
        ;;
    "restart")
        check_admin
        print_header "Restarting ClaraCore Service"
        service_restart
        ;;
    "enable")
        check_admin
        print_header "Enabling ClaraCore Service"
        service_enable
        ;;
    "disable")
        check_admin
        print_header "Disabling ClaraCore Service"
        service_disable
        ;;
    "logs")
        print_header "ClaraCore Service Logs"
        show_logs
        ;;
    "help"|"--help"|"-h")
        show_help
        ;;
    "")
        echo -e "${RED}Error: No command specified${NC}"
        echo ""
        show_help
        exit 1
        ;;
    *)
        echo -e "${RED}Error: Unknown command '$1'${NC}"
        echo ""
        show_help
        exit 1
        ;;
esac