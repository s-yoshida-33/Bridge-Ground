// src/ui/renderer.js
document.addEventListener('DOMContentLoaded', async () => {
    const bridgeApi = window.bridgeApi; 

    const mainView = document.getElementById('main-view');
    const settingsView = document.getElementById('settings-view');
    const settingsButton = document.getElementById('settings-button');
    const mainButton = document.getElementById('main-button');
    const saveButton = document.getElementById('save-settings-button');
    const syncButton = document.getElementById('sync-button');
    // const syncStatus = document.getElementById('sync-status'); // Removed in favor of detailed status
    const msgDiv = document.getElementById('messages');
    
    // New Progress Elements
    const progressWrapper = document.getElementById('sync-progress-wrapper');
    const mainProgressBar = document.getElementById('main-progress-bar');
    // const subProgressBar = document.getElementById('sub-progress-bar'); // Removed
    const mainStatusText = document.getElementById('main-status-text');
    // const subStatusText = document.getElementById('sub-status-text'); // Removed

    const localApiUrlInput = document.getElementById('local-api-url');
    const copyApiUrlButton = document.getElementById('copy-api-url-button');
    const copyMessage = document.getElementById('copy-message');

    // Password Toggle Elements
    const passwordInput = document.getElementById('api-password');
    const togglePasswordButton = document.getElementById('toggle-password-button');
    
    let currentConfig = await bridgeApi.getConfig();
    let isPasswordEditable = false;

    // --- Version Display ---
    try {
        const version = await bridgeApi.getAppVersion();
        const verElem = document.getElementById('app-version');
        if (verElem) {
            verElem.textContent = version;
        }
    } catch (e) {
        console.error("Failed to get app version:", e);
    }

    // --- Password Toggle Handler ---
    togglePasswordButton.addEventListener('click', () => {
        isPasswordEditable = !isPasswordEditable;
        
        if (isPasswordEditable) {
            // Edit mode: Enable input, show plain text with current password
            passwordInput.disabled = false;
            passwordInput.type = 'text'; // Show text to allow checking what is typed
            passwordInput.value = currentConfig.apiSettings.password || ''; // Show actual password
            passwordInput.placeholder = '新しいパスワードを入力';
            togglePasswordButton.textContent = 'キャンセル';
            passwordInput.focus();
        } else {
            // View/Protected mode: Disable input, mask it, restore dummy dots if saved
            passwordInput.disabled = true;
            passwordInput.type = 'password';
            togglePasswordButton.textContent = '編集';
            
            // Restore visual state based on whether a password is truly saved
            if (currentConfig.apiSettings.password) {
                passwordInput.value = '********'; // Dummy visual
            } else {
                passwordInput.value = '';
                passwordInput.placeholder = '未設定';
            }
        }
    });

    // --- Initialize UI with Config Data ---
    function updateLocalApiUrlDisplay(port) {
        localApiUrlInput.value = `http://localhost:${port}`;
    }
    
    // Initial display
    updateLocalApiUrlDisplay(currentConfig.serverSettings.port);

    // --- Copy to Clipboard Handler ---
    copyApiUrlButton.addEventListener('click', () => {
        localApiUrlInput.select();
        localApiUrlInput.setSelectionRange(0, 99999); // For mobile devices
        navigator.clipboard.writeText(localApiUrlInput.value).then(() => {
            copyMessage.style.display = 'block';
            setTimeout(() => {
                copyMessage.style.display = 'none';
            }, 2000);
        }).catch(err => {
            console.error('Failed to copy: ', err);
        });
    });

    // --- Sync Progress Listener ---
    bridgeApi.onSyncProgress((data) => {
        progressWrapper.style.display = 'block';
        
        if (data.main) {
            mainProgressBar.style.width = `${data.main.percentage}%`;
            mainProgressBar.textContent = `${data.main.percentage}%`;
            mainStatusText.textContent = data.main.message;
        }
        
        // if (data.sub) {
        //     subProgressBar.style.width = `${data.sub.percentage}%`;
        //     subProgressBar.textContent = `${data.sub.percentage}%`;
        //     subStatusText.textContent = data.sub.message;
        // }
        
        // If sync is complete (100%), we can re-enable button after a short delay
        if (data.main && data.main.percentage === 100) {
            setTimeout(() => {
                syncButton.disabled = false;
                
                // Refresh counts with stats if available
                updateDataCounts(data.main.stats);
                
                // Hide progress indicator after completion
                setTimeout(() => {
                    progressWrapper.style.display = 'none';
                    // Reset bars for next time
                    mainProgressBar.style.width = '0%';
                    // subProgressBar.style.width = '0%';
                    // subStatusText.textContent = '';
                }, 2000);

            }, 1000);
        }
    });

    // --- View Toggle ---
    
    function showView(viewId) {
        mainView.style.display = (viewId === 'main') ? 'block' : 'none';
        settingsView.style.display = (viewId === 'settings') ? 'block' : 'none';
        settingsButton.style.display = (viewId === 'main') ? 'block' : 'none';
        mainButton.style.display = (viewId === 'settings') ? 'block' : 'none';
    }

    settingsButton.addEventListener('click', () => {
        loadSettings(currentConfig); // Load current config when opening settings
        showView('settings');
    });

    mainButton.addEventListener('click', () => {
        showView('main');
        // Refresh dynamic UI elements here if needed
    });

    // Start on the main view
    showView('main'); 
    
    // --- Configuration Handlers ---

    function loadSettings(config) {
        document.getElementById('api-url').value = config.apiSettings.baseUrl;
        document.getElementById('api-username').value = config.apiSettings.username;
        
        // Reset password UI state
        isPasswordEditable = false;
        passwordInput.disabled = true;
        passwordInput.type = 'password';
        togglePasswordButton.textContent = '編集';

        if (config.apiSettings.password) {
            passwordInput.value = '********'; // Dummy visual for saved password
            passwordInput.placeholder = ""; 
        } else {
            passwordInput.value = '';
            passwordInput.placeholder = "未設定";
        }
        
        document.getElementById('server-port-input').value = config.serverSettings.port;
        document.getElementById('auto-sync-enabled').checked = config.syncSettings.autoSyncEnabled;
        document.getElementById('sync-interval').value = config.syncSettings.syncIntervalMinutes;
        document.getElementById('sync-on-startup').checked = config.syncSettings.syncOnStartup;

        // Sync Targets
        const targets = config.syncSettings.syncTargets || {
            shops: true, shopNews: true, eventNews: true, specials: true, genres: true
        };
        document.getElementById('sync-target-shops').checked = targets.shops;
        document.getElementById('sync-target-shop-news').checked = targets.shopNews;
        document.getElementById('sync-target-event-news').checked = targets.eventNews;
        document.getElementById('sync-target-specials').checked = targets.specials;
        document.getElementById('sync-target-genres').checked = targets.genres;
    }

    function showMessage(type, text) {
        msgDiv.className = type;
        msgDiv.textContent = text;
        msgDiv.style.display = 'block';
        setTimeout(() => { msgDiv.style.display = 'none'; }, 5000);
    }
    
    saveButton.addEventListener('click', async () => {
        const portInput = document.getElementById('server-port-input').value;
        const intervalInput = document.getElementById('sync-interval').value;
        
        // Basic input validation
        if (portInput < 1024 || portInput > 65535) {
            showMessage('error', 'サーバーポートは1024から65535の範囲で設定してください。');
            return;
        }
        if (intervalInput < 1) {
            showMessage('error', '同期間隔は1分以上で設定してください。');
            return;
        }

        const newConfig = {
            apiSettings: {
                baseUrl: document.getElementById('api-url').value,
                username: document.getElementById('api-username').value,
                // Use new password if editable mode was active and value is not empty, otherwise keep old
                password: (isPasswordEditable && passwordInput.value) ? passwordInput.value : currentConfig.apiSettings.password 
            },
            serverSettings: {
                port: parseInt(portInput)
            },
            syncSettings: {
                autoSyncEnabled: document.getElementById('auto-sync-enabled').checked,
                syncIntervalMinutes: parseInt(intervalInput),
                syncOnStartup: document.getElementById('sync-on-startup').checked,
                syncTargets: {
                    shops: document.getElementById('sync-target-shops').checked,
                    shopNews: document.getElementById('sync-target-shop-news').checked,
                    eventNews: document.getElementById('sync-target-event-news').checked,
                    specials: document.getElementById('sync-target-specials').checked,
                    genres: document.getElementById('sync-target-genres').checked
                }
            }
        };

        try {
            await bridgeApi.saveConfig(newConfig);
            currentConfig = newConfig; // Update in-memory config
            
            // Update UI to reflect saved state (e.g. password placeholder)
            loadSettings(newConfig);

            // Update main view elements affected by port change
            updateLocalApiUrlDisplay(newConfig.serverSettings.port);

            showMessage('success', '設定が保存されました。アプリの再起動が必要な場合があります。');
            
            // Navigate back to main screen after save
            setTimeout(() => {
                showView('main');
            }, 1000);

        } catch (error) {
            showMessage('error', '設定の保存に失敗しました。');
            console.error("Save config error:", error);
        }
    });

    // --- Manual Sync Handler ---

    syncButton.addEventListener('click', async () => {
        syncButton.disabled = true;
        progressWrapper.style.display = 'block';
        mainProgressBar.style.width = '0%';
        mainProgressBar.textContent = '0%';
        // subProgressBar.style.width = '0%';
        // subProgressBar.textContent = '0%';
        mainStatusText.textContent = '同期を開始します...';
        // subStatusText.textContent = '';

        try {
            const result = await bridgeApi.startManualSync(); 
            if (!result.success) {
                 mainStatusText.textContent = `同期失敗: ${result.message}`;
                 syncButton.disabled = false;
            }
            // Success case is handled by progress events finishing up
        } catch (error) {
            mainStatusText.textContent = `同期中にエラーが発生しました。`;
            console.error(error);
            syncButton.disabled = false;
        }
    });

    // --- Data Counts Refresh (New Functionality) ---
    async function updateDataCounts(latestStats = null) {
        try {
            const counts = await bridgeApi.getDataCounts(); // IPC Call to get counts
            
            const formatCount = (total, key) => {
                // If latestStats exists, verify the key exists AND the value is greater than 0
                // JavaScript objects: key in latestStats checks for existence
                if (latestStats && (key in latestStats)) {
                    const diff = latestStats[key];
                    if (diff !== 0) {
                         const sign = diff > 0 ? '+' : '';
                         return `${total} (今回: ${sign}${diff})`;
                    }
                }
                return total;
            };

            document.getElementById('count-shops').textContent = formatCount(counts.shops, 'shops');
            // Mapping DB keys (snake_case from IPC) to SyncStats keys (camelCase from SyncManager)
            document.getElementById('count-shop-news').textContent = formatCount(counts.shop_news, 'shopNews');
            document.getElementById('count-event-news').textContent = formatCount(counts.event_news, 'eventNews');
            document.getElementById('count-specials').textContent = formatCount(counts.specials, 'specials');
            
        } catch (error) {
            console.error("Failed to fetch data counts:", error);
        }
    }
    
    // Initial data load when app starts
    updateDataCounts();
});