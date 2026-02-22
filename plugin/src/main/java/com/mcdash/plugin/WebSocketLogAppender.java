package com.mcdash.plugin;

import com.google.gson.JsonObject;
import org.apache.logging.log4j.LogManager;
import org.apache.logging.log4j.core.LogEvent;
import org.apache.logging.log4j.core.Logger;
import org.apache.logging.log4j.core.appender.AbstractAppender;
import org.java_websocket.client.WebSocketClient;

import java.text.SimpleDateFormat;
import java.util.Date;

public class WebSocketLogAppender extends AbstractAppender {

    private final WebSocketClient webSocketClient;
    private final SimpleDateFormat timeFormat;

    /**
     * Constructs a custom Log4j Appender that forwards log events to a WebSocket client.
     * * @param webSocketClient The connected client ready to send JSON payloads.
     */
    public WebSocketLogAppender(WebSocketClient webSocketClient) {
        super("MCDashLogAppender", null, null, false, null);
        this.webSocketClient = webSocketClient;
        this.timeFormat = new SimpleDateFormat("HH:mm:ss");
    }

    /**
     * Registers this appender with the server's root logger.
     * Once called, this class will start intercepting all console logs.
     */
    public void attach() {
        this.start();
        ((Logger) LogManager.getRootLogger()).addAppender(this);
    }

    /**
     * Removes this appender from the server's root logger and stops it.
     * Crucial for clean shutdowns to prevent memory leaks.
     */
    public void detach() {
        ((Logger) LogManager.getRootLogger()).removeAppender(this);
        this.stop();
    }

    /**
     * The core method called by Log4j every time a log is generated.
     * It formats the log message, strips color codes, and sends it to the backend.
     * * @param event The log event containing the message, level, and timestamp.
     */
    @Override
    public void append(LogEvent event) {
        // Ensure the WebSocket is open before trying to send anything
        if (webSocketClient == null || !webSocketClient.isOpen()) {
            return;
        }

        String logLevel = event.getLevel().name();
        String rawMessage = event.getMessage().getFormattedMessage();
        String cleanMessage = stripAnsiCodes(rawMessage);
        String timestamp = timeFormat.format(new Date(event.getTimeMillis()));
        
        String formattedLogLine = String.format("[%s %s]: %s", timestamp, logLevel, cleanMessage);
        
        sendPayload(logLevel, formattedLogLine);
    }

    /**
     * Removes terminal ANSI escape codes (like colors) from a string.
     * * @param message The raw console string that might contain color codes.
     * @return A plain text string safe for displaying on the web.
     */
    private String stripAnsiCodes(String message) {
        return message.replaceAll("\\x1B\\[[0-9;]*[a-zA-Z]", "");
    }

    /**
     * Wraps the log data into a JSON structure and sends it over the WebSocket.
     * * @param level   The severity level of the log (e.g., INFO, ERROR).
     * @param message The fully formatted, plain-text log line.
     */
    private void sendPayload(String level, String message) {
        JsonObject payload = new JsonObject();
        payload.addProperty("level", level);
        payload.addProperty("message", message);
        
        JsonObject rootJson = new JsonObject();
        rootJson.addProperty("event", "console_log");
        rootJson.add("payload", payload);
        
        webSocketClient.send(rootJson.toString());
    }
}