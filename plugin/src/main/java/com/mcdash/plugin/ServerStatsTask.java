package com.mcdash.plugin;

import com.google.gson.JsonObject;
import org.bukkit.Bukkit;
import org.java_websocket.client.WebSocketClient;

public class ServerStatsTask implements Runnable {

    private final WebSocketClient webSocketClient;

    public ServerStatsTask(WebSocketClient webSocketClient) {
        this.webSocketClient = webSocketClient;
    }

    @Override
    public void run() {
        // Only send stats if we are actually connected to Go
        if (webSocketClient == null || !webSocketClient.isOpen()) {
            return;
        }

        // 1. Gather Players
        int onlinePlayers = Bukkit.getOnlinePlayers().size();
        int maxPlayers = Bukkit.getMaxPlayers();

        // 2. Gather TPS (Ticks Per Second - index 0 is the last 1 minute average)
        double tps = Math.min(20.0, Bukkit.getServer().getTPS()[0]);
        String formattedTps = String.format("%.2f", tps);

        // 3. Gather RAM Usage (in MB)
        Runtime runtime = Runtime.getRuntime();
        long usedMemory = (runtime.totalMemory() - runtime.freeMemory()) / 1048576L;
        long maxMemory = runtime.maxMemory() / 1048576L;

        // 4. Package it into a JSON Payload
        JsonObject payload = new JsonObject();
        payload.addProperty("players", onlinePlayers);
        payload.addProperty("max_players", maxPlayers);
        payload.addProperty("tps", formattedTps);
        payload.addProperty("ram_used", usedMemory);
        payload.addProperty("ram_max", maxMemory);

        JsonObject rootJson = new JsonObject();
        rootJson.addProperty("event", "server_stats");
        rootJson.add("payload", payload);

        // 5. Send to Go!
        webSocketClient.send(rootJson.toString());
    }
}