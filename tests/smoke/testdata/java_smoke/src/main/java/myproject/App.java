package myproject;

import com.pulumi.Pulumi;

public class App {
    public static void main(String[] args) {
        Pulumi.run(ctx -> {
            ctx.export("greeting", "hello, java");
        });
    }
}
