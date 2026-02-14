package cloud.bananalabs.fiber.bananagine;

import cloud.bananalabs.fiber.models.*;
import com.google.gson.Gson;
import com.google.gson.reflect.TypeToken;
import okhttp3.*;

import java.io.IOException;
import java.lang.reflect.Type;
import java.util.List;
import java.util.concurrent.CompletableFuture;

public class BananagineClient {

    private static final MediaType JSON = MediaType.get("application/json");

    private final OkHttpClient http;
    private final Gson gson;
    private final String baseUrl;

    public BananagineClient(String baseUrl, OkHttpClient http, Gson gson) {
        this.baseUrl = baseUrl;
        this.http = http;
        this.gson = gson;
    }

    public CompletableFuture<Void> updatePlayerCount(String serverId, int players) {
        CompletableFuture<Void> future = new CompletableFuture<>();

        String json = "{\"players\":" + players + "}";
        Request request = new Request.Builder()
                .url(baseUrl + "/registry/servers/" + serverId + "/players")
                .put(RequestBody.create(json, JSON))
                .build();

        http.newCall(request).enqueue(new Callback() {
            @Override
            public void onResponse(Call call, Response response) {
                try (response) {
                    future.complete(null);
                }
            }

            @Override
            public void onFailure(Call call, IOException e) {
                future.completeExceptionally(e);
            }
        });

        return future;
    }

    public CompletableFuture<Void> registerServer(ServerRegistration server) {
        CompletableFuture<Void> future = new CompletableFuture<>();

        Request request = new Request.Builder()
                .url(baseUrl + "/registry/servers")
                .post(RequestBody.create(gson.toJson(server), JSON))
                .build();

        http.newCall(request).enqueue(new Callback() {
            @Override
            public void onResponse(Call call, Response response) {
                try (response) {
                    if (response.isSuccessful()) {
                        future.complete(null);
                    } else {
                        future.completeExceptionally(new IOException("Register failed: " + response.code()));
                    }
                }
            }

            @Override
            public void onFailure(Call call, IOException e) {
                future.completeExceptionally(e);
            }
        });

        return future;
    }

    public CompletableFuture<List<ServerInfo>> listServers() {
        CompletableFuture<List<ServerInfo>> future = new CompletableFuture<>();

        Request request = new Request.Builder()
                .url(baseUrl + "/registry/servers")
                .get()
                .build();

        http.newCall(request).enqueue(new Callback() {
            @Override
            public void onResponse(Call call, Response response) {
                try (response) {
                    if (response.isSuccessful() && response.body() != null) {
                        Type listType = new TypeToken<List<ServerInfo>>(){}.getType();
                        List<ServerInfo> servers = gson.fromJson(response.body().charStream(), listType);
                        future.complete(servers != null ? servers : List.of());
                    } else {
                        future.completeExceptionally(new IOException("List failed: " + response.code()));
                    }
                } catch (Exception e) {
                    future.completeExceptionally(e);
                }
            }

            @Override
            public void onFailure(Call call, IOException e) {
                future.completeExceptionally(e);
            }
        });

        return future;
    }

    public CompletableFuture<Void> unregisterServer(String serverId) {
        CompletableFuture<Void> future = new CompletableFuture<>();

        Request request = new Request.Builder()
                .url(baseUrl + "/registry/servers/" + serverId)
                .delete()
                .build();

        http.newCall(request).enqueue(new Callback() {
            @Override
            public void onResponse(Call call, Response response) {
                try (response) {
                    future.complete(null);
                }
            }

            @Override
            public void onFailure(Call call, IOException e) {
                future.completeExceptionally(e);
            }
        });

        return future;
    }

    public CompletableFuture<Void> spawnServer(String template) {
        CompletableFuture<Void> future = new CompletableFuture<>();

        String json = "{\"template\":\"" + template + "\"}";
        Request request = new Request.Builder()
                .url(baseUrl + "/orchestration/servers")
                .post(RequestBody.create(json, JSON))
                .build();

        http.newCall(request).enqueue(new Callback() {
            @Override
            public void onResponse(Call call, Response response) {
                try (response) {
                    if (response.isSuccessful()) {
                        future.complete(null);
                    } else {
                        future.completeExceptionally(new IOException("Spawn failed: " + response.code()));
                    }
                }
            }

            @Override
            public void onFailure(Call call, IOException e) {
                future.completeExceptionally(e);
            }
        });

        return future;
    }

    public CompletableFuture<Void> shutdownServer(String serverId) {
        CompletableFuture<Void> future = new CompletableFuture<>();

        Request request = new Request.Builder()
                .url(baseUrl + "/orchestration/servers/" + serverId)
                .delete()
                .build();

        http.newCall(request).enqueue(new Callback() {
            @Override
            public void onResponse(Call call, Response response) {
                try (response) {
                    if (response.isSuccessful()) {
                        future.complete(null);
                    } else {
                        future.completeExceptionally(new IOException("Shutdown failed: " + response.code()));
                    }
                }
            }

            @Override
            public void onFailure(Call call, IOException e) {
                future.completeExceptionally(e);
            }
        });

        return future;
    }
}
