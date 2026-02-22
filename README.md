# Beacon

**Beacon** is a lightweight, real-time web monitoring and management suite for Minecraft servers. It provides server administrators with a live tap into their server's console, health metrics, and player activity through a sleek, modern web dashboard.

---

## Current Features

### Live Console & Command Execution
* **Real-Time Streaming**: Console logs are streamed instantly from the server to your browser.
* **Log History**: The backend maintains a buffer of the last 1,000 log lines, so you see the full context even after refreshing the page.
* **Two-Way Communication**: Send commands directly to the Minecraft console from the web interface.

### Server Health Dashboard
* **Live Metrics**: Real-time tracking of Server TPS, RAM usage, and player counts.

### Player List
* **Online Overview**: A dedicated tab showing every player currently connected to the server.
* **Player Data**: View real-time player pings and unique UUIDs.
* **Visual Avatars**: Automatically pulls 3D player heads using the Crafatar API.

---

## Roadmap (Upcoming Features)

* **User Authentication**: Secure the dashboard with a login system to prevent unauthorized access.
* **Multi-Server Support**: Monitor and manage multiple Minecraft instances from a single Beacon dashboard.
* **Player Management**: Kick, ban, or teleport players directly from the Player List tab.
* **Performance Graphs**: Historical charts for RAM and TPS to help identify lag spikes over time.
* **Metrics**: Graphical charts comparing current with previous metrics.
* **Customizable Themes**: Support for light/dark modes and custom accent colors.