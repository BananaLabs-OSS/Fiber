package cloud.bananalabs.fiber.models;

import com.google.gson.annotations.SerializedName;

public class ServerInfo {
    @SerializedName("ID")
    private String id;

    @SerializedName("Type")
    private String type;

    @SerializedName("Mode")
    private String mode;

    @SerializedName("Host")
    private String host;

    @SerializedName("Port")
    private int port;

    @SerializedName("Players")
    private int players;

    @SerializedName("MaxPlayers")
    private int maxPlayers;

    public String getId() { return id; }
    public String getType() { return type; }
    public String getMode() { return mode; }
    public String getHost() { return host; }
    public int getPort() { return port; }
    public int getPlayers() { return players; }
    public int getMaxPlayers() { return maxPlayers; }
}