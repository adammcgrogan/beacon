package net.trybeacon.plugin.commands;

import com.google.gson.JsonObject;
import com.google.gson.JsonArray;
import net.kyori.adventure.text.Component;
import net.kyori.adventure.text.event.ClickEvent;
import net.kyori.adventure.text.event.HoverEvent;
import net.kyori.adventure.text.format.NamedTextColor;
import net.trybeacon.plugin.BeaconPlugin;
import net.trybeacon.plugin.websocket.BackendClient;
import org.bukkit.command.Command;
import org.bukkit.command.CommandExecutor;
import org.bukkit.command.CommandSender;
import org.bukkit.command.TabCompleter;
import org.bukkit.entity.Player;
import org.bukkit.permissions.PermissionAttachmentInfo;

import java.security.SecureRandom;
import java.util.Collections;
import java.util.List;
import java.util.Locale;

public class BeaconCommand implements CommandExecutor, TabCompleter {

    private static final SecureRandom SECURE_RANDOM = new SecureRandom();
    private static final char[] HEX = "0123456789abcdef".toCharArray();
    private final BeaconPlugin plugin;

    public BeaconCommand(BeaconPlugin plugin) {
        this.plugin = plugin;
    }

    @Override
    public boolean onCommand(CommandSender sender, Command command, String label, String[] args) {
        if (!(sender instanceof Player player)) {
            sender.sendMessage("Only players may use this command.");
            return true;
        }

        if (!player.isOp() && !player.hasPermission("beacon.panel")) {
            player.sendMessage(Component.text("You do not have permission to use this command.", NamedTextColor.RED));
            return true;
        }

        if (args.length == 0 || !"panel".equalsIgnoreCase(args[0])) {
            player.sendMessage(Component.text("Usage: /beacon panel", NamedTextColor.YELLOW));
            return true;
        }

        BackendClient backendClient = plugin.getBackendClient();
        if (backendClient == null || !backendClient.isOpen()) {
            player.sendMessage(Component.text("Beacon backend is offline. Try again in a few seconds.", NamedTextColor.RED));
            return true;
        }

        String token = secureTokenHex(32);
        long expiresAtUnix = (System.currentTimeMillis() / 1000L) + plugin.getPanelTokenExpirySeconds();

        JsonObject payload = new JsonObject();
        payload.addProperty("token", token);
        payload.addProperty("player_uuid", player.getUniqueId().toString());
        payload.addProperty("player_name", player.getName());
        payload.addProperty("expires_at_unix", expiresAtUnix);
        payload.add("permissions", collectPermissions(player));

        boolean sent = backendClient.sendEvent("auth_token_issued", payload);
        if (!sent) {
            player.sendMessage(Component.text("Failed to reach Beacon backend. Please try again.", NamedTextColor.RED));
            return true;
        }

        String baseUrl = plugin.getBackendPublicUrl().replaceAll("/+$", "");
        String authUrl = baseUrl + "/auth?token=" + token;

        player.sendMessage(Component.text("Open your Beacon panel:", NamedTextColor.GREEN));
        player.sendMessage(
                Component.text(authUrl, NamedTextColor.AQUA)
                        .clickEvent(ClickEvent.openUrl(authUrl))
                        .hoverEvent(HoverEvent.showText(Component.text("Click to open Beacon panel")))
        );
        return true;
    }

    @Override
    public List<String> onTabComplete(CommandSender sender, Command command, String alias, String[] args) {
        if (args.length == 1) {
            String input = args[0].toLowerCase(Locale.ROOT);
            if ("panel".startsWith(input)) {
                return List.of("panel");
            }
        }
        return Collections.emptyList();
    }

    private static String secureTokenHex(int numBytes) {
        byte[] bytes = new byte[numBytes];
        SECURE_RANDOM.nextBytes(bytes);
        char[] out = new char[numBytes * 2];
        for (int i = 0; i < bytes.length; i++) {
            int v = bytes[i] & 0xFF;
            out[i * 2] = HEX[v >>> 4];
            out[i * 2 + 1] = HEX[v & 0x0F];
        }
        return new String(out);
    }

    private static JsonArray collectPermissions(Player player) {
        JsonArray permissions = new JsonArray();
        if (player.isOp()) {
            permissions.add("beacon.access.*");
            permissions.add("beacon.panel");
        }
        for (PermissionAttachmentInfo perm : player.getEffectivePermissions()) {
            if (!perm.getValue()) continue;
            permissions.add(perm.getPermission().toLowerCase(Locale.ROOT));
        }
        return permissions;
    }
}
