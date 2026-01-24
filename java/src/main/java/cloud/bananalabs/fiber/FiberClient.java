package cloud.bananalabs.fiber;

import cloud.bananalabs.fiber.bananagine.BananagineClient;
import cloud.bananalabs.fiber.bananasplit.BananasplitClient;

public class FiberClient {

    private final BananagineClient bananagine;
    private final BananasplitClient bananasplit;

    public FiberClient(String bananagineUrl, String bananasplitUrl) {
        this.bananagine = new BananagineClient(bananagineUrl);
        this.bananasplit = new BananasplitClient(bananasplitUrl);
    }

    public BananagineClient bananagine() {
        return bananagine;
    }

    public BananasplitClient bananasplit() {
        return bananasplit;
    }
}