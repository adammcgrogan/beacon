package net.trybeacon.plugin;

import org.bukkit.Bukkit;
import org.bukkit.plugin.java.JavaPlugin;
import org.bukkit.scheduler.BukkitTask;
import net.trybeacon.plugin.logging.WebSocketLogAppender;
import net.trybeacon.plugin.tasks.ServerStatsTask;
import net.trybeacon.plugin.websocket.BackendClient;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.net.URI;
import java.net.URISyntaxException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.util.concurrent.TimeUnit;

public class BeaconPlugin extends JavaPlugin {

    private static final long RECONNECT_INTERVAL_TICKS = 100L; // 5 seconds
    private static final int DEFAULT_BACKEND_PORT = 8080;

    private BackendClient webSocketClient;
    private WebSocketLogAppender logAppender;
    private BukkitTask statsTask;
    private BukkitTask reconnectTask;
    private Process backendProcess;
    private Thread backendOutputThread;
    private int backendPort;
    private volatile boolean shuttingDown;
    private volatile boolean connectionAttemptInFlight;

    @Override
    public void onEnable() {
        saveDefaultConfig();
        backendPort = getConfig().getInt("backend.port", DEFAULT_BACKEND_PORT);

        shuttingDown = false;
        connectionAttemptInFlight = false;

        if (!startBackendProcess()) {
            getLogger().severe("Failed to start embedded Go backend process. Disabling plugin.");
            Bukkit.getPluginManager().disablePlugin(this);
            return;
        }

        getLogger().info("Beacon Plugin is starting! Attempting to connect to Go backend on port " + backendPort + "...");
        connectToWebSocket();
        startReconnectLoop();
    }

    @Override
    public void onDisable() {
        shuttingDown = true;
        connectionAttemptInFlight = false;
        stopReconnectLoop();
        stopStreamingTasks();

        if (webSocketClient != null && !webSocketClient.isClosed() && !webSocketClient.isClosing()) {
            webSocketClient.close();
        }

        stopBackendProcess();
        getLogger().info("Beacon Plugin disabled. Connection closed.");
    }

    private synchronized void connectToWebSocket() {
        if (shuttingDown) return;
        if (connectionAttemptInFlight) return;

        if (webSocketClient != null && (webSocketClient.isOpen() || webSocketClient.isClosing())) {
            return;
        }

        try {
            URI serverUri = new URI("ws://localhost:" + backendPort + "/ws");
            webSocketClient = new BackendClient(serverUri, this);
            connectionAttemptInFlight = true;
            webSocketClient.connect();
        } catch (URISyntaxException e) {
            connectionAttemptInFlight = false;
            getLogger().severe("Invalid WebSocket URI: " + e.getMessage());
        }
    }

    /**
     * Called by BackendClient when a connection is successfully opened.
     */
    public synchronized void onBackendConnected(BackendClient client) {
        if (shuttingDown) return;
        if (client != webSocketClient) return;

        connectionAttemptInFlight = false;
        stopReconnectLoop();
        stopStreamingTasks();

        logAppender = new WebSocketLogAppender(client);
        logAppender.attach();

        statsTask = Bukkit.getScheduler().runTaskTimer(this, new ServerStatsTask(client), 0L, 40L);
    }

    /**
     * Called by BackendClient when the socket closes.
     */
    public synchronized void onBackendDisconnected(BackendClient client) {
        if (client != webSocketClient) return;
        connectionAttemptInFlight = false;
        stopStreamingTasks();

        if (!shuttingDown) {
            getLogger().warning("Backend disconnected. Reconnect loop active.");
            startReconnectLoop();
        }
    }

    public synchronized void onBackendError(BackendClient client, Exception ex) {
        if (client != webSocketClient) return;
        connectionAttemptInFlight = false;
    }

    private synchronized void startReconnectLoop() {
        if (shuttingDown) return;
        if (reconnectTask != null && !reconnectTask.isCancelled()) return;

        reconnectTask = Bukkit.getScheduler().runTaskTimer(this, () -> {
            if (shuttingDown) return;
            if (!isBackendProcessRunning() && !startBackendProcess()) {
                getLogger().warning("Backend process is not running and restart failed.");
                return;
            }
            if (webSocketClient != null && webSocketClient.isOpen()) return;
            connectToWebSocket();
        }, 0L, RECONNECT_INTERVAL_TICKS);
    }

    private synchronized void stopReconnectLoop() {
        if (reconnectTask != null) {
            reconnectTask.cancel();
            reconnectTask = null;
        }
    }

    private synchronized void stopStreamingTasks() {
        if (statsTask != null) {
            statsTask.cancel();
            statsTask = null;
        }

        if (logAppender != null) {
            logAppender.detach();
            logAppender = null;
        }
    }

    private boolean startBackendProcess() {
        String binaryName = getPackagedBackendBinaryName();
        if (binaryName == null) {
            getLogger().severe("Unsupported host platform for embedded backend: os=" + System.getProperty("os.name")
                    + ", arch=" + System.getProperty("os.arch"));
            return false;
        }

        Path backendDir = getDataFolder().toPath().resolve("backend");
        Path binaryPath = backendDir.resolve(binaryName);
        String resourcePath = "backend/" + binaryName;

        try {
            Files.createDirectories(backendDir);
            try (InputStream binaryStream = getResource(resourcePath)) {
                if (binaryStream == null) {
                    getLogger().severe("Backend binary not found in JAR at " + resourcePath);
                    return false;
                }
                Files.copy(binaryStream, binaryPath, StandardCopyOption.REPLACE_EXISTING);
            }

            if (!ensureExecutable(binaryPath)) {
                getLogger().severe("Unable to mark backend binary executable: " + binaryPath);
                return false;
            }

            ProcessBuilder processBuilder = new ProcessBuilder(
                    binaryPath.toAbsolutePath().toString(),
                    "--port",
                    String.valueOf(backendPort)
            );
            processBuilder.directory(getDataFolder());
            processBuilder.redirectErrorStream(true);
            backendProcess = processBuilder.start();
            startBackendLogPump();
            return true;
        } catch (IOException ex) {
            getLogger().severe("Unable to start Go backend process: " + ex.getMessage());
            return false;
        }
    }

    private void startBackendLogPump() {
        if (backendProcess == null) return;

        backendOutputThread = new Thread(() -> {
            try (BufferedReader reader = new BufferedReader(new InputStreamReader(backendProcess.getInputStream()))) {
                String line;
                while ((line = reader.readLine()) != null) {
                    getLogger().info("[backend] " + line);
                }
            } catch (IOException ignored) {
            }
        }, "beacon-backend-log-pump");
        backendOutputThread.setDaemon(true);
        backendOutputThread.start();
    }

    private void stopBackendProcess() {
        if (backendProcess == null) return;

        if (backendProcess.isAlive()) {
            backendProcess.destroy();
            try {
                if (!backendProcess.waitFor(3, TimeUnit.SECONDS)) {
                    backendProcess.destroyForcibly();
                }
            } catch (InterruptedException ex) {
                Thread.currentThread().interrupt();
                backendProcess.destroyForcibly();
            }
        }

        if (backendOutputThread != null && backendOutputThread.isAlive()) {
            backendOutputThread.interrupt();
            backendOutputThread = null;
        }

        backendProcess = null;
    }

    private boolean isBackendProcessRunning() {
        return backendProcess != null && backendProcess.isAlive();
    }

    private boolean isWindows() {
        return System.getProperty("os.name").toLowerCase().contains("win");
    }

    private String getPackagedBackendBinaryName() {
        String os = normalizeOperatingSystem();
        String arch = normalizeArchitecture();

        if (os == null || arch == null) {
            return null;
        }

        return "beacon-backend-" + os + "-" + arch + (os.equals("windows") ? ".exe" : "");
    }

    private String normalizeOperatingSystem() {
        String rawOs = System.getProperty("os.name").toLowerCase();
        if (rawOs.contains("linux")) return "linux";
        if (rawOs.contains("mac") || rawOs.contains("darwin")) return "darwin";
        if (rawOs.contains("win")) return "windows";
        return null;
    }

    private String normalizeArchitecture() {
        String rawArch = System.getProperty("os.arch").toLowerCase();
        if (rawArch.equals("amd64") || rawArch.equals("x86_64")) return "amd64";
        if (rawArch.equals("arm64") || rawArch.equals("aarch64")) return "arm64";
        return null;
    }

    private boolean ensureExecutable(Path binaryPath) {
        if (isWindows()) {
            return true;
        }

        try {
            Process chmodProcess = new ProcessBuilder("chmod", "+x", binaryPath.toAbsolutePath().toString())
                    .redirectErrorStream(true)
                    .start();

            boolean completed = chmodProcess.waitFor(5, TimeUnit.SECONDS);
            if (!completed) {
                chmodProcess.destroyForcibly();
                return false;
            }

            return chmodProcess.exitValue() == 0 && Files.isExecutable(binaryPath);
        } catch (IOException | InterruptedException ex) {
            if (ex instanceof InterruptedException) {
                Thread.currentThread().interrupt();
            }
            return false;
        }
    }
}
