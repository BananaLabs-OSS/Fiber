package cloud.bananalabs.fiber.models;

public class ServerRegistration {

    private String id;
    private String type;
    private String host;
    private int port;
    private String mode;

    public ServerRegistration(String id, String type, String host, int port) {
        this.id = id;
        this.type = type;
        this.host = host;
        this.port = port;
    }

    public ServerRegistration(String id, String type, String host, int port, String mode) {
        this(id, type, host, port);
        this.mode = mode;
    }

    public String getId() { return id; }
    public String getType() { return type; }
    public String getHost() { return host; }
    public int getPort() { return port; }
    public String getMode() { return mode; }
}