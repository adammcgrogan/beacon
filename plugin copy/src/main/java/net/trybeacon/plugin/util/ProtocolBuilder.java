package net.trybeacon.plugin.util;

import com.google.gson.JsonObject;

public class ProtocolBuilder {
    
    /** Helper to easily build the standard event envelope */
    public static String buildEvent(String eventName, JsonObject payload) {
        JsonObject root = new JsonObject();
        root.addProperty("event", eventName);
        root.add("payload", payload);
        return root.toString();
    }
}