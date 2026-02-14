package cloud.bananalabs.fiber.models;

import com.google.gson.annotations.SerializedName;

public class ServerInfo {
    @SerializedName("id")
    private String id;

    @SerializedName("type")
    private String type;

    @SerializedName("mode")
    private String mode;

    @SerializedName("host")
    private String host;

    @SerializedName("port")
    private int port;

    @SerializedName("players")
    private int players;

    @SerializedName("maxPlayers")
    private int maxPlayers;

    public String getId() { return id; }
    public String getType() { return type; }
    public String getMode() { return mode; }
    public String getHost() { return host; }
    public int getPort() { return port; }
    public int getPlayers() { return players; }
    public int getMaxPlayers() { return maxPlayers; }
}
