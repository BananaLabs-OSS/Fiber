package cloud.bananalabs.fiber.models;

import com.google.gson.annotations.SerializedName;

public class ServerRegistration {


    @SerializedName("id")
    private String id;

    @SerializedName("type")
    private String type;

    @SerializedName("host")
    private String host;

    @SerializedName("port")
    private int port;

    @SerializedName("webhookPort")
    private int webhookPort;

    @SerializedName("mode")
    private String mode;

    public ServerRegistration(String id, String type, String host, int port, String mode) {
        this.id = id;
        this.type = type;
        this.host = host;
        this.port = port;
        this.webhookPort = 8080;
        this.mode = mode;
    }

    public ServerRegistration(String id, String type, String host, int port, int webhookPort, String mode) {
        this.id = id;
        this.type = type;
        this.host = host;
        this.port = port;
        this.webhookPort = webhookPort;
        this.mode = mode;
    }

    public String getId() { return id; }
    public String getType() { return type; }
    public String getHost() { return host; }
    public int getPort() { return port; }
    public int getWebhookPort() { return webhookPort; }
    public String getMode() { return mode; }
}
