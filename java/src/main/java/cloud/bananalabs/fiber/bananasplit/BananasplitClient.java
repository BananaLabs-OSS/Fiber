package cloud.bananalabs.fiber.bananasplit;

import cloud.bananalabs.fiber.models.*;
import com.google.gson.Gson;
import okhttp3.*;

import java.io.IOException;
import java.util.concurrent.CompletableFuture;

public class BananasplitClient {

    private static final MediaType JSON = MediaType.get("application/json");

    private final OkHttpClient http;
    private final Gson gson;
    private final String baseUrl;

    public BananasplitClient(String baseUrl, OkHttpClient http, Gson gson) {
        this.baseUrl = baseUrl;
        this.http = http;
        this.gson = gson;
    }

    public CompletableFuture<QueueResponse> joinQueue(String uuid, String mode, String lobbyServer) {
        CompletableFuture<QueueResponse> future = new CompletableFuture<>();

        QueueJoinRequest body = new QueueJoinRequest(uuid, mode, lobbyServer);
        Request request = new Request.Builder()
                .url(baseUrl + "/queue/join")
                .post(RequestBody.create(gson.toJson(body), JSON))
                .build();

        http.newCall(request).enqueue(new Callback() {
            @Override
            public void onResponse(Call call, Response response) {
                try (response) {
                    if (response.isSuccessful() && response.body() != null) {
                        QueueResponse result = gson.fromJson(response.body().charStream(), QueueResponse.class);
                        future.complete(result);
                    } else {
                        future.completeExceptionally(new IOException("Join queue failed: " + response.code()));
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

    public CompletableFuture<Void> leaveQueue(String uuid, String mode) {
        CompletableFuture<Void> future = new CompletableFuture<>();

        QueueLeaveRequest body = new QueueLeaveRequest(uuid, mode);
        Request request = new Request.Builder()
                .url(baseUrl + "/queue/leave")
                .post(RequestBody.create(gson.toJson(body), JSON))
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

    public CompletableFuture<QueueSizeResponse> getQueueSize(String mode) {
        CompletableFuture<QueueSizeResponse> future = new CompletableFuture<>();

        Request request = new Request.Builder()
                .url(baseUrl + "/queue/" + mode + "/size")
                .get()
                .build();

        http.newCall(request).enqueue(new Callback() {
            @Override
            public void onResponse(Call call, Response response) {
                try (response) {
                    if (response.isSuccessful() && response.body() != null) {
                        QueueSizeResponse result = gson.fromJson(response.body().charStream(), QueueSizeResponse.class);
                        future.complete(result);
                    } else {
                        future.completeExceptionally(new IOException("Queue size failed: " + response.code()));
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
}
