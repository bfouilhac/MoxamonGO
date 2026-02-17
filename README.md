# MOXA Config Server

Serveur de configuration web pour équipements MOXA (UC-8112-LX, UC-2100, UC-3100, UC-5100 series).

Interface légère permettant de configurer à distance les interfaces réseau et ports série des passerelles MOXA via un navigateur web.

## Fonctionnalités

### Configuration Réseau

- Affichage des interfaces réseau (eth0, eth1, lan0, etc.)
- Configuration IP statique ou DHCP
- Paramétrage du masque de sous-réseau
- Configuration de la passerelle par défaut
- Configuration des serveurs DNS
- Affichage de l'adresse MAC et du statut (up/down)
- Sauvegarde automatique avant modification

### Configuration Ports Série

- Détection automatique des ports physiques MOXA (ttyM0, ttyM1)
- Sélection du mode : RS232, RS422, RS485-2W, RS485-4W
- Configuration du débit (300 à 921600 bauds)
- Paramétrage des bits de données (5, 6, 7, 8)
- Paramétrage des bits de stop (1, 2)
- Configuration de la parité (none, odd, even, mark, space)
- Contrôle de flux (none, hardware, software)
- Sauvegarde automatique avant modification

### Caractéristiques Techniques

- Binaire unique sans dépendances (~5-8 Mo)
- Consommation mémoire faible (~10-15 Mo RAM)
- Interface web responsive
- API REST complète
- Compatible systemd et init.d (BusyBox)
- Démarrage automatique au boot

## Prérequis

### Machine de développement (Windows 11)

- Go 1.21 ou supérieur
- Client SSH (intégré à Windows 10/11)

### Équipement MOXA

- Linux Moxa 4.x (kernel 4.4.0+)
- Accès SSH ou console série
- Accès root (sudo)
- ~10 Mo d'espace disque disponible

## Installation complète (Windows vers MOXA)

### Étape 1 : Installer Go

Télécharge et installe Go depuis :

```
https://go.dev/dl/go1.21.13.windows-amd64.msi
```

ou en faisant la commande : 

```
winget install GoLang.go
```

Après installation, ouvre un **nouveau** terminal PowerShell et vérifie :

```powershell
go version
```

### Étape 2 : Préparer l'environnement

Place toi dans le répertoire que tu veux puis on stocke tout le code téléchargé (dossiers `cmd`, `internal`, `scripts` ...) 

Enfin, depuis l'explorateur de fichier, clique sur la barre de chemin au dessus des fichiers puis tape `powershell`. Ça permet d'ouvrir le powershell directement dans le dossier qui contient tout le code

### Étape 3 : Lancer le script de compilation

Une fois le powershell ouvert dans le bon dossier, faire la commande :

```powershell
.\scripts\build.ps1 -a <IP_MOXA> 
```

En renseignant l'adresse IPv4 de l'appareil.

Ce script permet de paramétrer les variables, lancer le build avec Go et construire & copier l'archive sur l'appareil. 

### Étape 4 : Installer sur le MOXA

Connecte-toi en SSH :

```powershell
ssh moxa@<IP_MOXA>
```

Puis exécute :

```bash
cd /tmp && tar xzf moxa-config-1.0.0.tar.gz && cd moxa-config-1.0.0 && sudo sh ./install.sh
```

### Étape 5 : Accéder à l'interface

Ouvre ton navigateur à l'adresse :

```
http://<IP_MOXA>:8080
```

## Utilisation

### Interface Web

L'interface comporte deux onglets :

**Réseau** : Liste les interfaces réseau détectées. Cliquer sur "Configurer" pour modifier les paramètres IP, masque, passerelle et DNS.

**Ports Série** : Liste les ports ttyM0 et ttyM1. Cliquer sur "Configurer" pour modifier le mode (RS232/422/485) et les paramètres de communication.

### API REST

#### Réseau

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| GET | `/api/network/interfaces` | Liste les interfaces |
| GET | `/api/network/interface/{name}` | Détails d'une interface |
| POST | `/api/network/configure` | Configure une interface |
| GET | `/api/network/backups` | Liste les sauvegardes |
| POST | `/api/network/restore/{name}` | Restaure un backup |

#### Ports Série

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| GET | `/api/serial/ports` | Liste les ports |
| GET | `/api/serial/port/{device}` | Détails d'un port |
| POST | `/api/serial/configure` | Configure un port |
| GET | `/api/serial/options` | Options disponibles |

#### Système

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| GET | `/api/health` | État du service |
| GET | `/api/system/info` | Infos système |

### Exemples API

Lister les interfaces réseau :

```bash
curl http://192.168.1.100:8080/api/network/interfaces
```

Configurer eth0 en statique avec DNS :

```bash
curl -X POST http://192.168.1.100:8080/api/network/configure \
  -H "Content-Type: application/json" \
  -d '{
    "name": "eth0",
    "dhcp": false,
    "ip": "192.168.1.50",
    "netmask": "255.255.255.0",
    "gateway": "192.168.1.1",
    "dns": ["8.8.8.8", "8.8.4.4"]
  }'
```

Configurer eth1 en DHCP :

```bash
curl -X POST http://192.168.1.100:8080/api/network/configure \
  -H "Content-Type: application/json" \
  -d '{"name": "eth1", "dhcp": true}'
```

Configurer un port série en RS485 :

```bash
curl -X POST http://192.168.1.100:8080/api/serial/configure \
  -H "Content-Type: application/json" \
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

### Vérifier le statut

```bash
sudo /etc/init.d/moxa-config status
```

### Redémarrer le service

```bash
sudo /etc/init.d/moxa-config restart
```

### Arrêter le service

```bash
sudo /etc/init.d/moxa-config stop
```

### Consulter les logs

```bash
tail -f /var/log/moxa-config.log
```

## Désinstallation

```bash
sudo systemctl stop moxa-config
sudo systemctl disable moxa-config
sudo rm /etc/systemd/system/moxa-config.service
sudo systemctl daemon-reload
sudo rm -rf /opt/moxa-config  
```

Les configurations sont conservées dans `/etc/moxa-config`.

## Structure du projet

```
moxa-config/
├── cmd/server/
│   ├── main.go
│   └── web/
│       ├── index.html
│       ├── style.css
│       └── app.js
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── handlers/
│   │   ├── network.go
│   │   └── serial.go
│   ├── models/
│   │   └── models.go
│   └── services/
│       ├── network.go
│       └── serial.go
├── scripts/
│   ├── build.sh
│   └── install.sh
├── go.mod
└── README.md
```

## Configuration

### Variables d'environnement

| Variable | Défaut | Description |
|----------|--------|-------------|
| MOXA_PORT | 8080 | Port HTTP du serveur |
| MOXA_CONFIGDIR | /etc/moxa-config | Répertoire des configurations |
| MOXA_BACKUPDIR | /var/moxa-config/backups | Répertoire des sauvegardes |
| MOXA_LOGFILE | /var/log/moxa-config.log | Fichier de log |

### Changer le port

Édite le script init.d :

```bash
sudo vi /etc/init.d/moxa-config
```

Modifie la ligne :

```bash
export MOXA_PORT=8888
```

Puis redémarre :

```bash
sudo /etc/init.d/moxa-config restart
```

## Compatibilité

Testé sur :

- MOXA UC-8112-LX (ARM Cortex-A8)
- MOXA UC-2112-LX
- MOXA UC-3111-LX

Devrait fonctionner sur tous les UC-xxxx avec Linux Moxa.

## Dépannage

### Le service ne démarre pas

Vérifier les logs :

```bash
cat /var/log/moxa-config.log
```

Tester manuellement :

```bash
/opt/moxa-config/moxa-config
```

### Erreur "command not found" lors de l'installation

Le fichier a des fins de ligne Windows. Corriger avec :

```bash
sed -i 's/\r$//' install.sh
sudo sh ./install.sh
```

Ou lancer directement avec sh :

```bash
sudo sh ./install.sh
```

### Interface réseau non détectée

Seules les interfaces commençant par `eth` ou `lan` sont détectées. Pour d'autres noms, modifier `internal/services/network.go`.

### Port série non détecté

Seuls les ports `/dev/ttyM0` et `/dev/ttyM1` sont affichés (ports physiques MOXA). Les autres ports (ttyUSB, ttyS, etc.) sont ignorés.

### Impossible de changer le mode RS232/422/485

Le changement de mode nécessite la commande `setinterface` ou `mx-set-serial-mode` fournie par MOXA. Vérifier qu'elle est disponible :

```bash
which setinterface
which mx-set-serial-mode
```

## Licence

MIT