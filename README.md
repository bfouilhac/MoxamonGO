# MoxamonGO

Serveur de configuration web pour équipements MOXA (UC-8112-LX, UC-2100, UC-3100, UC-5100 series).

Interface légère permettant de configurer à distance les interfaces réseau, bridges et ports série des passerelles MOXA via un navigateur web.

## Fonctionnalités

### Configuration réseau

- Affichage des interfaces réseau (eth0, eth1, lan0, br0, etc.)
- Configuration IP statique ou DHCP
- Paramétrage du masque de sous-réseau, passerelle et DNS
- Affichage de l'adresse MAC et du statut (up/down)
- Sauvegarde automatique avant modification
- Détection des interfaces esclaves de bridge

### Bridge réseau

- Création de bridges (br0, br1, etc.) avec sélection des interfaces
- Configuration IP obligatoire sur le bridge
- Suppression du bridge avec restauration des interfaces
- Persistance dans /etc/network/interfaces via pre-up (sans bridge-utils)
- Sauvegarde et restauration automatique des configurations

### Configuration ports série

- Détection automatique des ports physiques MOXA (ttyM0, ttyM1)
- Sélection du mode : RS232, RS422, RS485-2W, RS485-4W
- Configuration du débit (300 à 921600 bauds)
- Paramétrage des bits de données, stop et parité
- Contrôle de flux (none, hardware, software)
- Sauvegarde automatique avant modification

### Scanner de découverte

- Exécutable Windows (moxa-scanner.exe) pour détecter les UC sur le réseau
- Envoi broadcast UDP sur le port 54321
- Détection cross-subnet (les UC répondent en unicast et broadcast)
- Affichage du nom produit, adresse IP, version et hostname

### Sécurité

- Authentification par login/mot de passe
- Sessions avec expiration automatique (24h)
- Hashage bcrypt des mots de passe
- HTTPS avec certificat auto-signé
- Changement de mot de passe via l'interface

### Caractéristiques techniques

- Binaire unique sans dépendances (5-8 Mo)
- Consommation mémoire faible (10-15 Mo RAM)
- Interface web responsive avec auto-refresh
- API REST complète
- Compatible init.d (BusyBox)
- Démarrage automatique au boot
- Service de découverte UDP intégré

## Prérequis

### Machine de développement (Windows)

- Go 1.21 ou supérieur
- Client SSH (intégré à Windows 10/11)

### Équipement MOXA

- Linux Moxa 4.x (kernel 4.4.0+)
- Accès SSH ou console série
- Accès root (sudo)
- 10 Mo d'espace disque disponible

## Installation

### Étape 1 : Installer Go

```
winget install GoLang.go
```

Ou télécharger depuis https://go.dev/dl/

Vérifier l'installation :

```powershell
go version
```

### Étape 2 : Compiler et déployer

Depuis le répertoire du projet :

```powershell
.\scripts\build.ps1 -a <IP_MOXA>
```

Ce script compile le serveur pour ARM, crée l'archive et la transfère sur l'UC.

### Étape 3 : Installer sur le MOXA

```bash
ssh moxa@<IP_MOXA>
cd /tmp && tar xzf moxa-config-1.0.0.tar.gz && cd moxa-config-1.0.0 && sudo sh ./install.sh
```

### Étape 4 : Accéder à l'interface

```
https://<IP_MOXA>:8080
```

Identifiants par défaut : admin / admin

## Scanner de découverte

Le script de build génère également `moxa-scanner.exe` pour Windows.

```powershell
.\moxa-scanner.exe
```

Affiche toutes les UC détectées sur le réseau :

```
+----------------+-----------------+
| Produit        | Adresse IP      |
+----------------+-----------------+
| UC-8112-LX     | 192.168.1.90    |
| UC-2112-LX     | 192.168.3.50    |
+----------------+-----------------+

2 UC trouvé(s)
```

## Utilisation

### Interface Web

L'interface comporte deux onglets :

**Réseau** : Liste les interfaces réseau et bridges. Bouton "Créer un bridge" pour configurer un pont réseau. Les interfaces esclaves sont grisées et indiquent leur bridge parent.

**Ports série** : Liste les ports ttyM0 et ttyM1. Cliquer sur "Configurer" pour modifier le mode et les paramètres.

### Créer un bridge

1. Cliquer sur "Créer un bridge"
2. Sélectionner les interfaces à inclure (eth0, eth1, etc.)
3. Configurer l'IP, masque, passerelle et DNS
4. Valider

Le bridge est créé immédiatement et persiste après reboot.

### Supprimer un bridge

1. Cliquer sur "Supprimer" sur la carte du bridge
2. Confirmer la suppression

Les interfaces sont libérées et leurs configurations précédentes restaurées.

## API REST

### Authentification

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| POST | /api/auth/login | Connexion |
| POST | /api/auth/logout | Déconnexion |
| GET | /api/auth/check | Vérifier la session |
| POST | /api/auth/change-password | Changer le mot de passe |

### Reseau

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| GET | /api/network/interfaces | Liste les interfaces |
| GET | /api/network/interface/{name} | Détails d'une interface |
| POST | /api/network/configure | Configure une interface |
| GET | /api/network/backups | Liste les sauvegardes |
| POST | /api/network/restore/{name} | Restaure un backup |

### Bridge

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| GET | /api/bridge/list | Liste les bridges |
| GET | /api/bridge/{name} | Détails d'un bridge |
| GET | /api/bridge/available | Interfaces disponibles |
| POST | /api/bridge/create | Crée un bridge |
| POST | /api/bridge/configure | Configure un bridge |
| POST | /api/bridge/delete/{name} | Supprime un bridge |

### Ports série

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| GET | /api/serial/ports | Liste les ports |
| GET | /api/serial/port/{device} | Détails d'un port |
| POST | /api/serial/configure | Configure un port |
| GET | /api/serial/options | Options disponibles |

### Système

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| GET | /api/health | État du service |
| GET | /api/system/info | Infos système |

## Exemples API

Créer un bridge :

```bash
curl -X POST https://<IP_MOXA>:8080/api/bridge/create \
  -H "Content-Type: application/json" \
  -H "Cookie: session=<TOKEN>" \
  -d '{
    "name": "br0",
    "interfaces": ["eth0", "eth1"],
    "ip": "192.168.1.90",
    "netmask": "255.255.255.0",
    "gateway": "192.168.1.1"
  }'
```

Supprimer un bridge :

```bash
curl -X POST https://<IP_MOXA>:8080/api/bridge/delete/br0 \
  -H "Cookie: session=<TOKEN>"
```

Configurer eth0 en IP statique :

```bash
curl -X POST https://<IP_MOXA>:8080/api/network/configure \
  -H "Content-Type: application/json" \
  -H "Cookie: session=<TOKEN>" \
  -d '{
    "name": "eth0",
    "dhcp": false,
    "ip": "192.168.1.50",
    "netmask": "255.255.255.0",
    "gateway": "192.168.1.1",
    "dns": ["8.8.8.8", "8.8.4.4"]
  }'
```

Configurer un port série en RS485 :

```bash
curl -X POST https://<IP_MOXA>:8080/api/serial/configure \
  -H "Content-Type: application/json" \
  -H "Cookie: session=<TOKEN>" \
  -d '{
    "device": "/dev/ttyM0",
    "mode": "RS485-2W",
    "baudRate": 115200,
    "dataBits": 8,
    "stopBits": 1,
    "parity": "none",
    "flowControl": "none"
  }'
```

## Gestion du service

```bash
# Statut
sudo /etc/init.d/moxa-config status

# Redémarrer
sudo /etc/init.d/moxa-config restart

# Arrêter
sudo /etc/init.d/moxa-config stop

# Logs
tail -f /var/log/moxa-config.log
```

## Structure du projet

```
moxa-config/
  cmd/
    server/
      main.go
      web/
        index.html
        login.html
        style.css
        app.js
    scanner/
      main.go
  internal/
    auth/
      auth.go
      handlers.go
    config/
      config.go
    discovery/
      discovery.go
    handlers/
      bridge.go
      network.go
      serial.go
    models/
      models.go
    services/
      bridge.go
      network.go
      serial.go
  scripts/
    build.ps1
    install.sh
    moxa-config.service
  go.mod
  go.sum
  README.md
```

## Configuration

### Variables d'environnement

| Variable | Défaut | Description |
|----------|--------|-------------|
| MOXA_PORT | 8080 | Port HTTPS du serveur |
| MOXA_CONFIGDIR | /etc/moxa-config | Répertoire des configurations |
| MOXA_BACKUPDIR | /var/moxa-config/backups | Répertoire des sauvegardes |
| MOXA_LOGFILE | /var/log/moxa-config.log | Fichier de log |
| MOXA_TLS | true | Activer HTTPS |

### Changer le port

```bash
sudo vi /etc/init.d/moxa-config
# Modifier : export MOXA_PORT=8888
sudo /etc/init.d/moxa-config restart
```

## Compatibilité

Testé sur :

- MOXA UC-8112-LX (ARM Cortex-A8)
- MOXA UC-2112-LX
- MOXA UC-3111-LX

Compatible avec tous les UC-xxxx sous Linux Moxa.

## Dépannage

### Le service ne démarre pas

```bash
cat /var/log/moxa-config.log
/opt/moxa-config/moxa-config
```

### Interface réseau non détectée

Les interfaces détectées sont celles commençant par eth, lan, br ou wlan.

### Port série non détecté

Seuls les ports /dev/ttyM0 et /dev/ttyM1 sont affichés (ports physiques MOXA).

### Bridge non persisté après reboot

Vérifier le contenu de /etc/network/interfaces. La configuration doit contenir les directives pre-up.

### Scanner ne détecte pas toutes les UC

- Vérifier que le port UDP 54321 n'est pas bloqué
- Les UC doivent avoir le service moxa-config actif
- Les UC sur d'autres sous-réseaux sont détectées si elles sont sur le même segment L2

## Désinstallation

```bash
sudo /etc/init.d/moxa-config stop
sudo rm /etc/init.d/moxa-config
sudo rm -rf /opt/moxa-config
sudo reboot
```

Les configurations sont conservées dans /etc/moxa-config.

## Licence

MIT
