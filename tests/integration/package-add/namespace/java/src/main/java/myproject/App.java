package myproject;

import com.pulumi.Pulumi;
import com.pulumi.core.Output;

public class App {
    public static void main(String[] args) {
        Pulumi.run(ctx -> {
            ctx.export("exampleOutput", Output.of("example"));
        });
    }
}
