const API = {
    async get(endpoint) {
        const res = await fetch(`/api${endpoint}`);
        return res.json();
    },
    async post(endpoint, data) {
        const res = await fetch(`/api${endpoint}`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });
        return res.json();
    }
};

let serialOptions = null;
let autoRefreshInterval = null;
let networkData = [];
let serialData = [];
const AUTO_REFRESH_DELAY = 3000;

async function checkAuth() {
    try {
        const res = await fetch('/api/auth/check');
        const data = await res.json();

        if (!data.authenticated) {
            window.location.href = '/login.html';
            return false;
        }

        if (data.mustChange) {
            window.location.href = '/login.html';
            return false;
        }

        return true;
    } catch (err) {
        window.location.href = '/login.html';
        return false;
    }
}

function setupLogout() {
    document.getElementById('logout-btn').addEventListener('click', async () => {
        await fetch('/api/auth/logout', { method: 'POST' });
        window.location.href = '/login.html';
    });
}

async function init() {
    if (!await checkAuth()) return;

    await loadNetworkInterfaces();
    await loadSerialPorts();
    setupTabs();
    setupAutoRefresh();
    setupLogout();
}

function setupTabs() {
    document.querySelectorAll('.tab').forEach(tab => {
        tab.addEventListener('click', () => {
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            document.querySelectorAll('.panel').forEach(p => p.classList.remove('active'));
            tab.classList.add('active');
            document.getElementById(tab.dataset.tab).classList.add('active');
        });
    });
}

function setupAutoRefresh() {
    const toggle = document.getElementById('auto-refresh-toggle');

    function startAutoRefresh() {
        if (autoRefreshInterval) return;
        autoRefreshInterval = setInterval(() => {
            const activeTab = document.querySelector('.tab.active');
            if (activeTab.dataset.tab === 'network') {
                refreshNetworkData();
            } else {
                refreshSerialData();
            }
        }, AUTO_REFRESH_DELAY);
    }

    function stopAutoRefresh() {
        if (autoRefreshInterval) {
            clearInterval(autoRefreshInterval);
            autoRefreshInterval = null;
        }
    }

    toggle.addEventListener('change', () => {
        if (toggle.checked) {
            startAutoRefresh();
        } else {
            stopAutoRefresh();
        }
    });

    startAutoRefresh();
}

async function loadNetworkInterfaces() {
    const container = document.getElementById('network-list');
    container.innerHTML = '<div class="loading">Chargement...</div>';

    const res = await API.get('/network/interfaces');
    if (!res.success) {
        container.innerHTML = `<div class="loading">Erreur: ${res.message}</div>`;
        return;
    }

    if (!res.data || res.data.length === 0) {
        container.innerHTML = '<div class="loading">Aucune interface détectée</div>';
        return;
    }

    networkData = res.data;
    renderNetworkInterfaces();
}

async function refreshNetworkData() {
    const res = await API.get('/network/interfaces');
    if (!res.success || !res.data) return;

    const hasChanged = JSON.stringify(networkData) !== JSON.stringify(res.data);
    if (hasChanged) {
        networkData = res.data;
        renderNetworkInterfaces();
    }
}

function renderNetworkInterfaces() {
    const container = document.getElementById('network-list');

    container.innerHTML = networkData.map(iface => {
        const isBridge = iface.name.startsWith('br');
        const isSlave = iface.bridgeMaster && iface.bridgeMaster !== '';
        const badgeHtml = isBridge ? '<span class="badge bridge">Bridge</span>' : '';
        const slaveInfoHtml = isSlave ? `<div class="card-row slave-info"><span class="label">Esclave de</span><span class="value">${iface.bridgeMaster}</span></div>` : '';

        let footerHtml = '';
        if (isSlave) {
            footerHtml = `<button class="btn secondary" disabled>Esclave de ${iface.bridgeMaster}</button>`;
        } else if (isBridge) {
            footerHtml = `
                <button class="btn primary" onclick='editBridge(${JSON.stringify(iface)})'>Configurer</button>
                <button class="btn danger" onclick='deleteBridge("${iface.name}")'>Supprimer</button>
            `;
        } else {
            footerHtml = `<button class="btn primary" onclick='editNetwork(${JSON.stringify(iface)})'>Configurer</button>`;
        }

        return `
        <div class="card ${isSlave ? 'slave' : ''} ${isBridge ? 'bridge-card' : ''}" data-id="${iface.name}">
            <div class="card-header">
                <h3>${iface.name} ${badgeHtml}</h3>
                <span class="status ${iface.status}">${iface.status}</span>
            </div>
            <div class="card-body">
                ${slaveInfoHtml}
                <div class="card-row">
                    <span class="label">Mode</span>
                    <span class="value">${iface.dhcp ? 'DHCP' : 'Statique'}</span>
                </div>
                <div class="card-row">
                    <span class="label">Adresse IP</span>
                    <span class="value">${iface.ip || '-'}</span>
                </div>
                <div class="card-row">
                    <span class="label">Masque</span>
                    <span class="value">${iface.netmask || '-'}</span>
                </div>
                <div class="card-row">
                    <span class="label">Passerelle</span>
                    <span class="value">${iface.gateway || '-'}</span>
                </div>
                <div class="card-row">
                    <span class="label">DNS</span>
                    <span class="value">${(iface.dns && iface.dns.length > 0) ? iface.dns.join(', ') : '-'}</span>
                </div>
                <div class="card-row">
                    <span class="label">MAC</span>
                    <span class="value">${iface.mac || '-'}</span>
                </div>
            </div>
            <div class="card-footer">
                ${footerHtml}
            </div>
        </div>
    `;
    }).join('');
}

async function loadSerialPorts() {
    const container = document.getElementById('serial-list');
    container.innerHTML = '<div class="loading">Chargement...</div>';

    const optRes = await API.get('/serial/options');
    if (optRes.success) {
        serialOptions = optRes.data;
    }

    const res = await API.get('/serial/ports');
    if (!res.success) {
        container.innerHTML = `<div class="loading">Erreur: ${res.message}</div>`;
        return;
    }

    if (!res.data || res.data.length === 0) {
        container.innerHTML = '<div class="loading">Aucun port série détecté</div>';
        return;
    }

    serialData = res.data;
    renderSerialPorts();
}

async function refreshSerialData() {
    const res = await API.get('/serial/ports');
    if (!res.success || !res.data) return;

    const hasChanged = JSON.stringify(serialData) !== JSON.stringify(res.data);
    if (hasChanged) {
        serialData = res.data;
        renderSerialPorts();
    }
}

function renderSerialPorts() {
    const container = document.getElementById('serial-list');

    container.innerHTML = serialData.map(port => `
        <div class="card" data-id="${port.device}">
            <div class="card-header">
                <h3>${port.device}</h3>
            </div>
            <div class="card-body">
                <div class="card-row">
                    <span class="label">Mode</span>
                    <span class="value">${port.mode || 'RS232'}</span>
                </div>
                <div class="card-row">
                    <span class="label">Débit</span>
                    <span class="value">${port.baudRate} baud</span>
                </div>
                <div class="card-row">
                    <span class="label">Format</span>
                    <span class="value">${port.dataBits}-${port.parity[0].toUpperCase()}-${port.stopBits}</span>
                </div>
                <div class="card-row">
                    <span class="label">Contrôle flux</span>
                    <span class="value">${port.flowControl}</span>
                </div>
            </div>
            <div class="card-footer">
                <button class="btn primary" onclick='editSerial(${JSON.stringify(port)})'>Configurer</button>
            </div>
        </div>
    `).join('');
}

function editNetwork(iface) {
    const dnsValue = (iface.dns && iface.dns.length > 0) ? iface.dns.join(', ') : '';

    document.getElementById('modal-title').textContent = `Configurer ${iface.name}`;
    document.getElementById('modal-body').innerHTML = `
        <form id="network-form">
            <input type="hidden" name="name" value="${iface.name}">
            <div class="form-group">
                <label class="checkbox-group">
                    <input type="checkbox" name="dhcp" ${iface.dhcp ? 'checked' : ''} onchange="toggleDHCP(this)">
                    Obtenir une adresse IP automatiquement (DHCP)
                </label>
            </div>
            <div id="static-fields" ${iface.dhcp ? 'style="display:none"' : ''}>
                <div class="form-group">
                    <label>Adresse IP</label>
                    <input type="text" name="ip" value="${iface.ip || ''}" pattern="^\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}$">
                </div>
                <div class="form-group">
                    <label>Masque de sous-réseau</label>
                    <input type="text" name="netmask" value="${iface.netmask || '255.255.255.0'}">
                </div>
                <div class="form-group">
                    <label>Passerelle</label>
                    <input type="text" name="gateway" value="${iface.gateway || ''}">
                </div>
                <div class="form-group">
                    <label>DNS (séparés par des virgules)</label>
                    <input type="text" name="dns" value="${dnsValue}" placeholder="8.8.8.8, 8.8.4.4">
                </div>
            </div>
        </form>
    `;

    document.getElementById('modal-save').onclick = saveNetwork;
    document.getElementById('modal').classList.remove('hidden');
}

function toggleDHCP(checkbox) {
    document.getElementById('static-fields').style.display = checkbox.checked ? 'none' : 'block';
}

async function saveNetwork() {
    const form = document.getElementById('network-form');
    const dnsInput = form.dns.value.trim();
    const dnsArray = dnsInput ? dnsInput.split(',').map(s => s.trim()).filter(s => s) : [];

    const data = {
        name: form.name.value,
        dhcp: form.dhcp.checked,
        ip: form.ip.value,
        netmask: form.netmask.value,
        gateway: form.gateway.value,
        dns: dnsArray
    };

    const res = await API.post('/network/configure', data);
    if (res.success) {
        showToast('Configuration appliquée', 'success');
        closeModal();
        setTimeout(loadNetworkInterfaces, 1000);
    } else {
        showToast(res.message, 'error');
    }
}

function editSerial(port) {
    const baudOptions = (serialOptions?.baudRates || [9600, 19200, 38400, 57600, 115200])
        .map(b => `<option value="${b}" ${port.baudRate === b ? 'selected' : ''}>${b}</option>`)
        .join('');

    const parityOptions = (serialOptions?.parities || ['none', 'odd', 'even'])
        .map(p => `<option value="${p}" ${port.parity === p ? 'selected' : ''}>${p}</option>`)
        .join('');

    const flowOptions = (serialOptions?.flowControl || ['none', 'hardware', 'software'])
        .map(f => `<option value="${f}" ${port.flowControl === f ? 'selected' : ''}>${f}</option>`)
        .join('');

    const modeOptions = (serialOptions?.modes || ['RS232', 'RS422', 'RS485-2W', 'RS485-4W'])
        .map(m => `<option value="${m}" ${port.mode === m ? 'selected' : ''}>${m}</option>`)
        .join('');

    document.getElementById('modal-title').textContent = `Configurer ${port.device}`;
    document.getElementById('modal-body').innerHTML = `
        <form id="serial-form">
            <input type="hidden" name="device" value="${port.device}">
            <div class="form-group">
                <label>Mode</label>
                <select name="mode">${modeOptions}</select>
            </div>
            <div class="form-group">
                <label>Débit (baud)</label>
                <select name="baudRate">${baudOptions}</select>
            </div>
            <div class="form-group">
                <label>Bits de données</label>
                <select name="dataBits">
                    ${[5,6,7,8].map(b => `<option value="${b}" ${port.dataBits === b ? 'selected' : ''}>${b}</option>`).join('')}
                </select>
            </div>
            <div class="form-group">
                <label>Bits de stop</label>
                <select name="stopBits">
                    <option value="1" ${port.stopBits === 1 ? 'selected' : ''}>1</option>
                    <option value="2" ${port.stopBits === 2 ? 'selected' : ''}>2</option>
                </select>
            </div>
            <div class="form-group">
                <label>Parité</label>
                <select name="parity">${parityOptions}</select>
            </div>
            <div class="form-group">
                <label>Contrôle de flux</label>
                <select name="flowControl">${flowOptions}</select>
            </div>
        </form>
    `;

    document.getElementById('modal-save').onclick = saveSerial;
    document.getElementById('modal').classList.remove('hidden');
}

async function saveSerial() {
    const form = document.getElementById('serial-form');
    const data = {
        device: form.device.value,
        mode: form.mode.value,
        baudRate: parseInt(form.baudRate.value),
        dataBits: parseInt(form.dataBits.value),
        stopBits: parseInt(form.stopBits.value),
        parity: form.parity.value,
        flowControl: form.flowControl.value
    };

    const res = await API.post('/serial/configure', data);
    if (res.success) {
        showToast('Configuration appliquée', 'success');
        closeModal();
        setTimeout(loadSerialPorts, 500);
    } else {
        showToast(res.message, 'error');
    }
}

function closeModal() {
    document.getElementById('modal').classList.add('hidden');
}

async function openBridgeModal() {
    const res = await API.get('/bridge/available');
    if (!res.success) {
        showToast('Erreur: ' + res.message, 'error');
        return;
    }

    const availableIfaces = res.data || [];
    if (availableIfaces.length < 1) {
        showToast('Aucune interface disponible pour créer un bridge', 'error');
        return;
    }

    const ifaceCheckboxes = availableIfaces.map(iface => `
        <label class="checkbox-item">
            <input type="checkbox" name="interfaces" value="${iface}">
            ${iface}
        </label>
    `).join('');

    document.getElementById('modal-title').textContent = 'Créer un bridge';
    document.getElementById('modal-body').innerHTML = `
        <form id="bridge-form">
            <div class="form-group">
                <label>Nom du bridge</label>
                <input type="text" name="name" value="br0" pattern="^br\\d+$" required>
                <small>Format: br0, br1, br2...</small>
            </div>
            <div class="form-group">
                <label>Interfaces à inclure</label>
                <div class="checkbox-group-list">
                    ${ifaceCheckboxes}
                </div>
            </div>
            <div class="form-group">
                <label>Adresse IP *</label>
                <input type="text" name="ip" required pattern="^\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}$" placeholder="192.168.1.100">
            </div>
            <div class="form-group">
                <label>Masque de sous-réseau *</label>
                <input type="text" name="netmask" value="255.255.255.0" required>
            </div>
            <div class="form-group">
                <label>Passerelle</label>
                <input type="text" name="gateway" placeholder="192.168.1.1">
            </div>
            <div class="form-group">
                <label>DNS (séparés par des virgules)</label>
                <input type="text" name="dns" placeholder="8.8.8.8, 8.8.4.4">
            </div>
        </form>
    `;

    document.getElementById('modal-save').onclick = createBridge;
    document.getElementById('modal').classList.remove('hidden');
}

async function createBridge() {
    const form = document.getElementById('bridge-form');
    const checkedBoxes = form.querySelectorAll('input[name="interfaces"]:checked');
    const interfaces = Array.from(checkedBoxes).map(cb => cb.value);

    if (interfaces.length < 1) {
        showToast('Sélectionnez au moins une interface', 'error');
        return;
    }

    const dnsInput = form.dns.value.trim();
    const dnsArray = dnsInput ? dnsInput.split(',').map(s => s.trim()).filter(s => s) : [];

    const data = {
        name: form.name.value,
        interfaces: interfaces,
        ip: form.ip.value,
        netmask: form.netmask.value,
        gateway: form.gateway.value,
        dns: dnsArray
    };

    const res = await API.post('/bridge/create', data);
    if (res.success) {
        showToast('Bridge créé avec succès', 'success');
        closeModal();
        setTimeout(loadNetworkInterfaces, 1000);
    } else {
        showToast(res.message, 'error');
    }
}

function editBridge(bridge) {
    const dnsValue = (bridge.dns && bridge.dns.length > 0) ? bridge.dns.join(', ') : '';

    document.getElementById('modal-title').textContent = `Configurer ${bridge.name}`;
    document.getElementById('modal-body').innerHTML = `
        <form id="bridge-config-form">
            <input type="hidden" name="name" value="${bridge.name}">
            <div class="form-group">
                <label>Adresse IP *</label>
                <input type="text" name="ip" value="${bridge.ip || ''}" required pattern="^\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}$">
            </div>
            <div class="form-group">
                <label>Masque de sous-réseau *</label>
                <input type="text" name="netmask" value="${bridge.netmask || '255.255.255.0'}" required>
            </div>
            <div class="form-group">
                <label>Passerelle</label>
                <input type="text" name="gateway" value="${bridge.gateway || ''}">
            </div>
            <div class="form-group">
                <label>DNS (séparés par des virgules)</label>
                <input type="text" name="dns" value="${dnsValue}" placeholder="8.8.8.8, 8.8.4.4">
            </div>
        </form>
    `;

    document.getElementById('modal-save').onclick = saveBridgeConfig;
    document.getElementById('modal').classList.remove('hidden');
}

async function saveBridgeConfig() {
    const form = document.getElementById('bridge-config-form');
    const dnsInput = form.dns.value.trim();
    const dnsArray = dnsInput ? dnsInput.split(',').map(s => s.trim()).filter(s => s) : [];

    const data = {
        name: form.name.value,
        ip: form.ip.value,
        netmask: form.netmask.value,
        gateway: form.gateway.value,
        dns: dnsArray
    };

    const res = await API.post('/bridge/configure', data);
    if (res.success) {
        showToast('Bridge configuré avec succès', 'success');
        closeModal();
        setTimeout(loadNetworkInterfaces, 1000);
    } else {
        showToast(res.message, 'error');
    }
}

async function deleteBridge(name) {
    if (!confirm(`Supprimer le bridge ${name} ?\n\nLes interfaces seront libérées et leurs configurations précédentes restaurées si disponibles.`)) {
        return;
    }

    const res = await API.post(`/bridge/delete/${name}`, {});
    if (res.success) {
        showToast('Bridge supprimé avec succès', 'success');
        setTimeout(loadNetworkInterfaces, 1000);
    } else {
        showToast(res.message, 'error');
    }
}

function showToast(message, type) {
    const toast = document.getElementById('toast');
    toast.textContent = message;
    toast.className = `toast ${type}`;
    setTimeout(() => toast.classList.add('hidden'), 3000);
}

document.addEventListener('DOMContentLoaded', init);