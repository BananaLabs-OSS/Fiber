package cloud.bananalabs.fiber.bananasplit;

import cloud.bananalabs.fiber.models.*;
import com.google.gson.Gson;
import okhttp3.*;

import java.io.IOException;

public class BananasplitClient {

    private static final MediaType JSON = MediaType.get("application/json");

    private final OkHttpClient http;
    private final Gson gson;
    private final String baseUrl;

    public BananasplitClient(String baseUrl) {
        this.baseUrl = baseUrl;
        this.http = new OkHttpClient();
        this.gson = new Gson();
    }

    public QueueResponse joinQueue(String uuid, String mode, String lobbyServer) throws IOException {
        QueueJoinRequest body = new QueueJoinRequest(uuid, mode, lobbyServer);
        Request request = new Request.Builder()
                .url(baseUrl + "/queue/join")
                .post(RequestBody.create(gson.toJson(body), JSON))
                .build();

        try (Response response = http.newCall(request).execute()) {
            return gson.fromJson(response.body().string(), QueueResponse.class);
        }
    }

    public void leaveQueue(String uuid, String mode) throws IOException {
        QueueLeaveRequest body = new QueueLeaveRequest(uuid, mode);
        Request request = new Request.Builder()
                .url(baseUrl + "/queue/leave")
                .post(RequestBody.create(gson.toJson(body), JSON))
                .build();

        try (Response response = http.newCall(request).execute()) {
            // Just checking it succeeded
        }
    }

    public QueueSizeResponse getQueueSize(String mode) throws IOException {
        Request request = new Request.Builder()
                .url(baseUrl + "/queue/" + mode + "/size")
                .get()
                .build();

        try (Response response = http.newCall(request).execute()) {
            return gson.fromJson(response.body().string(), QueueSizeResponse.class);
        }
    }

    public QueueStatusResponse getQueueStatus(String mode, String uuid) throws IOException {
        Request request = new Request.Builder()
                .url(baseUrl + "/queue/" + mode + "/status/" + uuid)
                .get()
                .build();

        try (Response response = http.newCall(request).execute()) {
            return gson.fromJson(response.body().string(), QueueStatusResponse.class);
        }
    }

    public void matchComplete(MatchCompleteRequest body) throws IOException {
        Request request = new Request.Builder()
                .url(baseUrl + "/match-complete")
                .post(RequestBody.create(gson.toJson(body), JSON))
                .build();

        try (Response response = http.newCall(request).execute()) {
            // Just checking it succeeded
        }
    }
}