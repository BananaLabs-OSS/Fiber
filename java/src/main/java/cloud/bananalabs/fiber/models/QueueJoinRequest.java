package cloud.bananalabs.fiber.models;


public class QueueJoinRequest {

    private String uuid;
    private String mode;
    private String lobbyServer;

    public QueueJoinRequest(String uuid, String mode, String lobbyServer) {
        this.uuid = uuid;
        this.mode = mode;
        this.lobbyServer = lobbyServer;
    }

    public String getUuid() { return uuid; }
    public String getMode() { return mode; }
    public String getLobbyServer() { return lobbyServer; }
}