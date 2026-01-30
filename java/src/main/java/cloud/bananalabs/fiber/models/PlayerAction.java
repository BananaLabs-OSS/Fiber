package cloud.bananalabs.fiber.models;

public class PlayerAction {

    private String uuid;
    private String action;  // "lobby", "requeue", or "disconnect"

    public PlayerAction(String uuid, String action) {
        this.uuid = uuid;
        this.action = action;
    }

    public String getUuid() { return uuid; }
    public String getAction() { return action; }
}