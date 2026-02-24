package net.trybeacon.plugin.websocket;

import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import com.google.gson.JsonArray;
import net.trybeacon.plugin.BeaconPlugin;
import net.trybeacon.plugin.files.FileManagerService;
import org.bukkit.Bukkit;
import org.bukkit.command.CommandMap;
import org.java_websocket.client.WebSocketClient;
import org.java_websocket.handshake.ServerHandshake;

import java.net.URI;
import java.util.List;

public class BackendClient extends WebSocketClient {
    
    private final BeaconPlugin plugin;
    private final FileManagerService fileManagerService;

    public BackendClient(URI serverUri, BeaconPlugin plugin) {
        super(serverUri);
        this.plugin = plugin;
        this.fileManagerService = new FileManagerService(plugin);
    }

    @Override
    public void onOpen(ServerHandshake handshakeData) {
        plugin.getLogger().info("✅ Connected successfully to Go Backend!");
        plugin.onBackendConnected(this);

        JsonObject envPayload = new JsonObject();
        envPayload.addProperty("software", Bukkit.getName() + " " + Bukkit.getVersion());
        envPayload.addProperty("java", "Java " + System.getProperty("java.version"));
        envPayload.addProperty("os", System.getProperty("os.name") + " (" + System.getProperty("os.arch") + ")");

        JsonObject envJson = new JsonObject();
        envJson.addProperty("event", "server_env");
        envJson.add("payload", envPayload);

        this.send(envJson.toString());
    }

    @Override
    public void onMessage(String message) { 
        try {
            JsonObject json = JsonParser.parseString(message).getAsJsonObject();
            String event = json.has("event") ? json.get("event").getAsString() : "";
            
            // Check if the Go backend is sending us a command from the web UI
            if (event.equals("console_command")) {
                String command = json.get("command").getAsString();

                // Commands MUST be run on the main Server Thread
                Bukkit.getScheduler().runTask(plugin, () -> {
                    Bukkit.getServer().dispatchCommand(Bukkit.getConsoleSender(), command);
                });
            }

            if (event.equals("console_tab_complete")) {
                String requestId = json.has("request_id") ? json.get("request_id").getAsString() : "";
                String command = json.has("command") ? json.get("command").getAsString() : "";

                Bukkit.getScheduler().runTask(plugin, () -> {
                    List<String> completions = getCompletions(command);

                    JsonObject payload = new JsonObject();
                    payload.addProperty("request_id", requestId);
                    payload.addProperty("command", command);

                    JsonArray completionsArray = new JsonArray();
                    for (String completion : completions) {
                        completionsArray.add(completion);
                    }
                    payload.add("completions", completionsArray);

                    JsonObject envelope = new JsonObject();
                    envelope.addProperty("event", "console_tab_complete_result");
                    envelope.add("payload", payload);
                    send(envelope.toString());
                });
            }

            if (event.equals("world_action")) {
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

            if (event.equals("file_manager_request")) {
                JsonObject payload = json.getAsJsonObject("payload");
                Bukkit.getScheduler().runTaskAsynchronously(plugin, () -> handleFileManagerRequest(payload));
            }

        } catch (Exception e) {
            // Ignore messages that aren't valid JSON
        }
    }

    @Override
    public void onClose(int code, String reason, boolean remote) {
        plugin.getLogger().warning("❌ Disconnected from backend. Reason: " + reason);
        plugin.onBackendDisconnected(this);
    }

    @Override
    public void onError(Exception ex) {
        plugin.getLogger().severe("⚠️ WebSocket error: " + ex.getMessage());
        plugin.onBackendError(this, ex);
    }

    private void handleFileManagerRequest(JsonObject payload) {
        JsonObject responsePayload = new JsonObject();
        String requestId = payload.has("request_id") ? payload.get("request_id").getAsString() : "";
        responsePayload.addProperty("request_id", requestId);

        try {
            String action = payload.get("action").getAsString();
            String rawPath = payload.has("path") ? payload.get("path").getAsString() : "";
            String content = payload.has("content") ? payload.get("content").getAsString() : "";
            JsonObject data = fileManagerService.performAction(action, rawPath, content);

            responsePayload.addProperty("ok", true);
            responsePayload.add("data", data);
        } catch (Exception ex) {
            responsePayload.addProperty("ok", false);
            responsePayload.addProperty("error", ex.getMessage() == null ? "file operation failed" : ex.getMessage());
        }

        JsonObject envelope = new JsonObject();
        envelope.addProperty("event", "file_manager_response");
        envelope.add("payload", responsePayload);
        this.send(envelope.toString());
    }

    private List<String> getCompletions(String commandLine) {
        try {
            CommandMap commandMap = Bukkit.getServer().getCommandMap();
            List<String> results = commandMap.tabComplete(Bukkit.getConsoleSender(), commandLine);
            return results != null ? results : List.of();
        } catch (Exception ex) {
            plugin.getLogger().fine("Tab completion unavailable: " + ex.getMessage());
        }

        return List.of();
    }
}
