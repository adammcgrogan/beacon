package net.trybeacon.plugin.tasks;

import com.google.gson.JsonArray;
import com.google.gson.JsonObject;
import org.bukkit.Bukkit;
import org.bukkit.Statistic; // Added for playtime
import org.bukkit.entity.Player;
import org.java_websocket.client.WebSocketClient;

public class ServerStatsTask implements Runnable {

    private final WebSocketClient webSocketClient;

    public ServerStatsTask(WebSocketClient webSocketClient) {
        this.webSocketClient = webSocketClient;
    }

    @Override
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
            
            playerObj.addProperty("first_join", p.getFirstPlayed()); // Timestamp in ms
            playerObj.addProperty("playtime", p.getStatistic(Statistic.PLAY_ONE_MINUTE)); 
            playerObj.addProperty("world", p.getWorld().getName());

            playerArray.add(playerObj);
        }
        
        JsonArray worldArray = new JsonArray();
        for (org.bukkit.World w : Bukkit.getWorlds()) {
            JsonObject worldObj = new JsonObject();
            worldObj.addProperty("name", w.getName());
            worldObj.addProperty("environment", w.getEnvironment().name());
            worldObj.addProperty("players", w.getPlayers().size());
            worldObj.addProperty("chunks", w.getLoadedChunks().length);
            worldObj.addProperty("entities", w.getEntities().size());
            worldObj.addProperty("time", w.getTime());
            worldObj.addProperty("storming", w.hasStorm());
            worldArray.add(worldObj);
        }
        
        // Send a second packet right after server_stats
        JsonObject worldJson = new JsonObject();
        worldJson.addProperty("event", "world_stats");
        worldJson.add("payload", worldArray);
        webSocketClient.send(worldJson.toString());

        payload.add("player_list", playerArray);

        JsonObject rootJson = new JsonObject();
        rootJson.addProperty("event", "server_stats");
        rootJson.add("payload", payload);

        webSocketClient.send(rootJson.toString());
    }
}