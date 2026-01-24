package cloud.bananalabs.fiber.models;

import java.util.List;

public class TransferRequest {

    private List<String> players;
    private String targetServer;
    private String matchId;

    public List<String> getPlayers() { return players; }
    public String getTargetServer() { return targetServer; }
    public String getMatchId() { return matchId; }
}