package net.trybeacon.plugin.files;

import com.google.gson.JsonArray;
import com.google.gson.JsonObject;
import net.trybeacon.plugin.BeaconPlugin;

import java.io.File;
import java.io.IOException;
import java.io.InputStream;
import java.nio.ByteBuffer;
import java.nio.charset.CharacterCodingException;
import java.nio.charset.CodingErrorAction;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Base64;
import java.util.Comparator;
import java.util.List;
import java.util.zip.GZIPInputStream;

public class FileManagerService {

    private final BeaconPlugin plugin;

    public FileManagerService(BeaconPlugin plugin) {
        this.plugin = plugin;
    }

    public JsonObject performAction(String action, String rawPath, String content) throws IOException {
        return switch (action) {
            case "meta" -> fileMeta(rawPath);
            case "list" -> fileList(rawPath);
            case "read_text" -> fileReadText(rawPath);
            case "write_text" -> fileWriteText(rawPath, content);
            case "write_binary" -> fileWriteBinary(rawPath, content);
            case "create_dir" -> fileCreateDir(rawPath);
            case "delete" -> fileDelete(rawPath);
            case "download" -> fileDownload(rawPath);
            default -> throw new IllegalArgumentException("unsupported action");
        };
    }

    private JsonObject fileMeta(String rawPath) throws IOException {
        Path path = resolveExistingPath(rawPath);

        JsonObject data = new JsonObject();
        data.addProperty("path", relativePath(path));
        data.addProperty("name", path.equals(serverRoot()) ? "/" : path.getFileName().toString());
        data.addProperty("is_dir", Files.isDirectory(path));
        data.addProperty("size", Files.size(path));
        data.addProperty("mod_time", Files.getLastModifiedTime(path).toInstant().toString());
        return data;
    }

    private JsonObject fileList(String rawPath) throws IOException {
        Path path = resolveExistingPath(rawPath);
        if (!Files.isDirectory(path)) {
            throw new IllegalArgumentException("path is not a directory");
        }

        List<Path> children = new ArrayList<>();
        try (var stream = Files.list(path)) {
            stream.forEach(children::add);
        }

        children.sort(Comparator
                .comparing((Path p) -> !Files.isDirectory(p))
                .thenComparing(p -> p.getFileName().toString().toLowerCase()));

        JsonArray entries = new JsonArray();
        for (Path child : children) {
            JsonObject entry = new JsonObject();
            entry.addProperty("path", relativePath(child));
            entry.addProperty("name", child.getFileName().toString());
            entry.addProperty("is_dir", Files.isDirectory(child));
            entry.addProperty("size", Files.size(child));
            entry.addProperty("mod_time", Files.getLastModifiedTime(child).toInstant().toString());
            entries.add(entry);
        }

        JsonObject data = new JsonObject();
        data.addProperty("path", relativePath(path));
        data.add("entries", entries);
        return data;
    }

    private JsonObject fileReadText(String rawPath) throws IOException {
        Path path = resolveExistingPath(rawPath);
        if (Files.isDirectory(path)) {
            throw new IllegalArgumentException("path is a directory");
        }

        byte[] contentBytes;
        if (path.getFileName().toString().toLowerCase().endsWith(".gz")) {
            try (InputStream gz = new GZIPInputStream(Files.newInputStream(path))) {
                contentBytes = gz.readAllBytes();
            }
        } else {
            contentBytes = Files.readAllBytes(path);
        }

        String content;
        try {
            content = StandardCharsets.UTF_8.newDecoder()
                    .onMalformedInput(CodingErrorAction.REPORT)
                    .onUnmappableCharacter(CodingErrorAction.REPORT)
                    .decode(ByteBuffer.wrap(contentBytes))
                    .toString();
        } catch (CharacterCodingException ex) {
            throw new IllegalArgumentException("file is not valid UTF-8 and cannot be edited in the text editor");
        }

        JsonObject data = new JsonObject();
        data.addProperty("path", relativePath(path));
        data.addProperty("name", path.getFileName().toString());
        data.addProperty("content", content);
        data.addProperty("size", contentBytes.length);
        data.addProperty("modified_at", Files.getLastModifiedTime(path).toInstant().toString());
        return data;
    }

    private JsonObject fileWriteText(String rawPath, String content) throws IOException {
        Path path = resolvePath(rawPath);
        if (Files.isDirectory(path)) {
            throw new IllegalArgumentException("path is a directory");
        }
        if (path.getFileName().toString().toLowerCase().endsWith(".gz")) {
            throw new IllegalArgumentException("editing .gz files is not supported");
        }

        if (path.getParent() != null) {
            Files.createDirectories(path.getParent());
        }

        Files.writeString(path, content, StandardCharsets.UTF_8);

        JsonObject data = new JsonObject();
        data.addProperty("ok", true);
        return data;
    }

    private JsonObject fileWriteBinary(String rawPath, String base64Content) throws IOException {
        Path path = resolvePath(rawPath);
        if (Files.isDirectory(path)) {
            throw new IllegalArgumentException("path is a directory");
        }

        if (path.getParent() != null) {
            Files.createDirectories(path.getParent());
        }

        byte[] bytes = Base64.getDecoder().decode(base64Content);
        Files.write(path, bytes);

        JsonObject data = new JsonObject();
        data.addProperty("ok", true);
        return data;
    }

    private JsonObject fileCreateDir(String rawPath) throws IOException {
        Path path = resolvePath(rawPath);
        Files.createDirectories(path);

        JsonObject data = new JsonObject();
        data.addProperty("ok", true);
        return data;
    }

    private void deleteRecursively(Path path) throws IOException {
        if (Files.isDirectory(path)) {
            try (var stream = Files.list(path)) {
                for (Path child : stream.toList()) {
                    deleteRecursively(child);
                }
            }
        }
        Files.delete(path);
    }

    private JsonObject fileDelete(String rawPath) throws IOException {
        Path path = resolveExistingPath(rawPath);
        if (path.equals(serverRoot())) {
            throw new IllegalArgumentException("cannot delete server root directory");
        }

        deleteRecursively(path);

        JsonObject data = new JsonObject();
        data.addProperty("ok", true);
        return data;
    }

    private JsonObject fileDownload(String rawPath) throws IOException {
        Path path = resolveExistingPath(rawPath);
        if (Files.isDirectory(path)) {
            throw new IllegalArgumentException("path is a directory");
        }

        JsonObject data = new JsonObject();
        data.addProperty("file_name", path.getFileName().toString());
        data.addProperty("content_base64", Base64.getEncoder().encodeToString(Files.readAllBytes(path)));
        return data;
    }

    private Path serverRoot() throws IOException {
        File pluginFolder = plugin.getDataFolder();
        File pluginsDir = pluginFolder.getParentFile();
        File root = pluginsDir != null ? pluginsDir.getParentFile() : new File(".");
        if (root == null) {
            root = new File(".");
        }
        return root.toPath().toRealPath();
    }

    // Resolves a path, asserting its location within bounds, but without throwing if it doesn't exist
    private Path resolvePath(String rawPath) throws IOException {
        String relative = rawPath == null ? "" : rawPath.trim();
        while (relative.startsWith("/")) {
            relative = relative.substring(1);
        }

        Path root = serverRoot();
        Path candidate = relative.isEmpty() ? root : root.resolve(relative).normalize();
        
        if (!candidate.startsWith(root)) {
            throw new IllegalArgumentException("path escapes root");
        }
        return candidate;
    }

    private Path resolveExistingPath(String rawPath) throws IOException {
        Path candidate = resolvePath(rawPath);
        if (!Files.exists(candidate)) {
            throw new IllegalArgumentException("file or directory not found");
        }

        Path real = candidate.toRealPath();
        if (!real.startsWith(serverRoot())) {
            throw new IllegalArgumentException("path escapes root");
        }
        return real;
    }

    private String relativePath(Path path) throws IOException {
        Path root = serverRoot();
        if (path.equals(root)) {
            return "";
        }
        return root.relativize(path).toString().replace('\\', '/');
    }
}