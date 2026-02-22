package com.mcdash.plugin;

import org.java_websocket.client.WebSocketClient;
import org.java_websocket.handshake.ServerHandshake;
import java.net.URI;
import org.bukkit.Bukkit;

public class BackendWebSocketClient extends WebSocketClient {
    
    private final MCDashPlugin plugin;

    /**
     * Constructs a new WebSocket client to communicate with the Go backend.
     * * @param serverUri The URI of the Go backend WebSocket endpoint.
     * @param plugin    The main plugin instance, used for logging and callbacks.
     */
    public BackendWebSocketClient(URI serverUri, MCDashPlugin plugin) {
        super(serverUri);
        this.plugin = plugin;
    }

    /**
     * Triggered automatically when the WebSocket connection is successfully established.
     * Starts the log capture process now that we have a place to send the logs.
     *
     * @param handshakedata Information about the server handshake.
     */
    @Override
    public void onOpen(ServerHandshake handshakedata) {
        plugin.getLogger().info("✅ Connected successfully to Go Backend!");
        plugin.startLogCapture();

        ServerStatsTask statsTask = new ServerStatsTask(this);
        Bukkit.getScheduler().runTaskTimerAsynchronously(plugin, statsTask, 0L, 40L);
    }

    /**
     * Triggered when a message is received from the Go backend.
     * Currently unused as the plugin only sends data, but required by the interface.
     *
     * @param message The string message received from the server.
     */
    @Override
    public void onMessage(String message) { 
        // No incoming messages from backend expected yet
    }

    /**
     * Triggered when the connection to the Go backend is closed.
     * * @param code   The HTTP status code.
     * @param reason The reason for the disconnection.
     * @param remote Whether the disconnection was initiated by the remote host.
     */
    @Override
    public void onClose(int code, String reason, boolean remote) {
        plugin.getLogger().warning("❌ Disconnected from backend. Reason: " + reason);
    }

    /**
     * Triggered when an error occurs in the WebSocket connection.
     * * @param ex The exception representing the error.
     */
    @Override
    public void onError(Exception ex) {
        plugin.getLogger().severe("⚠️ WebSocket error: " + ex.getMessage());
    }
}