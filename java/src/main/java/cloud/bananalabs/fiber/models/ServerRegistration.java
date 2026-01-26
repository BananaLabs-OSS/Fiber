package cloud.bananalabs.fiber.models;

import com.google.gson.annotations.SerializedName;

public class ServerRegistration {


    @SerializedName("ID")
    private String id;

    @SerializedName("Type")
    private String type;

    @SerializedName("Host")
    private String host;

    @SerializedName("Port")
    private int port;

    @SerializedName("WebhookPort")
    private int webhookPort;

    @SerializedName("Mode")
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