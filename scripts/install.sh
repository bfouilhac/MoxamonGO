#!/bin/sh
# install.sh - Installation de moxa-config sur équipements MOXA
# Compatible avec BusyBox/ash
set -e

INSTALL_DIR="/opt/moxa-config"
CONFIG_DIR="/etc/moxa-config"
BACKUP_DIR="/var/moxa-config/backups"
LOG_DIR="/var/log"
SERVICE_NAME="moxa-config"
PORT="${PORT:-8080}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    printf "${GREEN}[INFO]${NC} %s\n" "$1"
}

log_warn() {
    printf "${YELLOW}[WARN]${NC} %s\n" "$1"
}

log_error() {
    printf "${RED}[ERROR]${NC} %s\n" "$1"
}

check_root() {
    if [ "$(id -u)" -ne 0 ]; then
        log_error "Ce script doit être exécuté en tant que root"
        exit 1
    fi
}

detect_init_system() {
    if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
        echo "systemd"
    elif [ -f /etc/init.d/rcS ] || [ -d /etc/init.d ]; then
        echo "sysvinit"
    else
        echo "unknown"
    fi
}

stop_service() {
    log_info "Arrêt du service existant..."
    case $(detect_init_system) in
        systemd)
            systemctl stop "$SERVICE_NAME" 2>/dev/null || true
            ;;
        sysvinit)
            /etc/init.d/"$SERVICE_NAME" stop 2>/dev/null || true
            ;;
    esac
    sleep 1
}

create_directories() {
    log_info "Création des répertoires..."
    mkdir -p "$INSTALL_DIR"
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$BACKUP_DIR"
    mkdir -p "/etc/network/interfaces.d" 2>/dev/null || true
}

install_binary() {
    log_info "Installation du binaire..."

    if [ -f "./moxa-config" ]; then
        cp ./moxa-config "$INSTALL_DIR/moxa-config"
    else
        log_error "Binaire moxa-config non trouvé"
        exit 1
    fi

    chmod 755 "$INSTALL_DIR/moxa-config"
    log_info "Binaire installé: $INSTALL_DIR/moxa-config"
}

install_service_systemd() {
    log_info "Installation du service systemd..."

    cat > /etc/systemd/system/"$SERVICE_NAME".service << EOF
[Unit]
Description=MOXA Configuration Server
After=network.target

[Service]
Type=simple
ExecStart=$INSTALL_DIR/moxa-config
Environment=MOXA_PORT=$PORT
Environment=MOXA_CONFIGDIR=$CONFIG_DIR
Environment=MOXA_BACKUPDIR=$BACKUP_DIR
Environment=MOXA_LOGFILE=$LOG_DIR/moxa-config.log
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable "$SERVICE_NAME"
    systemctl start "$SERVICE_NAME"
}

install_service_sysvinit() {
    log_info "Installation du service init.d..."

    cat > /etc/init.d/"$SERVICE_NAME" << EOF
#!/bin/sh
### BEGIN INIT INFO
# Provides:          moxa-config
# Required-Start:    \$network \$local_fs
# Required-Stop:     \$network \$local_fs
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Description:       MOXA Configuration Server
### END INIT INFO

DAEMON="$INSTALL_DIR/moxa-config"
PIDFILE="/var/run/moxa-config.pid"
LOGFILE="$LOG_DIR/moxa-config.log"

export MOXA_PORT=$PORT
export MOXA_CONFIGDIR=$CONFIG_DIR
export MOXA_BACKUPDIR=$BACKUP_DIR
export MOXA_LOGFILE=\$LOGFILE

case "\$1" in
    start)
        echo "Starting moxa-config..."
        if [ -f "\$PIDFILE" ] && kill -0 \$(cat "\$PIDFILE") 2>/dev/null; then
            echo "Already running"
            exit 0
        fi
        nohup \$DAEMON >> \$LOGFILE 2>&1 &
        echo \$! > "\$PIDFILE"
        echo "Started (PID: \$(cat \$PIDFILE))"
        ;;
    stop)
        echo "Stopping moxa-config..."
        if [ -f "\$PIDFILE" ]; then
            kill \$(cat "\$PIDFILE") 2>/dev/null || true
            rm -f "\$PIDFILE"
        fi
        echo "Stopped"
        ;;
    restart)
        \$0 stop
        sleep 1
        \$0 start
        ;;
    status)
        if [ -f "\$PIDFILE" ] && kill -0 \$(cat "\$PIDFILE") 2>/dev/null; then
            echo "Running (PID: \$(cat \$PIDFILE))"
        else
            echo "Not running"
            exit 1
        fi
        ;;
    *)
        echo "Usage: \$0 {start|stop|restart|status}"
        exit 1
        ;;
esac
exit 0
EOF

    chmod 755 /etc/init.d/"$SERVICE_NAME"

    if command -v update-rc.d >/dev/null 2>&1; then
        update-rc.d "$SERVICE_NAME" defaults
    else
        ln -sf /etc/init.d/"$SERVICE_NAME" /etc/rc2.d/S99"$SERVICE_NAME" 2>/dev/null || true
        ln -sf /etc/init.d/"$SERVICE_NAME" /etc/rc3.d/S99"$SERVICE_NAME" 2>/dev/null || true
        ln -sf /etc/init.d/"$SERVICE_NAME" /etc/rc5.d/S99"$SERVICE_NAME" 2>/dev/null || true
    fi

    /etc/init.d/"$SERVICE_NAME" start
}

install_service() {
    case $(detect_init_system) in
        systemd)
            install_service_systemd
            ;;
        sysvinit)
            install_service_sysvinit
            ;;
        *)
            log_warn "Système init non reconnu, installation manuelle requise"
            log_info "Pour démarrer manuellement: $INSTALL_DIR/moxa-config"
            ;;
    esac
}

verify_installation() {
    log_info "Vérification de l'installation..."
    sleep 2

    if command -v curl >/dev/null 2>&1; then
        if curl -s "https://localhost:$PORT/api/health" | grep -q "ok"; then
            log_info "Service opérationnel!"
        else
            log_warn "Service démarré mais health check échoué"
        fi
    elif command -v wget >/dev/null 2>&1; then
        if wget -qO- "https://localhost:$PORT/api/health" | grep -q "ok"; then
            log_info "Service opérationnel!"
        else
            log_warn "Service démarré mais health check échoué"
        fi
    else
        log_warn "curl/wget non disponible, vérification manuelle requise"
    fi
}

get_ip() {
    ip -4 addr show | grep -oP '(?<=inet\s)\d+(\.\d+){3}' | grep -v '127.0.0.1' | head -1
}

print_summary() {
    IP=$(get_ip)

    echo ""
    echo "  Installation terminée avec succès!"
    echo ""
    echo "  Interface web: https://${IP:-localhost}:$PORT"
    echo "  API réseau:    https://${IP:-localhost}:$PORT/api/network"
    echo "  API série:     https://${IP:-localhost}:$PORT/api/serial"
    echo ""
    echo "  Commandes utiles:"
    case $(detect_init_system) in
        systemd)
            echo "    Status:  systemctl status $SERVICE_NAME"
            echo "    Logs:    journalctl -u $SERVICE_NAME -f"
            echo "    Restart: systemctl restart $SERVICE_NAME"
            ;;
        sysvinit)
            echo "    Status:  /etc/init.d/$SERVICE_NAME status"
            echo "    Logs:    tail -f $LOG_DIR/moxa-config.log"
            echo "    Restart: /etc/init.d/$SERVICE_NAME restart"
            ;;
    esac
    echo ""
}

uninstall() {
    log_info "Désinstallation de moxa-config..."

    stop_service

    case $(detect_init_system) in
        systemd)
            systemctl disable "$SERVICE_NAME" 2>/dev/null || true
            rm -f /etc/systemd/system/"$SERVICE_NAME".service
            systemctl daemon-reload
            ;;
        sysvinit)
            if command -v update-rc.d >/dev/null 2>&1; then
                update-rc.d -f "$SERVICE_NAME" remove
            fi
            rm -f /etc/init.d/"$SERVICE_NAME"
            rm -f /etc/rc*.d/*"$SERVICE_NAME"
            ;;
    esac

    rm -rf "$INSTALL_DIR"
    log_info "Désinstallation terminée"
    log_info "Configurations conservées dans $CONFIG_DIR"
}

case "${1:-install}" in
    install)
        check_root
        stop_service
        create_directories
        install_binary
        install_service
        verify_installation
        print_summary
        ;;
    uninstall)
        check_root
        uninstall
        ;;
    *)
        echo "Usage: $0 {install|uninstall}"
        exit 1
        ;;
esac
