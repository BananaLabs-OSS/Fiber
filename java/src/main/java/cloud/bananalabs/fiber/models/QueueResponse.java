package cloud.bananalabs.fiber.models;

public class QueueResponse {

    private String status;
    private String mode;
    private int position;

    public String getStatus() { return status; }
    public String getMode() { return mode; }
    public int getPosition() { return position; }
}