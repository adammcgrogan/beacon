package net.trybeacon.plugin.permissions;

import net.milkbowl.vault.permission.Permission;
import net.trybeacon.plugin.BeaconPlugin;
import org.bukkit.Bukkit;
import org.bukkit.plugin.RegisteredServiceProvider;

import java.util.LinkedHashMap;
import java.util.Map;
import java.util.UUID;

public class VaultPermissionService {

    private final BeaconPlugin plugin;
    private Permission provider;

    public VaultPermissionService(BeaconPlugin plugin) {
        this.plugin = plugin;
    }

    public boolean initialize() {
        if (Bukkit.getPluginManager().getPlugin("Vault") == null) {
            plugin.getLogger().warning("Vault not detected. Access permission management will be unavailable.");
            return false;
        }

        RegisteredServiceProvider<Permission> rsp = Bukkit.getServicesManager().getRegistration(Permission.class);
        if (rsp == null) {
            plugin.getLogger().warning("No Vault permission provider found. Access permission management unavailable.");
            return false;
        }

        provider = rsp.getProvider();
        return provider != null;
    }

    public boolean isReady() {
        return provider != null;
    }

    public Map<String, Boolean> snapshotPermissions(UUID playerUUID, String fallbackPlayerName, Iterable<String> permissionNodes) {
        Map<String, Boolean> result = new LinkedHashMap<>();
        if (!isReady()) {
            return result;
        }

        String playerName = resolvePlayerName(playerUUID, fallbackPlayerName);
        if (playerName == null || playerName.isBlank()) {
            return result;
        }

        for (String node : permissionNodes) {
            boolean has = provider.playerHas(null, Bukkit.getOfflinePlayer(playerName), node);
            result.put(node, has);
        }
        return result;
    }

    public boolean setPermission(UUID playerUUID, String fallbackPlayerName, String permissionNode, boolean enabled) {
        if (!isReady()) {
            return false;
        }

        boolean changed;
        if (enabled) {
            changed = provider.playerAdd(null, Bukkit.getOfflinePlayer(playerUUID), permissionNode);
        } else {
            changed = provider.playerRemove(null, Bukkit.getOfflinePlayer(playerUUID), permissionNode);
        }

        if (changed) {
            return true;
        }

        // Some providers may return false for "no-op" updates even when
        // the resulting state already matches the requested value.
        return hasPermission(playerUUID, fallbackPlayerName, permissionNode) == enabled;
    }

    public boolean hasPermission(UUID playerUUID, String fallbackPlayerName, String permissionNode) {
        if (!isReady()) {
            return false;
        }

        if (playerUUID != null) {
            var online = Bukkit.getPlayer(playerUUID);
            if (online != null) {
                return online.hasPermission(permissionNode);
            }
        }

        String playerName = resolvePlayerName(playerUUID, fallbackPlayerName);
        if (playerName == null || playerName.isBlank()) {
            return false;
        }
        return provider.playerHas(null, Bukkit.getOfflinePlayer(playerName), permissionNode);
    }

    private String resolvePlayerName(UUID playerUUID, String fallbackPlayerName) {
        if (playerUUID != null) {
            var online = Bukkit.getPlayer(playerUUID);
            if (online != null) {
                return online.getName();
            }
            var offline = Bukkit.getOfflinePlayer(playerUUID);
            if (offline.getName() != null && !offline.getName().isBlank()) {
                return offline.getName();
            }
        }
        return fallbackPlayerName;
    }
}
