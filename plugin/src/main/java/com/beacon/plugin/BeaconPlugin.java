package com.beacon.plugin;

import com.beacon.plugin.logging.WebSocketLogAppender;
import com.beacon.plugin.websocket.BackendClient;
import org.bukkit.plugin.java.JavaPlugin;

import java.net.URI;
import java.net.URISyntaxException;

public class BeaconPlugin extends JavaPlugin {
    
    private BackendClient webSocketClient;
    private WebSocketLogAppender logAppender;

    @Override
    public void onEnable() {
        getLogger().info("Beacon Plugin is starting! Attempting to connect to Go backend...");
        connectToWebSocket();
    }

    @Override
    public void onDisable() {
        // 1. Stop listening to the console logs
        if (logAppender != null) {
            logAppender.detach();
        }

        // 2. Safely close the WebSocket connection
        if (webSocketClient != null && !webSocketClient.isClosed()) {
            webSocketClient.close();
        }
        
        getLogger().info("Beacon Plugin disabled. Connection closed.");
    }

    private void connectToWebSocket() {
        try {
            URI serverUri = new URI("ws://localhost:8080/ws");
            webSocketClient = new BackendClient(serverUri, this);
            webSocketClient.connect();
        } catch (URISyntaxException e) {
            getLogger().severe("Invalid WebSocket URI: " + e.getMessage());
        }
    }

    /**
     * Called by the BackendClient once a connection is successfully opened.
     */
    public void startLogCapture() {
        logAppender = new WebSocketLogAppender(webSocketClient);
        logAppender.attach();
    }
}