package net.trybeacon.plugin;

import org.bukkit.Bukkit;
import org.bukkit.plugin.java.JavaPlugin;
import org.bukkit.scheduler.BukkitTask;
import net.trybeacon.plugin.logging.WebSocketLogAppender;
import net.trybeacon.plugin.tasks.ServerStatsTask;
import net.trybeacon.plugin.websocket.BackendClient;

import java.net.URI;
import java.net.URISyntaxException;

public class BeaconPlugin extends JavaPlugin {

    private static final long RECONNECT_INTERVAL_TICKS = 100L; // 5 seconds

    private BackendClient webSocketClient;
    private WebSocketLogAppender logAppender;
    private BukkitTask statsTask;
    private BukkitTask reconnectTask;
    private volatile boolean shuttingDown;
    private volatile boolean connectionAttemptInFlight;

    @Override
    public void onEnable() {
        shuttingDown = false;
        connectionAttemptInFlight = false;
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
            URI serverUri = new URI("ws://localhost:8080/ws");
            webSocketClient = new BackendClient(serverUri, this);
            connectionAttemptInFlight = true;
            webSocketClient.connect();
        } catch (URISyntaxException e) {
            connectionAttemptInFlight = false;
            getLogger().severe("Invalid WebSocket URI: " + e.getMessage());
        }
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