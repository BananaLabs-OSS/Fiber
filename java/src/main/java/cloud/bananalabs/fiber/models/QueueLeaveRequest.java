package cloud.bananalabs.fiber.models;

public class QueueLeaveRequest {

    private String uuid;
    private String mode;

    public QueueLeaveRequest(String uuid, String mode) {
        this.uuid = uuid;
        this.mode = mode;
    }

    public String getUuid() { return uuid; }
    public String getMode() { return mode; }
}