package net.trybeacon.plugin;

import net.trybeacon.plugin.commands.BeaconCommand;
import net.trybeacon.plugin.permissions.VaultPermissionService;
import org.bukkit.Bukkit;
import org.bukkit.configuration.file.FileConfiguration;
import org.bukkit.configuration.file.YamlConfiguration;
import org.bukkit.plugin.java.JavaPlugin;
import org.bukkit.scheduler.BukkitTask;
import net.trybeacon.plugin.logging.WebSocketLogAppender;
import net.trybeacon.plugin.tasks.ServerStatsTask;
import net.trybeacon.plugin.websocket.BackendClient;

import java.io.File;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.net.URI;
import java.net.URISyntaxException;
import java.nio.charset.StandardCharsets;
import java.util.HashSet;
import java.util.Set;

public class BeaconPlugin extends JavaPlugin {

    private static final long RECONNECT_INTERVAL_TICKS = 100L; // 5 seconds

    private BackendClient webSocketClient;
    private WebSocketLogAppender logAppender;
    private BukkitTask statsTask;
    private BukkitTask reconnectTask;
    private volatile boolean shuttingDown;
    private volatile boolean connectionAttemptInFlight;
    private String backendWebSocketUrl;
    private String backendPublicUrl;
    private int panelTokenExpirySeconds;
    private VaultPermissionService vaultPermissionService;

    @Override
    public void onEnable() {
        shuttingDown = false;
        connectionAttemptInFlight = false;
        saveDefaultConfig();
        syncConfig();
        loadConfig();
        vaultPermissionService = new VaultPermissionService(this);
        vaultPermissionService.initialize();
        registerCommands();
        getLogger().info("Beacon Plugin is starting! Attempting to connect to Go backend...");
        connectToWebSocket();
        startReconnectLoop();
    }

    @Override
    public void onDisable() {
        shuttingDown = true;
        connectionAttemptInFlight = false;
        stopReconnectLoop();
        stopStreamingTasks();

        if (webSocketClient != null && !webSocketClient.isClosed() && !webSocketClient.isClosing()) {
            webSocketClient.close();
        }

        getLogger().info("Beacon Plugin disabled. Connection closed.");
    }

    private synchronized void connectToWebSocket() {
        if (shuttingDown) return;
        if (connectionAttemptInFlight) return;

        if (webSocketClient != null && (webSocketClient.isOpen() || webSocketClient.isClosing())) {
            return;
        }

        try {
            URI serverUri = new URI(backendWebSocketUrl);
            webSocketClient = new BackendClient(serverUri, this);
            connectionAttemptInFlight = true;
            webSocketClient.connect();
        } catch (URISyntaxException e) {
            connectionAttemptInFlight = false;
            getLogger().severe("Invalid WebSocket URI: " + e.getMessage());
        }
    }

    private void loadConfig() {
        backendWebSocketUrl = getConfig().getString("backend.websocket-url", "ws://localhost:8080/ws");
        backendPublicUrl = getConfig().getString("backend.public-url", "http://localhost:8080");
        panelTokenExpirySeconds = Math.max(30, getConfig().getInt("auth.token-expiration-seconds", 300));
    }

    private void syncConfig() {
        File configFile = new File(getDataFolder(), "config.yml");
        FileConfiguration liveConfig = getConfig();

        try (InputStream stream = getResource("config.yml")) {
            if (stream == null) {
                getLogger().warning("Could not load default config.yml for migration.");
                return;
            }

            YamlConfiguration defaultConfig = YamlConfiguration.loadConfiguration(
                    new InputStreamReader(stream, StandardCharsets.UTF_8)
            );

            boolean hasUpdated = false;
            Set<String> defaultKeys = defaultConfig.getKeys(true);
            Set<String> liveKeys = new HashSet<>(liveConfig.getKeys(true));

            for (String key : defaultKeys) {
                if (!liveConfig.contains(key)) {
                    liveConfig.set(key, defaultConfig.get(key));
                    hasUpdated = true;
                }
            }

            for (String key : liveKeys) {
                if (!defaultConfig.contains(key)) {
                    liveConfig.set(key, null);
                    hasUpdated = true;
                }
            }

            if (hasUpdated) {
                liveConfig.save(configFile);
                reloadConfig();
                getLogger().info("Updated config.yml to match latest defaults.");
            }
        } catch (Exception ex) {
            getLogger().severe("Failed to migrate config.yml: " + ex.getMessage());
            ex.printStackTrace();
        }
    }

    private void registerCommands() {
        if (getCommand("beacon") != null) {
            BeaconCommand command = new BeaconCommand(this);
            getCommand("beacon").setExecutor(command);
            getCommand("beacon").setTabCompleter(command);
        }
    }

    public BackendClient getBackendClient() {
        return webSocketClient;
    }

    public String getBackendPublicUrl() {
        return backendPublicUrl;
    }

    public int getPanelTokenExpirySeconds() {
        return panelTokenExpirySeconds;
    }

    public VaultPermissionService getVaultPermissionService() {
        return vaultPermissionService;
    }

    /**
     * Called by BackendClient when a connection is successfully opened.
     */
    public synchronized void onBackendConnected(BackendClient client) {
        if (shuttingDown) return;
        if (client != webSocketClient) return;

        connectionAttemptInFlight = false;
        stopReconnectLoop();
        stopStreamingTasks();

        logAppender = new WebSocketLogAppender(client);
        logAppender.attach();

        statsTask = Bukkit.getScheduler().runTaskTimer(this, new ServerStatsTask(client), 0L, 40L);
    }

    /**
     * Called by BackendClient when the socket closes.
     */
    public synchronized void onBackendDisconnected(BackendClient client) {
        if (client != webSocketClient) return;
        connectionAttemptInFlight = false;
        stopStreamingTasks();

        if (!shuttingDown) {
            getLogger().warning("Backend disconnected. Reconnect loop active.");
            startReconnectLoop();
        }
    }

    public synchronized void onBackendError(BackendClient client, Exception ex) {
        if (client != webSocketClient) return;
        connectionAttemptInFlight = false;
    }

    private synchronized void startReconnectLoop() {
        if (shuttingDown) return;
        if (reconnectTask != null && !reconnectTask.isCancelled()) return;

        reconnectTask = Bukkit.getScheduler().runTaskTimer(this, () -> {
            if (shuttingDown) return;
            if (webSocketClient != null && webSocketClient.isOpen()) return;
            connectToWebSocket();
        }, 0L, RECONNECT_INTERVAL_TICKS);
    }

    private synchronized void stopReconnectLoop() {
        if (reconnectTask != null) {
            reconnectTask.cancel();
            reconnectTask = null;
        }
    }

    private synchronized void stopStreamingTasks() {
        if (statsTask != null) {
            statsTask.cancel();
            statsTask = null;
        }

        if (logAppender != null) {
            logAppender.detach();
            logAppender = null;
        }
    }
}