# MoxamonGO

Serveur de configuration web pour equipements MOXA (UC-8112-LX, UC-2100, UC-3100, UC-5100 series).

Interface legere permettant de configurer a distance les interfaces reseau, bridges et ports serie des passerelles MOXA via un navigateur web.

## Fonctionnalites

### Configuration reseau

- Affichage des interfaces reseau (eth0, eth1, lan0, br0, etc.)
- Configuration IP statique ou DHCP
- Parametrage du masque de sous-reseau, passerelle et DNS
- Affichage de l'adresse MAC et du statut (up/down)
- Sauvegarde automatique avant modification
- Detection des interfaces esclaves de bridge

### Bridge reseau

- Creation de bridges (br0, br1, etc.) avec selection des interfaces
- Configuration IP obligatoire sur le bridge
- Suppression du bridge avec restauration des interfaces
- Persistance dans /etc/network/interfaces via pre-up (sans bridge-utils)
- Sauvegarde et restauration automatique des configurations

### Configuration ports serie

- Detection automatique des ports physiques MOXA (ttyM0, ttyM1)
- Selection du mode : RS232, RS422, RS485-2W, RS485-4W
- Configuration du debit (300 a 921600 bauds)
- Parametrage des bits de donnees, stop et parite
- Controle de flux (none, hardware, software)
- Sauvegarde automatique avant modification

### Scanner de decouverte

- Executable Windows (moxa-scanner.exe) pour detecter les UC sur le reseau
- Envoi broadcast UDP sur le port 54321
- Detection cross-subnet (les UC repondent en unicast et broadcast)
- Affichage du nom produit, adresse IP, version et hostname

### Securite

- Authentification par login/mot de passe
- Sessions avec expiration automatique (24h)
- Hashage bcrypt des mots de passe
- HTTPS avec certificat auto-signe
- Changement de mot de passe via l'interface

### Caracteristiques techniques

- Binaire unique sans dependances (5-8 Mo)
- Consommation memoire faible (10-15 Mo RAM)
- Interface web responsive avec auto-refresh
- API REST complete
- Compatible init.d (BusyBox)
- Demarrage automatique au boot
- Service de decouverte UDP integre

## Prerequis

### Machine de developpement (Windows)

- Go 1.21 ou superieur
- Client SSH (integre a Windows 10/11)

### Equipement MOXA

- Linux Moxa 4.x (kernel 4.4.0+)
- Acces SSH ou console serie
- Acces root (sudo)
- 10 Mo d'espace disque disponible

## Installation

### Etape 1 : Installer Go

```
winget install GoLang.go
```

Ou telecharger depuis https://go.dev/dl/

Verifier l'installation :

```powershell
go version
```

### Etape 2 : Compiler et deployer

Depuis le repertoire du projet :

```powershell
.\scripts\build.ps1 -a <IP_MOXA>
```

Ce script compile le serveur pour ARM, cree l'archive et la transfère sur l'UC.

### Etape 3 : Installer sur le MOXA

```bash
ssh moxa@<IP_MOXA>
cd /tmp && tar xzf moxa-config-1.0.0.tar.gz && cd moxa-config-1.0.0 && sudo sh ./install.sh
```

### Etape 4 : Acceder a l'interface

```
https://<IP_MOXA>:8080
```

Identifiants par defaut : admin / admin

## Scanner de decouverte

Le script de build genere egalement `moxa-scanner.exe` pour Windows.

```powershell
.\moxa-scanner.exe
```

Affiche toutes les UC detectees sur le reseau :

```
+----------------+-----------------+
| Produit        | Adresse IP      |
+----------------+-----------------+
| UC-8112-LX     | 192.168.1.90    |
| UC-2112-LX     | 192.168.3.50    |
+----------------+-----------------+

2 UC trouve(s)
```

## Utilisation

### Interface Web

L'interface comporte deux onglets :

**Reseau** : Liste les interfaces reseau et bridges. Bouton "Creer un bridge" pour configurer un pont reseau. Les interfaces esclaves sont grisees et indiquent leur bridge parent.

**Ports serie** : Liste les ports ttyM0 et ttyM1. Cliquer sur "Configurer" pour modifier le mode et les parametres.

### Creer un bridge

1. Cliquer sur "Creer un bridge"
2. Selectionner les interfaces a inclure (eth0, eth1, etc.)
3. Configurer l'IP, masque, passerelle et DNS
4. Valider

Le bridge est cree immediatement et persiste apres reboot.

### Supprimer un bridge

1. Cliquer sur "Supprimer" sur la carte du bridge
2. Confirmer la suppression

Les interfaces sont liberees et leurs configurations precedentes restaurees.

## API REST

### Authentification

| Methode | Endpoint | Description |
|---------|----------|-------------|
| POST | /api/auth/login | Connexion |
| POST | /api/auth/logout | Deconnexion |
| GET | /api/auth/check | Verifier la session |
| POST | /api/auth/change-password | Changer le mot de passe |

### Reseau

| Methode | Endpoint | Description |
|---------|----------|-------------|
| GET | /api/network/interfaces | Liste les interfaces |
| GET | /api/network/interface/{name} | Details d'une interface |
| POST | /api/network/configure | Configure une interface |
| GET | /api/network/backups | Liste les sauvegardes |
| POST | /api/network/restore/{name} | Restaure un backup |

### Bridge

| Methode | Endpoint | Description |
|---------|----------|-------------|
| GET | /api/bridge/list | Liste les bridges |
| GET | /api/bridge/{name} | Details d'un bridge |
| GET | /api/bridge/available | Interfaces disponibles |
| POST | /api/bridge/create | Cree un bridge |
| POST | /api/bridge/configure | Configure un bridge |
| POST | /api/bridge/delete/{name} | Supprime un bridge |

### Ports serie

| Methode | Endpoint | Description |
|---------|----------|-------------|
| GET | /api/serial/ports | Liste les ports |
| GET | /api/serial/port/{device} | Details d'un port |
| POST | /api/serial/configure | Configure un port |
| GET | /api/serial/options | Options disponibles |

### Systeme

| Methode | Endpoint | Description |
|---------|----------|-------------|
| GET | /api/health | Etat du service |
| GET | /api/system/info | Infos systeme |

## Exemples API

Creer un bridge :

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

Configurer eth0 en statique :

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

Configurer un port serie en RS485 :

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

# Redemarrer
sudo /etc/init.d/moxa-config restart

# Arreter
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

| Variable | Defaut | Description |
|----------|--------|-------------|
| MOXA_PORT | 8080 | Port HTTPS du serveur |
| MOXA_CONFIGDIR | /etc/moxa-config | Repertoire des configurations |
| MOXA_BACKUPDIR | /var/moxa-config/backups | Repertoire des sauvegardes |
| MOXA_LOGFILE | /var/log/moxa-config.log | Fichier de log |
| MOXA_TLS | true | Activer HTTPS |

### Changer le port

```bash
sudo vi /etc/init.d/moxa-config
# Modifier : export MOXA_PORT=8888
sudo /etc/init.d/moxa-config restart
```

## Compatibilite

Teste sur :

- MOXA UC-8112-LX (ARM Cortex-A8)
- MOXA UC-2112-LX
- MOXA UC-3111-LX

Compatible avec tous les UC-xxxx sous Linux Moxa.

## Depannage

### Le service ne demarre pas

```bash
cat /var/log/moxa-config.log
/opt/moxa-config/moxa-config
```

### Interface reseau non detectee

Les interfaces detectees sont celles commencant par eth, lan, br ou wlan.

### Port serie non detecte

Seuls les ports /dev/ttyM0 et /dev/ttyM1 sont affiches (ports physiques MOXA).

### Bridge non persiste apres reboot

Verifier le contenu de /etc/network/interfaces. La configuration doit contenir les directives pre-up.

### Scanner ne detecte pas toutes les UC

- Verifier que le port UDP 54321 n'est pas bloque
- Les UC doivent avoir le service moxa-config actif
- Les UC sur d'autres sous-reseaux sont detectees si elles sont sur le meme segment L2

## Desinstallation

```bash
sudo /etc/init.d/moxa-config stop
sudo rm /etc/init.d/moxa-config
sudo rm -rf /opt/moxa-config
sudo reboot
```

Les configurations sont conservees dans /etc/moxa-config.

## Licence

MIT
