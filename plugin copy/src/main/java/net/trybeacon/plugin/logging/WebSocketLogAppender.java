package net.trybeacon.plugin.logging;

import com.google.gson.JsonObject;

import net.trybeacon.plugin.util.ProtocolBuilder;

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
        
        // Grab the message and strip out any hidden terminal codes
        String rawMessage = event.getMessage().getFormattedMessage();
        String cleanMessage = stripAnsiCodes(rawMessage);
        
        String timestamp = timeFormat.format(new Date(event.getTimeMillis()));
        
        // Simple, clean text line
        String logLine = String.format("[%s %s]: %s", timestamp, logLevel, cleanMessage);
        
        JsonObject payload = new JsonObject();
        payload.addProperty("level", logLevel);
        payload.addProperty("message", logLine);
        
        client.send(ProtocolBuilder.buildEvent("console_log", payload));
    }

    /**
     * Removes terminal ANSI escape codes from a string to keep the web UI clean.
     */
    private String stripAnsiCodes(String message) {
        if (message == null) return "";
        return message.replaceAll("\\x1B\\[[0-9;]*[a-zA-Z]", "");
    }
}