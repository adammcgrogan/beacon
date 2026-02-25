package net.trybeacon.plugin.tasks;

import com.google.gson.JsonArray;
import com.google.gson.JsonObject;
import org.bukkit.Bukkit;
import org.bukkit.GameRule;
import org.bukkit.Registry;
import org.bukkit.Statistic;
import org.bukkit.entity.Player;
import org.java_websocket.client.WebSocketClient;

import java.io.File;

public class ServerStatsTask implements Runnable {

    private final WebSocketClient webSocketClient;

    public ServerStatsTask(WebSocketClient webSocketClient) {
        this.webSocketClient = webSocketClient;
    }

    @Override
    @SuppressWarnings("removal")
    public void run() {
        if (webSocketClient == null || !webSocketClient.isOpen()) {
            return;
        }

        JsonObject payload = new JsonObject();
        payload.addProperty("players", Bukkit.getOnlinePlayers().size());
        payload.addProperty("max_players", Bukkit.getMaxPlayers());
        payload.addProperty("tps", String.format("%.2f", Math.min(20.0, Bukkit.getServer().getTPS()[0])));
        
        Runtime runtime = Runtime.getRuntime();
        payload.addProperty("ram_used", (runtime.totalMemory() - runtime.freeMemory()) / 1048576L);
        payload.addProperty("ram_max", runtime.maxMemory() / 1048576L);

        JsonArray playerArray = new JsonArray();
        for (Player p : Bukkit.getOnlinePlayers()) {
            JsonObject playerObj = new JsonObject();
            playerObj.addProperty("name", p.getName());
            playerObj.addProperty("uuid", p.getUniqueId().toString());
            playerObj.addProperty("ping", p.getPing()); 
            playerObj.addProperty("first_join", p.getFirstPlayed());
            playerObj.addProperty("playtime", p.getStatistic(Statistic.PLAY_ONE_MINUTE)); 
            playerObj.addProperty("world", p.getWorld().getName());
            playerArray.add(playerObj);
        }
        
        JsonArray worldArray = new JsonArray();
        
        // LOADED worlds
        for (org.bukkit.World w : Bukkit.getWorlds()) {
            JsonObject worldObj = new JsonObject();
            worldObj.addProperty("name", w.getName());
            worldObj.addProperty("environment", w.getEnvironment().name());
            worldObj.addProperty("loaded", true);
            worldObj.addProperty("players", w.getPlayers().size());
            worldObj.addProperty("chunks", w.getLoadedChunks().length);
            worldObj.addProperty("entities", w.getEntities().size());
            worldObj.addProperty("time", w.getTime());
            worldObj.addProperty("storming", w.hasStorm());
            worldObj.addProperty("difficulty", w.getDifficulty().name());
            worldObj.addProperty("seed", String.valueOf(w.getSeed()));
            
            // Current Gamerules
            JsonObject gamerulesObj = new JsonObject();
            for (GameRule<?> rule : Registry.GAME_RULE) {
                try {
                    Object val = w.getGameRuleValue(rule);
                    if (val != null) {
                        gamerulesObj.addProperty(rule.getName(), String.valueOf(val));
                    }
                } catch (IllegalArgumentException e) {
                    // Ignore gamerules that are registered globally but invalid for this specific world
                }
            }
            worldObj.add("gamerules", gamerulesObj);
            
            worldArray.add(worldObj);
        }

        // UNLOADED worlds
        File worldContainer = Bukkit.getWorldContainer();
        File[] files = worldContainer.listFiles();
        if (files != null) {
            for (File file : files) {
                if (file.isDirectory() && new File(file, "level.dat").exists()) {
                    String folderName = file.getName();
                    if (Bukkit.getWorld(folderName) == null) {
                        JsonObject worldObj = new JsonObject();
                        worldObj.addProperty("name", folderName);
                        worldObj.addProperty("loaded", false);
                        worldObj.addProperty("environment", "UNKNOWN");
                        worldObj.addProperty("difficulty", "N/A");
                        worldObj.addProperty("seed", "N/A");
                        worldObj.add("gamerules", new JsonObject());
                        worldArray.add(worldObj);
                    }
                }
            }
        }

        // Default Gamerules dynamically from Bukkit Registry
        JsonObject defaultGamerulesObj = new JsonObject();
        if (!Bukkit.getWorlds().isEmpty()) {
            org.bukkit.World firstWorld = Bukkit.getWorlds().get(0);
            for (GameRule<?> rule : Registry.GAME_RULE) {
                try {
                    Object defaultValue = firstWorld.getGameRuleDefault(rule);
                    if (defaultValue != null) {
                        defaultGamerulesObj.addProperty(rule.getName(), String.valueOf(defaultValue));
                    }
                } catch (IllegalArgumentException e) {
                    // Ignore gamerules that are registered globally but invalid for this specific world
                }
            }
        }
        payload.add("default_gamerules", defaultGamerulesObj);
        
        // Send world stats packet
        JsonObject worldJson = new JsonObject();
        worldJson.addProperty("event", "world_stats");
        worldJson.add("payload", worldArray);
        webSocketClient.send(worldJson.toString());

        payload.add("player_list", playerArray);

        // Send server stats packet
        JsonObject rootJson = new JsonObject();
        rootJson.addProperty("event", "server_stats");
        rootJson.add("payload", payload);

        webSocketClient.send(rootJson.toString());
    }
}