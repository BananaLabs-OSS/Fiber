package cloud.bananalabs.fiber;

import cloud.bananalabs.fiber.bananagine.BananagineClient;
import cloud.bananalabs.fiber.bananasplit.BananasplitClient;
import com.google.gson.Gson;
import okhttp3.ConnectionPool;
import okhttp3.Dispatcher;
import okhttp3.OkHttpClient;

import java.util.concurrent.TimeUnit;

public class FiberClient {

    private final OkHttpClient http;
    private final BananagineClient bananagine;
    private final BananasplitClient bananasplit;

    public FiberClient(String bananagineUrl, String bananasplitUrl) {
        Gson gson = new Gson();

        // Async dispatcher - no blocking
        Dispatcher dispatcher = new Dispatcher();
        dispatcher.setMaxRequests(64);
        dispatcher.setMaxRequestsPerHost(8);

        this.http = new OkHttpClient.Builder()
                .connectTimeout(5, TimeUnit.SECONDS)
                .readTimeout(5, TimeUnit.SECONDS)
                .writeTimeout(5, TimeUnit.SECONDS)
                .connectionPool(new ConnectionPool(10, 60, TimeUnit.SECONDS))
                .retryOnConnectionFailure(true)
                .dispatcher(dispatcher)
                .build();

        this.bananagine = new BananagineClient(bananagineUrl, http, gson);
        this.bananasplit = new BananasplitClient(bananasplitUrl, http, gson);
    }

    public BananagineClient bananagine() {
        return bananagine;
    }

    public BananasplitClient bananasplit() {
        return bananasplit;
    }

    public void shutdown() {
        http.dispatcher().executorService().shutdown();
        http.connectionPool().evictAll();
    }
}
