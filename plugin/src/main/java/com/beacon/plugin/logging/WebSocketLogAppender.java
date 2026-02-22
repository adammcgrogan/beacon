package com.beacon.plugin.logging;

import com.beacon.plugin.util.ProtocolBuilder;
import com.google.gson.JsonObject;
import org.apache.logging.log4j.LogManager;
import org.apache.logging.log4j.core.LogEvent;
import org.apache.logging.log4j.core.Logger;
import org.apache.logging.log4j.core.appender.AbstractAppender;
import org.java_websocket.client.WebSocketClient;

import java.text.SimpleDateFormat;
import java.util.Date;

public class WebSocketLogAppender extends AbstractAppender {

    private final WebSocketClient client;
    private final SimpleDateFormat timeFormat = new SimpleDateFormat("HH:mm:ss");

    public WebSocketLogAppender(WebSocketClient client) {
        super("BeaconLogAppender", null, null, false, null);
        this.client = client;
    }

    public void attach() {
        this.start();
        ((Logger) LogManager.getRootLogger()).addAppender(this);
    }

    public void detach() {
        ((Logger) LogManager.getRootLogger()).removeAppender(this);
        this.stop();
    }

    @Override
    public void append(LogEvent event) {
        if (client == null || !client.isOpen()) return;

        String logLevel = event.getLevel().name();
        String message = stripAnsiCodes(event.getMessage().getFormattedMessage());
        String timestamp = timeFormat.format(new Date(event.getTimeMillis()));
        
        String logLine = String.format("[%s %s]: %s", timestamp, logLevel, message);
        
        JsonObject payload = new JsonObject();
        payload.addProperty("level", logLevel);
        payload.addProperty("message", logLine);
        
        client.send(ProtocolBuilder.buildEvent("console_log", payload));
    }

    private String stripAnsiCodes(String message) {
        return message.replaceAll("\\x1B\\[[0-9;]*[a-zA-Z]", "");
    }
}