package cloud.bananalabs.fiber.models;

import java.util.List;

public class MatchUpdate {

    private String status;
    private int need;
    private List<String> players;

    public MatchUpdate(String status, int need, List<String> players) {
        this.status = status;
        this.need = need;
        this.players = players;
    }

    public String getStatus() { return status; }
    public int getNeed() { return need; }
    public List<String> getPlayers() { return players; }
}