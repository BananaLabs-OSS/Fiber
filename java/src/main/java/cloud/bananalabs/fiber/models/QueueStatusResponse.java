package cloud.bananalabs.fiber.models;

public class QueueStatusResponse {

    private boolean queued;
    private int position;
    private String mode;

    public boolean isQueued() { return queued; }
    public int getPosition() { return position; }
    public String getMode() { return mode; }
}