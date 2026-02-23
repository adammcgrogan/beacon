package net.trybeacon.plugin.websocket;

import com.google.gson.JsonObject;
import com.google.gson.JsonParser;

import net.trybeacon.plugin.BeaconPlugin;
import net.trybeacon.plugin.tasks.ServerStatsTask;

import org.bukkit.Bukkit;
import org.java_websocket.client.WebSocketClient;
import org.java_websocket.handshake.ServerHandshake;

import java.net.URI;

public class BackendClient extends WebSocketClient {
    
    private final BeaconPlugin plugin;

    public BackendClient(URI serverUri, BeaconPlugin plugin) {
        super(serverUri);
        this.plugin = plugin;
    }

    @Override
    public void onOpen(ServerHandshake handshakedata) {
        plugin.getLogger().info("✅ Connected successfully to Go Backend!");
        
        // Tell the main plugin class to start capturing logs now that we are connected
        plugin.startLogCapture();

        // Start the repeating task to send server stats every 2 seconds (40 ticks)
        ServerStatsTask statsTask = new ServerStatsTask(this);
        Bukkit.getScheduler().runTaskTimer(plugin, statsTask, 0L, 40L);
    }

    @Override
    public void onMessage(String message) { 
        try {
            JsonObject json = JsonParser.parseString(message).getAsJsonObject();
            
            // Check if the Go backend is sending us a command from the web UI
            if (json.has("event") && json.get("event").getAsString().equals("console_command")) {
                String command = json.get("command").getAsString();

                // Commands MUST be run on the main Server Thread
                Bukkit.getScheduler().runTask(plugin, () -> {
                    Bukkit.getServer().dispatchCommand(Bukkit.getConsoleSender(), command);
                });
            }

            if (json.has("event") && json.get("event").getAsString().equals("world_action")) {
                JsonObject payload = json.getAsJsonObject("payload");
                String action = payload.get("action").getAsString();
                String worldName = payload.get("world").getAsString();
                
                Bukkit.getScheduler().runTask(plugin, () -> {
                    org.bukkit.World world = Bukkit.getWorld(worldName);
                    if (world == null) return;
                    
                    switch (action) {
                        case "set_day":
                            world.setTime(1000);
                            break;
                        case "set_night":
                            world.setTime(13000);
                            break;
                        case "toggle_weather":
                            world.setStorm(!world.hasStorm());
                            if (!world.hasStorm()) world.setWeatherDuration(0);
                            break;
                    }
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