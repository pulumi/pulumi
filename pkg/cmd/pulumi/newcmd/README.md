# my-java-component

A Pulumi component resource.

## Usage

```java
import com.pulumi.Pulumi;
import myjavacomponent.MyJavaComponent;

public class App {
    public static void main(String[] args) {
        Pulumi.run(ctx -> {
            var component = new MyJavaComponent("my-java-component-instance", 
                new MyJavaComponent.MyJavaComponentArgs().message("Hello from component!"));

            ctx.export("message", component.message);
        });
    }
}
```

## Development

Build:

```bash
mvn package
```
