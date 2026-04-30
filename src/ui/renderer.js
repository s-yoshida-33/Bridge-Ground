// src/ui/renderer.js
document.addEventListener('DOMContentLoaded', async () => {
    const bridgeApi = window.bridgeApi;

    // --- Elements ---
    const syncButton       = document.getElementById('sync-button');
    const progressWrapper  = document.getElementById('sync-progress-wrapper');
    const mainProgressBar  = document.getElementById('main-progress-bar');
    const mainStatusText   = document.getElementById('main-status-text');
    const localApiUrlInput = document.getElementById('local-api-url');
    const copyApiUrlButton = document.getElementById('copy-api-url-button');
    const copyMessage      = document.getElementById('copy-message');
    const passwordInput    = document.getElementById('api-password');
    const togglePasswordButton = document.getElementById('toggle-password-button');
    const saveButton       = document.getElementById('save-settings-button');
    const msgDiv           = document.getElementById('messages');

    let currentConfig = {};
    let isPasswordEditable = false;

    // --- Version ---
    try {
        const version = await bridgeApi.getAppVersion();
        const el = document.getElementById('app-version');
        if (el) el.textContent = version;
    } catch (e) { console.error('version error', e); }

    // --- Tab Navigation ---
    const tabButtons = document.querySelectorAll('.tab-btn');
    const views = document.querySelectorAll('.view');

    function showTab(tabName) {
        tabButtons.forEach(b => b.classList.toggle('active', b.dataset.tab === tabName));
        views.forEach(v => v.classList.toggle('active', v.id === 'view-' + tabName));
        if (tabName === 'logs') loadLogs();
        if (tabName === 'settings') loadSettings(currentConfig);
    }

    tabButtons.forEach(btn => btn.addEventListener('click', () => showTab(btn.dataset.tab)));

    // --- Config Load ---
    try {
        currentConfig = await bridgeApi.getConfig();
        updateLocalApiUrlDisplay(
            currentConfig.serverSettings ? currentConfig.serverSettings.port : 8090
        );
    } catch (e) { console.error('getConfig error', e); }

    function updateLocalApiUrlDisplay(port) {
        localApiUrlInput.value = `http://localhost:${port}`;
    }

    // --- Copy URL ---
    copyApiUrlButton.addEventListener('click', () => {
        navigator.clipboard.writeText(localApiUrlInput.value).then(() => {
            copyMessage.style.display = 'block';
            setTimeout(() => { copyMessage.style.display = 'none'; }, 2000);
        }).catch(() => {
            localApiUrlInput.select();
            document.execCommand('copy');
        });
    });

    // --- Sync Progress ---
    bridgeApi.onSyncProgress((data) => {
        progressWrapper.style.display = 'block';
        if (data.main) {
            mainProgressBar.style.width  = `${data.main.percentage}%`;
            mainProgressBar.textContent  = `${data.main.percentage}%`;
            mainStatusText.textContent   = data.main.message;
        }
        if (data.main && data.main.percentage === 100) {
            setTimeout(() => {
                syncButton.disabled = false;
                updateDataCounts(data.main.stats);
                setTimeout(() => {
                    progressWrapper.style.display = 'none';
                    mainProgressBar.style.width = '0%';
                }, 2000);
            }, 1000);
        }
    });

    // --- Manual Sync ---
    syncButton.addEventListener('click', async () => {
        syncButton.disabled = true;
        progressWrapper.style.display = 'block';
        mainProgressBar.style.width = '0%';
        mainProgressBar.textContent = '0%';
        mainStatusText.textContent  = '同期を開始します...';
        try {
            const result = await bridgeApi.startManualSync();
            if (!result.success) {
                mainStatusText.textContent = `同期失敗: ${result.message}`;
                syncButton.disabled = false;
            }
        } catch (err) {
            mainStatusText.textContent = '同期中にエラーが発生しました。';
            console.error(err);
            syncButton.disabled = false;
        }
    });

    // --- Data Counts ---
    async function updateDataCounts(latestStats = null) {
        try {
            const counts = await bridgeApi.getDataCounts();
            const fmt = (total, key) => {
                if (latestStats && key in latestStats && latestStats[key] !== 0) {
                    const sign = latestStats[key] > 0 ? '+' : '';
                    return `${total} (${sign}${latestStats[key]})`;
                }
                return total != null ? total : '-';
            };
            document.getElementById('count-shops').textContent      = fmt(counts.shops, 'shops');
            document.getElementById('count-shop-news').textContent  = fmt(counts.shop_news, 'shopNews');
            document.getElementById('count-event-news').textContent = fmt(counts.event_news, 'eventNews');
            document.getElementById('count-specials').textContent   = fmt(counts.specials, 'specials');
            document.getElementById('count-sales').textContent      = fmt(counts.sales, 'sales');
        } catch (e) { console.error('counts error', e); }
    }
    updateDataCounts();

    // --- Settings Load ---
    function loadSettings(config) {
        const api  = config.apiSettings  || {};
        const sync = config.syncSettings || {};
        const srv  = config.serverSettings || {};
        const sys  = config.systemSettings || {};

        document.getElementById('api-url').value      = api.baseUrl  || '';
        document.getElementById('api-username').value = api.username || '';

        isPasswordEditable = false;
        passwordInput.disabled = true;
        passwordInput.type     = 'password';
        togglePasswordButton.textContent = '編集';

        // Browser context returns passwordSet flag; Lorca returns actual password
        const hasPassword = api.passwordSet !== undefined ? api.passwordSet : !!api.password;
        if (hasPassword) {
            passwordInput.value       = '••••••••';
            passwordInput.placeholder = '';
        } else {
            passwordInput.value       = '';
            passwordInput.placeholder = '未設定';
        }

        document.getElementById('server-port-input').value = srv.port || 8090;
        document.getElementById('run-on-startup').checked  = sys.runOnStartup || false;
        document.getElementById('start-hidden').checked    = sys.startHidden   || false;
        document.getElementById('auto-sync-enabled').checked    = sync.autoSyncEnabled     || false;
        document.getElementById('sync-interval').value          = sync.syncIntervalMinutes || 60;
        document.getElementById('sync-on-startup').checked      = sync.syncOnStartup       || false;

        const targets = sync.syncTargets || {};
        document.getElementById('sync-target-shops').checked      = targets.shops     || false;
        document.getElementById('sync-target-shop-news').checked  = targets.shopNews  || false;
        document.getElementById('sync-target-event-news').checked = targets.eventNews || false;
        document.getElementById('sync-target-specials').checked   = targets.specials  || false;
        document.getElementById('sync-target-sales').checked      = targets.sales     || false;
        document.getElementById('sync-target-shop-app').checked   = targets.shopApp   || false;
        document.getElementById('sync-target-genres').checked     = targets.genres    || false;
        document.getElementById('sync-target-floors').checked     = targets.floors    || false;
    }

    // --- Password Toggle ---
    togglePasswordButton.addEventListener('click', () => {
        isPasswordEditable = !isPasswordEditable;
        if (isPasswordEditable) {
            passwordInput.disabled = false;
            passwordInput.type     = 'text';
            passwordInput.value    = (currentConfig.apiSettings || {}).password || '';
            passwordInput.placeholder = '新しいパスワードを入力';
            togglePasswordButton.textContent = 'キャンセル';
            passwordInput.focus();
        } else {
            passwordInput.disabled = true;
            passwordInput.type     = 'password';
            togglePasswordButton.textContent = '編集';
            const hasPassword = ((currentConfig.apiSettings || {}).passwordSet !== undefined)
                ? currentConfig.apiSettings.passwordSet
                : !!((currentConfig.apiSettings || {}).password);
            passwordInput.value       = hasPassword ? '••••••••' : '';
            passwordInput.placeholder = hasPassword ? '' : '未設定';
        }
    });

    // --- Message ---
    function showMessage(type, text) {
        msgDiv.className = type;
        msgDiv.textContent = text;
        msgDiv.style.display = 'block';
        setTimeout(() => { msgDiv.style.display = 'none'; }, 5000);
    }

    // --- Save Settings ---
    saveButton.addEventListener('click', async () => {
        const portVal     = parseInt(document.getElementById('server-port-input').value);
        const intervalVal = parseInt(document.getElementById('sync-interval').value);

        if (portVal < 1024 || portVal > 65535) {
            showMessage('error', 'サーバーポートは 1024〜65535 の範囲で設定してください。');
            return;
        }
        if (intervalVal < 1) {
            showMessage('error', '同期間隔は 1 分以上で設定してください。');
            return;
        }

        // Password: send empty string if not edited (backend keeps existing)
        let newPassword = '';
        if (isPasswordEditable && passwordInput.value) {
            newPassword = passwordInput.value;
        }

        const newConfig = {
            apiSettings: {
                baseUrl:  document.getElementById('api-url').value,
                username: document.getElementById('api-username').value,
                password: newPassword
            },
            serverSettings: { port: portVal },
            systemSettings: {
                runOnStartup: document.getElementById('run-on-startup').checked,
                startHidden:  document.getElementById('start-hidden').checked
            },
            syncSettings: {
                autoSyncEnabled:     document.getElementById('auto-sync-enabled').checked,
                syncIntervalMinutes: intervalVal,
                syncOnStartup:       document.getElementById('sync-on-startup').checked,
                syncTargets: {
                    shops:     document.getElementById('sync-target-shops').checked,
                    shopNews:  document.getElementById('sync-target-shop-news').checked,
                    eventNews: document.getElementById('sync-target-event-news').checked,
                    specials:  document.getElementById('sync-target-specials').checked,
                    sales:     document.getElementById('sync-target-sales').checked,
                    shopApp:   document.getElementById('sync-target-shop-app').checked,
                    genres:    document.getElementById('sync-target-genres').checked,
                    floors:    document.getElementById('sync-target-floors').checked
                }
            }
        };

        try {
            await bridgeApi.saveConfig(newConfig);
            currentConfig = await bridgeApi.getConfig();
            loadSettings(currentConfig);
            updateLocalApiUrlDisplay(portVal);
            showMessage('success', '設定が保存されました。アプリの再起動が必要な場合があります。');
            setTimeout(() => showTab('main'), 1200);
        } catch (err) {
            showMessage('error', '設定の保存に失敗しました。');
            console.error(err);
        }
    });

    // --- Log View ---
    let allLogEntries = [];
    let activeLevel   = 'ALL';

    const logContainer   = document.getElementById('log-container');
    const logRefreshBtn  = document.getElementById('log-refresh-btn');
    const logDownloadBtn = document.getElementById('log-download-btn');
    const logCountEl     = document.getElementById('log-count');
    const filterButtons  = document.querySelectorAll('.log-filter-btn');
    const logDateFrom    = document.getElementById('log-date-from');
    const logDateTo      = document.getElementById('log-date-to');

    // Default date range: today
    const todayStr = new Date().toISOString().slice(0, 10);
    const minDate  = (() => {
        const d = new Date();
        d.setDate(d.getDate() - 30);
        return d.toISOString().slice(0, 10);
    })();
    logDateFrom.value = todayStr;
    logDateTo.value   = todayStr;
    logDateFrom.min   = minDate;
    logDateTo.min     = minDate;
    logDateFrom.max   = todayStr;
    logDateTo.max     = todayStr;

    // Keep from <= to
    logDateFrom.addEventListener('change', () => {
        if (logDateFrom.value > logDateTo.value) logDateTo.value = logDateFrom.value;
        loadLogs();
    });
    logDateTo.addEventListener('change', () => {
        if (logDateTo.value < logDateFrom.value) logDateFrom.value = logDateTo.value;
        loadLogs();
    });

    filterButtons.forEach(btn => {
        btn.addEventListener('click', () => {
            activeLevel = btn.dataset.level;
            filterButtons.forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            renderLogs();
        });
    });

    logRefreshBtn.addEventListener('click', loadLogs);

    logDownloadBtn.addEventListener('click', () => {
        if (allLogEntries.length === 0) return;
        const lines = allLogEntries.map(e =>
            (e.timestamp && e.level && e.tag)
                ? `[${e.timestamp}] [${e.level}] [${e.tag}] ${e.message}`
                : (e.message || '')
        );
        const blob = new Blob([lines.join('\n')], { type: 'text/plain; charset=utf-8' });
        const url  = URL.createObjectURL(blob);
        const a    = document.createElement('a');
        a.href     = url;
        const from = logDateFrom.value;
        const to   = logDateTo.value;
        a.download = from === to
            ? `bridge-ground-${from}.log`
            : `bridge-ground-${from}_to_${to}.log`;
        a.click();
        URL.revokeObjectURL(url);
    });

    async function loadLogs() {
        logContainer.innerHTML = '<div class="log-empty">読み込み中...</div>';
        try {
            allLogEntries = await bridgeApi.getLogs(logDateFrom.value, logDateTo.value);
            renderLogs();
        } catch (e) {
            logContainer.innerHTML = '<div class="log-empty">ログの読み込みに失敗しました。</div>';
        }
    }

    function renderLogs() {
        const filtered = activeLevel === 'ALL'
            ? allLogEntries
            : allLogEntries.filter(e => {
                if (activeLevel === 'ERROR') return e.level === 'ERROR' || e.level === 'FATAL';
                return e.level === activeLevel;
            });

        logCountEl.textContent = `${filtered.length} 件`;

        if (filtered.length === 0) {
            logContainer.innerHTML = '<div class="log-empty">該当するログエントリがありません。</div>';
            return;
        }

        const html = filtered.map(e => {
            const lvl    = (e.level || '').toUpperCase();
            const cls    = lvl ? `log-level-${lvl.toLowerCase()}` : '';
            const ts     = e.timestamp ? `<span class="log-ts">${esc(e.timestamp)}</span>` : '';
            const badge  = lvl ? `<span class="log-level-badge">[${esc(lvl)}]</span>` : '';
            const tag    = e.tag ? `<span class="log-tag">[${esc(e.tag)}]</span>` : '';
            const msg    = `<span class="log-msg">${esc(e.message || '')}</span>`;
            return `<div class="log-entry ${cls}">${ts}${badge}${tag}${msg}</div>`;
        }).join('');

        logContainer.innerHTML = html;
        logContainer.scrollTop = logContainer.scrollHeight;
    }

    function esc(s) {
        return String(s)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;');
    }
});
