package com.beacon.plugin.tasks;

import com.beacon.plugin.util.ProtocolBuilder;
import com.google.gson.JsonArray;
import com.google.gson.JsonObject;
import org.bukkit.Bukkit;
import org.bukkit.entity.Player;
import org.java_websocket.client.WebSocketClient;

public class ServerStatsTask implements Runnable {

    private final WebSocketClient client;

    public ServerStatsTask(WebSocketClient client) {
        this.client = client;
    }

    @Override
    public void run() {
        if (client == null || !client.isOpen()) return;

        JsonObject payload = new JsonObject();
        payload.addProperty("players", Bukkit.getOnlinePlayers().size());
        payload.addProperty("max_players", Bukkit.getMaxPlayers());
        payload.addProperty("tps", String.format("%.2f", Math.min(20.0, Bukkit.getServer().getTPS()[0])));
        
        Runtime runtime = Runtime.getRuntime();
        payload.addProperty("ram_used", (runtime.totalMemory() - runtime.freeMemory()) / 1048576L);
        payload.addProperty("ram_max", runtime.maxMemory() / 1048576L);
        
        payload.add("player_list", buildPlayerList());

        client.send(ProtocolBuilder.buildEvent("server_stats", payload));
    }

    private JsonArray buildPlayerList() {
        JsonArray playerArray = new JsonArray();
        for (Player p : Bukkit.getOnlinePlayers()) {
            JsonObject player = new JsonObject();
            player.addProperty("name", p.getName());
            player.addProperty("uuid", p.getUniqueId().toString());
            player.addProperty("ping", p.getPing()); 
            playerArray.add(player);
        }
        return playerArray;
    }
}