package com.mcdash.plugin;

import org.bukkit.plugin.java.JavaPlugin;
import java.net.URI;
import java.net.URISyntaxException;

public class MCDashPlugin extends JavaPlugin {
    
    private BackendWebSocketClient webSocketClient;
    private WebSocketLogAppender logAppender;

    /**
     * Called by the Bukkit server when the plugin is enabled.
     * This is the entry point of the plugin where we initiate the backend connection.
     */
    @Override
    public void onEnable() {
        getLogger().info("MCDash Plugin is starting! Attempting to connect to Go backend...");
        connectToWebSocket();
    }

    /**
     * Called by the Bukkit server when the plugin is disabled (e.g., server shutdown).
     * Ensures that we cleanly detach our log listener and close the WebSocket connection
     * to prevent memory leaks or hanging connections.
     */
    @Override
    public void onDisable() {
        if (logAppender != null) {
            logAppender.detach();
        }

        if (webSocketClient != null && !webSocketClient.isClosed()) {
            webSocketClient.close();
        }
        
        getLogger().info("MCDash Plugin disabled. Connection closed.");
    }

    /**
     * Attempts to establish a WebSocket connection to the Go backend server.
     * If the URI is invalid, it logs a severe error.
     */
    private void connectToWebSocket() {
        try {
            URI serverUri = new URI("ws://localhost:8080/ws");
            webSocketClient = new BackendWebSocketClient(serverUri, this);
            webSocketClient.connect();
        } catch (URISyntaxException e) {
            getLogger().severe("Invalid WebSocket URI: " + e.getMessage());
        }
    }

    /**
     * Initializes the custom Log4j appender and attaches it to the root logger.
     * This is called by the WebSocket client once a successful connection is made.
     */
    public void startLogCapture() {
        logAppender = new WebSocketLogAppender(webSocketClient);
        logAppender.attach();
    }
}