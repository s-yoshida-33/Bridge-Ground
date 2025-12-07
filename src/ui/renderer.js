// src/ui/renderer.js
document.addEventListener('DOMContentLoaded', async () => {
    // IPC communication via contextBridge in preload.js (assumed)
    const bridgeApi = window.bridgeApi; 

    // --- State & Initialization ---
    const config = await bridgeApi.getConfig();
    
    document.getElementById('server-port').textContent = config.serverSettings.port;
    document.getElementById('server-port-link').textContent = config.serverSettings.port;
    document.getElementById('api-link').href = `http://localhost:${config.serverSettings.port}/api/shops`;

    // --- Event Listeners ---
    
    // Manual Sync Button
    document.getElementById('sync-button').addEventListener('click', async () => {
        document.getElementById('sync-status').textContent = '同期を開始しました...';
        document.getElementById('sync-button').disabled = true;

        try {
            const result = await bridgeApi.startManualSync(); // Call main process
            if (result.success) {
                document.getElementById('sync-status').textContent = '✅ 同期が完了しました。';
                // TODO: Refresh data counts after successful sync
            } else {
                document.getElementById('sync-status').textContent = `❌ 同期失敗: ${result.message}`;
            }
        } catch (error) {
            document.getElementById('sync-status').textContent = `❌ 同期中にエラーが発生しました。`;
            console.error(error);
        } finally {
            document.getElementById('sync-button').disabled = false;
        }
    });

    // Settings Button (Simplified: just opens a log message for now)
    document.getElementById('settings-button').addEventListener('click', () => {
        alert('設定画面のロジックを実装する必要があります。');
    });

    // TODO: Implement logic to fetch and display actual data counts from the DB via main process.
});