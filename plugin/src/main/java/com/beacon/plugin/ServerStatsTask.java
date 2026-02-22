package com.beacon.plugin;

import com.google.gson.JsonArray;
import com.google.gson.JsonObject;
import org.bukkit.Bukkit;
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

        int onlinePlayers = Bukkit.getOnlinePlayers().size();
        int maxPlayers = Bukkit.getMaxPlayers();
        double tps = Math.min(20.0, Bukkit.getServer().getTPS()[0]);
        String formattedTps = String.format("%.2f", tps);

        Runtime runtime = Runtime.getRuntime();
        long usedMemory = (runtime.totalMemory() - runtime.freeMemory()) / 1048576L;
        long maxMemory = runtime.maxMemory() / 1048576L;

        JsonArray playerArray = new JsonArray();
        for (Player p : Bukkit.getOnlinePlayers()) {
            JsonObject playerObj = new JsonObject();
            playerObj.addProperty("name", p.getName());
            playerObj.addProperty("uuid", p.getUniqueId().toString());
            playerObj.addProperty("ping", p.getPing()); 
            playerArray.add(playerObj);
        }

        JsonObject payload = new JsonObject();
        payload.addProperty("players", onlinePlayers);
        payload.addProperty("max_players", maxPlayers);
        payload.addProperty("tps", formattedTps);
        payload.addProperty("ram_used", usedMemory);
        payload.addProperty("ram_max", maxMemory);
        
        payload.add("player_list", playerArray);

        JsonObject rootJson = new JsonObject();
        rootJson.addProperty("event", "server_stats");
        rootJson.add("payload", payload);

        webSocketClient.send(rootJson.toString());
    }
}