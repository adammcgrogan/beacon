package com.beacon.plugin;

import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import org.bukkit.Bukkit;
import org.java_websocket.client.WebSocketClient;
import org.java_websocket.handshake.ServerHandshake;

import java.net.URI;

public class BackendWebSocketClient extends WebSocketClient {
    
    private final BeaconPlugin plugin;

    public BackendWebSocketClient(URI serverUri, BeaconPlugin plugin) {
        super(serverUri);
        this.plugin = plugin;
    }

    @Override
    public void onOpen(ServerHandshake handshakedata) {
        plugin.getLogger().info("✅ Connected successfully to Go Backend!");
        plugin.startLogCapture();

        // Start sending server stats every 2 seconds
        ServerStatsTask statsTask = new ServerStatsTask(this);
        Bukkit.getScheduler().runTaskTimerAsynchronously(plugin, statsTask, 0L, 40L);
    }

    @Override
    public void onMessage(String message) { 
        try {
            // Parse the incoming JSON message from the Go backend
            JsonObject json = JsonParser.parseString(message).getAsJsonObject();
            
            if (json.has("event") && json.get("event").getAsString().equals("console_command")) {
                String command = json.get("command").getAsString();

                // Pass the command to the main server thread to execute safely!
                Bukkit.getScheduler().runTask(plugin, () -> {
                    Bukkit.getServer().dispatchCommand(Bukkit.getConsoleSender(), command);
                });
            }
        } catch (Exception e) {
            // Ignore messages that aren't valid JSON
        }
    }

    @Override
    public void onClose(int code, String reason, boolean remote) {
        plugin.getLogger().warning("❌ Disconnected from backend. Reason: " + reason);
    }

    @Override
    public void onError(Exception ex) {
        plugin.getLogger().severe("⚠️ WebSocket error: " + ex.getMessage());
    }
}