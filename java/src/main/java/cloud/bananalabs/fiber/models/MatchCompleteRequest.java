package cloud.bananalabs.fiber.models;

import java.util.List;

public class MatchCompleteRequest {

    private String serverId;
    private String matchId;
    private List<PlayerAction> players;

    public MatchCompleteRequest(String serverId, String matchId, List<PlayerAction> players) {
        this.serverId = serverId;
        this.matchId = matchId;
        this.players = players;
    }

    public String getServerId() { return serverId; }
    public String getMatchId() { return matchId; }
    public List<PlayerAction> getPlayers() { return players; }
}