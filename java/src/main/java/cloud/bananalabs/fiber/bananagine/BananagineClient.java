package cloud.bananalabs.fiber.bananagine;

import cloud.bananalabs.fiber.models.*;
import com.google.gson.Gson;
import com.google.gson.reflect.TypeToken;
import okhttp3.*;

import java.io.IOException;
import java.lang.reflect.Type;
import java.util.List;

public class BananagineClient {

    private static final MediaType JSON = MediaType.get("application/json");

    private final OkHttpClient http;
    private final Gson gson;
    private final String baseUrl;

    public BananagineClient(String baseUrl) {
        this.baseUrl = baseUrl;
        this.http = new OkHttpClient();
        this.gson = new Gson();
    }

    // Server Registry
    public void registerServer(ServerRegistration server) throws IOException {
        Request request = new Request.Builder()
                .url(baseUrl + "/registry/servers/")
                .post(RequestBody.create(gson.toJson(server), JSON))
                .build();

        try (Response response = http.newCall(request).execute()) {
            if (!response.isSuccessful()) throw new IOException("Register failed: " + response.code());
        }
    }

    public List<ServerInfo> listServers() throws IOException {
        Request request = new Request.Builder()
                .url(baseUrl + "/registry/servers/")
                .get()
                .build();

        try (Response response = http.newCall(request).execute()) {
            if (!response.isSuccessful()) throw new IOException("List failed: " + response.code());
            String json = response.body().string();
            Type listType = new TypeToken<List<ServerInfo>>(){}.getType();
            List<ServerInfo> servers = gson.fromJson(json, listType);
            return servers != null ? servers : List.of();
        }
    }

    public String getServer(String serverId) throws IOException {
        Request request = new Request.Builder()
                .url(baseUrl + "/registry/servers/" + serverId)
                .get()
                .build();

        try (Response response = http.newCall(request).execute()) {
            return response.body().string();
        }
    }

    public void updateServer(String serverId, String jsonBody) throws IOException {
        Request request = new Request.Builder()
                .url(baseUrl + "/registry/servers/" + serverId)
                .put(RequestBody.create(jsonBody, JSON))
                .build();

        try (Response response = http.newCall(request).execute()) {
            if (!response.isSuccessful()) throw new IOException("Update failed: " + response.code());
        }
    }

    public void unregisterServer(String serverId) throws IOException {
        Request request = new Request.Builder()
                .url(baseUrl + "/registry/servers/" + serverId)
                .delete()
                .build();

        try (Response response = http.newCall(request).execute()) {
            // OK if 200 or 404
        }
    }

    // Match Registry
    public void updateMatch(String serverId, String matchId, MatchUpdate match) throws IOException {
        Request request = new Request.Builder()
                .url(baseUrl + "/registry/servers/" + serverId + "/matches/" + matchId)
                .put(RequestBody.create(gson.toJson(match), JSON))
                .build();

        try (Response response = http.newCall(request).execute()) {
            if (!response.isSuccessful()) throw new IOException("Match update failed: " + response.code());
        }
    }

    public void removeMatch(String serverId, String matchId) throws IOException {
        Request request = new Request.Builder()
                .url(baseUrl + "/registry/servers/" + serverId + "/matches/" + matchId)
                .delete()
                .build();

        try (Response response = http.newCall(request).execute()) {
            // OK
        }
    }

    // Orchestration
    public ServerInfo spawnServer(String template) throws IOException {
        String json = "{\"template\":\"" + template + "\"}";
        RequestBody body = RequestBody.create(json, JSON);

        Request request = new Request.Builder()
                .url(baseUrl + "/orchestration/servers/")
                .post(body)
                .build();

        try (Response response = http.newCall(request).execute()) {
            if (!response.isSuccessful()) throw new IOException("Spawn failed: " + response.code());
            return gson.fromJson(response.body().string(), ServerInfo.class);
        }
    }

    public void shutdownServer(String serverId) throws IOException {
        Request request = new Request.Builder()
                .url(baseUrl + "/orchestration/servers/" + serverId)
                .delete()
                .build();

        try (Response response = http.newCall(request).execute()) {
            if (!response.isSuccessful()) throw new IOException("Shutdown failed: " + response.code());
        }
    }
}