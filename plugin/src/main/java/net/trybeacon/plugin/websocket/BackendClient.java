package net.trybeacon.plugin.websocket;

import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import com.google.gson.JsonArray;
import net.trybeacon.plugin.BeaconPlugin;
import net.trybeacon.plugin.files.FileManagerService;
import net.trybeacon.plugin.permissions.VaultPermissionService;
import org.bukkit.Bukkit;
import org.bukkit.command.CommandMap;
import org.bukkit.WorldCreator;
import org.bukkit.entity.Player;
import org.bukkit.permissions.PermissionAttachmentInfo;
import org.java_websocket.client.WebSocketClient;
import org.java_websocket.handshake.ServerHandshake;

import java.io.File;
import java.net.URI;
import java.util.ArrayList;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.UUID;

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

        JsonObject pathPayload = new JsonObject();
        pathPayload.addProperty("plugin_data_dir", plugin.getDataFolder().getAbsolutePath());

        JsonObject pathJson = new JsonObject();
        pathJson.addProperty("event", "plugin_paths");
        pathJson.add("payload", pathPayload);
        this.send(pathJson.toString());
    }

    @Override
    public void onMessage(String message) { 
        try {
            JsonObject json = JsonParser.parseString(message).getAsJsonObject();
            String event = json.has("event") ? json.get("event").getAsString() : "";
            
            if (event.equals("console_command")) {
                String command = json.get("command").getAsString();
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
                    if (action.equals("load")) {
                        WorldCreator creator = new WorldCreator(worldName);
                        
                        // Automatically detect dimension types to prevent portal breakage
                        if (worldName.endsWith("_nether")) {
                            creator.environment(org.bukkit.World.Environment.NETHER);
                        } else if (worldName.endsWith("_the_end")) {
                            creator.environment(org.bukkit.World.Environment.THE_END);
                        }
                        
                        Bukkit.getScheduler().runTask(plugin, () -> {
                            Bukkit.createWorld(creator);
                        });
                        return;
                    }
                    
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
                        case "save":
                            world.save();
                            break;
                        case "set_gamerule":
                            if (payload.has("rule") && payload.has("value")) {
                                world.setGameRuleValue(payload.get("rule").getAsString(), payload.get("value").getAsString());
                            }
                            break;
                        case "unload":
                            evacuateWorld(world);
                            Bukkit.unloadWorld(world, true);
                            break;
                        case "reset":
                            // 1. Prevent resetting the primary world
                            if (world.equals(Bukkit.getWorlds().get(0))) {
                                plugin.getLogger().warning("❌ Cannot reset the primary world while the server is running.");
                                // Optionally send a message back to the frontend here
                                return;
                            }

                            // 2. Safely teleport players out
                            evacuateWorld(world);
                            File worldFolder = world.getWorldFolder();
                            
                            // 3. Attempt to unload. If it fails, ABORT.
                            boolean unloaded = Bukkit.unloadWorld(world, false); 
                            if (!unloaded) {
                                plugin.getLogger().severe("❌ Failed to unload world '" + worldName + "'. Aborting reset to prevent corruption.");
                                return;
                            }
                            
                            // 4. Delete folder asynchronously ONLY if unload was successful
                            Bukkit.getScheduler().runTaskAsynchronously(plugin, () -> {
                                deleteDirectory(worldFolder);
                                
                                // 5. Re-create on main thread
                                Bukkit.getScheduler().runTask(plugin, () -> {
                                    WorldCreator creator = new WorldCreator(worldName);
                                    if (worldName.endsWith("_nether")) {
                                        creator.environment(org.bukkit.World.Environment.NETHER);
                                    } else if (worldName.endsWith("_the_end")) {
                                        creator.environment(org.bukkit.World.Environment.THE_END);
                                    }
                                    Bukkit.createWorld(creator);
                                    plugin.getLogger().info("✅ Dimension '" + worldName + "' has been successfully reset.");
                                });
                            });
                            break;
                    }
                });
            }

            if (event.equals("file_manager_request")) {
                JsonObject payload = json.getAsJsonObject("payload");
                Bukkit.getScheduler().runTaskAsynchronously(plugin, () -> handleFileManagerRequest(payload));
            }

            if (event.equals("player_permissions_request")) {
                JsonObject payload = json.getAsJsonObject("payload");
                Bukkit.getScheduler().runTask(plugin, () -> handlePlayerPermissionsRequest(payload));
            }

            if (event.equals("permission_admin_request")) {
                JsonObject payload = json.getAsJsonObject("payload");
                Bukkit.getScheduler().runTask(plugin, () -> handlePermissionAdminRequest(payload));
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

    private void evacuateWorld(org.bukkit.World targetWorld) {
        org.bukkit.World mainWorld = Bukkit.getWorlds().get(0);
        for (Player p : targetWorld.getPlayers()) {
            if (mainWorld != null && !mainWorld.equals(targetWorld)) {
                p.teleport(mainWorld.getSpawnLocation());
            } else {
                p.kickPlayer("World is restarting or unloading.");
            }
        }
    }

    private void deleteDirectory(File path) {
        if (path.exists()) {
            File[] files = path.listFiles();
            if (files != null) {
                for (File file : files) {
                    if (file.isDirectory()) {
                        deleteDirectory(file);
                    } else {
                        file.delete();
                    }
                }
            }
            path.delete();
        }
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

    private void handlePlayerPermissionsRequest(JsonObject payload) {
        String requestId = payload.has("request_id") ? payload.get("request_id").getAsString() : "";
        String playerUUID = payload.has("player_uuid") ? payload.get("player_uuid").getAsString() : "";

        JsonObject responsePayload = new JsonObject();
        responsePayload.addProperty("request_id", requestId);
        responsePayload.addProperty("player_uuid", playerUUID);

        Player player = null;
        try {
            player = Bukkit.getPlayer(java.util.UUID.fromString(playerUUID));
        } catch (IllegalArgumentException ignored) {
            // invalid uuid
        }

        if (player == null) {
            responsePayload.addProperty("online", false);
            responsePayload.add("permissions", new JsonArray());
        } else {
            responsePayload.addProperty("online", true);
            JsonArray permissions = new JsonArray();
            if (player.isOp()) {
                permissions.add("beacon.access.*");
                permissions.add("beacon.panel");
            }
            for (PermissionAttachmentInfo perm : player.getEffectivePermissions()) {
                if (!perm.getValue()) continue;
                permissions.add(perm.getPermission().toLowerCase(Locale.ROOT));
            }
            responsePayload.add("permissions", permissions);
        }

        sendEvent("player_permissions_response", responsePayload);
    }

    public boolean sendEvent(String eventName, JsonObject payload) {
        if (!this.isOpen()) {
            return false;
        }
        JsonObject envelope = new JsonObject();
        envelope.addProperty("event", eventName);
        envelope.add("payload", payload);
        this.send(envelope.toString());
        return true;
    }

    private void handlePermissionAdminRequest(JsonObject payload) {
        String requestId = payload.has("request_id") ? payload.get("request_id").getAsString() : "";
        String action = payload.has("action") ? payload.get("action").getAsString() : "";
        String playerUUIDRaw = payload.has("player_uuid") ? payload.get("player_uuid").getAsString() : "";
        String playerName = payload.has("player_name") ? payload.get("player_name").getAsString() : "";

        JsonObject responsePayload = new JsonObject();
        responsePayload.addProperty("request_id", requestId);
        responsePayload.addProperty("action", action);
        responsePayload.addProperty("player_uuid", playerUUIDRaw);

        VaultPermissionService permissionService = plugin.getVaultPermissionService();
        if (permissionService == null || !permissionService.isReady()) {
            responsePayload.addProperty("ok", false);
            responsePayload.addProperty("error", "vault permission provider unavailable");
            sendEvent("permission_admin_response", responsePayload);
            return;
        }

        UUID playerUUID = null;
        if (!playerUUIDRaw.isBlank()) {
            try {
                playerUUID = UUID.fromString(playerUUIDRaw);
            } catch (IllegalArgumentException ignored) {
                playerUUID = null;
            }
        }

        try {
            if ("snapshot".equals(action)) {
                JsonArray permissionNodes = payload.has("permission_nodes") ? payload.getAsJsonArray("permission_nodes") : new JsonArray();
                List<String> nodes = new ArrayList<>();
                for (int i = 0; i < permissionNodes.size(); i++) {
                    nodes.add(permissionNodes.get(i).getAsString().toLowerCase(Locale.ROOT));
                }
                Map<String, Boolean> snapshot = permissionService.snapshotPermissions(playerUUID, playerName, nodes);

                JsonObject permissionsJson = new JsonObject();
                for (Map.Entry<String, Boolean> entry : snapshot.entrySet()) {
                    permissionsJson.addProperty(entry.getKey(), entry.getValue());
                }

                responsePayload.addProperty("ok", true);
                responsePayload.add("permissions", permissionsJson);
                sendEvent("permission_admin_response", responsePayload);
                return;
            }

            if ("set".equals(action)) {
                String permissionNode = payload.has("permission_node") ? payload.get("permission_node").getAsString().toLowerCase(Locale.ROOT) : "";
                boolean enabled = payload.has("enabled") && payload.get("enabled").getAsBoolean();

                boolean changed = permissionService.setPermission(playerUUID, playerName, permissionNode, enabled);
                responsePayload.addProperty("ok", changed);
                if (!changed) {
                    responsePayload.addProperty("error", "permission backend rejected update");
                }
                sendEvent("permission_admin_response", responsePayload);
                return;
            }

            responsePayload.addProperty("ok", false);
            responsePayload.addProperty("error", "unsupported permission admin action");
            sendEvent("permission_admin_response", responsePayload);
        } catch (Exception ex) {
            responsePayload.addProperty("ok", false);
            responsePayload.addProperty("error", ex.getMessage() == null ? "permission action failed" : ex.getMessage());
            sendEvent("permission_admin_response", responsePayload);
        }
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
